ALTER TABLE leaderboard_entries
    ADD COLUMN IF NOT EXISTS projects_completed INT NOT NULL DEFAULT 0;

DROP INDEX IF EXISTS idx_leaderboard_goal_days_money;

CREATE INDEX IF NOT EXISTS idx_leaderboard_goal_days
    ON leaderboard_entries (goal, days ASC, money DESC, projects_completed DESC);
