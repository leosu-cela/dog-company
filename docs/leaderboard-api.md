# DogOffice 排行榜 API spec

本文件描述 `dog-company` 後端排行榜端點。v2 接案制改用 **IPO 上市**作為勝利條件，排行榜記錄 IPO 達成時的天數、資金、辦公室、員工數、**完成案件數**。

> 規格版本：draft-6（2026-05-02，新增 `company_name` 欄位作為顯示名；`nickname` 仍寫入帳號但前端不再使用）
> 基準 API base：`https://dog-company-production.up.railway.app/api/v1`

---

## v2 變更摘要

| 項目 | v1（draft-1）| v2（本版）|
|---|---|---|
| 勝利條件 | 資金 ≥ $50,000 單一條件 | IPO 四條件：信譽≥80、資金≥$50k、辦公室≥Lv3、完成案≥30 |
| 提交欄位 | days / money / goal / office_level / staff_count | **新增 `projects_completed`** |
| `goal` 用途 | 記錄目標金額 | 沿用，固定 50000；保留欄位給未來擴充 |

---

## 總覽

| Method | Path | Auth | 說明 |
|--------|------|------|------|
| GET    | `/leaderboard` | **optional** | 取得全球榜 top N + 自己的最佳（一次拿） |
| POST   | `/leaderboard` | Bearer | 提交達標紀錄（達成豪華總部時觸發）|
| POST   | `/leaderboard/run` | Bearer | 開新局時呼叫，後端紀錄 wall-clock 起始時間（v8 新增）|

**認證**：
- `GET /leaderboard` 可匿名（匿名時不回 `me` 欄位）
- 帶 Bearer token 時，`me` 欄位會包含該 user 的個人最佳
- `POST /leaderboard` / `POST /leaderboard/run` 必須 Bearer token

**Response 格式**：沿用 `CommonResponse`：`{ code, message, data }`。

---

## 資料模型

### LeaderboardEntry（DB 儲存 + API 回傳）

```json
{
  "id": 42,
  "user_id": 7,
  "nickname": "leosu",
  "company_name": "旺財事務所",
  "days": 58,
  "money": 52340,
  "goal": 50000,
  "office_level": 4,
  "staff_count": 9,
  "projects_completed": 32,
  "submitted_at": "2026-04-26T10:20:30Z"
}
```

| 欄位 | 型別 | 說明 |
|------|------|------|
| `id` | int | 資料庫流水號（server 產生） |
| `user_id` | int | 提交者 user.id |
| `nickname` | string | 帳號（server 用 `users.account` 填）。**v6 起前端不再用作顯示**，欄位保留但不主動呈現 |
| `company_name` | string | **v6 新增**：玩家自訂的公司名（顯示來源）。長度 2-8 字元（rune count），白名單 CJK + 英數 + 空白 |
| `days` | int, ≥1 | IPO 達成時的 day 數（**主排序鍵**）|
| `money` | int, ≥0 | 達成時資金 |
| `goal` | int | 目標金額（固定 50000）|
| `office_level` | int, 0-4 | 達成時辦公室等級（≥3 才可達 IPO）|
| `staff_count` | int, ≥0 | 達成時員工數 |
| `projects_completed` | int, ≥30 | **完成案件累計數（v2 新增；IPO 條件之一）**|
| `submitted_at` | ISO string | server 紀錄時間 |

### SubmitPayload（POST 請求 body）

```json
{
  "days": 58,
  "money": 52340,
  "goal": 50000,
  "office_level": 4,
  "staff_count": 9,
  "projects_completed": 32,
  "company_name": "旺財事務所"
}
```

`user_id` / `nickname` / `submitted_at` / `id` **不由客端提交**，server 自行填入。
`company_name` 客端必填；server 會做 sanity check（長度、白名單、髒話）。

---

## Endpoints

### GET `/leaderboard`

取得最快達標 top 10。

**主排序**：`days ASC`（最快達 IPO 者優先）。
**次排序（同天數）**：`money DESC` → `projects_completed DESC`（沒做完更多案就同分時，多接案的優先）。

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
        "projects_completed": 30,
        "submitted_at": "2026-04-25T22:15:00Z"
      }
    ],
    "me": {
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
        "projects_completed": 33,
        "submitted_at": "2026-04-26T08:00:00Z"
      }
    }
  }
}
```

- `entries`：全球 top N
- `me`：**僅當帶 Bearer token 且該 user 有紀錄時出現**。內含 `rank`（全球排名，1-based）+ 該 user 在此 goal 下的最佳紀錄
- 不登入 / 該 user 無紀錄 → 不回傳 `me` 欄位（或回 `null`）

---

### POST `/leaderboard/run`

**v8 新增**。前端在「開新局」時呼叫，後端 upsert `(user_id, goal)` 對應的 `started_at = now()`。
送排行榜時驗證真實 wall-clock 時長下界，擋偽造紀錄。

**呼叫時機**：
- 玩家從 splash 點開始
- 破產後 restart
- **載入雲端存檔不應呼叫**，沿用原始 `started_at`（不重置）

**Request body**（可空）：
```json
{ "goal": 50000 }
```
缺欄位時 server 自動補 `goal=50000`。

**Response 200**：
```json
{
  "code": 0,
  "message": "ok",
  "data": { "started_at": "2026-05-04T12:34:56Z" }
}
```

冪等：同 `(user_id, goal)` 重複呼叫會覆蓋 `started_at` 為現在（用於 restart 重置計時）。

---

### POST `/leaderboard`

提交一筆豪華總部達成紀錄。

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

- `days >= 1`、`days <= 365`
- `money >= goal`（達不到 $50k 不能提交）
- `money <= goal * 5`（v2 後期單筆 tier5 案 $1900~2600，整局可累到 $250k+，上限放寬到 5 倍）
- `goal` 限定白名單：目前只接受 `50000`
- `office_level` ∈ [3, 4]（IPO 條件要求 ≥ Lv3，所以 0~2 直接 reject）
- `staff_count` ∈ [0, 50]
- **`projects_completed >= 30`**（IPO 條件要求 ≥30 完成案，否則 reject）
- `projects_completed <= 365 * 3`（一天最多平均 3 案）
- **`company_name`**（v6 新增）：
  - trim 後 rune count ∈ [2, 8]
  - 字元白名單：`[\p{Han}A-Za-z0-9 ]`（CJK + 英數 + ASCII 空白）
  - 不得含髒話清單（後端 `internal/leaderboard/profanity.go` 維護，獨立於前端清單）
- 同 `user_id + goal + days + money + projects_completed` 組合 1 分鐘內重複送 → dedupe（靜默成功）
- **Run 時長下界（v8 新增）**：必須先呼叫過 `POST /leaderboard/run`，且 `now - started_at >= days * 4.9` 秒（`MinSecondsPerDay = 4.9`，遊戲最快 1 天 5 秒，留 0.1 秒緩衝）。失敗回 `4002`，message 為 `no active run; please start a new game first` 或 `run duration too short (min Xs)`。提交成功後 server 自動刪除該 run 紀錄，避免重送

不過 → **400**，message 指明欄位。

---

## DB Schema 建議

### `leaderboard_entries`

```sql
CREATE TABLE leaderboard_entries (
  id                  BIGSERIAL PRIMARY KEY,
  user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  nickname            VARCHAR(64) NOT NULL,
  company_name        VARCHAR(32) NULL,        -- v6 起；舊資料為 NULL，前端 fallback「未命名公司」
  days                INT NOT NULL,
  money               INT NOT NULL,
  goal                INT NOT NULL,
  office_level        SMALLINT NOT NULL,
  staff_count         SMALLINT NOT NULL,
  projects_completed  INT NOT NULL DEFAULT 0,
  submitted_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 主查詢：按 goal 篩 + days ASC + money DESC + projects_completed DESC
CREATE INDEX idx_leaderboard_goal_days ON leaderboard_entries(goal, days ASC, money DESC, projects_completed DESC);

-- dedupe / 個人歷史
CREATE INDEX idx_leaderboard_user_goal ON leaderboard_entries(user_id, goal, submitted_at DESC);
```

### `leaderboard_runs`（v8 新增）

```sql
CREATE TABLE leaderboard_runs (
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  goal       INT NOT NULL,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (user_id, goal)
);
```

紀錄當前進行中的 run。送排行榜成功後 server 自動刪除該筆。

### Migration 檔名建議

```
000005_create_leaderboard.up.sql                              （v1 既有）
000006_add_projects_completed_to_leaderboard.up.sql           （v2 加欄位）
000006_add_projects_completed_to_leaderboard.down.sql
000011_create_leaderboard_runs.up.sql                         （v8 新增 run 表）
000013_drop_updated_at_from_leaderboard_runs.up.sql           （v8.1 移除冗餘欄位）
```

v2 增量 migration（既有 DB 升級用）：
```sql
ALTER TABLE leaderboard_entries
  ADD COLUMN projects_completed INT NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_leaderboard_goal_days;
CREATE INDEX idx_leaderboard_goal_days
  ON leaderboard_entries(goal, days ASC, money DESC, projects_completed DESC);
```

> **重寫提示**：v1 資料的 `projects_completed` 全為 0，會排在同天數同資金紀錄的最後，不影響排行公平性，可保留。或全清也無妨。

---

## Error Code

| code | HTTP | 意義 |
|------|------|------|
| 0 | 200 | ok |
| 4000 | 400 | payload 格式錯誤 |
| 4002 | 400 | sanity check 失敗 |
| 4010 | 401 | 未登入 / token 失效 |
| 5000 | 500 | 伺服器內部錯誤 |

---

## 前端行為契約

- 玩家達成 IPO 四條件時：若已登入 → POST /leaderboard；未登入 → 只存本機 localStorage
- 開排行榜 Panel 時：GET /leaderboard；失敗或未登入 → 顯示本機 localStorage 紀錄
- 排行榜面板只打一次 `GET /leaderboard`：頂部顯示「我的最佳 + 全球排名」（`me` 欄位），下方列全球 top N（`entries`）
- 不自動重試 POST：失敗就吞掉、寫 localStorage 作為備份
- UI 在每筆紀錄上要展示 `projects_completed`（IPO 上市的標誌之一）

---

## Open Questions

1. 是否要把 `reputation` 也記到排行榜？目前未加，因 days 與 money 已能反映實力。若要加，會是 v3。
2. 是否支援刪除自己的紀錄？目前設計為不可刪。
3. Rate limit 建議：同 user 每 30 秒最多 1 次 POST。
