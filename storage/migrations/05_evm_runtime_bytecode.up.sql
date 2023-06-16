BEGIN;

-- A schema for keeping track of analyzers' internal state/progess.
-- TODO: Move chain.processed_blocks and chain.*_analysis tables here.
CREATE SCHEMA IF NOT EXISTS analysis;
GRANT USAGE ON SCHEMA analysis TO PUBLIC;

CREATE DOMAIN eth_addr BYTEA CHECK(length(VALUE) = 20);

-- Used to keep track of potential contract addresses, and our progress in
-- downloading their runtime bytecode. ("Runtime" in the sense of ETH terminology
-- which talks about a contract's "runtime bytecode" as opposed to "creation bytecode".)
CREATE TABLE analysis.evm_contract_code (
    runtime runtime NOT NULL,
    contract_candidate oasis_addr NOT NULL,
    PRIMARY KEY (runtime, contract_candidate),
    -- Meaning of is_contract:
    --   TRUE:  downloaded runtime bytecode
    --   FALSE: download failed because `contract_candidate` is not a contract (= does not have code)
    --   NULL:  not yet attempted
    is_contract BOOLEAN
);
-- Allow the analyzer to quickly retrieve addresses that have not been downloaded yet.
CREATE INDEX ix_evm_contract_code_todo ON analysis.evm_contract_code (runtime, contract_candidate) WHERE is_contract IS NULL;

-- Bootstrap the table with the set of addresses we known are contracts because they are the result of an evm.Create tx.
INSERT INTO analysis.evm_contract_code (runtime, contract_candidate, is_contract)
  SELECT runtime, contract_address, NULL
  FROM chain.evm_contracts
ON CONFLICT (runtime, contract_candidate) DO NOTHING;

ALTER TABLE chain.evm_contracts
  ADD COLUMN runtime_bytecode BYTEA;

-- Grant others read-only use. This does NOT apply to future tables in the schema.
GRANT SELECT ON ALL TABLES IN SCHEMA analysis TO PUBLIC;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA analysis TO PUBLIC;

COMMIT;
