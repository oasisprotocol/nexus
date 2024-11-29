BEGIN;

CREATE TABLE chain.runtime_events_related_accounts
(
    runtime runtime NOT NULL,
    round UINT63 NOT NULL,
    event_index UINT31 NOT NULL,
    PRIMARY KEY (runtime, round, event_index),

    related_account TEXT NOT NULL
);

-- Used for fetching all events related to an account (sorted by round).
CREATE INDEX ix_runtime_events_related_accounts_related_account_round ON chain.runtime_events_related_accounts(runtime, related_account, round);

DROP INDEX IF EXISTS chain.ix_runtime_events_related_accounts;

-- TODO: we need a more high-level ("go") migration for this. Basically do a re-index but just of the runtime_events table.
-- This is not something our migration framework currently supports.
-- If we plan to avoid the need for many reindexing in the future, we should consider implementing support for this.
-- Alternatively, we plan on doing more reindexing in future, then we can just wait with this change until we do the next reindexing.

ALTER TABLE chain.runtime_events DROP COLUMN related_accounts;


COMMIT;
