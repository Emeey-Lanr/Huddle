-- The name in a store's web address: chiomas-bags in chiomas-bags.<base domain>.
-- The same rules live in pkg/services/store_slug.go; keep the two reserved
-- lists identical.
ALTER TABLE stores ADD COLUMN IF NOT EXISTS slug VARCHAR(40);

-- Existing stores: "Chioma's Bags" becomes chiomas-bags. A name that is taken,
-- reserved or has no usable letters gets a short suffix from the store's id.
WITH named AS (
    SELECT id, created_at,
           trim(both '-' from left(trim(both '-' from regexp_replace(
               regexp_replace(lower(store_name), '[''’]', '', 'g'), '[^a-z0-9]+', '-', 'g')), 33)) AS base
    FROM stores
), ranked AS (
    SELECT id, base, row_number() OVER (PARTITION BY base ORDER BY created_at, id) AS n
    FROM named
)
UPDATE stores s
SET slug = CASE
    WHEN r.base = '' THEN 'store-' || left(md5(s.id::text), 6)
    WHEN r.n > 1 OR r.base = ANY (ARRAY['admin', 'api', 'app', 'assets', 'blog', 'cdn', 'dashboard', 'docs', 'ftp', 'help', 'mail', 'seller', 'static', 'status', 'store', 'stores', 'support', 'www'])
        THEN r.base || '-' || left(md5(s.id::text), 6)
    ELSE r.base
END
FROM ranked r
WHERE r.id = s.id;

-- Stores created outside services.CreateStore still get a valid, unique slug.
ALTER TABLE stores ALTER COLUMN slug SET DEFAULT 'store-' || left(md5(gen_random_uuid()::text), 12);
ALTER TABLE stores ALTER COLUMN slug SET NOT NULL;
ALTER TABLE stores ADD CONSTRAINT stores_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$');
CREATE UNIQUE INDEX IF NOT EXISTS stores_slug_unique ON stores (slug);
