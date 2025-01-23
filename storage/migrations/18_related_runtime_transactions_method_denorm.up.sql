BEGIN;

-- Add 'likely native transfer' field to runtime transactions.
ALTER TABLE chain.runtime_transactions
	ADD COLUMN likely_native_transfer BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE chain.runtime_transactions AS rt
SET likely_native_transfer = TRUE
WHERE rt.method = 'accounts.Transfer'
   OR (rt.method = 'evm.Call' AND (rt.body ->> 'data') = '');

-- Drop unneeded indexes which were previously used to filter for 'likely native transfer'.
DROP INDEX chain.ix_runtime_transactions_evm_call_empty_data;
DROP INDEX chain.ix_runtime_transactions_evm_call_non_empty_data;

-- Create indexes for native transfer and method fields.
CREATE INDEX ix_runtime_transactions_native_transfer_round ON chain.runtime_transactions (runtime, likely_native_transfer, round, tx_index);
-- This combination is needed, since explorer needs to obtain evm.Call's which are not native transfers.
CREATE INDEX ix_runtime_transactions_method_native_transfer_round ON chain.runtime_transactions (runtime, method, likely_native_transfer, round, tx_index);

ALTER TABLE chain.runtime_related_transactions
	ADD COLUMN method TEXT,
	ADD COLUMN likely_native_transfer BOOLEAN NOT NULL DEFAULT FALSE;

-- Update runtime related transactions table to include the method and likely_native_transfer fields.
-- We have this denormalized data since efficient filtering by method for a specific account is important.
DO $$
DECLARE
    runtime_list public.runtime[] := ARRAY['emerald', 'sapphire', 'cipher', 'pontusx_dev', 'pontusx_test'];
    current_runtime public.runtime;
    batch_size INT := 50000;
    min_round INT;
    max_round INT;
    start_round INT;
    end_round INT;
    affected_rows INT;
BEGIN
    -- Loop over each runtime in the list.
    FOREACH current_runtime IN ARRAY runtime_list
    LOOP
        -- Get the minimum and maximum round numbers for the current runtime.
        SELECT MIN(tx_round), MAX(tx_round)
          INTO min_round, max_round
          FROM chain.runtime_related_transactions
         WHERE runtime = current_runtime;

        IF min_round IS NULL THEN
            RAISE NOTICE 'No transactions found for runtime %', current_runtime;
            CONTINUE;
        END IF;

        start_round := min_round;

        -- Process rounds in batches.
        WHILE start_round <= max_round LOOP
            end_round := start_round + batch_size - 1;

            UPDATE chain.runtime_related_transactions AS rrt
            SET
                method = rt.method,
                likely_native_transfer = rt.likely_native_transfer
            FROM chain.runtime_transactions AS rt
            WHERE rrt.runtime = current_runtime
              AND rt.runtime = current_runtime
              AND rrt.tx_round = rt.round
              AND rrt.tx_index = rt.tx_index
              AND rrt.tx_round BETWEEN start_round AND end_round;

            GET DIAGNOSTICS affected_rows = ROW_COUNT;
            RAISE NOTICE 'Runtime %: Updated % rows for rounds % to %', current_runtime, affected_rows, start_round, end_round;

            start_round := end_round + 1;
        END LOOP;
    END LOOP;

    RAISE NOTICE 'Batch update completed for all runtimes.';
END $$;

-- We commit here, to ensure index creation below works.
COMMIT;

BEGIN;

CREATE INDEX ix_runtime_related_transactions_address_method_round ON chain.runtime_related_transactions (runtime, account_address, method, tx_round, tx_index);
CREATE INDEX ix_runtime_related_transactions_address_native_transfer_round ON chain.runtime_related_transactions (runtime, account_address, likely_native_transfer, tx_round, tx_index);
-- This combination is needed, since explorer needs to obtain evm.Call's which are not native transfers.
CREATE INDEX ix_runtime_related_transactions_address_method_native_transfer_round ON chain.runtime_related_transactions (runtime, account_address, method, likely_native_transfer, tx_round, tx_index);

COMMIT;
