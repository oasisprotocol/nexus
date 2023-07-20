package queries

import (
	"fmt"
)

func TotalCountQuery(inner string) string {
	return fmt.Sprintf(`
		WITH subquery AS (%s)
			SELECT count(*) FROM subquery`, inner)
}

const (
	Status = `
		SELECT height, processed_time
			FROM analysis.processed_blocks
			WHERE analyzer=$1 AND processed_time IS NOT NULL
		ORDER BY height DESC
		LIMIT 1`

	Blocks = `
		SELECT height, block_hash, time, num_txs
			FROM chain.blocks
			WHERE ($1::bigint IS NULL OR height >= $1::bigint) AND
						($2::bigint IS NULL OR height <= $2::bigint) AND
						($3::timestamptz IS NULL OR time >= $3::timestamptz) AND
						($4::timestamptz IS NULL OR time < $4::timestamptz)
		ORDER BY height DESC
		LIMIT $5::bigint
		OFFSET $6::bigint`

	Block = `
		SELECT height, block_hash, time, num_txs
			FROM chain.blocks
			WHERE height = $1::bigint`

	Transactions = `
		SELECT
				chain.transactions.block as block,
				chain.transactions.tx_index as tx_index,
				chain.transactions.tx_hash as tx_hash,
				chain.transactions.sender as sender,
				chain.transactions.nonce as nonce,
				chain.transactions.fee_amount as fee_amount,
				chain.transactions.method as method,
				chain.transactions.body as body,
				chain.transactions.code as code,
				chain.transactions.module as module,
				chain.transactions.message as message,
				chain.blocks.time as time
			FROM chain.transactions
			JOIN chain.blocks ON chain.transactions.block = chain.blocks.height
			LEFT JOIN chain.accounts_related_transactions ON chain.transactions.block = chain.accounts_related_transactions.tx_block
				AND chain.transactions.tx_index = chain.accounts_related_transactions.tx_index
				-- When related_address ($4) is NULL and hence we do no filtering on it, avoid the join altogether.
				-- Otherwise, every tx will be returned as many times as there are related addresses for it.
				AND $4::text IS NOT NULL
			WHERE ($1::bigint IS NULL OR chain.transactions.block = $1::bigint) AND
					($2::text IS NULL OR chain.transactions.method = $2::text) AND
					($3::text IS NULL OR chain.transactions.sender = $3::text) AND
					($4::text IS NULL OR chain.accounts_related_transactions.account_address = $4::text) AND
					($5::numeric IS NULL OR chain.transactions.fee_amount >= $5::numeric) AND
					($6::numeric IS NULL OR chain.transactions.fee_amount <= $6::numeric) AND
					($7::bigint IS NULL OR chain.transactions.code = $7::bigint) AND
					($8::timestamptz IS NULL OR chain.blocks.time >= $8::timestamptz) AND
					($9::timestamptz IS NULL OR chain.blocks.time < $9::timestamptz)
			ORDER BY chain.transactions.block DESC, chain.transactions.tx_index
			LIMIT $10::bigint
			OFFSET $11::bigint`

	Transaction = `
		SELECT block, tx_index, tx_hash, sender, nonce, fee_amount, method, body, code, module, message, chain.blocks.time
			FROM chain.transactions
			JOIN chain.blocks ON chain.transactions.block = chain.blocks.height
			WHERE tx_hash = $1::text`

	Events = `
		SELECT tx_block, tx_index, tx_hash, type, body
			FROM chain.events
			WHERE ($1::bigint IS NULL OR tx_block = $1::bigint) AND
					($2::integer IS NULL OR tx_index = $2::integer) AND
					($3::text IS NULL OR tx_hash = $3::text) AND
					($4::text IS NULL OR type = $4::text) AND
					($5::text IS NULL OR ARRAY[$5::text] <@ related_accounts)
			ORDER BY tx_block DESC, tx_index
			LIMIT $6::bigint
			OFFSET $7::bigint`

	Entities = `
		SELECT id, address
			FROM chain.entities
		ORDER BY id
		LIMIT $1::bigint
		OFFSET $2::bigint`

	Entity = `
		SELECT id, address
			FROM chain.entities
			WHERE id = $1::text`

	EntityNodeIds = `
		SELECT id
			FROM chain.nodes
			WHERE entity_id = $1::text`

	EntityNodes = `
		SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM chain.nodes
			WHERE entity_id = $1::text
		ORDER BY id
		LIMIT $2::bigint
		OFFSET $3::bigint`

	EntityNode = `
		SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM chain.nodes
			WHERE entity_id = $1::text AND id = $2::text`

	Accounts = `
		SELECT
			address,
			COALESCE(nonce, 0),
			COALESCE(general_balance, 0),
			COALESCE(escrow_balance_active, 0),
			COALESCE(escrow_balance_debonding, 0)
		FROM chain.accounts
		WHERE ($1::numeric IS NULL OR general_balance >= $1::numeric) AND
					($2::numeric IS NULL OR general_balance <= $2::numeric) AND
					($3::numeric IS NULL OR escrow_balance_active >= $3::numeric) AND
					($4::numeric IS NULL OR escrow_balance_active <= $4::numeric) AND
					($5::numeric IS NULL OR escrow_balance_debonding >= $5::numeric) AND
					($6::numeric IS NULL OR escrow_balance_debonding <= $6::numeric) AND
					($7::numeric IS NULL OR general_balance + escrow_balance_active + escrow_balance_debonding >= $7::numeric) AND
					($8::numeric IS NULL OR general_balance + escrow_balance_active + escrow_balance_debonding <= $8::numeric)
		ORDER BY address
		LIMIT $9::bigint
		OFFSET $10::bigint`

	Account = `
		SELECT
			address,
			COALESCE(nonce, 0),
			COALESCE(general_balance, 0),
			COALESCE(escrow_balance_active, 0),
			COALESCE(escrow_balance_debonding, 0),
			COALESCE (
				(SELECT COALESCE(ROUND(SUM(shares * escrow_balance_active / escrow_total_shares_active)), 0) AS delegations_balance
				FROM chain.delegations
				JOIN chain.accounts ON chain.accounts.address = chain.delegations.delegatee
				WHERE delegator = $1::text AND escrow_total_shares_active != 0)
			, 0) AS delegations_balance,
			COALESCE (
				(SELECT COALESCE(ROUND(SUM(shares * escrow_balance_debonding / escrow_total_shares_debonding)), 0) AS debonding_delegations_balance
				FROM chain.debonding_delegations
				JOIN chain.accounts ON chain.accounts.address = chain.debonding_delegations.delegatee
				WHERE delegator = $1::text AND escrow_total_shares_debonding != 0)
			, 0) AS debonding_delegations_balance
		FROM chain.accounts
		WHERE address = $1::text`

	AccountAllowances = `
		SELECT beneficiary, allowance
			FROM chain.allowances
			WHERE owner = $1::text`

	Delegations = `
		SELECT delegatee, shares, escrow_balance_active, escrow_total_shares_active
			FROM chain.delegations
			JOIN chain.accounts ON chain.delegations.delegatee = chain.accounts.address
			WHERE delegator = $1::text
		ORDER BY shares DESC, delegatee
		LIMIT $2::bigint
		OFFSET $3::bigint`

	DelegationsTo = `
		-- delegatee_info is used to calculate the escrow amount of the delegators in base units
		WITH delegatee_info AS (
			SELECT escrow_balance_active, escrow_total_shares_active
			FROM chain.accounts
			WHERE address = $1::text
		)
		SELECT delegator, shares, delegatee_info.escrow_balance_active, delegatee_info.escrow_total_shares_active
			FROM chain.delegations, delegatee_info
			WHERE delegatee = $1::text
		ORDER BY shares DESC, delegator
		LIMIT $2::bigint
		OFFSET $3::bigint`

	DebondingDelegations = `
		SELECT delegatee, shares, debond_end, escrow_balance_debonding, escrow_total_shares_debonding
			FROM chain.debonding_delegations
			JOIN chain.accounts ON chain.debonding_delegations.delegatee = chain.accounts.address
			WHERE delegator = $1::text
		ORDER BY debond_end, shares DESC, delegator
		LIMIT $2::bigint
		OFFSET $3::bigint`

	DebondingDelegationsTo = `
		-- delegatee_info is used to calculate the debonding escrow amount of the delegators in base units
		WITH delegatee_info AS (
			SELECT escrow_balance_debonding, escrow_total_shares_debonding
			FROM chain.accounts
			WHERE address = $1::text
		)
		SELECT delegator, shares, debond_end, delegatee_info.escrow_balance_debonding, delegatee_info.escrow_total_shares_debonding
			FROM chain.debonding_delegations, delegatee_info
			WHERE delegatee = $1::text
		ORDER BY debond_end, shares DESC, delegator
		LIMIT $2::bigint
		OFFSET $3::bigint`

	Epochs = `
		SELECT id, start_height,
			(CASE id WHEN (SELECT max(id) FROM chain.epochs) THEN NULL ELSE end_height END) AS end_height
			FROM chain.epochs
		ORDER BY id DESC
		LIMIT $1::bigint
		OFFSET $2::bigint`

	Epoch = `
		SELECT id, start_height, end_height
			FROM chain.epochs
			WHERE id = $1::bigint`

	Proposals = `
		SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM chain.proposals
			WHERE ($1::text IS NULL OR submitter = $1::text) AND
						($2::text IS NULL OR state = $2::text)
		ORDER BY id DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`

	Proposal = `
		SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM chain.proposals
			WHERE id = $1::bigint`

	ProposalVotes = `
		SELECT voter, vote
			FROM chain.votes
			WHERE proposal = $1::bigint
		ORDER BY proposal DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	Validator = `
		SELECT id, start_height
			FROM chain.epochs
		ORDER BY id DESC
		LIMIT 1`

	ValidatorData = `
		SELECT
				chain.entities.id AS entity_id,
				chain.entities.address AS entity_address,
				chain.nodes.id AS node_address,
				chain.accounts.escrow_balance_active AS escrow,
				COALESCE(chain.commissions.schedule, '{}'::JSONB) AS commissions_schedule,
				CASE WHEN EXISTS(SELECT null FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND voting_power > 0) THEN true ELSE false END AS active,
				CASE WHEN EXISTS(SELECT null FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND chain.nodes.roles like '%validator%') THEN true ELSE false END AS status,
				chain.entities.meta AS meta
			FROM chain.entities
			JOIN chain.accounts ON chain.entities.address = chain.accounts.address
			LEFT JOIN chain.commissions ON chain.entities.address = chain.commissions.address
			JOIN chain.nodes ON chain.entities.id = chain.nodes.entity_id
				AND chain.nodes.roles like '%validator%'
				AND chain.nodes.voting_power = (
					SELECT max(voting_power)
					FROM chain.nodes
					WHERE chain.entities.id = chain.nodes.entity_id
						AND chain.nodes.roles like '%validator%'
				)
			WHERE chain.entities.id = $1::text`

	Validators = `
		SELECT id, start_height
			FROM chain.epochs
			ORDER BY id DESC`

	ValidatorsData = `
		SELECT
				chain.entities.id AS entity_id,
				chain.entities.address AS entity_address,
				chain.nodes.id AS node_address,
				chain.accounts.escrow_balance_active AS escrow,
				COALESCE(chain.commissions.schedule, '{}'::JSONB) AS commissions_schedule,
				CASE WHEN EXISTS(SELECT NULL FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND voting_power > 0) THEN true ELSE false END AS active,
				CASE WHEN EXISTS(SELECT NULL FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND chain.nodes.roles like '%validator%') THEN true ELSE false END AS status,
				chain.entities.meta AS meta
			FROM chain.entities
			JOIN chain.accounts ON chain.entities.address = chain.accounts.address
			LEFT JOIN chain.commissions ON chain.entities.address = chain.commissions.address
			JOIN chain.nodes ON chain.entities.id = chain.nodes.entity_id
				AND chain.nodes.roles like '%validator%'
				AND chain.nodes.voting_power = (
					SELECT max(voting_power)
					FROM chain.nodes
					WHERE chain.entities.id = chain.nodes.entity_id
						AND chain.nodes.roles like '%validator%'
				)
		ORDER BY escrow_balance_active DESC
		LIMIT $1::bigint
		OFFSET $2::bigint`

	RuntimeBlocks = `
		SELECT round, block_hash, timestamp, num_transactions, size, gas_used
			FROM chain.runtime_blocks
			WHERE (runtime = $1) AND
						($2::bigint IS NULL OR round >= $2::bigint) AND
						($3::bigint IS NULL OR round <= $3::bigint) AND
						($4::timestamptz IS NULL OR timestamp >= $4::timestamptz) AND
						($5::timestamptz IS NULL OR timestamp < $5::timestamptz)
		ORDER BY round DESC
		LIMIT $6::bigint
		OFFSET $7::bigint`

	RuntimeBlock = `
		SELECT round, block_hash, timestamp, num_transactions, size, gas_used
			FROM chain.runtime_blocks
			WHERE (runtime = $1) AND (round = $2::bigint)`

	RuntimeTransactions = `
		SELECT
			txs.round,
			txs.tx_index,
			txs.timestamp,
			txs.tx_hash,
			txs.tx_eth_hash,
			signer0.signer_address AS sender0, -- oh god we didn't even use the same word between the db and the api
			signer0_preimage.context_identifier AS sender0_preimage_context_identifier,
			signer0_preimage.context_version AS sender0_preimage_context_version,
			signer0_preimage.address_data AS sender0_preimage_data,
			signer0.nonce AS nonce0,
			txs.fee,
			txs.gas_limit,
			txs.gas_used,
			COALESCE(FLOOR(txs.fee / NULLIF(txs.gas_limit, 0)) * txs.gas_used, 0) AS charged_fee, -- charged_fee=gas_price * gas_used
			txs.size,
			txs.method,
			txs.body,
			txs.to,
			to_preimage.context_identifier AS to_preimage_context_identifier,
			to_preimage.context_version AS to_preimage_context_version,
			to_preimage.address_data AS to_preimage_data,
			txs.amount,
			txs.evm_encrypted_format,
			txs.evm_encrypted_public_key,
			txs.evm_encrypted_data_nonce,
			txs.evm_encrypted_data_data,
			txs.evm_encrypted_result_nonce,
			txs.evm_encrypted_result_data,
			txs.success,
			txs.error_module,
			txs.error_code,
			txs.error_message
		FROM chain.runtime_transactions AS txs
		LEFT JOIN chain.runtime_transaction_signers AS signer0 ON
		    (signer0.runtime = txs.runtime) AND
		    (signer0.round = txs.round) AND
		    (signer0.tx_index = txs.tx_index) AND
		    (signer0.signer_index = 0)
		LEFT JOIN chain.address_preimages AS signer0_preimage ON
			(signer0.signer_address = signer0_preimage.address) AND
			-- For now, the only user is the explorer, where we only care
			-- about Ethereum-compatible addresses, so only get those. Can
			-- easily enable for other address types though.
			(signer0_preimage.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth') AND (signer0_preimage.context_version = 0)
		LEFT JOIN chain.address_preimages AS to_preimage ON
			(txs.to = to_preimage.address) AND
			-- For now, the only user is the explorer, where we only care
			-- about Ethereum-compatible addresses, so only get those. Can
			-- easily enable for other address types though.
			(to_preimage.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth') AND (to_preimage.context_version = 0)
		LEFT JOIN chain.runtime_related_transactions AS rel ON
			(txs.round = rel.tx_round) AND
			(txs.tx_index = rel.tx_index) AND
			(txs.runtime = rel.runtime) AND
			-- When related_address ($4) is NULL and hence we do no filtering on it, avoid the join altogether.
			-- Otherwise, every tx will be returned as many times as there are related addresses for it.
			($4::text IS NOT NULL)
		WHERE
			(txs.runtime = $1) AND
			($2::bigint IS NULL OR txs.round = $2::bigint) AND
			($3::text IS NULL OR txs.tx_hash = $3::text OR txs.tx_eth_hash = $3::text) AND
			($4::text IS NULL OR rel.account_address = $4::text) AND
			($5::timestamptz IS NULL OR txs.timestamp >= $5::timestamptz) AND
			($6::timestamptz IS NULL OR txs.timestamp < $6::timestamptz)
		ORDER BY txs.round DESC, txs.tx_index DESC
		LIMIT $7::bigint
		OFFSET $8::bigint
		`

	RuntimeEvents = `
		SELECT
			evs.round,
			evs.tx_index,
			evs.tx_hash,
			evs.tx_eth_hash,
			evs.timestamp,
			evs.type,
			evs.body,
			evs.evm_log_name,
			evs.evm_log_params,
			tokens.symbol,
			CASE -- NOTE: There are three queries that use this CASE via copy-paste; edit both if changing.
				WHEN tokens.token_type = 20 THEN 'ERC20'
				WHEN tokens.token_type = 721 THEN 'ERC721'
				ELSE 'unexpected_other_type' -- Our openapi spec doesn't allow us to output this, but better this than a null value (which causes nil dereference)
			END AS token_type,
			tokens.decimals
		FROM chain.runtime_events as evs
		LEFT JOIN chain.address_preimages AS preimages ON
			DECODE(evs.body ->> 'address', 'base64')=preimages.address_data
		LEFT JOIN chain.evm_tokens as tokens ON
			(evs.runtime=tokens.runtime) AND
			(preimages.address=tokens.token_address) AND
			(evs.evm_log_name='Transfer')
		WHERE 
			(evs.runtime = $1) AND
			($2::bigint IS NULL OR evs.round = $2::bigint) AND
			($3::integer IS NULL OR evs.tx_index = $3::integer) AND
			($4::text IS NULL OR evs.tx_hash = $4::text OR evs.tx_eth_hash = $4::text) AND
			($5::text IS NULL OR evs.type = $5::text) AND
			($6::bytea IS NULL OR evs.evm_log_signature = $6::bytea) AND
			($7::text IS NULL OR evs.related_accounts @> ARRAY[$7::text])
		ORDER BY evs.round DESC, evs.tx_index, evs.type, evs.body::text
		LIMIT $8::bigint
		OFFSET $9::bigint`

	RuntimeEvmContract = `
		SELECT
			creation_tx,
			(
				SELECT tx_eth_hash FROM chain.runtime_transactions rtt
				WHERE (rtt.runtime = $1) AND (rtt.tx_hash = creation_tx)
				ORDER BY timestamp DESC LIMIT 1  -- Technically more than one tx might share the same hash, but it's so vanishingly rare that this hack is fine.
			) AS eth_creation_tx,
			creation_bytecode,
			runtime_bytecode,
			compilation_metadata,
			source_files
		FROM chain.evm_contracts
		WHERE (runtime = $1) AND (contract_address = $2::text)`

	AddressPreimage = `
		SELECT context_identifier, context_version, address_data
			FROM chain.address_preimages
			WHERE address = $1::text`

	RuntimeAccountStats = `
		SELECT
			COALESCE (
				(SELECT sum(amount) from chain.runtime_transfers where runtime = $1 AND sender = $2::text)
				, 0) AS total_sent,
			COALESCE (
				(SELECT sum(amount) from chain.runtime_transfers where runtime = $1 AND receiver = $2::text)
				, 0) AS total_received,
			COALESCE (
				(SELECT count(*) from chain.runtime_related_transactions where runtime= $1 AND account_address = $2::text)
				, 0) AS num_txns`

	//nolint:gosec // Linter suspects a hardcoded access token.
	EvmTokens = `
		WITH holders AS (
			SELECT token_address, COUNT(*) AS cnt
			FROM chain.evm_token_balances
			WHERE (runtime = $1)
			GROUP BY token_address
		)
		SELECT
			tokens.token_address AS contract_addr,
			preimages.address_data as eth_contract_addr,
			tokens.token_name AS name,
			tokens.symbol,
			tokens.decimals,
			tokens.total_supply,
			CASE -- NOTE: There are three queries that use this CASE via copy-paste; edit both if changing.
				WHEN tokens.token_type = 20 THEN 'ERC20'
				WHEN tokens.token_type = 721 THEN 'ERC721'
				ELSE 'unexpected_other_type' -- Our openapi spec doesn't allow us to output this, but better this than a null value (which causes nil dereference)
			END AS type,
			holders.cnt AS num_holders,
			CASE
				WHEN contracts.verification_info_downloaded_at IS NOT NULL THEN TRUE
				ELSE FALSE
			END AS verified 
		FROM chain.evm_tokens AS tokens
		JOIN chain.address_preimages AS preimages ON (token_address = preimages.address)
		JOIN holders USING (token_address)
		LEFT JOIN chain.evm_contracts as contracts ON (tokens.runtime = contracts.runtime AND tokens.token_address = contracts.contract_address)
		WHERE
			(tokens.runtime = $1) AND
			($2::oasis_addr IS NULL OR tokens.token_address = $2::oasis_addr) AND
			($3::text IS NULL OR tokens.token_name ILIKE '%' || $3 || '%' OR tokens.symbol ILIKE '%' || $3 || '%') AND
			tokens.token_type != 0 -- exclude unknown-type tokens; they're often just contracts that emitted Transfer events but don't expose the token ticker, name, balance etc.
		ORDER BY num_holders DESC
		LIMIT $4::bigint
		OFFSET $5::bigint`

	//nolint:gosec // Linter suspects a hardcoded credentials token.
	EvmTokenHolders = `
		SELECT
			balances.account_address AS holder_addr,
			preimages.address_data as eth_holder_addr,
			balances.balance AS balance
		FROM chain.evm_token_balances AS balances
		JOIN chain.address_preimages AS preimages ON (balances.account_address = preimages.address AND preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND preimages.context_version = 0)
		WHERE
			(balances.runtime = $1::runtime) AND
			(balances.token_address = $2::oasis_addr)
		ORDER BY balance DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`

	AccountRuntimeSdkBalances = `
		SELECT
			balance AS balance,
			symbol AS token_symbol
		FROM chain.runtime_sdk_balances
		WHERE runtime = $1 AND
			account_address = $2::text AND
			balance != 0
		ORDER BY balance DESC
		LIMIT 1000  -- To prevent huge responses. Hardcoded because API exposes this as a subfield that does not lend itself to pagination.
	`

	AccountRuntimeEvmBalances = `
		SELECT
			balances.balance AS balance,
			balances.token_address AS token_address,
			preimages.address_data AS token_address_eth,
			tokens.symbol AS token_symbol,
			tokens.token_name AS token_name,
			CASE -- NOTE: There are three queries that use this CASE via copy-paste; edit both if changing.
				WHEN tokens.token_type = 20 THEN 'ERC20'
				WHEN tokens.token_type = 721 THEN 'ERC721'
				ELSE 'unexpected_other_type' -- Our openapi spec doesn't allow us to output this, but better this than a null value (which causes nil dereference)
			END AS token_type,
			tokens.decimals AS token_decimals
		FROM chain.evm_token_balances AS balances
		JOIN chain.address_preimages AS preimages ON (preimages.address = balances.token_address AND preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND preimages.context_version = 0)
		JOIN chain.evm_tokens         AS tokens USING (runtime, token_address)
		WHERE runtime = $1 AND
			balances.account_address = $2::text AND
			tokens.token_type != 0 AND -- exclude unknown-type tokens; they're often just contracts that emitted Transfer events but don't expose the token ticker, name, balance etc.
			balances.balance != 0
		ORDER BY balance DESC
		LIMIT 1000  -- To prevent huge responses. Hardcoded because API exposes this as a subfield that does not lend itself to pagination.
	`

	RuntimeActiveNodes = `
		SELECT COUNT(*) AS active_nodes
		FROM chain.runtime_nodes
		WHERE runtime_id = $1::text
	`

	FineTxVolumes = `
		SELECT window_start, tx_volume
		FROM stats.min5_tx_volume
		WHERE layer = $1::text
		ORDER BY
			window_start DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`

	TxVolumes = `
		SELECT window_start, tx_volume
		FROM stats.daily_tx_volume
		WHERE layer = $1::text
		ORDER BY
			window_start DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`

	// FineDailyActiveAccounts returns the fine-grained query for daily active account windows.
	FineDailyActiveAccounts = `
		SELECT window_end, active_accounts
		FROM stats.daily_active_accounts
		WHERE layer = $1::text
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`

	// DailyActiveAccounts returns the query for daily sampled daily active account windows.
	DailyActiveAccounts = `
		SELECT date_trunc('day', window_end) as window_end, active_accounts
		FROM stats.daily_active_accounts
		WHERE (layer = $1::text AND (window_end AT TIME ZONE 'UTC')::time = '00:00:00')
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`
)
