BEGIN;

CREATE EXTENSION IF NOT EXISTS pg_trgm;

DROP INDEX chain.ix_rofl_apps_metadata_name;
CREATE INDEX ix_rofl_apps_metadata_name ON chain.rofl_apps USING GIN (metadata_name gin_trgm_ops);

COMMIT;
