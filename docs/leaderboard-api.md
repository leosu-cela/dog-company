# DogOffice 排行榜 API spec

本文件描述 `dog-company` 後端需要新增的排行榜端點。用於 race-to-target 玩法：玩家衝到 $50,000 資金目標時記錄達標天數，做全域最快榜。

> 規格版本：draft-1（2026-04-19）
> 基準 API base：`https://dog-company-production.up.railway.app/api/v1`

---

## 總覽

| Method | Path | Auth | 說明 |
|--------|------|------|------|
| GET    | `/leaderboard` | **optional** | 取得全球排行榜 top N |
| GET    | `/leaderboard/me` | Bearer | 取得當前使用者自己的所有達標紀錄 |
| POST   | `/leaderboard` | Bearer | 提交達標紀錄（玩家完成 race 時觸發） |

**認證**：
- `GET /leaderboard` 可匿名（非登入玩家也能看榜）
- `GET /leaderboard/me` 必須 Bearer token
- `POST` 必須 Bearer token（不允許假冒身份）

**Response 格式**：沿用 `CommonResponse`：`{ code, message, data }`。

---

## 資料模型

### LeaderboardEntry（DB 儲存 + API 回傳）

```json
{
  "id": 42,
  "user_id": 7,
  "nickname": "leosu",
  "days": 58,
  "money": 52340,
  "goal": 50000,
  "office_level": 4,
  "staff_count": 9,
  "submitted_at": "2026-04-19T10:20:30Z"
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `id` | int | 資料庫流水號（server 產生） |
| `user_id` | int | 提交者 user.id |
| `nickname` | string | 顯示名稱。server 直接用 `users.account` 填入 |
| `days` | int, >=1 | 達標時的 day 數 |
| `money` | int, >=0 | 達標當下資金 |
| `goal` | int | 目標金額（固定 50000，保留欄位給未來擴充） |
| `office_level` | int, 0-4 | 達標時辦公室等級 |
| `staff_count` | int, >=0 | 達標時員工數 |
| `submitted_at` | ISO string | server 紀錄時間 |

### SubmitPayload（POST 請求 body）

```json
{
  "days": 58,
  "money": 52340,
  "goal": 50000,
  "office_level": 4,
  "staff_count": 9
}
```

`user_id` / `nickname` / `submitted_at` / `id` **不由客端提交**，server 自行填入。

---

## Endpoints

### GET `/leaderboard`

取得最快達標 top 10（按 `days ASC`，同天數按 `money DESC`）。

**Query params**（optional）：
- `limit`：預設 10，上限 50
- `goal`：篩特定目標。不帶 = 回 default 50000

**Response 200**：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "entries": [
      {
        "id": 17,
        "user_id": 3,
        "nickname": "fastdog",
        "days": 42,
        "money": 51200,
        "goal": 50000,
        "office_level": 4,
        "staff_count": 8,
        "submitted_at": "2026-04-18T22:15:00Z"
      }
      // ... up to `limit` entries
    ],
    "my_best": {
      "rank": 15,
      "entry": {
        "id": 88,
        "user_id": 7,
        "nickname": "leosu",
        "days": 62,
        "money": 50450,
        "goal": 50000,
        "office_level": 4,
        "staff_count": 10,
        "submitted_at": "2026-04-19T08:00:00Z"
      }
    }
  }
}
```

- `entries`：全球 top N
- `my_best`：**僅當帶 Bearer token 且該 user 有紀錄時出現**。內含 `rank`（全球排名，1-based）+ 該 user 在此 goal 下的最佳紀錄
- 不登入 / 該 user 無紀錄 → 不回傳 `my_best` 欄位（或回傳 `null`）

不登入也能打。無紀錄時回 `{ entries: [] }`。

---

### GET `/leaderboard/me`

取得當前使用者的所有達標紀錄（個人歷史）。

**Query params**（optional）：
- `limit`：預設 20，上限 50
- `goal`：篩特定目標；不帶 = 回全部

**排序**：`submitted_at DESC`（最新的在前）

**Response 200**：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "entries": [
      {
        "id": 42,
        "user_id": 7,
        "nickname": "leosu",
        "days": 58,
        "money": 52340,
        "goal": 50000,
        "office_level": 4,
        "staff_count": 9,
        "submitted_at": "2026-04-19T10:20:30Z"
      }
    ]
  }
}
```

**Response 401**：未登入 / token 失效。

---

### POST `/leaderboard`

提交一筆達標紀錄。

**Request body**：見上面 SubmitPayload。

**Behavior**：
1. 必須 Bearer token（否則 401）
2. Sanity check（下段）
3. 從 `users` 表取 `account` 當 `nickname` 寫入
4. 插入新一筆紀錄（同一 user 可有多筆，各代表不同 run）

**Response 200**：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 42,
    "rank": 5,
    "total": 120
  }
}
```

| 欄位 | 說明 |
|---|---|
| `id` | 新紀錄的 ID |
| `rank` | 此紀錄目前排第幾（1-based） |
| `total` | 目前同 `goal` 下的總紀錄數 |

**Response 400**：payload 錯誤 / sanity check 失敗。

**Response 401**：未登入 / token 失效。

---

## Sanity Check（POST 時）

- `days >= 1`、`days <= 365`（一局遊戲超過一年視為異常）
- `money >= goal`（達不到目標不能提交）
- `money <= goal * 3`（合理上限，防止外掛）
- `goal` 限定白名單：目前只接受 `50000`；未來擴充時增加
- `office_level` ∈ [0, 4]
- `staff_count` ∈ [0, 50]
- 同 `user_id + goal + days + money` 組合 1 分鐘內重複送視為 dedupe（靜默成功，不重複寫）

不過 → **400**，message 指明欄位。

---

## DB Schema 建議

### `leaderboard_entries`

```sql
CREATE TABLE leaderboard_entries (
  id            BIGSERIAL PRIMARY KEY,
  user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  nickname      VARCHAR(64) NOT NULL,
  days          INT NOT NULL,
  money         INT NOT NULL,
  goal          INT NOT NULL,
  office_level  SMALLINT NOT NULL,
  staff_count   SMALLINT NOT NULL,
  submitted_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 主查詢用：按 goal 篩 + days ASC + money DESC
CREATE INDEX idx_leaderboard_goal_days ON leaderboard_entries(goal, days ASC, money DESC);

-- dedupe 查詢用
CREATE INDEX idx_leaderboard_user_goal ON leaderboard_entries(user_id, goal, submitted_at DESC);
```

### Migration 檔名建議

```
000005_create_leaderboard.up.sql
000005_create_leaderboard.down.sql
```

---

## Error Code 建議

| code | HTTP | 意義 |
|------|------|------|
| 0 | 200 | ok |
| 4000 | 400 | payload 格式錯誤 |
| 4002 | 400 | sanity check 失敗 |
| 4010 | 401 | 未登入 / token 失效 |
| 5000 | 500 | 伺服器內部錯誤 |

---

## 前端行為契約

- 玩家達成 $50k 時：若已登入 → POST /leaderboard；未登入 → 只存本機 localStorage
- 開排行榜 Panel 時：GET /leaderboard；失敗或未登入 → 顯示本機 localStorage 的紀錄（降級 UX）
- 排行榜 Panel 同時顯示 **全球榜 tab** 與 **我的歷史 tab**（我的歷史純 localStorage，不上傳）
- 不自動重試 POST：失敗就吞掉、寫 localStorage 作為備份，避免重複污染排行榜

---

## Open Questions

1. 是否支援刪除自己的紀錄？目前設計為不可刪（永久記錄）。若要支援，另加 DELETE `/leaderboard/:id`。
2. 是否做反外掛？目前只做 sanity check；若發現作弊潮再加 rate limit 或 server-side 模擬驗證。
3. Rate limit 建議：同 user 每 30 秒最多 1 次 POST，防止連續送假資料。
