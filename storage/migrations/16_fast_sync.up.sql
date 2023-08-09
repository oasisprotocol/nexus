-- Add a column to record whether a block was analyzed in fast-sync mode or not.

BEGIN;

ALTER TABLE analysis.processed_blocks
    ADD COLUMN is_fast_sync BOOL NOT NULL DEFAULT false;

COMMIT;
