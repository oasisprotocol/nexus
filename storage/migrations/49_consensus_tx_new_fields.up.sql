BEGIN;

-- NOTE: We intentionally use NUMERIC(1000,0) instead of the uint_numeric domain.
-- Adding a domain-typed column on these large tables caused a table rewrite
-- (DataFileRead for hours) due to domain constraint plumbing being applied to
-- every partition/chunk. Using the base type + NULLable column avoids the rewrite
-- and is metadata-only (fast).
ALTER TABLE chain.transactions ADD COLUMN gas_used NUMERIC(1000,0) NULL;

ALTER TABLE chain.blocks ADD COLUMN size NUMERIC(1000,0) NULL;
ALTER TABLE chain.blocks ADD COLUMN gas_used NUMERIC(1000,0) NULL;

COMMIT;
