BEGIN;

ALTER TABLE chain.evm_tokens ADD COLUMN num_transfers UINT63 NOT NULL DEFAULT 0;
ALTER TABLE analysis.evm_tokens ADD COLUMN num_transfers UINT63 NOT NULL DEFAULT 0;

-- Backfill chain.evm_tokens.num_transfers
WITH transfers AS (
    SELECT runtime, DECODE(body ->> 'address', 'base64') AS eth_addr, COUNT(*) AS num_xfers
        FROM chain.runtime_events
        GROUP BY runtime, eth_addr
)
UPDATE chain.evm_tokens as tokens
    SET num_transfers = transfers.num_xfers
    FROM transfers
    LEFT JOIN chain.address_preimages as preimages 
    ON
        preimages.address_data = transfers.eth_addr AND
        preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
        preimages.context_version = 0
    WHERE
        tokens.runtime = transfers.runtime AND
        tokens.token_address = preimages.address;

-- Backfill analysis.evm_tokens.num_transfers for tokens that haven't been processed yet.
-- These values will be inserted into chain.evm_tokens when the token is downloaded.
WITH transfers AS (
    SELECT runtime, DECODE(body ->> 'address', 'base64') AS eth_addr, COUNT(*) AS num_xfers
        FROM chain.runtime_events
        GROUP BY runtime, eth_addr
)
UPDATE analysis.evm_tokens as tokens
    SET num_transfers = transfers.num_xfers
    FROM transfers
    LEFT JOIN chain.address_preimages as preimages 
    ON
        preimages.address_data = transfers.eth_addr AND
        preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
        preimages.context_version = 0
    WHERE
        tokens.runtime = transfers.runtime AND
        tokens.token_address = preimages.address AND
        tokens.last_download_round IS NULL;

COMMIT;
