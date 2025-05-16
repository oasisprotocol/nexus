BEGIN;

ALTER TABLE chain.rofl_apps ADD COLUMN metadata_name TEXT;

-- Populate name from metadata JSON (it's optionally present in the metadata key-value pairs).
UPDATE chain.rofl_apps
  SET metadata_name = metadata->>'net.oasis.rofl.name'
  WHERE metadata ? 'net.oasis.rofl.name';

-- Create index on the name.
CREATE INDEX ix_rofl_apps_metadata_name ON chain.rofl_apps (runtime, metadata_name);

COMMIT;
