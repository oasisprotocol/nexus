BEGIN;

-- Tables in this namespace are intended only for use during fast-sync.
--
-- They contain append-only data that would normally (during slow-sync) be
-- inserted into existing tables, but doing so during fast-sync would
-- cause too much DB lock contention.
--
-- NOTE: The data in these tables is NOT COMPLETE in that it is never guaranteed
--       to cover the full set of heights indexed so far. It should be used to update
--       existing tables at the end of fast sync, not to overwrite them.
CREATE SCHEMA IF NOT EXISTS todo_updates;

CREATE TABLE todo_updates.epochs(epoch UINT63, height UINT63);

COMMIT;
