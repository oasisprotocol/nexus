BEGIN;

DROP INDEX IF EXISTS chain.ix_runtime_transfers_sender;
DROP INDEX IF EXISTS chain.ix_runtime_transfers_receiver;

ALTER TABLE chain.runtime_accounts
    ADD COLUMN total_sent UINT_NUMERIC NOT NULL DEFAULT 0,
    ADD COLUMN total_received UINT_NUMERIC NOT NULL DEFAULT 0;

COMMIT;
