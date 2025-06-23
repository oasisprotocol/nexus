BEGIN;

CREATE INDEX IF NOT EXISTS ix_roflmarket_instances_deployed_app_id ON chain.roflmarket_instances (runtime, (deployment->>'app_id')) WHERE deployment IS NOT NULL AND deployment->>'app_id' IS NOT NULL;

COMMIT;
