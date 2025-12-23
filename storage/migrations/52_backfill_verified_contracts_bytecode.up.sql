-- Backfill verified contracts into the bytecode analysis queue.
--
-- Some verified contracts were never queued for bytecode download because:
-- 1. They were only called internally by other contracts (not as direct tx recipients)
-- 2. They were created via encrypted transactions on Sapphire
-- 3. They were processed before commit 83c02272 which added detection of
--    contract candidates via log-emitting addresses
--
-- The Sourcify verifier now:
-- - Inserts address preimages for verified contracts
-- - Queues contracts for bytecode analysis
-- - Reprocesses contracts that are missing preimages
--
-- This migration queues any verified contracts (that have preimages) for
-- bytecode analysis if they're not already in the queue.
--
-- Reference: https://github.com/oasisprotocol/nexus/issues/1227

INSERT INTO analysis.evm_contract_code (runtime, contract_candidate)
SELECT
    c.runtime,
    c.contract_address
FROM chain.evm_contracts c
-- Only include contracts with preimages (required for bytecode fetching)
JOIN chain.address_preimages p
    ON c.contract_address = p.address
    AND p.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
-- Exclude contracts already in the analysis queue
LEFT JOIN analysis.evm_contract_code a
    ON a.runtime = c.runtime
    AND a.contract_candidate = c.contract_address
WHERE
    -- Only verified contracts
    c.verification_level IS NOT NULL
    -- Missing bytecode
    AND c.runtime_bytecode IS NULL
    -- Not already in queue
    AND a.contract_candidate IS NULL
ON CONFLICT (runtime, contract_candidate) DO NOTHING;
