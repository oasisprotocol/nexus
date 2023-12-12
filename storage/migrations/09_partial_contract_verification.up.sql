BEGIN;

CREATE TYPE public.sourcify_level AS ENUM ('partial', 'full');

ALTER TABLE chain.evm_contracts
    ADD COLUMN verification_level sourcify_level;

DROP INDEX chain.ix_evm_contracts_unverified;

COMMIT;
