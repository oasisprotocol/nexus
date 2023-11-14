BEGIN;

ALTER TABLE chain.evm_contracts
    ADD COLUMN verification_state INTEGER NOT NULL DEFAULT 0; -- See VerificationState in analyzer/evmverifier.

DROP INDEX chain.ix_evm_contracts_unverified;
CREATE INDEX ix_evm_contracts_unverified ON chain.evm_contracts (runtime) WHERE verification_state < 2;

COMMIT;
