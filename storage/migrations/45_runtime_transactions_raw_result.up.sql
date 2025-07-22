BEGIN;

ALTER TABLE chain.runtime_transactions ADD COLUMN raw_result BYTEA;

COMMIT;
