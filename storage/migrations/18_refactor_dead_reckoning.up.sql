BEGIN;

-------------------------------------
-- Convenience functions for converting between eth-style and oasis-style addresses
-- via lookups in the address_preimages table.

-- Convenience function for retrieving the ethereum address associated with a given oasis address.
CREATE OR REPLACE FUNCTION eth_preimage(addr oasis_addr)
RETURNS BYTEA
LANGUAGE plpgsql
AS $$
BEGIN
DECLARE
    result BYTEA;
BEGIN
    SELECT pre.address_data
    INTO result
    FROM chain.address_preimages pre
    WHERE pre.address = addr
    AND pre.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
    AND pre.context_version = 0;
    RETURN result;
END;
END;
$$;

CREATE OR REPLACE FUNCTION derive_oasis_addr(eth_addr bytea)
RETURNS TEXT
LANGUAGE plpgsql
AS $$
BEGIN
DECLARE
    result TEXT;
BEGIN
    -- Check if the bytea data is exactly 20 bytes in length; this prevents accidentally accepting strings that are hex-encodings of the actual address, or similar.
    IF length(eth_addr) <> 20 THEN
        RAISE EXCEPTION 'Input address must be a bytea and exactly 20 bytes in length';
    END IF;

    -- Look up the oasis-style address derived from evs.body.address.
    -- The derivation is just a keccak hash and we could theoretically compute it instead of looking it up,
    -- but the right hash function will only be easily available in postgres once openssl 3.2 is released
    -- (see https://github.com/openssl/openssl/issues/19304) and bundled into a postgres docker image, probably
    -- in early 2024.
    SELECT pre.address
    INTO result
    FROM chain.address_preimages pre
    WHERE pre.address_data = eth_addr
    AND pre.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
    AND pre.context_version = 0;
    RETURN result;
END;
END;
$$;


-------------------------------------
-- Moves the chain.evm_contracts.gas_used column to chain.runtime_accounts.gas_for_calling

ALTER TABLE chain.runtime_accounts
    -- Total gas used by all txs addressed to this account. Primarily meaningful for accounts that are contracts.
    ADD COLUMN gas_for_calling UINT63 NOT NULL DEFAULT 0;

UPDATE chain.runtime_accounts as ra
    SET gas_for_calling = c.gas_used
    FROM chain.evm_contracts c
    WHERE c.runtime = ra.runtime AND
          c.contract_address = ra.address;

ALTER TABLE chain.evm_contracts
    DROP COLUMN gas_used;


-------------------------------------
-- Merges the analysis.evm_tokens table into chain.evm_tokens
ALTER TABLE chain.evm_tokens
    ALTER COLUMN token_type DROP NOT NULL;
ALTER TABLE chain.evm_tokens
  -- Block analyzer bumps this when it sees the mutable fields of the token
  -- change (e.g. total supply) based on dead reckoning.
  ADD COLUMN last_mutate_round UINT63 NOT NULL DEFAULT 0,
  -- Token analyzer bumps this when it downloads info about the token.
  ADD COLUMN last_download_round UINT63;

WITH transfers AS (
    SELECT runtime, DECODE(body ->> 'address', 'base64') AS eth_addr, COUNT(*) AS num_xfers
        FROM chain.runtime_events
        WHERE evm_log_signature='\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef'::bytea -- ERC-20 and ERC-721 Transfer
        GROUP BY runtime, eth_addr
)
INSERT INTO chain.evm_tokens(runtime, token_address, total_supply, num_transfers, last_mutate_round, last_download_round)
SELECT
    runtime,
    token_address,
    total_supply,
    num_transfers,
    last_mutate_round,
    last_download_round
FROM analysis.evm_tokens AS tokens
ON CONFLICT(runtime, token_address) DO UPDATE SET
    total_supply = EXCLUDED.total_supply,
    num_transfers = EXCLUDED.num_transfers,
    last_mutate_round = EXCLUDED.last_mutate_round,
    last_download_round = EXCLUDED.last_download_round;

DROP TABLE analysis.evm_tokens;

COMMIT;
