-- v6: 加 company_name 欄位作為排行榜顯示用名稱（玩家自訂的公司名）。
-- nickname 欄位保留（仍寫入帳號），但前端不再 SELECT。

ALTER TABLE leaderboard_entries
    ADD COLUMN IF NOT EXISTS company_name VARCHAR(32) NULL;
