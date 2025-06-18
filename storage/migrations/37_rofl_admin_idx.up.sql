BEGIN;

CREATE INDEX IF NOT EXISTS ix_rofl_apps_admin ON chain.rofl_apps (runtime, admin);

COMMIT;
