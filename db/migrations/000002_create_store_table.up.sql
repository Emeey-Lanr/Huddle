CREATE TABLE IF NOT EXISTS stores (
    id UUID PRIMARY KEY,
    admin_id UUID NOT NULL,
    store_name VARCHAR (150) NOT NULL,
    store_description TEXT,
    api_key TEXT UNIQUE NOT NULL,
    store_id VARCHAR(50) UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),

     CONSTRAINT fk_stores_admin FOREIGN KEY (admin_id) REFERENCES admins(id) ON DELETE CASCADE
);




