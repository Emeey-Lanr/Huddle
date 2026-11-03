ALTER TABLE products DROP CONSTRAINT IF EXISTS fk_products_group;
DROP INDEX IF EXISTS products_group_idx;
ALTER TABLE products DROP COLUMN IF EXISTS group_id;
DROP TABLE IF EXISTS product_groups;
