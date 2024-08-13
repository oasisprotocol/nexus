BEGIN;

UPDATE chain.runtime_transactions SET fee_symbol = '' WHERE fee_symbol IS NULL; -- Will be populated on reindex.

ALTER TABLE chain.runtime_transactions ALTER COLUMN fee_symbol SET NOT NULL;

ALTER TABLE chain.runtime_transactions ALTER COLUMN fee_symbol SET DEFAULT '';

COMMIT;
