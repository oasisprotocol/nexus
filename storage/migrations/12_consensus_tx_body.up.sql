BEGIN;

ALTER TABLE chain.transactions
    DROP COLUMN body;

ALTER TABLE chain.transactions
    ADD COLUMN body JSONB;

COMMIT;
