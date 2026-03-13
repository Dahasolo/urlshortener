-- Флаг, указывающий на то, что URL должен считаться удалённым
ALTER TABLE urls ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN DEFAULT FALSE;
