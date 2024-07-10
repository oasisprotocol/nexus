BEGIN;

ALTER TABLE chain.blocks
    ADD COLUMN size_limit UINT_NUMERIC NOT NULL DEFAULT 0; -- Will be populated after reindex.

COMMIT;
