BEGIN;

CREATE INDEX IF NOT EXISTS ix_transactions_method_block_tx_index ON chain.transactions (method, block DESC, tx_index);
DROP INDEX IF EXISTS chain.ix_transactions_method_height;

CREATE INDEX IF NOT EXISTS ix_accounts_related_transactions_address_block_desc_tx_index ON chain.accounts_related_transactions (account_address, tx_block DESC, tx_index);
DROP INDEX IF EXISTS chain.ix_accounts_related_transactions_address_block;

-- Indexes for efficient query of 'likely native transfers':
-- EVM Calls, where the body is an empty data field (likely native transfers)
CREATE INDEX IF NOT EXISTS ix_runtime_transactions_evm_call_empty_data ON chain.runtime_transactions (runtime, round, tx_index) WHERE method = 'evm.Call' AND (body ->> 'data') = '';
-- EVM Calls, where the body is non-empty data field (likely not native transfers).
CREATE INDEX IF NOT EXISTS ix_runtime_transactions_evm_call_non_empty_data ON chain.runtime_transactions (runtime, round, tx_index) WHERE method = 'evm.Call' AND (body ->> 'data') != '';

COMMIT;
