DROP INDEX IF EXISTS idx_original_url_user;
DROP INDEX IF EXISTS idx_user_urls;

CREATE UNIQUE INDEX IF NOT EXISTS idx_original_url ON urls(original_url);

ALTER TABLE urls DROP COLUMN IF EXISTS user_id;
