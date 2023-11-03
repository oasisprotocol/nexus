BEGIN;

ALTER TABLE chain.evm_nfts
    ADD COLUMN metadata JSONB;

COMMIT;
