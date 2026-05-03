-- v8: 紀錄每個 (user_id, goal) 當前進行中的一場 run。
-- 用於排行榜送出時驗證真實 wall-clock 時長下界，擋掉偽造紀錄。
-- 開新局時 upsert started_at；送出排行榜時驗證 elapsed >= days * MinSecondsPerDay 後刪除。

CREATE TABLE IF NOT EXISTS leaderboard_runs (
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    goal       INT          NOT NULL,
    started_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, goal)
);
