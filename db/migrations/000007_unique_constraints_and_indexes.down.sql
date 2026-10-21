DROP INDEX IF EXISTS images_product_id_idx;
DROP INDEX IF EXISTS products_public_id_unique;
ALTER TABLE products ALTER COLUMN public_id DROP NOT NULL;
DROP INDEX IF EXISTS products_store_name_unique;
DROP INDEX IF EXISTS customers_store_email_unique;
DROP INDEX IF EXISTS stores_admin_store_name_unique;
DROP INDEX IF EXISTS admins_username_unique;
DROP INDEX IF EXISTS admins_email_unique;
