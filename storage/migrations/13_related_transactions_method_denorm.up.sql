BEGIN;

-- Update consensus account related transactions to include the method field.
ALTER TABLE chain.accounts_related_transactions
	ADD COLUMN method TEXT;

UPDATE chain.accounts_related_transactions AS art
	SET method = t.method
	FROM chain.transactions AS t
	WHERE art.tx_block = t.block AND art.tx_index = t.tx_index;

CREATE INDEX ix_accounts_related_transactions_address_method_block_tx_index ON chain.accounts_related_transactions (account_address, method, tx_block DESC, tx_index);

ALTER TABLE chain.accounts_related_transactions ALTER COLUMN method SET NOT NULL;

COMMIT;
