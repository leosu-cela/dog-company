CREATE TABLE IF NOT EXISTS leaderboard_entries (
    id           BIGSERIAL    PRIMARY KEY,
    user_id      BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    nickname     VARCHAR(64)  NOT NULL,
    days         INT          NOT NULL,
    money        INT          NOT NULL,
    goal         INT          NOT NULL,
    office_level SMALLINT     NOT NULL,
    staff_count  SMALLINT     NOT NULL,
    submitted_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_leaderboard_goal_days_money
    ON leaderboard_entries (goal, days ASC, money DESC);

CREATE INDEX IF NOT EXISTS idx_leaderboard_user_goal_submitted
    ON leaderboard_entries (user_id, goal, submitted_at DESC);
