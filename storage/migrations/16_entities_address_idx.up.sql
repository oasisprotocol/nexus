BEGIN;

CREATE INDEX IF NOT EXISTS ix_entities_address ON chain.entities USING hash (address);

COMMIT;
