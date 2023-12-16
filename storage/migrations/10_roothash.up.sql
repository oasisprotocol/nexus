BEGIN;

ALTER TABLE chain.events
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
    ADD COLUMN roothash_runtime_id HEX64,
    -- This is a nexus runtime name, for example `sapphire`. This is only set
    -- for supported runtimes.
    ADD COLUMN related_runtime runtime,
    ADD COLUMN related_runtime_round UINT63;

-- ix_events_roothash is the link between runtime blocks and consensus blocks.
-- Given a runtime block (runtime, round), you can look up the roothash events
-- with this index and find the events when the block was proposed (first
-- executor commit), committed to, and finalized.
CREATE INDEX ix_events_roothash
    ON chain.events (related_runtime, related_runtime_round)
    WHERE
        related_runtime IS NOT NULL AND
        related_runtime_round IS NOT NULL;

COMMIT;
