# DogOffice 事件 log API spec

紀錄玩家關鍵動作的輕量事件 log。後端 in-memory 緩衝，每 100 筆 / 5 分鐘 / 關機時批次 insert。

> 規格版本：draft-1（2026-05-04，新增）
> 基準 API base：`https://dog-company-production.up.railway.app/api/v1`

---

## 用途

- 反作弊分析：對比事件序列與 leaderboard submit，找出異常紀錄
- 留存統計：玩家完成豪華總部前平均做了幾次升級、買了哪些設施
- 故障重現：客訴時可看玩家做過什麼

**非用途**：金流憑證、必須 100% 持久的紀錄（server crash 會遺失 buffer 中的事件）

---

## 端點

| Method | Path | Auth | 說明 |
|--------|------|------|------|
| POST   | `/events` | Bearer | 紀錄一筆事件 |

---

## POST `/events`

**Request body**：
```json
{
  "type": "office_upgrade",
  "payload": { "from_level": 2, "to_level": 3 }
}
```

| 欄位 | 必填 | 說明 |
|------|------|------|
| `type` | ✓ | 事件類型字串，最長 64 字元 |
| `payload` | optional | 任意 JSON object，存到 `event_logs.payload`（jsonb） |

`user_id` 從 Bearer token 解出，`created_at` 由 server 在收到 request 時填入（不採用 client 時間，避免時差作弊）。

**Response 200**：
```json
{ "code": 0, "message": "ok" }
```

**Behavior**：
- 即時回應（不等 DB 寫入）
- 後端把事件丟入 in-memory buffer
- Buffer 達 100 筆 / 距上次 flush 滿 5 分鐘 / server graceful shutdown 時，批次 insert 到 `event_logs`
- DB 寫入失敗只 log，事件丟棄（best-effort）

---

## 標準事件類型

前端只送這兩種；其他兩種由後端內部寫入：

| type | 觸發時機 | 來源 | payload |
|------|---------|------|---------|
| `office_upgrade` | 升辦公室成功 | 前端 | `{ from_level, to_level }` |
| `buy_shop_item` | 買設施 / 升級設施 | 前端 | `{ item_id, level }` |
| `start_run` | 玩家開新局，server 紀錄 wall-clock 起點 | 後端 `StartRun` handler | `{ goal }` |
| `submit_leaderboard` | 送出豪華總部紀錄成功 | 後端 `Submit` handler | `{ goal, days, money, office_level, staff_count, projects_completed }` |

支援其他 type 字串（例如未來加 `hire_dog`、`fire_dog`），前端可自由擴充，後端不擋。

---

## Schema

```sql
CREATE TABLE event_logs (
  id         BIGSERIAL PRIMARY KEY,
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  event_type VARCHAR(64) NOT NULL,
  payload    JSONB NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_event_logs_user_created ON event_logs(user_id, created_at DESC);
CREATE INDEX idx_event_logs_type_created ON event_logs(event_type, created_at DESC);
```

Migration 檔名：`000012_create_event_logs.up.sql`

---

## 前端契約

- 失敗一律靜默忽略（fire-and-forget），不阻擋遊戲流程
- 未登入不送 API
- 不重試
- 不顯示 loading 狀態給玩家看

---

## 已知限制

1. **持久性**：Server 強制中斷（kill -9 / OOM）時，buffer 內未滿 100 筆的事件會遺失
2. **Multi-instance**：目前 buffer 是單機 in-memory，多 instance 部署時每個實例各有自己的 buffer。Flush 互不影響但會分散
3. **無 GET endpoint**：目前沒有讀事件的 API，需要直接連 DB 查
