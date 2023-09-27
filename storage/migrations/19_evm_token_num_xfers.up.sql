BEGIN;

-- Backfill num_transfers
WITH transfers AS (
    SELECT runtime, DECODE(body ->> 'address', 'base64') AS eth_addr, COUNT(*) AS num_xfers
        FROM chain.runtime_events
        WHERE evm_log_signature='\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef'::bytea -- ERC-20 and ERC-721 Transfer
        GROUP BY runtime, eth_addr
)
UPDATE chain.evm_tokens as tokens 
	SET 
		num_transfers = transfers.num_xfers
	FROM transfers
	LEFT JOIN chain.address_preimages as preimages
	ON 
		preimages.address_data = transfers.eth_addr AND
	    preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
	    preimages.context_version = 0
	WHERE
		tokens.runtime = transfers.runtime AND
		tokens.token_address = preimages.address;

-- Backfill total_supply
-- We dead-reckon total_supply using evm_token_balances and use
-- this value as a default for erc721 tokens, since they might 
-- not support totalSupply()
WITH total_supplies AS (
    SELECT runtime, token_address, sum(balance) as total_supply
        FROM chain.evm_token_balances 
        GROUP BY runtime, token_address 
)
UPDATE chain.evm_tokens as tokens
    SET
        total_supply = total_supplies.total_supply
    FROM total_supplies
    WHERE
        total_supplies.runtime = tokens.runtime AND 
        total_supplies.token_address = tokens.token_address AND
        tokens.token_type = 721;
-- Re-download all tokens to update total_supply, and potentially 
-- overwrite the dead-reckoned value
UPDATE chain.evm_tokens
    SET
        last_download_round = 0;

COMMIT;
