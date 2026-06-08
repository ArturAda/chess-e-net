INSERT INTO user_ratings (
    id,
    user_id,
    mode,
    board_size,
    time_limit_ms,
    rating,
    games_played,
    created_at,
    updated_at
)
SELECT
    gen_random_uuid(),
    user_id,
    'classic',
    board_size,
    time_limit_ms,
    rating,
    games_played,
    created_at,
    CURRENT_TIMESTAMP
FROM user_ratings
WHERE mode IN ('modern10', 'modern12')
ON CONFLICT (user_id, mode, board_size, time_limit_ms)
DO UPDATE SET
    rating = EXCLUDED.rating,
    games_played = GREATEST(user_ratings.games_played, EXCLUDED.games_played),
    updated_at = CURRENT_TIMESTAMP
WHERE EXCLUDED.games_played >= user_ratings.games_played;

DELETE FROM user_ratings
WHERE mode IN ('modern10', 'modern12');
