BEGIN;

ALTER TABLE chain.blocks
	DROP COLUMN type;

ALTER TABLE chain.blocks
	DROP COLUMN beacon;

ALTER TABLE chain.blocks
	RENAME COLUMN root_hash TO state_root;

ALTER TABLE chain.blocks
    ADD COLUMN gas_limit UINT_NUMERIC NOT NULL DEFAULT 0; -- Will be populated after reindex.

ALTER TABLE chain.blocks
	ADD COLUMN epoch UINT63 NOT NULL DEFAULT 0; -- Will be populated after reindex. Could migrate using chain.epochs table, but a reindex is planned anyway.

CREATE INDEX IF NOT EXISTS ix_blocks_block_hash ON chain.blocks (block_hash);

COMMIT;
