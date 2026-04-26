DROP INDEX IF EXISTS idx_leaderboard_goal_days;

CREATE INDEX IF NOT EXISTS idx_leaderboard_goal_days_money
    ON leaderboard_entries (goal, days ASC, money DESC);

ALTER TABLE leaderboard_entries
    DROP COLUMN IF EXISTS projects_completed;
