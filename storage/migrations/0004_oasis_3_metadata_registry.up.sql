-- Cache registry of signed statements about various entities that may be on the Oasis Network
-- https://github.com/oasisprotocol/metadata-registry

BEGIN;

-- Metadata Registry

ALTER TABLE oasis_3.entities ADD meta JSON;

COMMIT;
