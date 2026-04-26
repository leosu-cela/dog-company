-- Composite index supporting the global sort key
-- (days ASC, money DESC, projects_completed DESC, id ASC) scoped to a user+goal.
-- Targets FindBestByUserAndGoal, which previously could not satisfy ORDER BY
-- from the (user_id, goal, submitted_at) index and required a sort step.
CREATE INDEX IF NOT EXISTS idx_leaderboard_user_goal_sort
    ON leaderboard_entries (user_id, goal, days ASC, money DESC, projects_completed DESC, id ASC);

-- Index for future "recently-updated saves" queries / TTL-style cleanup.
CREATE INDEX IF NOT EXISTS idx_saves_updated_at
    ON saves (updated_at);
