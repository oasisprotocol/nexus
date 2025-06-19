BEGIN;

CREATE INDEX IF NOT EXISTS ix_roflmarket_instances_admin ON chain.roflmarket_instances (runtime, admin);

COMMIT;
