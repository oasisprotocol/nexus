BEGIN;

-- Add 'likely native transfer' field to runtime transactions.
ALTER TABLE chain.runtime_transactions
	ADD COLUMN likely_native_transfer BOOLEAN NOT NULL DEFAULT FALSE;
UPDATE chain.runtime_transactions AS rt
	SET likely_native_transfer = (rt.method = 'accounts.transfer' OR (rt.method = 'evm.Call' AND (rt.body ->> 'data') = ''));

-- Drop unneeded indexes which were previously used to filter for 'likely native transfer'.
DROP INDEX chain.ix_runtime_transactions_evm_call_empty_data;
DROP INDEX chain.ix_runtime_transactions_evm_call_non_empty_data;

-- Create indexes for native transfer and method fields.
CREATE INDEX ix_runtime_transactions_native_transfer_round ON chain.runtime_transactions (runtime, likely_native_transfer, round, tx_index);
-- This combination is needed, since explorer needs to obtain evm.Call's which are not native transfers.
CREATE INDEX ix_runtime_transactions_method_native_transfer_round ON chain.runtime_transactions (runtime, method, likely_native_transfer, round, tx_index);

-- Update runtime related transactions table to include the method and likely_native_transfer fields.
-- We have this denormalized data since efficient filtering by method for a specific account is important.
ALTER TABLE chain.runtime_related_transactions
	ADD COLUMN method TEXT,
	ADD COLUMN likely_native_transfer BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE chain.runtime_related_transactions AS rrt
	SET
		method = rt.method,
		likely_native_transfer = rt.likely_native_transfer
	FROM chain.runtime_transactions AS rt
	WHERE rrt.runtime = rt.runtime AND rrt.tx_round = rt.round AND rrt.tx_index = rt.tx_index;

CREATE INDEX ix_runtime_related_transactions_address_method_round ON chain.runtime_related_transactions (runtime, account_address, method, tx_round, tx_index);
CREATE INDEX ix_runtime_related_transactions_address_native_transfer_round ON chain.runtime_related_transactions (runtime, account_address, likely_native_transfer, tx_round, tx_index);
-- This combination is needed, since explorer needs to obtain evm.Call's which are not native transfers.
CREATE INDEX ix_runtime_related_transactions_address_method_native_transfer_round_evm_call ON chain.runtime_related_transactions (runtime, account_address, method, likely_native_transfer, tx_round, tx_index);

COMMIT;
