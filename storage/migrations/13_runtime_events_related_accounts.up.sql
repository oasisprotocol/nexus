BEGIN;

-- Update chain.runtime_events with event_index.
ALTER TABLE chain.runtime_events ADD COLUMN event_index UINT31;

-- Disable triggers for faster updates.
ALTER TABLE chain.runtime_events DISABLE TRIGGER ALL;

-- Populate the event_index column with sequential values for each (runtime, round)
-- to ensure uniqueness.
DO $$
DECLARE
  cur_runtime runtime;
  cur_round UINT63;
BEGIN
  -- Iterate over each unique (runtime, round) pair.
  FOR cur_runtime, cur_round IN
    SELECT runtime, round
    FROM chain.runtime_events
    GROUP BY runtime, round
  LOOP
    -- Assign event_index within the group.
    WITH ranked_events AS (
      SELECT ctid,
             row_number() OVER (ORDER BY timestamp ASC) - 1 AS event_idx
      FROM chain.runtime_events
      WHERE runtime = cur_runtime AND round = cur_round
    )
    UPDATE chain.runtime_events e
    SET event_index = ranked_events.event_idx
    FROM ranked_events
    WHERE e.ctid = ranked_events.ctid;
  END LOOP;
END $$;

-- Re-enable triggers.
ALTER TABLE chain.runtime_events ENABLE TRIGGER ALL;

ALTER TABLE chain.runtime_events
    ALTER COLUMN event_index SET NOT NULL;

-- Primary key for runtime events.
ALTER TABLE chain.runtime_events
    ADD CONSTRAINT pk_runtime_events PRIMARY KEY (runtime, round, event_index);

-- Create and populate the runtime events related accounts table.
CREATE TABLE chain.runtime_events_related_accounts
(
    runtime runtime NOT NULL,
    round UINT63 NOT NULL,
    event_index UINT31 NOT NULL,

    tx_index UINT31,
    type TEXT NOT NULL,
    account_address oasis_addr NOT NULL,
    FOREIGN KEY (runtime, round, event_index) REFERENCES chain.runtime_events(runtime, round, event_index) DEFERRABLE INITIALLY DEFERRED
);

-- Disable logging for faster insertion.
ALTER TABLE chain.runtime_events_related_accounts SET UNLOGGED;

-- Populate the runtime_events_related_accounts table.
COPY chain.runtime_events_related_accounts (runtime, round, event_index, tx_index, type, account_address)
FROM PROGRAM '
    psql -XtA -c "
    SELECT runtime, round, event_index, tx_index, type, unnest(related_accounts) AS account_address
    FROM chain.runtime_events
    WHERE related_accounts IS NOT NULL
    AND array_length(related_accounts, 1) > 0
    "'
WITH (FORMAT csv);

-- Restore logging.
ALTER TABLE chain.runtime_events_related_accounts SET LOGGED;

-- Used for fetching all events related to an account (sorted by round).
CREATE INDEX ix_runtime_events_related_accounts_account_address_round ON chain.runtime_events_related_accounts(runtime, account_address, round, tx_index);

-- Clean up.
DROP INDEX chain.ix_runtime_events_related_accounts;
ALTER TABLE chain.runtime_events DROP COLUMN related_accounts;

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
