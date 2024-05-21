BEGIN;

DROP INDEX IF EXISTS chain.ix_accounts_related_transactions_address;

CREATE INDEX ix_accounts_related_transactions_address_block ON chain.accounts_related_transactions(account_address, tx_block);

COMMIT;
