# DogOffice 存檔 API spec（v2 接案制）

本文件描述 `dog-company` 後端**重寫**後的存檔端點。極簡版：一人一份 current save，不做歷代封存。

> 規格版本：draft-8（2026-04-27，Dog 補 `learnedTraits` / `pendingTraitChoice` 升級特性欄位）
> 基準 API base：`https://dog-company-production.up.railway.app/api/v1`

> ⚠️ **v1 與 v2 不相容**。v1 存檔被棄用，伺服器收到 `version === 1` 的舊存檔可直接視為無效（前端 client 會在 `migrate()` 回 `null`，玩家從新狀態開始）。

---

## 總覽

| Method | Path | Auth | 說明 |
|--------|------|------|------|
| GET    | `/saves` | Bearer | 取得目前使用者的 current save |
| POST   | `/saves` | Bearer | 新增 / 覆寫 current save |
| DELETE | `/saves` | Bearer | 刪除 current save |

**認證**：`Authorization: Bearer <access_token>`，與 `/auth/me` 相同。

**Response 格式**：沿用 `CommonResponse`：`{ code, message, data }`。

---

## 資料模型

### SavePayload（POST /saves 的 body）

```json
{
  "version": 2,
  "revision": 3,
  "data": {
    "day": 42,
    "money": 1240,
    "reputation": 55,
    "tierBudget": 78,
    "companyBuffs": {
      "speedBoost": 2,
      "qualityBoost": 1,
      "teamworkBoost": 1,
      "charismaBoost": 0,
      "decor": 3
    },
    "officeLevel": 2,
    "purchases": { "snack": 3, "desk": 1, "coffee": 1 },

    "clients": [ /* Project[] */ ],
    "projectsCompleted": 12,
    "projectsFailed": 2,
    "lastRerollDay": 38,

    "vacancy": false,
    "vacancyTimer": 0,
    "bankrupt": false,
    "bankruptCountdown": 0,
    "tutorialStep": 7,
    "recruitmentClosed": false,

    "staff": [ /* Dog[] */ ],
    "log": [ /* LogEntry[]，最多 10 筆 */ ],

    "ipoAchievedAt": null,
    "ipoDismissed": false,

    "loanTaken": false,
    "loanRepayDaysLeft": 0
  }
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `version` | int | 存檔格式版本，**目前固定為 2**。`version=1` 會被前端視為無效 |
| `revision` | int, **optional** | 客端持有的存檔 revision。衝突偵測用。首次存檔可省略，server 從 1 起算 |
| `data` | object | 實際遊戲狀態。下面詳列 |

---

### GameSaveData v2 欄位

#### 公司基本狀態

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `day` | int | ≥1 | 遊戲第幾天 |
| `money` | int | ≥0 | 資金 |
| `reputation` | number | 0-100 | **信譽**（v2 新欄位，取代舊 health；可為小數，如 +0.5）|
| `tierBudget` | int | ≥0 | 案件稀有度預算（每天 morning 會由 client 重算）|
| `companyBuffs` | object | — | 公司全域 buff（見下） |
| `officeLevel` | int | 0-4 | 辦公室等級（容量 / IPO 條件用）|
| `officeSkin` | int | 0-officeLevel | 辦公室視覺造型（玩家可在已解鎖等級之間切換）；舊存檔缺此欄位視為 `officeLevel` |
| `purchases` | object | — | 商品 id → 購買次數（=等級，最高 5），如 `{ "snack": 3 }` |

#### `companyBuffs` 子物件

| key | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `speedBoost` | int | ≥0 | 來源：desk / coffee / gym |
| `qualityBoost` | int | ≥0 | 來源：policy |
| `teamworkBoost` | int | ≥0 | 來源：sofa / gym |
| `charismaBoost` | int | ≥0 | 預留 |
| `decor` | int | ≥0 | 來源：lamp，影響 walker 視覺 |

#### 接案系統（v2 新增）

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `clients` | array of Project | — | 案件陣列（offered + active + 最近結算 done/failed 共 ≤30 筆）|
| `projectsCompleted` | int | ≥0 | 累計完成案件數（IPO 條件之一：≥30）|
| `projectsFailed` | int | ≥0 | 累計失敗案件數 |
| `lastRerollDay` | int | ≥0 | 最近重 roll 收件匣的天數，0 = 沒重 roll 過 |

#### 招募 / 系統旗標

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `vacancy` | bool | — | 是否進入「人才荒」空窗期 |
| `vacancyTimer` | int | ≥0 | 空窗剩餘天數 |
| `bankrupt` | bool | — | 是否破產 |
| `bankruptCountdown` | int | 0-5 | **連續資金 ≤0 的天數**（達 5 → 破產）|
| `tutorialStep` | int | 0-7 | 教學進度。≥7 表示已看完 |
| `recruitmentClosed` | bool | — | 玩家手動暫停招募 |

#### 集合

| 欄位 | 型別 | 說明 |
|---|---|---|
| `staff` | array of Dog | 員工陣列 |
| `log` | array of LogEntry | 最近 10 筆日誌（`{ day, msg }`）|

#### IPO 勝利

| 欄位 | 型別 | 說明 |
|---|---|---|
| `ipoAchievedAt` | int \| null | IPO 達成天數，未達成 = `null` |
| `ipoDismissed` | bool | 玩家是否已關閉 IPO 慶祝 modal |

#### 銀行貸款（一次性救急機制）

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `loanTaken` | bool | — | 是否已借過。**一輩子限一次**，true 後永久不可再觸發貸款 modal |
| `loanRepayDaysLeft` | int | 0-80 | 剩餘還款天數，0 = 無貸款。每天 morning 自動扣 $5，扣到 0 為止 |

#### Dog v2 子物件結構

```json
{
  "id": "dog_42",
  "role": "工程師",
  "breed": "柯基",
  "emoji": "🐕",
  "name": "旺財",
  "traits": ["低調"],
  "flavor": "...",
  "passive": "...",
  "motto": "...",
  "stats": { "speed": 9, "quality": 7, "teamwork": 4, "charisma": 3 },
  "grade": "B",
  "expectedSalary": 22,
  "severance": 66,
  "patience": 5,
  "score": 0,
  "image": "/assets/dog-profiles/engineer-1.png",
  "isCEO": false,

  "morale": 70,
  "fatigue": 12,
  "loyalty": 55,
  "experience": 18,
  "assignedProjectId": "p_8",
  "daysAtCompany": 14,
  "unhappyLeaveDays": 0,
  "learnedTraits": ["overtime", "mentor"],
  "pendingTraitChoice": null
}
```

| 欄位 | 型別 | 範圍 | 說明 |
|---|---|---|---|
| `id` | string | — | 唯一識別 |
| `stats` | object | speed/quality/teamwork/charisma 各 1-10 | 4 維能力值（**v2 從 productivity/morale/stability/revenue 改成此 4 維**）|
| `grade` | string | `S` / `A` / `B` / `C` / `D` | CEO 不算 grade |
| `morale` | int | 0-100 | 個人士氣（**v2 從全公司 morale 改成個人**）|
| `fatigue` | int | 0-100 | 疲勞 |
| `loyalty` | int | 0-100 | 忠誠度（防挖角護盾）|
| `experience` | int | ≥0 | 累積經驗，達門檻自動升 grade |
| `assignedProjectId` | string \| null | — | 目前指派到的案 id |
| `daysAtCompany` | int | ≥0 | 在公司天數（自然累積 loyalty）|
| `unhappyLeaveDays` | int | ≥0 | 連續被拒請假次數（連 3 次自動離職）|
| `onLeaveDay` | int \| null | — | 准假當天的 day 編號；該日該員工 0 貢獻；隔天 runProjectsDay 自動清空 |
| `learnedTraits` | array of string | — | 已習得特性 id 列表（升級時 +1）；舊存檔缺此欄位視為 `[]` |
| `pendingTraitChoice` | object \| null | — | 待選 3 個特性。結構：`{ "choices": [...], "roundsLeft": 1 }`。`roundsLeft > 1` 代表選完還會接下一輪（S 直接面試會給 2 輪） |

**特性發放規則**（前端控制，後端只負責存）：
- 升級 D→C / C→B：不發特性
- 升級 B→A：1 輪選擇（roundsLeft=1）
- 升級 A→S：1 輪選擇
- 直接面試 A 級：1 輪選擇
- 直接面試 S 級：2 輪選擇（roundsLeft=2，第一輪選完自動進第二輪）

**`learnedTraits` 與 `pendingTraitChoice` 的 trait id 列舉**：`overtime` / `perfectionist` / `mentor` / `haggler` / `ironHeart` / `catalyst` / `enduring` / `social`。後端不需驗證 id 是否在列舉內（前端控制），但建議陣列長度 ≤ 8。

#### Project 子物件結構

```json
{
  "id": "p_8",
  "clientName": "貓貓重工",
  "clientTier": 2,
  "category": "tech",
  "title": "API 性能調校",
  "difficulty": 1.0,
  "workRequired": 60,
  "workDone": 24,
  "qualitySum": 168,
  "expectedQuality": 3.0,
  "reward": 240,
  "penalty": 96,
  "defaultDeadlineDays": 5,
  "deadlineDay": 47,
  "graceDays": 2,
  "assignedStaffIds": ["dog_42", "dog_15"],
  "status": "active",
  "reputationDelta": { "success": 3, "fail": -5 },
  "createdDay": 40,
  "acceptedDay": 42,
  "events": [],
  "pendingEvent": null,
  "rewardMul": 1.0,
  "qualityMul": 1.0,
  "eventCount": 0,
  "bonusEventChance": 0
}
```

| 欄位 | 型別 | 說明 |
|---|---|---|
| `id` | string | 唯一識別 |
| `clientTier` | int | 1-5 |
| `category` | string | `tech` / `design` / `marketing` / `service` |
| `status` | string | `offered` / `active` / `done` / `failed` / `late` |
| `deadlineDay` | int | 絕對 day 數；`offered` 狀態 = `-1` 表示未啟動 |
| `acceptedDay` | int, optional | 玩家接案時的 day；offered 狀態無此欄位 |
| `pendingEvent` | object \| null | 待玩家處理的事件（卡進度）|
| `events` | array | 已觸發過的事件紀錄（最多 1 個）|

`ProjectEvent` 結構：
```json
{
  "kind": "changeRequest",
  "triggeredDay": 45,
  "resolved": false,
  "targetDogId": "dog_42"
}
```

`kind` 列舉：`changeRequest` / `earlyDeliver` / `upsell` / `bugBurst` / `dogLeaveAsk` / `poaching`。

---

### 已棄用欄位（v1 → v2 移除）

下列欄位在 v2 不會出現，**收到也應忽略 / 拒絕**：

- `morale`（top-level，全公司士氣）→ 改用每隻 `Dog.morale`
- `health` → 改用 `reputation`
- `decor`（top-level）→ 移到 `companyBuffs.decor`
- `productivityBoost` / `stabilityBoost` / `trainingBoost` → 改用 `companyBuffs.speedBoost / qualityBoost / teamworkBoost`
- `activeChemistry` → 化學反應改成 per-project 觸發，不再持久化

---

### SaveMeta（GET / POST 成功回傳）

```json
{
  "version": 2,
  "revision": 3,
  "updated_at": "2026-04-26T12:34:56Z",
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
    "version": 2,
    "revision": 3,
    "updated_at": "2026-04-26T12:34:56Z",
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
1. **Version 檢查**：`version` 必須 == 2。`version === 1` → 400 / 也可選擇 silent ignore（前端不會送 v1）
2. 比對 `revision`：
   - 若資料庫沒有該 user 的 save → 允許任何 revision，視為首次存檔，server_revision 起始設為 1
   - 若有：server_revision 必須 == client_revision → 通過，server_revision += 1
   - 否則 → 409 衝突
3. Sanity check（下段）

**Response 200**（`UpsertResponse`，**不回傳 data 省頻寬**）：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "version": 2,
    "revision": 4,
    "updated_at": "2026-04-26T12:35:10Z"
  }
}
```

**Response 400**：payload 不合格式 / version 不支援 / sanity check 失敗。

**Response 409**：revision 衝突。
```json
{
  "code": 4090,
  "message": "save conflict",
  "data": {
    "server_revision": 5,
    "server_updated_at": "2026-04-26T12:36:00Z",
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

> **重寫建議**：因 v2 結構與 v1 完全不相容，可直接 `TRUNCATE saves`；或寫 migration 把 `WHERE version = 1` 的全刪。前端 client 對 v1 存檔已是「視為無效」，刪了不會有影響。

---

## Sanity Check（POST /saves 時建議執行）

**目的**：降低客端亂送成本，不追求完全防弊。

建議檢查：

1. **必要欄位非空** + 型別正確
2. **版本支援**：`version == 2` 才接受
3. **數值範圍**：
   - `reputation` ∈ [0, 100]
   - `day`, `money`, `officeLevel`, `tierBudget`, `projectsCompleted`, `projectsFailed`, `bankruptCountdown` >= 0
   - `bankruptCountdown` ≤ 5
   - `tutorialStep` ∈ [0, 7]
   - `loanRepayDaysLeft` ∈ [0, 80]
   - `companyBuffs` 子欄位皆 ≥ 0
4. **單調遞增**（與 server 上一次 revision 比對）：
   - `day >= prev.day`（同一場遊戲天數不會倒退）
   - `projectsCompleted >= prev.projectsCompleted`（完成數不能倒退）
5. **合理性**：
   - `staff.length <= OFFICE_LEVELS[officeLevel].maxStaff + 2`（給客端超載扣分一點彈性。v2 maxStaff 表為 `[3, 5, 7, 9, 12]`）
   - 每隻 `Dog.stats` 4 維各 ∈ [1, 10]
   - 每隻 `Dog.morale / fatigue / loyalty` ∈ [0, 100]
   - 每隻 `Dog.learnedTraits.length` ≤ 8（特性表上限）
   - `clients` 內每筆 `Project.status` 為合法列舉
   - `clients.length` ≤ 30（offered 5 + active N + 最近結算 5，留餘裕）

不過的 request → **400**，錯誤訊息指明哪個欄位。

---

## Versioning Policy

- **目前 `version = 2`**
- v1 永久停用
- 欄位新增且有合理預設：`version` 不變，後端解析時補預設
- 欄位移除 / 欄位語意改變：`version` + 1，後端保留至少 2 版

---

## Error Code 建議

| code | HTTP | 意義 |
|------|------|------|
| 0 | 200 | ok |
| 4000 | 400 | payload 格式錯誤 |
| 4001 | 400 | 不支援的 version（含 v1 棄用）|
| 4002 | 400 | sanity check 失敗（message 指明欄位）|
| 4010 | 401 | 未登入 / token 失效 |
| 4090 | 409 | revision 衝突 |
| 5000 | 500 | 伺服器內部錯誤 |

---

## 前端行為契約（給後端參考）

- 前端啟動 / 使用者登入後 `GET /saves`
- 進行遊戲中**每 3 天 + 關鍵動作**觸發 `POST /saves`
- 破產 / IPO 達成時會再 `POST /saves` 一次（`bankrupt=true` 或 `ipoAchievedAt!=null`），玩家按「重新開始」→ `DELETE /saves`
- 409 conflict：前端彈 modal 讓使用者選本地 or 雲端版本，不自動覆寫
- 401：前端有 auto refresh 機制
- v1 存檔：前端 `migrate()` 直接回 `null`，使用者會被視為「無存檔」從新狀態開始

---

## Open Questions

1. `data` 欄位大小上限：v2 因 `clients` 與 `staff` 較大，建議 server 設 **128KB** 拒絕超大 payload（v1 是 64KB）。
2. 是否要對 v1 存檔做特殊統計（顯示「你有 X 位玩家從 v1 升級」）？目前計畫直接砍掉。
