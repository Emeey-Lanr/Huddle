-- Groups sort a store's products for shoppers ("Shoes", "Bags"). A product is
-- in one group or none; deleting a group leaves its products ungrouped.
CREATE TABLE IF NOT EXISTS product_groups (
    id UUID PRIMARY KEY,
    public_id VARCHAR(40) NOT NULL,
    store_id UUID NOT NULL,
    name VARCHAR(40) NOT NULL,
    -- The group's name in the shop's web address: ?group=ankara-bags.
    slug VARCHAR(40) NOT NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_product_groups_store FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT product_groups_slug_format CHECK (slug ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?$'),
    CONSTRAINT product_groups_name_not_blank CHECK (btrim(name) <> '')
);

CREATE UNIQUE INDEX IF NOT EXISTS product_groups_public_id_unique ON product_groups (public_id);
CREATE UNIQUE INDEX IF NOT EXISTS product_groups_store_slug_unique ON product_groups (store_id, slug);
CREATE UNIQUE INDEX IF NOT EXISTS product_groups_store_name_unique ON product_groups (store_id, lower(name));

ALTER TABLE products ADD COLUMN IF NOT EXISTS group_id UUID;
ALTER TABLE products ADD CONSTRAINT fk_products_group FOREIGN KEY (group_id) REFERENCES product_groups(id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS products_group_idx ON products (group_id) WHERE group_id IS NOT NULL;
