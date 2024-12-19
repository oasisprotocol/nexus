BEGIN;

CREATE INDEX IF NOT EXISTS ix_blocks_proposer_entity_id_height ON chain.blocks (proposer_entity_id, height);

DROP INDEX IF EXISTS chain.ix_blocks_proposer_entity_id;

COMMIT;
