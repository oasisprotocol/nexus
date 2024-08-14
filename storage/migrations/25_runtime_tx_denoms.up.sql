BEGIN;

ALTER TABLE chain.runtime_transactions
    ADD COLUMN amount_symbol TEXT;

ALTER TABLE chain.runtime_transactions
    ADD COLUMN fee_symbol TEXT NOT NULL DEFAULT ''; -- Will be populated after reindex

COMMIT;
