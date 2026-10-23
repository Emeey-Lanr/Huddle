-- Custom domains a store's storefront runs on, as bare lowercase hostnames
-- ("chiomasbags.com"). Browsers on https://<domain> may call the API with
-- that store's key, so storefront domains don't have to be listed in
-- CORS_ALLOWED_ORIGINS.
CREATE TABLE IF NOT EXISTS store_domains (
    id UUID PRIMARY KEY,
    store_id UUID NOT NULL,
    domain VARCHAR(253) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_store_domains_store FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT store_domains_lowercase CHECK (domain = lower(domain))
);

-- A domain belongs to one store only.
CREATE UNIQUE INDEX IF NOT EXISTS store_domains_domain_unique ON store_domains (domain);
CREATE INDEX IF NOT EXISTS store_domains_store_idx ON store_domains (store_id);
