BEGIN;

CREATE TABLE chain.events_new
(
    tx_block UINT63 NOT NULL,
    tx_index  UINT31,
    FOREIGN KEY (tx_block, tx_index) REFERENCES chain.transactions(block, tx_index) DEFERRABLE INITIALLY DEFERRED,

    event_index UINT31 NOT NULL, -- Added.
    type    TEXT NOT NULL,  -- Enum with many values, see ConsensusEventType in api/spec/v1.yaml.
    PRIMARY KEY (tx_block, type, event_index), -- Added.

    body    JSONB,
    tx_hash   HEX64, -- could be fetched from `transactions` table; denormalized for efficiency
    -- related_accounts TEXT[], -- Removed.

    -- There's some mismatch between oasis-core's style in Go and nexus's
    -- style in SQL and JSON. oasis-core likes structures filled with nilable
    -- pointers, where one pointer is non-nil. nexus likes a type string plus
    -- a "body" blob. In roothash events though, the roothash/api Event
    -- structure additionally has a RuntimeID field, which nexus otherwise
    -- loses when it extracts the one non-nil event next to it. In the nexus,
    -- database, we're storing that runtime identifier in a column.
    --
    -- This is a runtime identifier, which is a binary "namespace," e.g.
    -- `000000000000000000000000000000000000000000000000f80306c9858e7279` for
    -- Sapphire on Mainnet. This is taken from the event from the node, and it
    -- is set even for runtimes that nexus isn't configured to analyze.
    roothash_runtime_id HEX64,
    roothash_runtime runtime,
    roothash_runtime_round UINT63
);

CREATE TABLE chain.events_related_accounts
(
    tx_block UINT63 NOT NULL,
    type    TEXT NOT NULL,
    event_index UINT31 NOT NULL,
    FOREIGN KEY (tx_block, type, event_index) REFERENCES chain.events_new(tx_block, type, event_index) DEFERRABLE INITIALLY DEFERRED,

    account_address oasis_addr NOT NULL,
    PRIMARY KEY (tx_block, type, event_index, account_address),

    tx_index  UINT31
);

-- Copy the data from the old table to the new ones.
DO $$
DECLARE
    batch_size INT := 50000;
    min_block UINT63;
    max_block UINT63;
    start_block UINT63;
    end_block UINT63;
    total_inserted INT := 0;
BEGIN
    SELECT MIN(tx_block), MAX(tx_block) INTO min_block, max_block FROM chain.events;

    IF min_block IS NULL THEN
          RAISE NOTICE 'No blocks found in chain.events';
          RETURN;
    END IF;

    -- Process blocks in batches.
    start_block := min_block;
    WHILE start_block <= max_block LOOP
        end_block := start_block + batch_size - 1;

        DROP TABLE IF EXISTS pg_temp.event_data;

        CREATE TEMP TABLE pg_temp.event_data AS
        SELECT
            tx_block,
            tx_index,
            row_number() OVER (PARTITION BY tx_block ORDER BY tx_index) - 1 AS event_index,
            type,
            body,
            tx_hash,
            related_accounts,
            roothash_runtime_id,
            roothash_runtime,
            roothash_runtime_round
        FROM chain.events
        WHERE tx_block BETWEEN start_block AND end_block
        ORDER BY tx_block;

        GET DIAGNOSTICS total_inserted = ROW_COUNT;
        RAISE NOTICE 'Processing blocks % - %, Selected % rows', start_block, end_block, total_inserted;

        -- Insert into chain.events_new.
        INSERT INTO chain.events_new (
            tx_block, tx_index, event_index, type, body, tx_hash,
            roothash_runtime_id, roothash_runtime, roothash_runtime_round
        )
        SELECT tx_block, tx_index, event_index, type, body, tx_hash,
                roothash_runtime_id, roothash_runtime, roothash_runtime_round
        FROM pg_temp.event_data
        ON CONFLICT DO NOTHING;

        GET DIAGNOSTICS total_inserted = ROW_COUNT;
        RAISE NOTICE 'Inserted % rows into chain.events_new', total_inserted;

        -- Insert into chain.events_related_accounts.
        INSERT INTO chain.events_related_accounts (
            tx_block, type, event_index, tx_index, account_address
        )
        SELECT
            tx_block, type, event_index, tx_index, unnest(related_accounts)
        FROM pg_temp.event_data
        WHERE related_accounts IS NOT NULL
        ON CONFLICT DO NOTHING;

        GET DIAGNOSTICS total_inserted = ROW_COUNT;
        RAISE NOTICE 'Inserted % rows into chain.events_related_accounts', total_inserted;

        start_block := end_block + 1;
    END LOOP;

    RAISE NOTICE 'Batch insert into both tables completed!';
END $$;

-- Clean up.
ALTER TABLE chain.events RENAME TO events_old;
ALTER TABLE chain.events_new RENAME TO events;

-- We commit here, to ensure index creation below works. Otherwise, we get:
-- pq: cannot CREATE INDEX "events" because it has pending trigger events
COMMIT;

BEGIN;

DROP TABLE chain.events_old;

-- Re-create all indexes on the new events table.
CREATE INDEX ix_events_tx_block ON chain.events (tx_block);
CREATE INDEX ix_events_tx_hash ON chain.events USING hash (tx_hash);
CREATE INDEX ix_events_type_block ON chain.events (type, tx_block DESC, tx_index);
-- ix_events_roothash is the link between runtime blocks and consensus blocks.
-- Given a runtime block (runtime, round), you can look up the roothash events
-- with this index and find the events when the block was proposed (first
-- executor commit), committed to, and finalized.
CREATE INDEX ix_events_roothash ON chain.events (roothash_runtime, roothash_runtime_round)
    WHERE
        roothash_runtime IS NOT NULL AND
        roothash_runtime_round IS NOT NULL;

-- Used for fetching all events related to an account (sorted by round).
CREATE INDEX ix_events_related_accounts_account_address_block ON chain.events_related_accounts(account_address, tx_block DESC, tx_index);

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
