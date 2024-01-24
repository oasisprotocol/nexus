-- Removed in https://github.com/oasisprotocol/nexus/pull/611
-- From there on statecheck manually handles the snapshots schemas.

-- BEGIN;

-- -- Schema for creating snapshot copies of the key tables.
-- CREATE SCHEMA IF NOT EXISTS snapshot;
-- GRANT USAGE ON SCHEMA snapshot TO PUBLIC;

-- -- Heights at which the key tables have been "snapshotted" (i.e. copied from
-- -- table chain.X to snapshot.X) as part of `tests/statecheck`.
-- CREATE TABLE snapshot.snapshotted_heights
-- (
--   analyzer TEXT NOT NULL,
--   height BIGINT PRIMARY KEY,
--   snapshot_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
-- );

-- COMMIT;
