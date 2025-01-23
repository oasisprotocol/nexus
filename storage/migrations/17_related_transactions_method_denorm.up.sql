BEGIN;

-- Update consensus account related transactions to include the method field.
ALTER TABLE chain.accounts_related_transactions
	ADD COLUMN method TEXT;

DO $$
DECLARE
    block_batch_size INT := 50000;
    start_block INT;
    end_block   INT;
    max_block   INT;
    affected_rows INT;
BEGIN
    -- Determine the block boundaries from the table to be updated.
    SELECT MIN(tx_block), MAX(tx_block)
      INTO start_block, max_block
      FROM chain.accounts_related_transactions;

    IF start_block IS NULL THEN
        RAISE NOTICE 'No records found in chain.accounts_related_transactions. Exiting.';
        RETURN;
    END IF;

    -- Process blocks in batches.
    WHILE start_block <= max_block LOOP
        end_block := start_block + block_batch_size - 1;

        UPDATE chain.accounts_related_transactions AS art
          SET method = t.method
         FROM chain.transactions AS t
         WHERE art.tx_block = t.block
           AND art.tx_index = t.tx_index
           AND art.tx_block BETWEEN start_block AND end_block;

        GET DIAGNOSTICS affected_rows = ROW_COUNT;
        RAISE NOTICE 'Updated % rows for blocks % to %', affected_rows, start_block, end_block;

        -- Move to the next batch.
        start_block := end_block + 1;
    END LOOP;

    RAISE NOTICE 'Batch update completed.';
END $$;

-- We commit here, to ensure index creation below works.
COMMIT;

BEGIN;

ALTER TABLE chain.accounts_related_transactions ALTER COLUMN method SET NOT NULL;

CREATE INDEX ix_accounts_related_transactions_address_method_block_tx_index ON chain.accounts_related_transactions (account_address, method, tx_block DESC, tx_index);

COMMIT;
