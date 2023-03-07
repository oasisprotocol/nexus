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
			FROM chain.processed_blocks
		ORDER BY processed_time DESC
		LIMIT 1`

	Blocks = `
		SELECT height, block_hash, time, num_txs
			FROM chain.blocks
			WHERE ($1::bigint IS NULL OR height >= $1::bigint) AND
						($2::bigint IS NULL OR height <= $2::bigint) AND
						($3::timestamptz IS NULL OR time >= $3::timestamptz) AND
						($4::timestamptz IS NULL OR time <= $4::timestamptz)
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
					($7::bigint IS NULL OR chain.transactions.code = $7::bigint)
			ORDER BY chain.transactions.block DESC, chain.transactions.tx_index
			LIMIT $8::bigint
			OFFSET $9::bigint`

	Transaction = `
		SELECT block, tx_index, tx_hash, sender, nonce, fee_amount, method, body, code, chain.blocks.time
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

	EventsRelAccounts = `
		SELECT event_block, tx_index, tx_hash, type, body
			FROM chain.accounts_related_events
			WHERE (account_address = $1::text) AND
					($2::bigint IS NULL OR event_block = $1::bigint) AND
					($3::integer IS NULL OR tx_index = $3::integer) AND
					($4::text IS NULL OR tx_hash = $4::text) AND
					($5::text IS NULL OR type = $5::text)
			ORDER BY event_block DESC, tx_index
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
		ORDER BY delegatee, shares
		LIMIT $2::bigint
		OFFSET $3::bigint`

	DebondingDelegations = `
		SELECT delegatee, shares, debond_end, escrow_balance_debonding, escrow_total_shares_debonding
			FROM chain.debonding_delegations
			JOIN chain.accounts ON chain.debonding_delegations.delegatee = chain.accounts.address
			WHERE delegator = $1::text
		ORDER BY debond_end
		LIMIT $2::bigint
		OFFSET $3::bigint`

	Epochs = `
		SELECT id, start_height, end_height
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
				COALESCE(chain.commissions.schedule, '{}'::json) AS commissions_schedule,
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
				COALESCE(chain.commissions.schedule, '{}'::json) AS commissions_schedule,
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
						($5::timestamptz IS NULL OR timestamp <= $5::timestamptz)
		ORDER BY round DESC
		LIMIT $6::bigint
		OFFSET $7::bigint`

	RuntimeTransactions = `
		SELECT
			txs.round,
			txs.tx_index,
			txs.tx_hash,
			txs.tx_eth_hash,
			txs.gas_used,
			txs.size,
			txs.timestamp,
			txs.raw,
			txs.result_raw,
			(
				SELECT
					json_object_agg(pre.address, encode(pre.address_data, 'hex'))
				FROM chain.runtime_related_transactions AS rel
				JOIN chain.address_preimages AS pre ON rel.account_address = pre.address
				WHERE txs.runtime = rel.runtime
					AND txs.round = rel.tx_round
					AND txs.tx_index = rel.tx_index
			) AS eth_addr_lookup
		FROM chain.runtime_transactions AS txs
		LEFT JOIN chain.runtime_related_transactions AS rel ON txs.round = rel.tx_round
			AND txs.tx_index = rel.tx_index
			AND txs.runtime = rel.runtime
			-- When related_address ($4) is NULL and hence we do no filtering on it, avoid the join altogether.
			-- Otherwise, every tx will be returned as many times as there are related addresses for it.
			AND $4::text IS NOT NULL
		WHERE (txs.runtime = $1) AND
					($2::bigint IS NULL OR txs.round = $2::bigint) AND
					($3::text IS NULL OR txs.tx_hash = $3::text OR txs.tx_eth_hash = $3::text) AND
					($4::text IS NULL OR rel.account_address = $4::text)
		ORDER BY txs.round DESC, txs.tx_index DESC
		LIMIT $5::bigint
		OFFSET $6::bigint
		`

	RuntimeEvents = `
		SELECT round, tx_index, tx_hash, type, body, evm_log_name, evm_log_params
			FROM chain.runtime_events
			WHERE (runtime = $1) AND
					($2::bigint IS NULL OR round = $2::bigint) AND
					($3::integer IS NULL OR tx_index = $3::integer) AND
					($4::text IS NULL OR tx_hash = $4::text) AND
					($5::text IS NULL OR type = $5::text) AND
					($6::text IS NULL OR evm_log_signature = $6::text) AND
					($7::text IS NULL OR related_accounts @> ARRAY[$7::text])
			ORDER BY round DESC, tx_index
			LIMIT $8::bigint
			OFFSET $9::bigint`

	AddressPreimage = `
		SELECT context_identifier, context_version, address_data
			FROM chain.address_preimages
			WHERE address = $1::text`

	RuntimeAccountStats = `
		SELECT
			COALESCE (
				(SELECT sum(amount) from chain.runtime_transfers where sender=$1::text)
				, 0) AS total_sent,
			COALESCE (
				(SELECT sum(amount) from chain.runtime_transfers where receiver=$1::text)
				, 0) AS total_received,
			COALESCE (
				(SELECT count(*) from chain.runtime_related_transactions where account_address=$1::text)
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
			encode(preimages.address_data, 'hex') as eth_contract_addr,
			tokens.token_name AS name,
			tokens.symbol,
			tokens.decimals,
			tokens.total_supply,
			CASE
				WHEN tokens.token_type = 20 THEN 'ERC20'
			END AS type,
			holders.cnt AS num_holders
		FROM chain.evm_tokens AS tokens
		JOIN chain.address_preimages AS preimages ON (token_address = preimages.address)
		JOIN holders USING (token_address)
		WHERE (tokens.runtime = $1)
		ORDER BY num_holders DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`

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
			tokens.symbol AS token_symbol,
			tokens.symbol AS token_name,
			'ERC20' AS token_type,  -- TODO: fetch from the table once available
			tokens.decimals AS token_decimals
		FROM chain.evm_token_balances AS balances
		JOIN chain.evm_tokens         AS tokens USING (runtime, token_address)
		WHERE runtime = $1 AND
			account_address = $2::text AND
			balance != 0
		ORDER BY balance DESC
		LIMIT 1000  -- To prevent huge responses. Hardcoded because API exposes this as a subfield that does not lend itself to pagination.
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
