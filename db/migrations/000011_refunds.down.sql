DROP TABLE IF EXISTS refunds;
DROP INDEX IF EXISTS payments_store_status_idx;
UPDATE payments SET status = 'paid' WHERE status = 'refunded';
ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_status_check;
ALTER TABLE payments ADD CONSTRAINT payments_status_check
    CHECK (status IN ('open', 'paid', 'expired', 'needs_refund', 'review'));
ALTER TABLE payments DROP COLUMN IF EXISTS charge_id;
