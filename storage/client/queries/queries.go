package queries

import (
	"fmt"
	"strings"
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

	TotalSupply = `
		SELECT total_supply
			FROM chain.blocks
			ORDER BY height DESC
			LIMIT 1`

	AddressesTotalBalance = `
		SELECT
			COALESCE(SUM(total_balance), 0)
			FROM views.accounts_list
			WHERE address = ANY($1)`

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

	// Common select for transactions queries.
	transactionsSelect = `
		SELECT
				t.block as block,
				t.tx_index as tx_index,
				t.tx_hash as tx_hash,
				t.sender as sender,
				t.nonce as nonce,
				t.fee_amount as fee_amount,
				t.max_gas as gas_limit,
				t.method as method,
				t.body as body,
				t.code as code,
				t.module as module,
				t.message as message,
				b.time as time`

	// When no filtering on related account, avoid the join altogether.
	TransactionsNoRelated = transactionsSelect + `
			FROM chain.transactions AS t
			JOIN chain.blocks AS b ON
				t.block = b.height
			WHERE
				($1::text IS NULL OR t.tx_hash = $1::text) AND
				($2::bigint IS NULL OR t.block = $2::bigint) AND
				($3::text IS NULL OR t.method = $3::text) AND
				($4::text IS NULL OR t.sender = $4::text) AND
				($5::timestamptz IS NULL OR b.time >= $5::timestamptz) AND
				($6::timestamptz IS NULL OR b.time < $6::timestamptz)
			ORDER BY t.block DESC, t.tx_index
			LIMIT $7::bigint
			OFFSET $8::bigint`

	// When filtering on related account, filter on the related account table.
	TransactionsWithRelated = `
		WITH filtered AS (
			SELECT
				art.tx_block, art.tx_index
			FROM
				chain.accounts_related_transactions art
			WHERE
				($2::bigint IS NULL OR art.tx_block = $2::bigint) AND
				($3::text IS NULL OR art.method = $3::text) AND
				($4::text IS NULL OR art.account_address = $4::text)
			ORDER BY art.tx_block DESC, art.tx_index
			LIMIT $7::bigint
			OFFSET $8::bigint
		)` +
		transactionsSelect + `
		FROM filtered AS art
		JOIN chain.transactions AS t ON
			t.block = art.tx_block AND
			t.tx_index = art.tx_index
		JOIN chain.blocks AS b ON
			t.block = b.height
		WHERE
			-- No filtering on tx_hash.
			($1::text IS NULL OR TRUE) AND
			-- No filtering on timestamps.
			($5::timestamptz IS NULL OR TRUE) AND
			($6::timestamptz IS NULL OR TRUE)
		ORDER BY art.tx_block DESC, art.tx_index`

	Events = `
		SELECT
				chain.events.tx_block,
				chain.events.tx_index,
				chain.events.tx_hash,
				chain.events.roothash_runtime_id,
				chain.events.roothash_runtime,
				chain.events.roothash_runtime_round,
				chain.events.type,
				chain.events.body,
				b.time
			FROM chain.events
			LEFT JOIN chain.blocks b ON tx_block = b.height
			LEFT JOIN chain.events_related_accounts rel ON
				chain.events.tx_block = rel.tx_block AND
				chain.events.event_index = rel.event_index AND
				-- When related_address ($5) is NULL and hence we do no filtering on it, avoid the join altogether.
				($5::text IS NOT NULL)
			WHERE ($1::bigint IS NULL OR chain.events.tx_block = $1::bigint) AND
					($2::integer IS NULL OR chain.events.tx_index = $2::integer) AND
					($3::text IS NULL OR chain.events.tx_hash = $3::text) AND
					($4::text IS NULL OR chain.events.type = $4::text) AND
					($5::text IS NULL OR rel.account_address = $5::text)
			ORDER BY chain.events.tx_block DESC, chain.events.tx_index
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
			COALESCE(tx_count, 0),
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

	AccountAllowances = `
		SELECT beneficiary, allowance
			FROM chain.allowances
			WHERE owner = $1::text`

	AccountIsEntity = `
		SELECT
			n.entity_id AS entity_node_for,
			e.id AS entity
		FROM
			chain.address_preimages ap
		LEFT JOIN chain.nodes n ON n.id = encode(ap.address_data, 'base64')
		LEFT JOIN chain.entities e ON e.id = encode(ap.address_data, 'base64')
		WHERE ap.address = $1`

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
			ORDER BY id DESC
			LIMIT 1`

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
		LIMIT 100
		-- Skip the latest block since signers are only processed in the next block.
		OFFSET 1`

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
		WITH entity AS (
			SELECT address_data
			FROM chain.address_preimages
			WHERE address = $1::text
		)
		SELECT
			epoch,
			escrow_balance_active,
			escrow_total_shares_active,
			escrow_balance_debonding,
			escrow_total_shares_debonding,
			num_delegators
		FROM history.validators
		WHERE id = (SELECT encode(address_data, 'base64') FROM entity) AND
			($2::bigint IS NULL OR epoch >= $2::bigint) AND
			($3::bigint IS NULL OR epoch <= $3::bigint)
		ORDER BY epoch DESC
		LIMIT $4::bigint
		OFFSET $5::bigint`

	RuntimeBlocks = `
		SELECT round, block_hash, timestamp, num_transactions, size, gas_used, min_gas_price
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

	runtimeTxSelect = `
		SELECT
			txs.round,
			txs.tx_index,
			txs.timestamp,
			txs.tx_hash,
			txs.tx_eth_hash,
			signers_data.signer_addresses,
			signers_data.signer_context_identifiers,
			signers_data.signer_context_versions,
			signers_data.signer_address_data,
			signers_data.signer_nonces,
			txs.fee,
			txs.fee_symbol,
			txs.fee_proxy_module,
			txs.fee_proxy_id,
			txs.gas_limit,
			txs.gas_used,
			CASE
				WHEN txs.tx_eth_hash IS NULL THEN txs.fee
				ELSE COALESCE(FLOOR(txs.fee / NULLIF(txs.gas_limit, 0)) * txs.gas_used, 0)
			END AS charged_fee,
			txs.size,
			txs.oasis_encrypted_format,
			txs.oasis_encrypted_public_key,
			txs.oasis_encrypted_data_nonce,
			txs.oasis_encrypted_data_data,
			txs.oasis_encrypted_result_nonce,
			txs.oasis_encrypted_result_data,
			txs.method,
			txs.likely_native_transfer,
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
			txs.error_message_raw,
			txs.error_params`

	runtimeTxCommonJoins = `
		LEFT JOIN LATERAL (
			SELECT
				ARRAY_AGG(signers.signer_address ORDER BY signers.signer_index) AS signer_addresses,
				ARRAY_AGG(signer_preimages.context_identifier ORDER BY signers.signer_index) AS signer_context_identifiers,
				ARRAY_AGG(signer_preimages.context_version ORDER BY signers.signer_index) AS signer_context_versions,
				ARRAY_AGG(signer_preimages.address_data ORDER BY signers.signer_index) AS signer_address_data,
				ARRAY_AGG(signers.nonce ORDER BY signers.signer_index) AS signer_nonces
			FROM chain.runtime_transaction_signers AS signers
			LEFT JOIN chain.address_preimages AS signer_preimages
				ON signers.signer_address = signer_preimages.address
				AND signer_preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth'
				AND signer_preimages.context_version = 0
			WHERE signers.runtime = txs.runtime AND signers.round = txs.round AND signers.tx_index = txs.tx_index

		) AS signers_data ON true
		LEFT JOIN chain.address_preimages AS to_preimage ON
			(txs.to = to_preimage.address) AND
			-- For now, the only user is the explorer, where we only care
			-- about Ethereum-compatible addresses, so only get those. Can
			-- easily enable for other address types though.
			(to_preimage.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth') AND
			(to_preimage.context_version = 0)`

	RuntimeTransactionsNoRelated = runtimeTxSelect + `
		FROM chain.runtime_transactions AS txs` +
		runtimeTxCommonJoins + `
		WHERE
			txs.runtime = $1 AND
			($2::bigint IS NULL OR txs.round = $2::bigint) AND
			($3::bigint IS NULL OR txs.tx_index = $3::bigint) AND
			($4::text IS NULL OR txs.tx_hash = $4::text OR txs.tx_eth_hash = $4::text) AND
			-- No filtering on related address.
			($5::text IS NULL OR true) AND
			-- No filtering on related rofl.
			($6::text IS NULL OR true) AND
			(
				-- No filtering on method.
				$7::text IS NULL OR
				(
					-- Special case to return are 'likely to be native transfers'.
					('native_transfers' = $7::text AND txs.likely_native_transfer) OR
					-- Special case to return all evm.Calls that are likely not native transfers.
					('evm.Call_no_native' = $7::text AND txs.method = 'evm.Call' AND NOT txs.likely_native_transfer) OR
					-- Regular case.
					(txs.method = $7::text)
				)
			) AND
			($8::timestamptz IS NULL OR txs.timestamp >= $8::timestamptz) AND
			($9::timestamptz IS NULL OR txs.timestamp < $9::timestamptz)
		ORDER BY txs.round DESC, txs.tx_index DESC
		LIMIT $10::bigint
		OFFSET $11::bigint`

	RuntimeTransactionsRelatedAddr = `
		WITH filtered AS (
			SELECT
				rel.runtime, rel.tx_round, rel.tx_index
			FROM
				chain.runtime_related_transactions AS rel
			WHERE
				rel.runtime = $1::runtime AND
				($2::bigint IS NULL OR rel.tx_round = $2::bigint) AND
				($3::bigint IS NULL OR rel.tx_index = $3::bigint) AND
				($5::text IS NULL OR rel.account_address = $5::text) AND
				(
					-- No filtering on method.
					$7::text IS NULL OR
					(
						-- Special case to return are 'likely to be native transfers'.
						('native_transfers' = $7::text AND rel.likely_native_transfer) OR
						-- Special case to return all evm.Calls that are likely not native transfers.
						('evm.Call_no_native' = $7::text AND rel.method = 'evm.Call' AND NOT rel.likely_native_transfer) OR
						-- Regular case.
						(rel.method = $7::text)
					)
				)
			ORDER BY rel.tx_round DESC, rel.tx_index DESC
			LIMIT $10::bigint
			OFFSET $11::bigint
		)` +
		runtimeTxSelect + `
		FROM filtered AS rel
		JOIN chain.runtime_transactions AS txs ON
			rel.runtime = txs.runtime AND
			rel.tx_round = txs.round AND
			rel.tx_index = txs.tx_index` +
		runtimeTxCommonJoins + `
		WHERE
			-- No filtering on tx_hash.
			($4::text IS NULL OR TRUE) AND
			-- No filtering on related rofl.
			($6::text IS NULL OR true) AND
			-- No filtering on timestamps.
			($8::timestamptz IS NULL OR TRUE) AND
			($9::timestamptz IS NULL OR TRUE)
		ORDER BY rel.tx_round DESC, rel.tx_index DESC`

	RuntimeTransactionsRelatedRofl = `
		WITH filtered AS (
			SELECT
				rel.runtime, rel.tx_round, rel.tx_index
			FROM
				chain.rofl_related_transactions AS rel
			WHERE
				rel.runtime = $1::runtime AND
				($2::bigint IS NULL OR rel.tx_round = $2::bigint) AND
				($3::bigint IS NULL OR rel.tx_index = $3::bigint) AND
				($6::text IS NULL OR rel.app_id = $6::text) AND
				(
					-- No filtering on method.
					$7::text IS NULL OR
					(
						-- Special case to return are 'likely to be native transfers'.
						('native_transfers' = $7::text AND rel.likely_native_transfer) OR
						-- Special case to return all evm.Calls that are likely not native transfers.
						('evm.Call_no_native' = $7::text AND rel.method = 'evm.Call' AND NOT rel.likely_native_transfer) OR
						-- Regular case.
						(rel.method = $7::text)
					)
				)
			ORDER BY rel.tx_round DESC, rel.tx_index DESC
			LIMIT $10::bigint
			OFFSET $11::bigint
		)` +
		runtimeTxSelect + `
		FROM filtered AS rel
		JOIN chain.runtime_transactions AS txs ON
			rel.runtime = txs.runtime AND
			rel.tx_round = txs.round AND
			rel.tx_index = txs.tx_index` +
		runtimeTxCommonJoins + `
		WHERE
			-- No filtering on tx_hash.
			($4::text IS NULL OR TRUE) AND
			-- No filtering on related address.
			($5::text IS NULL OR true) AND
			-- No filtering on timestamps.
			($8::timestamptz IS NULL OR TRUE) AND
			($9::timestamptz IS NULL OR TRUE)
		ORDER BY rel.tx_round DESC, rel.tx_index DESC`

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
		-- In case of EVM logs look up the oasis-style address derived from evs.body.address.
		-- The derivation is just a keccak hash and we could theoretically compute it instead of looking it up,
		-- but the implementing/importing the right hash function in postgres would take some work.
		LEFT JOIN chain.address_preimages AS preimages ON
			safe_base64_decode(evs.body ->> 'address')=preimages.address_data AND
			preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
			preimages.context_version = 0 AND
			evs.type = 'evm.log'
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
		LEFT JOIN chain.runtime_events_related_accounts as rel ON
			evs.runtime = rel.runtime AND
			evs.round = rel.round AND
			evs.event_index = rel.event_index AND
			-- When related_address ($7) is NULL and hence we do no filtering on it, avoid the join altogether.
			($7::text IS NOT NULL)
		WHERE
			(evs.runtime = $1) AND
			($2::bigint IS NULL OR evs.round = $2::bigint) AND
			($3::integer IS NULL OR evs.tx_index = $3::integer) AND
			($4::text IS NULL OR evs.tx_hash = $4::text OR evs.tx_eth_hash = $4::text) AND
			($5::text IS NULL OR evs.type = $5::text) AND
			($6::bytea IS NULL OR evs.evm_log_signature = $6::bytea) AND
			($7::text IS NULL OR rel.account_address = $7::text) AND
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
		LIMIT 1000  -- To prevent huge responses. Hardcoded because API exposes this as a subfield that does not lend itself to pagination.`

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
		LIMIT 1000  -- To prevent huge responses. Hardcoded because API exposes this as a subfield that does not lend itself to pagination.`

	RuntimeActiveNodes = `
		SELECT COUNT(*) AS active_nodes
		FROM chain.runtime_nodes
		WHERE runtime_id = $1::text`

	RuntimeRoflAppTransactionsFirstLast = `
		WITH
			app_ids AS (
				SELECT UNNEST($2::text[]) AS app_id
			),

			-- Get the first tx_round for each app_id.
			firsts AS (
				SELECT
					a.app_id,
					tx.tx_round AS first_tx_round,
					tx.tx_index AS first_tx_index
				FROM app_ids a
				JOIN LATERAL (
					SELECT tx_round, tx_index
					FROM %[1]s tx
					WHERE tx.runtime = $1::runtime AND tx.app_id = a.app_id
					ORDER BY tx.tx_round ASC, tx.tx_index ASC
					LIMIT 1
				) tx ON true
			),

			-- Get the last tx_round for each app_id.
			lasts AS (
				SELECT
					a.app_id,
					tx.tx_round AS last_tx_round,
					tx.tx_index AS last_tx_index
				FROM app_ids a
				JOIN LATERAL (
					SELECT tx_round, tx_index
					FROM %[1]s tx
					WHERE tx.runtime = $1::runtime AND tx.app_id = a.app_id
					ORDER BY tx.tx_round DESC, tx.tx_index DESC
					LIMIT 1
				) tx ON true
			)

		SELECT
			f.app_id,
			f.first_tx_round,
			f.first_tx_index,
			first_blk.timestamp AS first_tx_time,
			l.last_tx_round,
			l.last_tx_index,
			last_blk.timestamp AS last_tx_time
		FROM
			firsts f
			JOIN lasts l USING (app_id)
			LEFT JOIN chain.runtime_blocks AS first_blk
				ON first_blk.runtime = $1::runtime AND first_blk.round = f.first_tx_round
			LEFT JOIN chain.runtime_blocks AS last_blk
				ON last_blk.runtime = $1::runtime AND last_blk.round = l.last_tx_round`

	RuntimeRoflAppInstances = `
		SELECT
			rak,
			endorsing_node_id,
			endorsing_entity_id,
			rek,
			expiration_epoch,
			extra_keys
		FROM chain.rofl_instances
		WHERE
			runtime = $1::runtime AND
			app_id = $2::text AND
			($3::text IS NULL OR rak = $3::text)
		ORDER BY expiration_epoch DESC
		LIMIT $4::bigint
		OFFSET $5::bigint`

	RuntimeRoflAppInstanceTransactions = `
		WITH filtered AS (
			SELECT
				rel.runtime, rel.tx_round, rel.tx_index
			FROM
				chain.rofl_instance_transactions as rel
			WHERE
				rel.runtime = $1 AND
				rel.app_id = $2::text AND
				($3::text IS NULL OR rel.rak = $3::text) AND
				(
					-- No filtering on method.
					$4::text IS NULL OR
					(
						-- Special case to return are 'likely to be native transfers'.
						('native_transfers' = $4::text AND rel.likely_native_transfer) OR
						-- Special case to return all evm.Calls that are likely not native transfers.
						('evm.Call_no_native' = $4::text AND rel.method = 'evm.Call' AND NOT rel.likely_native_transfer) OR
						-- Regular case.
						(rel.method = $4::text)
					)
				)
			ORDER BY rel.tx_round DESC, rel.tx_index DESC
			LIMIT $5::bigint
			OFFSET $6::bigint
		)` +
		runtimeTxSelect + `
		FROM filtered as rel
		JOIN chain.runtime_transactions AS txs ON
			txs.runtime = rel.runtime AND
			txs.round = rel.tx_round AND
			txs.tx_index = rel.tx_index` +
		runtimeTxCommonJoins + `
		ORDER BY rel.tx_round DESC, rel.tx_index DESC`

	RuntimeRoflmarketProviders = `
		SELECT
			address,
			nodes,
			scheduler,
			payment_address,
			metadata,
			stake,
			offers_next_id,
			offers_count,
			instances_next_id,
			instances_count,
			created_at,
			updated_at,
			removed
		FROM chain.roflmarket_providers
		WHERE runtime = $1::runtime AND
			($2::oasis_addr IS NULL OR address = $2::oasis_addr) AND
			-- Exclude not yet processed providers.
			last_processed_round IS NOT NULL
		-- TODO: Should probably sort by something else.
		ORDER BY address
		LIMIT $3::bigint
		OFFSET $4::bigint`

	RuntimeRoflmarketProviderOffers = `
		SELECT
			id,
			provider,
			resources,
			payment,
			capacity,
			metadata,
			removed
		FROM chain.roflmarket_offers
		WHERE runtime = $1::runtime AND
			provider = $2::oasis_addr
		-- TODO: Should probably sort by something else.
		ORDER BY id DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`

	RuntimeRoflmarketProviderInstances = `
		SELECT
			id,
			provider,
			offer_id,
			status,
			creator,
			admin,
			node_id,
			metadata,
			resources,
			deployment,
			created_at,
			updated_at,
			paid_from,
			paid_until,
			payment,
			payment_address,
			refund_data,
			cmd_next_id,
			cmd_count,
			cmds,
			removed
		FROM chain.roflmarket_instances
		WHERE runtime = $1::runtime AND
			provider = $2::oasis_addr
		-- TODO: Should probably sort by something else.
		ORDER BY id DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`

	// FineTxVolumes returns the fine-grained query for 5-minute sampled tx volume windows.
	FineTxVolumes = `
		SELECT window_end, tx_volume
		FROM stats.min5_tx_volume
		WHERE layer = $1::text
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	// FineDailyTxVolumes returns the query for daily tx volume windows.
	FineDailyTxVolumes = `
		SELECT window_end, tx_volume
		FROM stats.daily_tx_volume
		WHERE layer = $1::text
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	// DailyTxVolumes returns the query for daily sampled daily tx volume windows.
	DailyTxVolumes = `
		SELECT window_end, tx_volume
		FROM stats.daily_tx_volume
		WHERE (layer = $1::text AND (window_end AT TIME ZONE 'UTC')::time = '00:00:00')
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	// FineDailyActiveAccounts returns the fine-grained query for daily active account windows.
	FineDailyActiveAccounts = `
		SELECT window_end, active_accounts
		FROM stats.daily_active_accounts
		WHERE layer = $1::text
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

	// DailyActiveAccounts returns the query for daily sampled daily active account windows.
	DailyActiveAccounts = `
		SELECT window_end, active_accounts
		FROM stats.daily_active_accounts
		WHERE (layer = $1::text AND (window_end AT TIME ZONE 'UTC')::time = '00:00:00')
		ORDER BY
			window_end DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`
)

func escapeLike(s string) string {
	escaped := strings.ReplaceAll(s, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `%`, `\%`)
	escaped = strings.ReplaceAll(escaped, `_`, `\_`)
	return escaped
}

// EVMTokens returns a SQL query  for selecting EVM tokens, optionally filtered
// by a list of token name substrings. Filtering arguments are added to the
// provided argument list.
//
// The query is dynamically constructed to support efficient AND-based substring
// filtering using ILIKE '%term%' per name fragment.
//
// Dynamic query generation is necessary here to allow PostgreSQL to utilize the
// pg_trgm GIN index on `token_name` — index usage is only possible when each
// ILIKE condition is written explicitly.
func EVMTokens(rawNames *[]string, args *[]interface{}) string {
	var clauses []string
	argOffset := len(*args) + 1

	if rawNames != nil {
		for i, name := range *rawNames {
			clauses = append(clauses, fmt.Sprintf("(tokens.token_name ILIKE $%d::text ESCAPE '\\' OR tokens.symbol ILIKE $%d::text ESCAPE '\\')", argOffset+i, argOffset+i))
			*args = append(*args, "%"+escapeLike(name)+"%")
		}
	}

	condition := "TRUE"
	if len(clauses) > 0 {
		condition = "(" + strings.Join(clauses, " AND ") + ")"
	}

	query := fmt.Sprintf(`
		WITH
		-- We use custom ordering groups to prioritize some well known tokens at the top of the list.
		-- Tokens within the same group are ordered by the default logic.
		custom_order AS (
			SELECT token_addr, group_order
			FROM unnest($7::text[], $8::int[]) AS u(token_addr, group_order)
		),
		holders AS (
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
			contracts.verification_level,
			COALESCE(custom_order.group_order, 9999) AS custom_sort_order
		FROM chain.evm_tokens AS tokens
		LEFT JOIN custom_order ON (tokens.token_address = custom_order.token_addr)
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
			%s AND
			($3::int IS NULL OR tokens.token_type = $3::int) AND
			tokens.token_type IS NOT NULL AND -- exclude token _candidates_ that we haven't inspected yet
			tokens.token_type != 0 -- exclude unknown-type tokens; they're often just contracts that emitted Transfer events but don't expose the token ticker, name, balance etc.
		ORDER BY
			custom_sort_order,
			CASE
				-- If sort_by is not "market_cap" then we sort by num_holders (below).
				WHEN $6::text IS NULL OR $6::text != 'market_cap' THEN NULL
				ELSE
				-- Otherwise, sort by market cap.
				(
					CASE
						-- For the reference token itself, it is 1:1 in value with, you know, itself.
						WHEN tokens.token_address = $5 THEN 1.0
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
					END * COALESCE(tokens.total_supply, 0)
				)
			END DESC,
			num_holders DESC,
			contract_addr
		LIMIT $%d::bigint
		OFFSET $%d::bigint`, condition, argOffset+len(clauses), argOffset+len(clauses)+1)

	return query
}

// RuntimeRoflApps returns a SQL query for selecting ROFL apps, optionally
// filtered by a list of metadata name substrings. Filtering arguments are added
// to the provided argument list.
//
// The query is dynamically constructed to support efficient AND-based substring
// filtering using ILIKE '%term%' per name fragment.
//
// Dynamic query generation is necessary here to allow PostgreSQL to utilize the
// pg_trgm GIN index on `metadata_name` — index usage is only possible when each
// ILIKE condition is written explicitly.
func RuntimeRoflApps(rawNames *[]string, args *[]interface{}) string {
	var clauses []string
	argOffset := len(*args) + 1

	if rawNames != nil {
		for i, name := range *rawNames {
			clauses = append(clauses, fmt.Sprintf("ra.metadata_name ILIKE $%d::text ESCAPE '\\'", argOffset+i))
			*args = append(*args, "%"+escapeLike(name)+"%")
		}
	}

	nameCondition := "TRUE"
	if len(clauses) > 0 {
		nameCondition = "(" + strings.Join(clauses, " AND ") + ")"
	}
	query := fmt.Sprintf(`
		WITH
			-- Latest epoch, to identify active instances.
			max_epoch AS (
				SELECT id FROM chain.epochs ORDER BY id DESC LIMIT 1
			),

			-- Aggregate active instance data per app.
			active_instances AS (
				SELECT
					ri.app_id,
					COUNT(*) AS num_active_instances,
					jsonb_agg(
						jsonb_build_object(
						'rak', ri.rak,
						'endorsing_node_id', ri.endorsing_node_id,
						'endorsing_entity_id', ri.endorsing_entity_id,
						'rek', ri.rek,
						'expiration_epoch', ri.expiration_epoch,
						'extra_keys', ri.extra_keys
						) ORDER BY ri.expiration_epoch DESC
					) AS instance_json
				FROM chain.rofl_instances ri
				JOIN max_epoch ON true
				WHERE ri.expiration_epoch > max_epoch.id
				GROUP BY ri.app_id
			)

		SELECT
			ra.id,
			ra.admin,
			preimages.context_identifier,
			preimages.context_version,
			preimages.address_data,
			ra.stake,
			ra.policy,
			ra.sek,
			ra.metadata,
			ra.secrets,
			ra.removed,
			COALESCE(ai.num_active_instances, 0) as num_active_instances,
			COALESCE(ai.instance_json, '[]'::jsonb) AS active_instances
		FROM chain.rofl_apps AS ra

		-- Resolve admin address preimage.
		LEFT JOIN chain.address_preimages AS preimages ON (
			preimages.address = ra.admin AND
			-- For now, the only user is the explorer, where we only care
			-- about Ethereum-compatible addresses, so only get those. Can
			-- easily enable for other address types though.
			preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
			preimages.context_version = 0
		)

		-- Join aggregated active instance data.
		LEFT JOIN active_instances ai ON ai.app_id = ra.id

		LEFT JOIN chain.accounts AS a ON a.address = ra.admin

		WHERE
			ra.runtime = $1::runtime AND
			($2::text IS NULL OR ra.id = $2::text) AND
			($3::oasis_addr IS NULL OR ra.admin = $3::oasis_addr) AND
			%s AND
			-- Exclude not yet processed apps.
			ra.last_processed_round IS NOT NULL

		ORDER BY num_active_instances DESC, ra.num_transactions DESC, ra.id DESC
		LIMIT $%d::bigint
		OFFSET $%d::bigint`, nameCondition, argOffset+len(clauses), argOffset+len(clauses)+1)

	return query
}
