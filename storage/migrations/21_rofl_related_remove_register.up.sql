BEGIN;

-- We don't track rofl.Register transactions as related to any ROFL address anymore.
-- Clean up such entries.
DELETE FROM chain.rofl_related_transactions
  WHERE method = 'rofl.Register';

-- Due to a bug there could be duplicates in the table.
DELETE FROM chain.rofl_instance_transactions a
	USING chain.rofl_instance_transactions b
	WHERE
		a.ctid > b.ctid AND
		a.runtime = b.runtime AND
		a.app_id = b.app_id AND
		a.rak = b.rak AND
		a.tx_round = b.tx_round AND
		a.tx_index = b.tx_index;

-- Add the primary key
ALTER TABLE chain.rofl_instance_transactions
	ADD CONSTRAINT rofl_instance_transactions_pk
	PRIMARY KEY (runtime, app_id, rak, tx_round, tx_index);

COMMIT;
