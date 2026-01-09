-- Add first_activity column to runtime_accounts.
ALTER TABLE chain.runtime_accounts ADD COLUMN first_activity TIMESTAMP WITH TIME ZONE;

-- State table for tracking backfill progress.
-- Stores the cursor position (last processed runtime, address).
CREATE TABLE analysis.runtime_accounts_first_activity_backfill_state (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1), -- singleton row
    last_runtime runtime,
    last_address oasis_addr
);

-- Initialize with NULL cursor (start from beginning).
INSERT INTO analysis.runtime_accounts_first_activity_backfill_state (id) VALUES (1);

-- Re-apply grants for new table.
GRANT SELECT ON ALL TABLES IN SCHEMA analysis TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA analysis TO PUBLIC;
