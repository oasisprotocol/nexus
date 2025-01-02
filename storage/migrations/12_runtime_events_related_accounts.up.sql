BEGIN;

CREATE TABLE chain.runtime_events_related_accounts
(
    runtime runtime NOT NULL,
    round UINT63 NOT NULL,
    tx_index UINT31 NOT NULL,
    type TEXT NOT NULL,
    type_index UINT31 NOT NULL,

    account_address oasis_addr NOT NULL,
    FOREIGN KEY (runtime, round, tx_index, type, type_index) REFERENCES chain.runtime_events(runtime, round, tx_index, type, type_index) DEFERRABLE INITIALLY DEFERRED
);

-- Used for fetching all events related to an account (sorted by round).
CREATE INDEX ix_runtime_events_related_accounts_related_account_round ON chain.runtime_events_related_accounts(runtime, account_address, round);

DROP INDEX IF EXISTS chain.ix_runtime_events_related_accounts;

-- TODO: we need a more high-level ("go") migration for this. Basically do a re-index but just of the runtime_events table.
-- This is not something our migration framework currently supports.
-- If we plan to avoid the need for many reindexing in the future, we should consider implementing support for this.
-- Alternatively, we plan on doing more reindexing in future, then we can just wait with this change until we do the next reindexing.

ALTER TABLE chain.runtime_events DROP COLUMN related_accounts;


COMMIT;
