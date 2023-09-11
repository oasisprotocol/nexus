BEGIN;

ALTER TABLE chain.evm_contracts
    ADD COLUMN gas_used UINT63 NOT NULL DEFAULT 0; -- moved to chain.runtime_accounts.gas_for_calling in 18_refactor_dead_reckoning.up.sql

-- Used in contract upserts.
CREATE INDEX ix_runtime_transactions_to ON chain.runtime_transactions(runtime, "to");

-- Backfill the table using chain.runtime_transactions
WITH txs AS (
    SELECT runtime, "to", SUM(gas_used) as total_gas
        FROM chain.runtime_transactions
        GROUP BY runtime, "to"
)
UPDATE chain.evm_contracts as contracts
    SET gas_used = txs.total_gas
    FROM txs
    WHERE contracts.runtime = txs.runtime AND
        contracts.contract_address = txs.to;

COMMIT;
