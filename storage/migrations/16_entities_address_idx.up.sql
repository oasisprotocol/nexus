BEGIN;

-- This would ideally not be needed, and address preimages table would be used instead,
-- but it looks like the address preimages table was not populated for consensus data.
-- https://github.com/oasisprotocol/nexus/issues/907
CREATE INDEX IF NOT EXISTS ix_entities_address ON chain.entities USING hash (address);

COMMIT;
