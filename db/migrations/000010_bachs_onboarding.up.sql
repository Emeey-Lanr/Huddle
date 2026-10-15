-- A Bachs sub-account belongs to one store.
CREATE UNIQUE INDEX IF NOT EXISTS stores_bachs_account_unique ON stores (bachs_account_id) WHERE bachs_account_id IS NOT NULL;

-- Last known status of each seller account's Bachs capabilities, from the
-- capability.updated webhook and from reads of the account. changed_at keeps
-- an older event that arrives late from overwriting a newer status.
CREATE TABLE IF NOT EXISTS bachs_capabilities (
    account_id VARCHAR(64) NOT NULL,
    name VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (account_id, name)
);
