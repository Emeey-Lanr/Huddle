-- The Bachs charge behind a payment, from the payment webhooks. Refunds are
-- made against it.
ALTER TABLE payments ADD COLUMN IF NOT EXISTS charge_id VARCHAR(100);

-- refunded: the payment's money went back to the shopper.
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('open', 'paid', 'expired', 'needs_refund', 'review', 'refunded'));

CREATE INDEX IF NOT EXISTS payments_store_status_idx ON payments (store_id, status);

-- One row per refund attempt. Bachs takes one refund per charge, full or
-- partial; a failed refund does not count, so the seller can try again.
CREATE TABLE IF NOT EXISTS refunds (
    id UUID PRIMARY KEY,
    public_id VARCHAR(50) NOT NULL UNIQUE,
    payment_id UUID NOT NULL,
    store_id UUID NOT NULL,
    bachs_refund_id VARCHAR(100) UNIQUE,
    amount NUMERIC(12,2) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    reason VARCHAR(500) NOT NULL DEFAULT '',
    -- When the refund is paid, cancel the order this payment paid (unless
    -- the seller is refunding part of an order they still deliver).
    cancel_order BOOLEAN NOT NULL DEFAULT TRUE,
    status VARCHAR(20) NOT NULL DEFAULT 'processing',
    failure_reason VARCHAR(500) NOT NULL DEFAULT '',
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_refunds_payment FOREIGN KEY (payment_id) REFERENCES payments(id) ON DELETE CASCADE,
    CONSTRAINT fk_refunds_store FOREIGN KEY (store_id) REFERENCES stores(id) ON DELETE CASCADE,
    CONSTRAINT refunds_status_check CHECK (status IN ('processing', 'paid', 'failed'))
);

CREATE INDEX IF NOT EXISTS refunds_payment_idx ON refunds (payment_id);
-- At most one refund in progress or done per payment.
CREATE UNIQUE INDEX IF NOT EXISTS refunds_one_active_per_payment ON refunds (payment_id) WHERE status IN ('processing', 'paid');
