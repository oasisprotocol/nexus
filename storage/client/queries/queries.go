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

	NodeHeight = `
		SELECT height
			FROM chain.latest_node_heights
			WHERE layer=$1`

	Blocks = `
		SELECT
			height,
			block_hash,
			time,
			num_txs,
			gas_limit,
			size_limit,
			epoch,
			state_root,
			ROW(
			    proposer_entity.id,
			    proposer_entity.address,
			    proposer_entity.meta
			) AS proposer_entity_info,
			ARRAY( -- Select block signers and map them into an array column named signer_node_infos.
				SELECT
					ROW(
						signer_entity.id,
						signer_entity.address,
						signer_entity.meta
					)
				FROM UNNEST(signer_entity_ids) AS signer_entity_id
				LEFT JOIN chain.entities AS signer_entity ON signer_entity.id = signer_entity_id
				ORDER BY signer_entity.address
			) AS signer_node_infos
		FROM chain.blocks
		LEFT JOIN chain.entities AS proposer_entity ON proposer_entity.id = proposer_entity_id
		WHERE
			($1::bigint IS NULL OR height >= $1::bigint) AND
			($2::bigint IS NULL OR height <= $2::bigint) AND
			($3::timestamptz IS NULL OR time >= $3::timestamptz) AND
			($4::timestamptz IS NULL OR time < $4::timestamptz) AND
			($5::text IS NULL OR block_hash = $5::text) AND
			($6::text IS NULL OR proposer_entity_id = COALESCE((
				SELECT id
				FROM chain.entities
				WHERE address = $6::text
				LIMIT 1
			), 'none'))
		ORDER BY height DESC
		LIMIT $7::bigint
		OFFSET $8::bigint`

	Block = `
		SELECT height, block_hash, time, num_txs, gas_limit, size_limit, epoch, state_root
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
				chain.transactions.max_gas as gas_limit,
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
				-- When related_address ($5) is NULL and hence we do no filtering on it, avoid the join altogether.
				-- Otherwise, every tx will be returned as many times as there are related addresses for it.
				AND $5::text IS NOT NULL
			WHERE ($1::text IS NULL OR chain.transactions.tx_hash = $1::text) AND
					($2::bigint IS NULL OR chain.transactions.block = $2::bigint) AND
					($3::text IS NULL OR chain.transactions.method = $3::text) AND
					($4::text IS NULL OR chain.transactions.sender = $4::text) AND
					($5::text IS NULL OR chain.accounts_related_transactions.account_address = $5::text) AND
					($6::timestamptz IS NULL OR chain.blocks.time >= $6::timestamptz) AND
					($7::timestamptz IS NULL OR chain.blocks.time < $7::timestamptz)
			ORDER BY chain.transactions.block DESC, chain.transactions.tx_index
			LIMIT $8::bigint
			OFFSET $9::bigint`

	Events = `
		SELECT tx_block, tx_index, tx_hash, roothash_runtime_id, roothash_runtime, roothash_runtime_round, type, body, b.time
			FROM chain.events
			LEFT JOIN chain.blocks b ON tx_block = b.height
			WHERE ($1::bigint IS NULL OR tx_block = $1::bigint) AND
					($2::integer IS NULL OR tx_index = $2::integer) AND
					($3::text IS NULL OR tx_hash = $3::text) AND
					($4::text IS NULL OR type = $4::text) AND
					($5::text IS NULL OR ARRAY[$5::text] <@ related_accounts)
			ORDER BY tx_block DESC, tx_index
			LIMIT $6::bigint
			OFFSET $7::bigint`

	RoothashMessages = `
		SELECT
		    runtime,
			round,
			message_index,
			type,
			body,
			error_module,
			error_code,
			result
		FROM chain.roothash_messages
		WHERE
			($1::runtime IS NULL OR runtime = $1) AND
			($2::bigint IS NULL OR round = $2) AND
			($3::text IS NULL OR type = $3) AND
			($4::oasis_addr IS NULL OR related_accounts @> ARRAY[$4])
		ORDER BY round DESC, message_index DESC
		LIMIT $5
		OFFSET $6`

	Entities = `
		SELECT id, address
			FROM chain.entities
		ORDER BY id
		LIMIT $1::bigint
		OFFSET $2::bigint`

	Entity = `
		SELECT id, address
			FROM chain.entities
			WHERE address = $1::text`

	EntityNodeIds = `
		SELECT nodes.id
		FROM chain.nodes as nodes
		JOIN chain.entities as entities
			ON entities.id = nodes.entity_id
		WHERE entities.address = $1::text
			ORDER BY nodes.id`

	EntityNodes = `
		SELECT nodes.id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
		FROM chain.nodes as nodes
		JOIN chain.entities as entities
			ON entities.id = nodes.entity_id
		WHERE entities.address = $1::text
		ORDER BY id
		LIMIT $2::bigint
		OFFSET $3::bigint`

	EntityNode = `
		SELECT nodes.id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
		FROM chain.nodes
		JOIN chain.entities
			ON nodes.entity_id = entities.id
		WHERE entities.address = $1::text AND nodes.id = $2::text`

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
			, 0) AS debonding_delegations_balance,
			first_activity
		FROM chain.accounts
		WHERE address = $1::text`

	// Uses periodically computed view.
	Accounts = `
		SELECT
			address,
			nonce,
			general_balance,
			escrow_balance_active,
			escrow_balance_debonding,
			delegations_balance,
			debonding_delegations_balance,
			first_activity
		FROM
			views.accounts_list
		ORDER BY total_balance DESC, address
		LIMIT $1::bigint
		OFFSET $2::bigint`

	AccountStats = `
		SELECT
			COUNT(*)
		FROM chain.accounts_related_transactions
		WHERE account_address = $1::text`

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
		WHERE ($1::bigint IS NULL OR id = $1::bigint)
		ORDER BY id DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	Proposals = `
		SELECT id, submitter, state, deposit, title, description, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, parameters_change_module, parameters_change, created_at, closes_at, invalid_votes
			FROM chain.proposals
			WHERE ($1::text IS NULL OR submitter = $1::text) AND
						($2::text IS NULL OR state = $2::text)
		ORDER BY id DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`

	Proposal = `
		SELECT id, submitter, state, deposit, title, description, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, parameters_change_module, parameters_change, created_at, closes_at, invalid_votes
			FROM chain.proposals
			WHERE id = $1::bigint`

	ProposalVotes = `
		SELECT votes.voter, votes.vote, votes.height, blocks.time
			FROM chain.votes as votes
			LEFT JOIN chain.blocks as blocks ON votes.height = blocks.height
			WHERE proposal = $1::bigint
		ORDER BY height DESC, voter ASC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	LatestEpochStart = `
		SELECT id, start_height
			FROM chain.epochs
			ORDER BY id DESC`

	ValidatorsAggStats = `
		SELECT
			COALESCE (
				(SELECT SUM(voting_power)
				FROM chain.nodes)
			, 0) AS total_voting_power,
			COALESCE (
				(SELECT COUNT(*)
				FROM (SELECT DISTINCT delegator FROM chain.delegations) AS _)
			, 0) AS total_delegators,
			COALESCE (
				(SELECT SUM(accts.escrow_balance_active)
				FROM chain.entities AS entities
				JOIN chain.accounts AS accts ON entities.address = accts.address)
			, 0) AS total_staked_balance`

	ValidatorLast100BlocksSigned = `
		SELECT height, COALESCE($1 = ANY(signer_entity_ids), FALSE)
			FROM chain.blocks
		ORDER BY height DESC
		LIMIT 100`

	ValidatorsData = `
		WITH
		-- Find all self-delegations for all accounts with active delegations.
		self_delegations AS (
			SELECT delegatee AS address, shares AS shares
				FROM chain.delegations
				WHERE delegator = delegatee
				GROUP BY delegator, delegatee
		),
		-- Compute the number of delegators for all accounts with active delegations.
		delegators_count AS (
			SELECT delegatee AS address, COUNT(*) AS count
				FROM chain.delegations
				GROUP BY delegatee
		),
		-- Each entity can have multiple nodes. For each entity, find the validator node with
		-- the highest voting power. Only one node per entity will have nonzero voting power.
		validator_nodes AS (
			SELECT entities.address, COALESCE(MAX(nodes.voting_power), 0) AS voting_power
				FROM chain.entities AS entities
				JOIN chain.nodes AS nodes ON
					entities.id = nodes.entity_id AND
					nodes.roles LIKE '%validator%'
				GROUP BY entities.address
		),
		-- Compute validator rank by in_validator_set desc, is_active desc, escrow balance active desc, entity_id desc, in a subquery to support querying a single validator by address.
		validator_rank AS (
			SELECT
				chain.entities.address,
				RANK() OVER (ORDER BY
					EXISTS(SELECT NULL FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND voting_power > 0) DESC,
					EXISTS(SELECT NULL FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND chain.nodes.roles LIKE '%validator%') DESC,
					chain.accounts.escrow_balance_active DESC,
					chain.entities.id DESC
				) AS rank
			FROM chain.entities
			JOIN chain.accounts ON chain.entities.address = chain.accounts.address
		)
		SELECT
			chain.entities.id AS entity_id,
			chain.entities.address AS entity_address,
			chain.nodes.id AS node_id,
			chain.accounts.escrow_balance_active AS active_balance,
			chain.accounts.escrow_total_shares_active AS active_shares,
			chain.accounts.escrow_balance_debonding AS debonding_balance,
			chain.accounts.escrow_total_shares_debonding AS debonding_shares,
			COALESCE (
				ROUND(COALESCE(self_delegations.shares, 0) * chain.accounts.escrow_balance_active / NULLIF(chain.accounts.escrow_total_shares_active, 0))
			, 0) AS self_delegation_balance,
			COALESCE (
				self_delegations.shares
			, 0) AS self_delegation_shares,
			history.validators.escrow_balance_active AS active_balance_24,
			COALESCE (
				delegators_count.count
			, 0) AS num_delegators,
			COALESCE (
				validator_nodes.voting_power
			, 0) AS voting_power,
			SUM(validator_nodes.voting_power)
				OVER (ORDER BY validator_rank.rank) AS voting_power_cumulative,
			COALESCE(chain.commissions.schedule, '{}'::JSONB) AS commissions_schedule,
			chain.blocks.time AS start_date,
			validator_rank.rank AS rank,
			EXISTS(SELECT NULL FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND chain.nodes.roles LIKE '%validator%') AS active,
			EXISTS(SELECT NULL FROM chain.nodes WHERE chain.entities.id = chain.nodes.entity_id AND voting_power > 0) AS in_validator_set,
			chain.entities.meta AS meta,
			chain.entities.logo_url as logo_url
		FROM chain.entities
		JOIN chain.accounts ON chain.entities.address = chain.accounts.address
		JOIN chain.blocks ON chain.entities.start_block = chain.blocks.height
		LEFT JOIN chain.commissions ON chain.entities.address = chain.commissions.address
		LEFT JOIN self_delegations ON chain.entities.address = self_delegations.address
		LEFT JOIN history.validators ON chain.entities.id = history.validators.id
			-- Find the epoch id from 24 hours ago. Each epoch is ~1hr.
			AND history.validators.epoch = (SELECT id - 24 from chain.epochs
				ORDER BY id DESC
				LIMIT 1)
		LEFT JOIN delegators_count ON chain.entities.address = delegators_count.address
		LEFT JOIN validator_nodes ON validator_nodes.address = entities.address
		JOIN validator_rank ON chain.entities.address = validator_rank.address
		LEFT JOIN chain.nodes ON chain.entities.id = chain.nodes.entity_id
			AND chain.nodes.roles LIKE '%validator%'
			AND chain.nodes.voting_power = validator_nodes.voting_power
		WHERE ($1::text IS NULL OR chain.entities.address = $1::text) AND
				($2::text IS NULL OR chain.entities.meta->>'name' LIKE '%' || $2::text || '%')
		ORDER BY rank
		LIMIT $3::bigint
		OFFSET $4::bigint`

	ValidatorHistory = `
		SELECT
			epoch,
			escrow_balance_active,
			escrow_total_shares_active,
			escrow_balance_debonding,
			escrow_total_shares_debonding,
			num_delegators
		FROM chain.entities
		JOIN history.validators ON chain.entities.id = history.validators.id
		WHERE (chain.entities.address = $1::text) AND
				($2::bigint IS NULL OR history.validators.epoch >= $2::bigint) AND
				($3::bigint IS NULL OR history.validators.epoch <= $3::bigint)
		ORDER BY epoch DESC
		LIMIT $4::bigint
		OFFSET $5::bigint`

	RuntimeBlocks = `
		SELECT round, block_hash, timestamp, num_transactions, size, gas_used
			FROM chain.runtime_blocks
			WHERE (runtime = $1) AND
						($2::bigint IS NULL OR round >= $2::bigint) AND
						($3::bigint IS NULL OR round <= $3::bigint) AND
						($4::timestamptz IS NULL OR timestamp >= $4::timestamptz) AND
						($5::timestamptz IS NULL OR timestamp < $5::timestamptz) AND
						($6::text IS NULL OR block_hash = $6::text)
		ORDER BY round DESC
		LIMIT $7::bigint
		OFFSET $8::bigint`

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
			txs.fee_symbol,
			txs.fee_proxy_module,
			txs.fee_proxy_id,
			txs.gas_limit,
			txs.gas_used,
			CASE
				WHEN txs.tx_eth_hash IS NULL THEN txs.fee 				     -- charged_fee=fee for non-EVM txs
				ELSE COALESCE(FLOOR(txs.fee / NULLIF(txs.gas_limit, 0)) * txs.gas_used, 0)   -- charged_fee=gas_price * gas_used for EVM txs
			END AS charged_fee,
			txs.size,
			txs.oasis_encrypted_format,
			txs.oasis_encrypted_public_key,
			txs.oasis_encrypted_data_nonce,
			txs.oasis_encrypted_data_data,
			txs.oasis_encrypted_result_nonce,
			txs.oasis_encrypted_result_data,
			txs.method,
			txs.body,
			txs.to,
			to_preimage.context_identifier AS to_preimage_context_identifier,
			to_preimage.context_version AS to_preimage_context_version,
			to_preimage.address_data AS to_preimage_data,
			txs.amount,
			txs.amount_symbol,
			txs.evm_encrypted_format,
			txs.evm_encrypted_public_key,
			txs.evm_encrypted_data_nonce,
			txs.evm_encrypted_data_data,
			txs.evm_encrypted_result_nonce,
			txs.evm_encrypted_result_data,
			txs.success,
			txs.evm_fn_name,
			txs.evm_fn_params,
			txs.error_module,
			txs.error_code,
			txs.error_message,
			txs.error_params
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
			($5::text IS NULL or txs.method = $5::text) AND
			($6::timestamptz IS NULL OR txs.timestamp >= $6::timestamptz) AND
			($7::timestamptz IS NULL OR txs.timestamp < $7::timestamptz) AND
			(signer0.signer_address IS NOT NULL) -- HACK: excludes malformed transactions that do not have the required fields
		ORDER BY txs.round DESC, txs.tx_index DESC
		LIMIT $8::bigint
		OFFSET $9::bigint
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
			tokens.token_type,
			tokens.decimals,
			pre_from.context_identifier AS from_preimage_context_identifier,
			pre_from.context_version AS from_preimage_context_version,
			pre_from.address_data AS from_preimage_address_data,
			pre_to.context_identifier AS to_preimage_context_identifier,
			pre_to.context_version AS to_preimage_context_version,
			pre_to.address_data AS to_preimage_address_data,
			pre_owner.context_identifier AS owner_preimage_context_identifier,
			pre_owner.context_version AS owner_preimage_context_version,
			pre_owner.address_data AS owner_preimage_address_data
		FROM chain.runtime_events as evs
		-- Look up the oasis-style address derived from evs.body.address.
		-- The derivation is just a keccak hash and we could theoretically compute it instead of looking it up,
		-- but the implementing/importing the right hash function in postgres would take some work.
		LEFT JOIN chain.address_preimages AS preimages ON
			DECODE(evs.body ->> 'address', 'base64')=preimages.address_data AND
			preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
			preimages.context_version = 0
		LEFT JOIN chain.address_preimages AS pre_from ON
			evs.body->>'from' = pre_from.address
		LEFT JOIN chain.address_preimages AS pre_to ON
			evs.body->>'to' = pre_to.address
		LEFT JOIN chain.address_preimages AS pre_owner ON
			evs.body->>'owner' = pre_owner.address
		LEFT JOIN chain.evm_tokens as tokens ON
			(evs.runtime=tokens.runtime) AND
			(preimages.address=tokens.token_address) AND
			(tokens.token_type IS NOT NULL) -- exclude token _candidates_ that we haven't inspected yet; we have no info about them (name, decimals, etc)
		WHERE
			(evs.runtime = $1) AND
			($2::bigint IS NULL OR evs.round = $2::bigint) AND
			($3::integer IS NULL OR evs.tx_index = $3::integer) AND
			($4::text IS NULL OR evs.tx_hash = $4::text OR evs.tx_eth_hash = $4::text) AND
			($5::text IS NULL OR evs.type = $5::text) AND
			($6::bytea IS NULL OR evs.evm_log_signature = $6::bytea) AND
			($7::text IS NULL OR evs.related_accounts @> ARRAY[$7::text]) AND
			($8::text IS NULL OR (
				-- Currently this only supports EVM smart contracts.
				evs.type = 'evm.log' AND
				evs.body ->> 'address' = encode(eth_preimage($8::oasis_addr), 'base64')
			)) AND
			($9::text IS NULL OR (
				-- Currently this only supports ERC-721 Transfer events.
				evs.type = 'evm.log' AND
				evs.evm_log_signature = '\xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef' AND
				jsonb_array_length(evs.body -> 'topics') = 4 AND
				evs.body -> 'topics' ->> 3 = $9::text
			))
		ORDER BY evs.round DESC, evs.tx_index, evs.type, evs.body::text
		LIMIT $10::bigint
		OFFSET $11::bigint`

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
			COALESCE((
				SELECT gas_for_calling FROM chain.runtime_accounts ra
				WHERE (ra.runtime = $1) AND (ra.address = $2::text)
			), 0) AS gas_for_calling,
			compilation_metadata,
			source_files,
			verification_level
		FROM chain.evm_contracts
		WHERE (runtime = $1) AND (contract_address = $2::text)`

	AddressPreimage = `
		SELECT context_identifier, context_version, address_data
			FROM chain.address_preimages
			WHERE address = $1::text`

	RuntimeAccountStats = `
		SELECT
			total_sent, total_received, num_txs
		FROM chain.runtime_accounts
		WHERE
			(runtime = $1) AND
			(address = $2::text)`

	//nolint:gosec // Linter suspects a hardcoded access token.
	EvmTokens = `
		WITH holders AS (
			SELECT token_address, COUNT(*) AS cnt
			FROM chain.evm_token_balances
			WHERE (runtime = $1) AND (balance != 0)
			GROUP BY token_address
		)
		SELECT
			tokens.token_address AS contract_addr,
			preimages.address_data as eth_contract_addr,
			tokens.token_name AS name,
			tokens.symbol,
			tokens.decimals,
			tokens.total_supply,
			tokens.num_transfers,
			tokens.token_type AS type,
			COALESCE(holders.cnt, 0) AS num_holders,
			ref_swap_pair_creations.pair_address AS ref_swap_pair_address,
			eth_preimage(ref_swap_pair_creations.pair_address) AS ref_swap_pair_address_eth,
			ref_swap_pair_creations.factory_address AS ref_swap_factory_address,
			eth_preimage(ref_swap_pair_creations.factory_address) AS ref_swap_factory_address_eth,
			ref_swap_pair_creations.token0_address AS ref_swap_token0_address,
			eth_preimage(ref_swap_pair_creations.token0_address) AS ref_swap_token0_address_eth,
			ref_swap_pair_creations.token1_address AS ref_swap_token1_address,
			eth_preimage(ref_swap_pair_creations.token1_address) AS ref_swap_token1_address_eth,
			ref_swap_pair_creations.create_round AS ref_swap_create_round,
			ref_swap_pairs.reserve0 AS ref_swap_reserve0,
			ref_swap_pairs.reserve1 AS ref_swap_reserve1,
			ref_swap_pairs.last_sync_round AS ref_swap_last_sync_round,
			ref_tokens.token_type AS ref_token_type,
			ref_tokens.token_name AS ref_token_name,
			ref_tokens.symbol AS ref_token_symbol,
			ref_tokens.decimals AS ref_token_decimals,
			contracts.verification_level
		FROM chain.evm_tokens AS tokens
		JOIN chain.address_preimages AS preimages ON (token_address = preimages.address AND preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND preimages.context_version = 0)
		LEFT JOIN holders USING (token_address)
		LEFT JOIN chain.evm_swap_pair_creations AS ref_swap_pair_creations ON
			ref_swap_pair_creations.runtime = tokens.runtime AND
			ref_swap_pair_creations.factory_address = $4 AND
			(
				(ref_swap_pair_creations.token0_address = tokens.token_address AND ref_swap_pair_creations.token1_address = $5) OR
				(ref_swap_pair_creations.token0_address = $5 AND ref_swap_pair_creations.token1_address = tokens.token_address)
			)
		LEFT JOIN chain.evm_swap_pairs AS ref_swap_pairs ON
			ref_swap_pairs.runtime = tokens.runtime AND
			ref_swap_pairs.pair_address = ref_swap_pair_creations.pair_address
		LEFT JOIN chain.evm_tokens AS ref_tokens ON
			ref_tokens.runtime = tokens.runtime AND
			ref_tokens.token_address = $5
		LEFT JOIN chain.evm_contracts as contracts ON (tokens.runtime = contracts.runtime AND tokens.token_address = contracts.contract_address)
		WHERE
			(tokens.runtime = $1) AND
			($2::oasis_addr IS NULL OR tokens.token_address = $2::oasis_addr) AND
			($3::text IS NULL OR tokens.token_name ILIKE '%' || $3 || '%' OR tokens.symbol ILIKE '%' || $3 || '%') AND
			tokens.token_type IS NOT NULL AND -- exclude token _candidates_ that we haven't inspected yet
			tokens.token_type != 0 -- exclude unknown-type tokens; they're often just contracts that emitted Transfer events but don't expose the token ticker, name, balance etc.
		ORDER BY
			(
				CASE
					-- For the reference token itself, it is 1:1 in value with, you know, itself.
					WHEN
						tokens.token_address = $5
					THEN 1.0
					-- The pool keeps a proportion of reserves so that reserve0 of token0 is worth about as much as reserve1 of token1.
					-- When token0 is the reference token, more reserve0 means token1 is worth more than the reference token.
					WHEN
						ref_swap_pair_creations.token0_address = $5 AND
						ref_swap_pairs.reserve0 IS NOT NULL AND
						ref_swap_pairs.reserve0 > 0 AND
						ref_swap_pairs.reserve1 IS NOT NULL AND
						ref_swap_pairs.reserve1 > 0
					THEN ref_swap_pairs.reserve0::REAL / ref_swap_pairs.reserve1::REAL
					-- When token1 is the reference token, more reserve1 means token0 is worth more than the reference token.
					WHEN
						ref_swap_pair_creations.token1_address = $5 AND
						ref_swap_pairs.reserve0 IS NOT NULL AND
						ref_swap_pairs.reserve0 > 0 AND
						ref_swap_pairs.reserve1 IS NOT NULL AND
						ref_swap_pairs.reserve1 > 0
					THEN ref_swap_pairs.reserve1::REAL / ref_swap_pairs.reserve0::REAL
					ELSE 0.0
				END *
				COALESCE(tokens.total_supply, 0)
			) DESC,
		    num_holders DESC,
		    contract_addr
		LIMIT $6::bigint
		OFFSET $7::bigint`

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
			(balances.token_address = $2::oasis_addr) AND
			(balances.balance != 0)
		ORDER BY balance DESC, holder_addr
		LIMIT $3::bigint
		OFFSET $4::bigint`

	EvmNfts = `
		WITH
			token_holders AS (
				SELECT token_address, COUNT(*) AS num_holders
				FROM chain.evm_token_balances
				WHERE (runtime = $1) AND (balance != 0)
				GROUP BY token_address
			)
		SELECT
			chain.evm_nfts.token_address,
			token_preimage.context_identifier,
			token_preimage.context_version,
			token_preimage.address_data,
			chain.evm_tokens.token_name,
			chain.evm_tokens.symbol,
			chain.evm_tokens.decimals,
			chain.evm_tokens.token_type,
			chain.evm_tokens.total_supply,
			chain.evm_tokens.num_transfers,
			COALESCE(token_holders.num_holders, 0) AS num_holders,
			chain.evm_contracts.verification_level,
			chain.evm_nfts.nft_id,
			chain.evm_nfts.owner,
			owner_preimage.context_identifier,
			owner_preimage.context_version,
			owner_preimage.address_data,
			chain.evm_nfts.num_transfers,
			chain.evm_nfts.metadata_uri,
			chain.evm_nfts.metadata_accessed,
			chain.evm_nfts.metadata,
			chain.evm_nfts.name,
			chain.evm_nfts.description,
			chain.evm_nfts.image
		FROM chain.evm_nfts
		LEFT JOIN chain.address_preimages AS token_preimage ON
			token_preimage.address = chain.evm_nfts.token_address
		LEFT JOIN chain.evm_tokens USING (runtime, token_address)
		LEFT JOIN token_holders USING (token_address)
		LEFT JOIN chain.evm_contracts ON
			chain.evm_contracts.runtime = chain.evm_tokens.runtime AND
			chain.evm_contracts.contract_address = chain.evm_tokens.token_address
		LEFT JOIN chain.address_preimages AS owner_preimage ON
			owner_preimage.address = chain.evm_nfts.owner
		WHERE
			chain.evm_nfts.runtime = $1::runtime AND
			($2::oasis_addr IS NULL OR chain.evm_nfts.token_address = $2::oasis_addr) AND
			($3::uint_numeric IS NULL OR chain.evm_nfts.nft_id = $3::uint_numeric) AND
			($4::oasis_addr IS NULL OR chain.evm_nfts.owner = $4::oasis_addr)
		ORDER BY token_address, nft_id
		LIMIT $5::bigint
		OFFSET $6::bigint`

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
			tokens.token_type,
			tokens.decimals AS token_decimals
		FROM chain.evm_token_balances AS balances
		JOIN chain.address_preimages  AS preimages ON (preimages.address = balances.token_address AND preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND preimages.context_version = 0)
		JOIN chain.evm_tokens         AS tokens USING (runtime, token_address)
		WHERE runtime = $1 AND
			balances.account_address = $2::text AND
			tokens.token_type IS NOT NULL AND -- exclude token _candidates_ that we haven't inspected yet
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

	// FineTxVolumes returns the fine-grained query for 5-minute sampled tx volume windows.
	FineTxVolumes = `
		SELECT window_end, tx_volume
		FROM stats.min5_tx_volume
		WHERE layer = $1::text
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`

	// FineDailyTxVolumes returns the query for daily tx volume windows.
	FineDailyTxVolumes = `
		SELECT window_end, tx_volume
		FROM stats.daily_tx_volume
		WHERE layer = $1::text
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`

	// DailyTxVolumes returns the query for daily sampled daily tx volume windows.
	DailyTxVolumes = `
		SELECT window_end, tx_volume
		FROM stats.daily_tx_volume
		WHERE (layer = $1::text AND (window_end AT TIME ZONE 'UTC')::time = '00:00:00')
		ORDER BY
			window_end DESC
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
		SELECT window_end, active_accounts
		FROM stats.daily_active_accounts
		WHERE (layer = $1::text AND (window_end AT TIME ZONE 'UTC')::time = '00:00:00')
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint
	`
)
