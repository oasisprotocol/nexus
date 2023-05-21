-- Add contract verification fields to evm_contracts table.

BEGIN;

ALTER TABLE chain.evm_contracts
  ADD COLUMN verification_info_downloaded_at TIMESTAMP WITH TIME ZONE, -- NULL for unverified contracts.
  ADD COLUMN abi JSONB,
    -- Contents of metadata.json, typically produced by the Solidity compiler.
  ADD COLUMN compilation_metadata JSONB,
  -- Each source file is a flat JSON object with keys "name", "content", "path", as returned by Sourcify.
  ADD COLUMN source_files JSONB CHECK (jsonb_typeof(source_files)='array');

-- Allow the analyzer to quickly retrieve contracts that have not been verified.
CREATE INDEX ix_evm_contracts_unverified ON chain.evm_contracts (runtime) WHERE verification_info_downloaded_at IS NULL;

COMMIT;
