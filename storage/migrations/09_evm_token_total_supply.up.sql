BEGIN;

ALTER TABLE analysis.evm_tokens ALTER COLUMN total_supply type NUMERIC(1000,0);
ALTER TABLE chain.evm_tokens ALTER COLUMN total_supply type NUMERIC(1000,0);

COMMIT;
