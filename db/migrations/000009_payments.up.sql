-- The seller's Bachs sub-account. Payments for the store's orders are charged
-- to it directly, so the money lands in the seller's own Bachs balance.
ALTER TABLE stores ADD COLUMN IF NOT EXISTS bachs_account_id VARCHAR(64);

-- One row per Bachs checkout. An order can have several, e.g. when the
-- shopper's first checkout expired and they tried again.
CREATE TABLE IF NOT EXISTS payments (
    id UUID PRIMARY KEY,
    public_id VARCHAR(50) NOT NULL UNIQUE,
    order_id UUID NOT NULL,
    store_id UUID NOT NULL,
    provider VARCHAR(20) NOT NULL DEFAULT 'bachs',
    checkout_id VARCHAR(100) NOT NULL UNIQUE,
    checkout_url TEXT NOT NULL,
    account_id VARCHAR(64),
    status VARCHAR(20) NOT NULL DEFAULT 'open',
    amount NUMERIC(12,2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    paid_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_payments_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT fk_payments_store FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    -- needs_refund: money arrived for an order that could no longer be filled.
    -- review: the paid amount, currency or account did not match the order.
    CONSTRAINT payments_status_check CHECK (status IN ('open', 'paid', 'expired', 'needs_refund', 'review'))
);

CREATE INDEX IF NOT EXISTS payments_order_idx ON payments (order_id);

-- Webhook event ids already handled. Bachs delivers at least once.
CREATE TABLE IF NOT EXISTS webhook_events (
    event_id VARCHAR(100) PRIMARY KEY,
    provider VARCHAR(20) NOT NULL,
    type VARCHAR(100) NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
