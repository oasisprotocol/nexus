BEGIN;

ALTER TABLE history.escrow_events
    ALTER COLUMN delegator DROP NOT NULL;

COMMIT;
