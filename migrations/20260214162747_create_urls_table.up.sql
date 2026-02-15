-- Создание таблицы для хранения URL
CREATE TABLE IF NOT EXISTS urls (
    id VARCHAR(255) PRIMARY KEY,
    original_url TEXT NOT NULL UNIQUE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Индекс для поиска по оригинальному URL
CREATE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);
