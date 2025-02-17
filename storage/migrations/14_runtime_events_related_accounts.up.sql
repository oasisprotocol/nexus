BEGIN;

-- It is faster to create a new table and copy the data over, than to do the updates
-- in the existing table.
CREATE TABLE chain.runtime_events_new
(
    runtime runtime NOT NULL,
    round UINT63 NOT NULL,
    tx_index UINT31,
    FOREIGN KEY (runtime, round, tx_index) REFERENCES chain.runtime_transactions(runtime, round, tx_index) DEFERRABLE INITIALLY DEFERRED,

    -- Unique event index within the block. We need this so that we can uniquely reference events.
    event_index UINT31 NOT NULL, -- Added.
    PRIMARY KEY (runtime, round, event_index), -- Added.

    tx_hash HEX64,
    tx_eth_hash HEX64,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,


    type TEXT NOT NULL,
    -- The raw event, as returned by the oasis-sdk runtime client.
    body JSONB NOT NULL,

    -- The events of type `evm.log` are further parsed into known event types, e.g. (ERC20) Transfer,
    -- to populate the `evm_log_name`, `evm_log_params`, and `evm_log_signature` fields.
    -- These fields are populated only when the ABI for the event is known. Typically, this is the
    -- case for events emitted by verified contracts.
    evm_log_name TEXT,
    evm_log_params JSONB,
    evm_log_signature BYTEA CHECK (octet_length(evm_log_signature) = 32),

    -- Internal tracking for parsing evm.Call events using the contract
    -- abi when available.
    abi_parsed_at TIMESTAMP WITH TIME ZONE

    -- related_accounts oasis_addr[] -- Removed.
);

CREATE TABLE chain.runtime_events_related_accounts
(
    runtime runtime NOT NULL,
    event_index UINT31 NOT NULL,
    round UINT63 NOT NULL,
    FOREIGN KEY (runtime, round, event_index) REFERENCES chain.runtime_events_new(runtime, round, event_index) DEFERRABLE INITIALLY DEFERRED,

    account_address oasis_addr NOT NULL,
    PRIMARY KEY (runtime, round, event_index, account_address),

    tx_index UINT31,
    type TEXT NOT NULL
);

-- Copy the data from the old table to the new ones.
DO $$
DECLARE
    batch_size INT := 50000;
    current_runtime public.runtime;
    min_round UINT63;
    max_round UINT63;
    start_round UINT63;
    end_round UINT63;
    total_inserted INT := 0;
    runtime_list public.runtime[] := ARRAY['emerald', 'sapphire', 'cipher', 'pontusx_dev', 'pontusx_test'];
BEGIN
    -- Loop over each runtime.
    FOREACH current_runtime IN ARRAY runtime_list
    LOOP
        -- Get the minimum and maximum round numbers for the current runtime.
        SELECT MIN(round), MAX(round)
          INTO min_round, max_round
          FROM chain.runtime_events
         WHERE runtime = current_runtime;

        IF min_round IS NULL THEN
            RAISE NOTICE 'No events found for runtime %', current_runtime;
            CONTINUE;
        END IF;

        -- Process rounds in batches.
        start_round := min_round;
        WHILE start_round <= max_round LOOP
            end_round := start_round + batch_size - 1;

            DROP TABLE IF EXISTS pg_temp.event_data;

            CREATE TEMP TABLE pg_temp.event_data AS
            SELECT
                runtime,
                round,
                tx_index,
                row_number() OVER (PARTITION BY runtime, round ORDER BY tx_index) - 1 AS event_index,
                tx_hash,
                tx_eth_hash,
                timestamp,
                type,
                body,
                evm_log_name,
                evm_log_params,
                evm_log_signature,
                abi_parsed_at,
                related_accounts
            FROM chain.runtime_events
            WHERE runtime = current_runtime
              AND round BETWEEN start_round AND end_round
            ORDER BY runtime, round;

            GET DIAGNOSTICS total_inserted = ROW_COUNT;
            RAISE NOTICE 'Processing rounds for runtime %: % - %, Selected % rows', current_runtime, start_round, end_round, total_inserted;

            -- Insert into chain.runtime_events_new.
            INSERT INTO chain.runtime_events_new (
                runtime, round, tx_index, event_index, tx_hash, tx_eth_hash, timestamp,
                type, body, evm_log_name, evm_log_params, evm_log_signature, abi_parsed_at
            )
            SELECT
                runtime, round, tx_index, event_index, tx_hash, tx_eth_hash, timestamp,
                type, body, evm_log_name, evm_log_params, evm_log_signature, abi_parsed_at
            FROM pg_temp.event_data
            ON CONFLICT DO NOTHING;

            GET DIAGNOSTICS total_inserted = ROW_COUNT;
            RAISE NOTICE 'Inserted % rows into chain.runtime_events_new', total_inserted;

            -- Insert into chain.runtime_events_related_accounts.
            INSERT INTO chain.runtime_events_related_accounts (
                runtime, round, event_index, tx_index, type, account_address
            )
            SELECT
                runtime, round, event_index, tx_index, type, unnest(related_accounts)
            FROM pg_temp.event_data
            WHERE related_accounts IS NOT NULL
            ON CONFLICT DO NOTHING;

            GET DIAGNOSTICS total_inserted = ROW_COUNT;
            RAISE NOTICE 'Inserted % rows into chain.runtime_events_related_accounts', total_inserted;

            start_round := end_round + 1;
        END LOOP;
    END LOOP;

    RAISE NOTICE 'Batch insert into both tables completed!';
END $$;

-- Clean up.
ALTER TABLE chain.runtime_events RENAME TO runtime_events_old;
ALTER TABLE chain.runtime_events_new RENAME TO runtime_events;

-- We commit here, to ensure index creation below works. Otherwise, we get:
-- pq: cannot CREATE INDEX "runtime_events" because it has pending trigger events
COMMIT;

BEGIN;

DROP TABLE chain.runtime_events_old;

-- Re-create all indexes on the new runtime_events table.
-- CREATE INDEX ix_runtime_events_round ON chain.runtime_events(runtime, round);  -- Not needed - prefix of primary key.
CREATE INDEX ix_runtime_events_tx_hash ON chain.runtime_events USING hash (tx_hash);
CREATE INDEX ix_runtime_events_tx_eth_hash ON chain.runtime_events USING hash (tx_eth_hash);
CREATE INDEX ix_runtime_events_evm_log_signature_round ON chain.runtime_events(runtime, evm_log_signature, round, tx_index); -- for fetching a certain event type, eg Transfers
CREATE INDEX ix_runtime_events_evm_log_params ON chain.runtime_events USING gin(evm_log_params);
CREATE INDEX ix_runtime_events_type_round ON chain.runtime_events (runtime, type, round, tx_index);
CREATE INDEX ix_runtime_events_nft_transfers ON chain.runtime_events (runtime, (body ->> 'address'), (body -> 'topics' ->> 3), round, tx_index)
    WHERE
        type = 'evm.log' AND
        evm_log_signature = '\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef' AND
        jsonb_array_length(body -> 'topics') = 4;
-- Index used for fetching events emitted by a specific contract.
CREATE INDEX ix_runtime_events_evm_contract_events ON chain.runtime_events (runtime, (body ->> 'address'), evm_log_signature, round, tx_index)
    WHERE
        type = 'evm.log';

-- Used for fetching all events related to an account (sorted by round).
CREATE INDEX IF NOT EXISTS ix_runtime_events_related_accounts_account_address_round ON chain.runtime_events_related_accounts(runtime, account_address, round, tx_index);

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
