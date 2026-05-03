-- v7: 排行榜每個 (user_id, goal) 只保留一筆最快紀錄。
-- 防止同一帳號狂灌資料；新提交較快才會覆寫該筆。

-- Step 1: 刪重複，每組 (user_id, goal) 只保留排序鍵最佳的那筆。
-- 排序鍵：days ASC, money DESC, projects_completed DESC, id ASC（與 FindBestByUserAndGoal 一致）。
DELETE FROM leaderboard_entries
WHERE id IN (
    SELECT id FROM (
        SELECT id,
               ROW_NUMBER() OVER (
                   PARTITION BY user_id, goal
                   ORDER BY days ASC, money DESC, projects_completed DESC, id ASC
               ) AS rn
        FROM leaderboard_entries
    ) t
    WHERE t.rn > 1
);

-- Step 2: 加 unique constraint 強制 DB 層唯一。
CREATE UNIQUE INDEX IF NOT EXISTS uq_leaderboard_user_goal
    ON leaderboard_entries (user_id, goal);
