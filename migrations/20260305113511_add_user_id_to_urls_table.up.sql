-- Добавление user_id
ALTER TABLE urls ADD COLUMN IF NOT EXISTS user_id VARCHAR(255);

-- Удаление старого индекса по original_url
DROP INDEX IF EXISTS idx_original_url;
ALTER TABLE urls DROP CONSTRAINT IF EXISTS urls_original_url_key;

-- Создание составного индекса для (original_url, user_id)
CREATE UNIQUE INDEX IF NOT EXISTS idx_original_url_user ON urls(original_url, user_id);

-- Индекс для быстрого поиска по пользователю
CREATE INDEX IF NOT EXISTS idx_user_urls ON urls(user_id);
