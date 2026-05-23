-- Включаем расширение для генерации UUID, если оно еще не включено
CREATE
EXTENSION IF NOT EXISTS pgcrypto;

-- Таблица пользователей
CREATE TABLE IF NOT EXISTS users
(
    id
    UUID
    PRIMARY
    KEY
    DEFAULT
    gen_random_uuid
(
),
    username VARCHAR
(
    50
) UNIQUE NOT NULL,
    email VARCHAR
(
    255
) UNIQUE NOT NULL,
    password_hash VARCHAR
(
    255
) NOT NULL,
    rating INT DEFAULT 1200,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
                             );

-- Таблица игр
CREATE TABLE IF NOT EXISTS games
(
    id
    UUID
    PRIMARY
    KEY
    DEFAULT
    gen_random_uuid
(
),
    white_id UUID NOT NULL REFERENCES users
(
    id
),
    black_id UUID NOT NULL REFERENCES users
(
    id
),
    mode VARCHAR
(
    50
) DEFAULT 'classic',
    status VARCHAR
(
    20
) DEFAULT 'active',
    turn VARCHAR
(
    10
) DEFAULT 'white',
    board_state TEXT,
    winner_id UUID REFERENCES users
(
    id
),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
                             );

-- Индексы для быстрого поиска (матчмейкинг и профиль)
CREATE INDEX idx_users_rating ON users (rating);
CREATE INDEX idx_games_status ON games (status);
CREATE INDEX idx_games_white_id ON games (white_id);
CREATE INDEX idx_games_black_id ON games (black_id);