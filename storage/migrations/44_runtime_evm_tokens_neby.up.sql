BEGIN;

ALTER TABLE chain.evm_tokens ADD COLUMN neby_derived_price NUMERIC(38, 18);

COMMIT;
