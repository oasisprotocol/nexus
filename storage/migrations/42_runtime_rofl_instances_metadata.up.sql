BEGIN;

ALTER TABLE chain.rofl_instances ADD COLUMN metadata JSONB;

-- Schedule for refresh all active rofl apps.
UPDATE chain.rofl_apps
	SET last_processed_round = NULL
	WHERE removed = FALSE;

COMMIT;
