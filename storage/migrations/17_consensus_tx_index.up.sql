BEGIN;

CREATE INDEX ix_transactions_method_height ON chain.transactions (method, block);

COMMIT;
