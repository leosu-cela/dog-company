CREATE TABLE IF NOT EXISTS users (
    id            BIGSERIAL     PRIMARY KEY,
    account       VARCHAR(255)  NOT NULL,
    password_hash VARCHAR(255)  NOT NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_account ON users (account);
