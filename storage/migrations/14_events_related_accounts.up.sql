BEGIN;

ALTER TABLE chain.events ADD COLUMN event_index UINT31;

-- Disable triggers for faster updates.
ALTER TABLE chain.events DISABLE TRIGGER ALL;

-- Populate the event_index column with sequential values for each tx_block to ensure uniqueness.
DO $$
DECLARE
  cur_block UINT63;
  cur_type TEXT;
BEGIN
  -- Iterate over each unique (tx_block, type) pair.
  FOR cur_block, cur_type IN
    SELECT tx_block, type
    FROM chain.events
    GROUP BY tx_block, type
  LOOP
    -- Assign event_index within the group.
    WITH ranked_events AS (
      SELECT ctid,
             row_number() OVER (ORDER BY timestamp ASC) - 1 AS event_idx
      FROM chain.events
      WHERE tx_block = cur_block AND type = cur_type
    )
    UPDATE chain.events e
    SET event_index = ranked_events.event_idx
    FROM ranked_events
    WHERE e.ctid = ranked_events.ctid;
  END LOOP;
END $$;

-- Re-enable triggers.
ALTER TABLE chain.events ENABLE TRIGGER ALL;

ALTER TABLE chain.events
    ALTER COLUMN event_index SET NOT NULL;

-- Primary key for runtime events.
ALTER TABLE chain.events
    ADD CONSTRAINT pk_events PRIMARY KEY (tx_block, type, event_index);

-- Create events related accounts table.
CREATE TABLE chain.events_related_accounts
(
	tx_block UINT63 NOT NULL,
	type    TEXT NOT NULL,
 	event_index UINT31 NOT NULL,

	tx_index  UINT31,
	account_address oasis_addr NOT NULL,

    FOREIGN KEY (tx_block, type, event_index) REFERENCES chain.events(tx_block, type, event_index) DEFERRABLE INITIALLY DEFERRED
);

-- Disable logging for faster insertion.
ALTER TABLE chain.events_related_accounts SET UNLOGGED;

-- Populate the events_related_accounts table.
COPY chain.events_related_accounts (tx_block, type, event_index, tx_index, account_address)
FROM PROGRAM '
    psql -XtA -c "
    SELECT tx_block, type, event_index, tx_index, unnest(related_accounts) AS account_address
    FROM chain.events
    WHERE related_accounts IS NOT NULL
    AND array_length(related_accounts, 1) > 0
    "'
WITH (FORMAT csv);

-- Restore logging.
ALTER TABLE chain.events_related_accounts SET LOGGED;

-- Used for fetching all events related to an account (sorted by round).
CREATE INDEX ix_events_related_accounts_account_address_block ON chain.events_related_accounts(account_address, tx_block DESC, tx_index); -- TODO: maybe also event index?

-- Clean up.
DROP INDEX chain.ix_events_related_accounts;
ALTER TABLE chain.events DROP COLUMN related_accounts;

-- Grant others read-only use.
-- (We granted already in 00_consensus.up.sql, but the grant does not apply to new tables.)
GRANT SELECT ON ALL TABLES IN SCHEMA chain TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA chain TO PUBLIC;

COMMIT;
