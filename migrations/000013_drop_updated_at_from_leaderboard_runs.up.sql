-- v8.1: 移除 leaderboard_runs.updated_at。
-- 原因：這張表只有 Upsert（新局 / restart）與 Delete（送出後）兩種操作，
-- updated_at 永遠等於 started_at，提供不了任何資訊。

ALTER TABLE leaderboard_runs DROP COLUMN IF EXISTS updated_at;
