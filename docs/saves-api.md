# DogOffice 存檔 API spec

本文件描述 `dog-company` 後端需要新增的存檔端點。極簡版：一人一份 current save，不做歷代封存、不做排行榜。

> 規格版本：draft-3（2026-04-18）
> 基準 API base：`https://dog-company-production.up.railway.app/api/v1`

---

## 總覽

| Method | Path | Auth | 說明 |
|--------|------|------|------|
| GET    | `/saves` | Bearer | 取得目前使用者的 current save |
| POST   | `/saves` | Bearer | 新增 / 覆寫 current save |
| DELETE | `/saves` | Bearer | 刪除 current save |

**認證**：`Authorization: Bearer <access_token>`，與現有 `/auth/me` 相同。401 回應維持現行格式。

**Response 格式**：沿用 `CommonResponse`：`{ code, message, data }`。

---

## 資料模型

### SavePayload（POST /saves 的 body）

```json
{
  "version": 1,
  "revision": 3,
  "data": {
    "day": 42,
    "money": 820,
    "morale": 67,
    "health": 74,
    "decor": 4,
    "productivityBoost": 3,
    "stabilityBoost": 2,
    "trainingBoost": 0,
    "officeLevel": 2,
    "vacancy": false,
    "vacancyTimer": 0,
    "bankrupt": false,
    "tutorialStep": 7,

    "staff": [ /* Dog[] */ ],
    "activeChemistry": [ /* ChemistryEntry[] */ ],
    "log": [ /* LogEntry[]，最多 10 筆 */ ]
  }
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `version` | int | 存檔格式版本，目前 **1**。未來欄位變動會升版 |
| `revision` | int | 客端持有的存檔 revision（首次存送 0）。衝突偵測用 |
| `data` | object | 實際遊戲狀態。下面詳列 |

### GameSaveData 欄位

**基本狀態**（沿用 GameState）：
- `day` (int, >=1)
- `money` (int, >=0)
- `morale` (int, 0-100)
- `health` (int, 0-100)
- `decor` (int, >=0)
- `productivityBoost`, `stabilityBoost`, `trainingBoost` (int, >=0)
- `officeLevel` (int, 0-N)
- `vacancy` (bool)
- `vacancyTimer` (int, >=0)
- `bankrupt` (bool)
- `tutorialStep` (int, 0-7)：教學進度。`>=7` 表示已看完；新裝置載入時可跳過教學

**集合**：
- `staff`：員工陣列（Dog 物件），每隻狗含 name/role/breed/stats/grade/salary 等
- `activeChemistry`：目前啟用的 chemistry combo 陣列
- `log`：最近 10 筆日誌（`{ day, msg }`）

**不存**：
queue / current candidate / candidatePatience / showSplash / activeTab / speedMultiplier / dayElapsed / miniGame / trainingSession / staffActionModal / toast / candidateReaction（UI 或瞬時狀態）

### SaveMeta（GET / POST 成功回傳）

```json
{
  "version": 1,
  "revision": 3,
  "updated_at": "2026-04-18T12:34:56Z",
  "data": { ... }
}
```

---

## Endpoints

### GET `/saves`

取得當前使用者的 save。

**Response 200（有存檔）**：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "version": 1,
    "revision": 3,
    "updated_at": "2026-04-18T12:34:56Z",
    "data": { ... }
  }
}
```

**Response 200（無存檔）**：
```json
{ "code": 0, "message": "ok", "data": null }
```

**Response 401**：未登入 / token 失效。

---

### POST `/saves`

覆寫 current save。

**Request body**：見上面 SavePayload。

**行為**：
1. 比對 `revision`：
   - 若資料庫沒有該 user 的 save → 允許任何 revision，視為首次存檔，server_revision 起始設為 1
   - 若有：server_revision 必須 == client_revision → 通過，server_revision += 1
   - 否則 → 409 衝突
2. Sanity check（下段）

**Response 200**：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "version": 1,
    "revision": 4,
    "updated_at": "2026-04-18T12:35:10Z"
  }
}
```

**Response 400**：payload 不合格式 / version 不支援 / sanity check 失敗。

**Response 409**：revision 衝突。帶上 server 目前狀態讓客端彈 conflict UI：
```json
{
  "code": 4090,
  "message": "save conflict",
  "data": {
    "server_revision": 5,
    "server_updated_at": "2026-04-18T12:36:00Z",
    "server_data": { ... }
  }
}
```

**Response 401**：未登入。

---

### DELETE `/saves`

刪除 current save。冪等（已無存檔也回 200）。

**Response 200**：
```json
{ "code": 0, "message": "ok", "data": null }
```

---

## DB Schema 建議

### `saves`

```sql
CREATE TABLE saves (
  user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  version      INT NOT NULL,
  revision     INT NOT NULL DEFAULT 1,
  data         JSONB NOT NULL,
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

一人一份，PK 即 user_id。不需要額外 index。

---

## Sanity Check（POST /saves 時建議執行）

**目的**：降低客端亂送成本，不追求完全防弊。

建議檢查：

1. **必要欄位非空** + 型別正確
2. **版本支援**：`version == 1` 才接受；未來往上擴
3. **數值範圍**：
   - `morale`, `health` ∈ [0, 100]
   - `day`, `money`, `officeLevel`, 各 `Boost` >= 0
4. **單調遞增**（與 server 上一次 revision 比對）：
   - `day >= prev.day`（同一場遊戲天數不會倒退）
5. **合理性**：
   - `staff.length <= OFFICE_LEVELS[officeLevel].maxStaff + 2`（給客端超載扣分一點彈性）

不過的 request → **400**，錯誤訊息指明哪個欄位。不要 silent cap，否則客端狀態會與 server 不同步。

---

## Versioning Policy

- **目前 `version = 1`**
- 欄位新增且有合理預設：`version` 不變，後端解析時補預設
- 欄位移除 / 欄位語意改變：`version` + 1，後端保留舊 version 解析路徑（至少保留 2 版）

---

## Error Code 建議

沿用 CommonResponse 的 `code`：

| code | HTTP | 意義 |
|------|------|------|
| 0 | 200 | ok |
| 4000 | 400 | payload 格式錯誤 |
| 4001 | 400 | 不支援的 version |
| 4002 | 400 | sanity check 失敗（message 指明欄位） |
| 4010 | 401 | 未登入 / token 失效 |
| 4090 | 409 | revision 衝突 |
| 5000 | 500 | 伺服器內部錯誤 |

---

## 前端行為契約（給後端參考）

- 前端首次啟動 / 使用者登入後會 `GET /saves`
- 進行遊戲中每 3 天 + 關鍵動作觸發 `POST /saves`
- 破產時會再 `POST /saves` 一次（`bankrupt=true` 的存檔），之後使用者按「重新開始」→ `DELETE /saves`
- 409 conflict：前端彈 modal 讓使用者選本地 or 雲端版本，不自動覆寫
- 401：前端已有 auto refresh 機制，後端收到過期 access token 回 401 即可

---

## Open Questions

1. `data` 欄位大小上限：建議 server 設 64KB 拒絕超大 payload。
2. `revision` overflow：用 `INT`（上限約 21 億）一輩子玩不到，保持即可。
