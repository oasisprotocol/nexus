BEGIN;

ALTER TABLE chain.runtime_blocks ADD COLUMN min_gas_price UINT_NUMERIC;

COMMIT;
