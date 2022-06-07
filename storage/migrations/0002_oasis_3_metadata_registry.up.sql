-- Cache registry of signed statements about various entities that may be on the Oasis Network
-- https://github.com/oasisprotocol/metadata-registry

BEGIN;

-- Metadata Registry

CREATE TABLE IF NOT EXISTS oasis_3.metadata
(
  id TEXT PRIMARY KEY,
  meta JSON
);

COMMIT;
