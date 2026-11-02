-- How a store's customer website looks. Only the fields a seller has changed
-- are saved; the API fills in defaults for the rest (see model.DefaultStoreTheme).
ALTER TABLE stores ADD COLUMN IF NOT EXISTS theme JSONB NOT NULL DEFAULT '{}';
