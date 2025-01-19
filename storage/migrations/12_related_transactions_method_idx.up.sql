BEGIN;

CREATE INDEX IF NOT EXISTS ix_transactions_method_block_tx_index ON chain.transactions (method, block DESC, tx_index);
DROP INDEX IF EXISTS chain.ix_transactions_method_height;

CREATE INDEX IF NOT EXISTS ix_accounts_related_transactions_address_block_desc_tx_index ON chain.accounts_related_transactions (account_address, tx_block DESC, tx_index);
DROP INDEX IF EXISTS chain.ix_accounts_related_transactions_address_block;

COMMIT;
