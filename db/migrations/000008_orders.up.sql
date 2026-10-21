CREATE TABLE IF NOT EXISTS orders (
    id UUID PRIMARY KEY,
    public_id VARCHAR(50) NOT NULL UNIQUE,
    store_id UUID NOT NULL,
    customer_id UUID,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    currency VARCHAR(3) NOT NULL,
    total NUMERIC(12,2) NOT NULL,
    customer_name VARCHAR(250) NOT NULL,
    customer_phone VARCHAR(20) NOT NULL,
    customer_email VARCHAR(250),
    delivery_method VARCHAR(10) NOT NULL,
    address TEXT,
    city VARCHAR(100),
    state VARCHAR(100),
    note TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_orders_store FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT fk_orders_customer FOREIGN KEY (customer_id) REFERENCES customers(id) ON DELETE SET NULL,
    CONSTRAINT orders_status_check CHECK (status IN ('pending', 'paid', 'fulfilled', 'cancelled', 'expired')),
    CONSTRAINT orders_delivery_method_check CHECK (delivery_method IN ('delivery', 'pickup'))
);

-- Seller order lists, a customer's orders, and the expiry sweep.
CREATE INDEX IF NOT EXISTS orders_store_created_idx ON orders (store_id, created_at DESC);
CREATE INDEX IF NOT EXISTS orders_customer_idx ON orders (customer_id);
CREATE INDEX IF NOT EXISTS orders_pending_expiry_idx ON orders (expires_at) WHERE status = 'pending';

-- Items copy the product's name and price at checkout, so later product
-- edits or deletes never change a past order.
CREATE TABLE IF NOT EXISTS order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL,
    product_id UUID,
    product_public_id VARCHAR(50) NOT NULL,
    product_name VARCHAR(200) NOT NULL,
    unit_price NUMERIC(12,2) NOT NULL,
    quantity INTEGER NOT NULL,
    line_total NUMERIC(12,2) NOT NULL,

    CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT fk_order_items_product FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE SET NULL,
    CONSTRAINT order_items_quantity_check CHECK (quantity > 0)
);

CREATE INDEX IF NOT EXISTS order_items_order_idx ON order_items (order_id);
