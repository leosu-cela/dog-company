ALTER TABLE users
    ADD COLUMN IF NOT EXISTS uid UUID NOT NULL DEFAULT gen_random_uuid();

CREATE UNIQUE INDEX IF NOT EXISTS uniq_users_uid ON users (uid);
