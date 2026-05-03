-- v9: 重要事件 log，記錄玩家的關鍵動作（升辦公室、買設施、開新局、送排行榜）。
-- 用於日後反作弊分析、留存統計。寫入採 in-memory 100 筆批次 insert 降 DB 壓力。
-- user_uid 直接存 JWT 帶來的 UUID，省去 uid → id 的查表。

CREATE TABLE IF NOT EXISTS event_logs (
    id         BIGSERIAL    PRIMARY KEY,
    user_uid   UUID         NOT NULL REFERENCES users(uid) ON DELETE CASCADE,
    event_type VARCHAR(64)  NOT NULL,
    payload    JSONB        NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_event_logs_user_created
    ON event_logs (user_uid, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_event_logs_type_created
    ON event_logs (event_type, created_at DESC);
