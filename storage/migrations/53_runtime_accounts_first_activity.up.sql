-- Add first_activity column to runtime_accounts.
-- Backfill is handled by the first_activity_backfill analyzer.
ALTER TABLE chain.runtime_accounts ADD COLUMN first_activity TIMESTAMP WITH TIME ZONE;
ALTER TABLE chain.runtime_accounts ADD COLUMN first_activity_backfilled BOOLEAN NOT NULL DEFAULT false;
