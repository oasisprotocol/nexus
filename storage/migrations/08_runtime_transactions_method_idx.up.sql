BEGIN;

CREATE INDEX IF NOT EXISTS ix_runtime_transactions_method_round ON chain.runtime_transactions (runtime, method, round, tx_index);

COMMIT;
