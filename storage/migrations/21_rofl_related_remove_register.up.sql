BEGIN;

-- We don't track rofl.Register transactions as related to any ROFL address anymore.
-- Clean up such entries.
DELETE FROM chain.rofl_related_transactions
  WHERE method = 'rofl.Register' or method = 'roflmarket.ProviderCreate' or method = 'roflmarket.ProviderUpdate' or method = 'roflmarket.InstanceCreate';

-- Due to a bug, there can be A LOT of duplicates in the chain.rofl_instance_transactions table.
-- Create a new empty table with the same structure, and mark instance to be re-analyzed.
TRUNCATE TABLE chain.rofl_instance_transactions;
ALTER TABLE chain.rofl_instance_transactions
	ADD CONSTRAINT rofl_instance_transactions_pk
	PRIMARY KEY (runtime, app_id, rak, tx_round, tx_index);

DROP INDEX IF EXISTS chain.ix_rofl_instance_transactions_app_id_rak_round_index;

-- Re-analyze the instance transactions.
UPDATE chain.rofl_instances
	SET last_processed_round = registration_round - 1;

COMMIT;
