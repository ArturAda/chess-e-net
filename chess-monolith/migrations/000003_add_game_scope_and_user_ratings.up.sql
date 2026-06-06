ALTER TABLE games
    ADD COLUMN IF NOT EXISTS board_size INT DEFAULT 8;

ALTER TABLE games
    ADD COLUMN IF NOT EXISTS time_limit_ms BIGINT DEFAULT 600000;

UPDATE games
SET board_size = 8
WHERE board_size IS NULL;

UPDATE games
SET time_limit_ms = 600000
WHERE time_limit_ms IS NULL;

ALTER TABLE games
    ALTER COLUMN board_size SET NOT NULL;

ALTER TABLE games
    ALTER COLUMN time_limit_ms SET NOT NULL;

CREATE TABLE IF NOT EXISTS user_ratings
(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    mode VARCHAR(50) NOT NULL,
    board_size INT NOT NULL,
    time_limit_ms BIGINT NOT NULL,
    rating INT NOT NULL DEFAULT 1200,
    games_played INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_ratings_scope_unique UNIQUE (user_id, mode, board_size, time_limit_ms),
    CONSTRAINT user_ratings_board_size_check CHECK (board_size > 0),
    CONSTRAINT user_ratings_time_limit_check CHECK (time_limit_ms > 0),
    CONSTRAINT user_ratings_rating_check CHECK (rating >= 100),
    CONSTRAINT user_ratings_games_played_check CHECK (games_played >= 0)
);

CREATE INDEX IF NOT EXISTS idx_user_ratings_scope_rating
    ON user_ratings (mode, board_size, time_limit_ms, rating DESC);
