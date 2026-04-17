CREATE TABLE IF NOT EXISTS saves (
    user_uid   UUID         PRIMARY KEY REFERENCES users(uid) ON DELETE CASCADE,
    version    INT          NOT NULL,
    revision   INT          NOT NULL DEFAULT 1,
    data       JSONB        NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
