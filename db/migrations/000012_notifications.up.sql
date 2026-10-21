-- Emails to send, queued in the same transaction as the change they report,
-- so a paid order is never missed and a rolled-back one never emailed. A
-- worker sends due rows and retries failures with a growing delay.
CREATE TABLE IF NOT EXISTS notifications (
    id UUID PRIMARY KEY,
    kind VARCHAR(30) NOT NULL,
    order_id UUID NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    last_error VARCHAR(500) NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_notifications_order FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
    CONSTRAINT notifications_status_check CHECK (status IN ('pending', 'sent', 'failed'))
);

-- One email of each kind per order.
CREATE UNIQUE INDEX IF NOT EXISTS notifications_kind_order_unique ON notifications (kind, order_id);
CREATE INDEX IF NOT EXISTS notifications_due_idx ON notifications (next_attempt_at) WHERE status = 'pending';
