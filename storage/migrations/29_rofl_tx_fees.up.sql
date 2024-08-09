BEGIN;

ALTER TABLE chain.runtime_transactions
    ADD COLUMN fee_proxy_module TEXT;

ALTER TABLE chain.runtime_transactions
    ADD COLUMN fee_proxy_id BYTEA;

COMMIT;
