-- Uniqueness rules (case-insensitive where people type the value) so that
-- concurrent requests cannot create duplicates the app-level checks miss.
CREATE UNIQUE INDEX IF NOT EXISTS admins_email_unique ON admins (lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS admins_username_unique ON admins (lower(username));
CREATE UNIQUE INDEX IF NOT EXISTS stores_admin_store_name_unique ON stores (admin_id, lower(store_name));
CREATE UNIQUE INDEX IF NOT EXISTS customers_store_email_unique ON customers (store_id, lower(email));
CREATE UNIQUE INDEX IF NOT EXISTS products_store_name_unique ON products (store_id, lower(name));

ALTER TABLE products ALTER COLUMN public_id SET NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS products_public_id_unique ON products (public_id);

-- Lookups by foreign key (Postgres does not index these automatically).
CREATE INDEX IF NOT EXISTS images_product_id_idx ON images (product_id);
