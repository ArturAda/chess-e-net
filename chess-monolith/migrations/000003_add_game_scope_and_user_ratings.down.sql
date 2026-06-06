DROP INDEX IF EXISTS idx_user_ratings_scope_rating;

DROP TABLE IF EXISTS user_ratings;

ALTER TABLE games
    DROP COLUMN IF EXISTS time_limit_ms;

ALTER TABLE games
    DROP COLUMN IF EXISTS board_size;
