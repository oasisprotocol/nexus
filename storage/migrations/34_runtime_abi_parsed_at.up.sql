BEGIN;

UPDATE chain.runtime_transactions
	SET abi_parsed_at = '0001-01-01'
	WHERE abi_parsed_at IS NULL;

ALTER TABLE chain.runtime_transactions
	ALTER COLUMN abi_parsed_at SET DEFAULT '0001-01-01',
	ALTER COLUMN abi_parsed_at SET NOT NULL;
DROP INDEX IF EXISTS chain.ix_runtime_transactions_to_abi_parsed_at;
CREATE INDEX IF NOT EXISTS ix_runtime_transactions_to_round_abi_parsed_at ON chain.runtime_transactions (runtime, "to", round DESC, abi_parsed_at) WHERE method = 'evm.Call' AND body IS NOT NULL;

UPDATE chain.runtime_events
	SET abi_parsed_at = '0001-01-01'
	WHERE abi_parsed_at IS NULL;
ALTER TABLE chain.runtime_events
	ALTER COLUMN abi_parsed_at SET DEFAULT '0001-01-01',
	ALTER COLUMN abi_parsed_at SET NOT NULL;
CREATE INDEX IF NOT EXISTS ix_runtime_events_abi_parsed_at ON chain.runtime_events (runtime, (body ->> 'address'), round DESC, abi_parsed_at) WHERE type = 'evm.log';

COMMIT;
