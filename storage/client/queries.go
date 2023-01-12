package client

import (
	"fmt"
)

// QueryFactory is a convenience type for creating API queries.
type QueryFactory struct {
	chainID string
	runtime string
}

func NewQueryFactory(chainID string, runtime string) QueryFactory {
	return QueryFactory{chainID, runtime}
}

func (qf QueryFactory) StatusQuery() string {
	return fmt.Sprintf(`
		SELECT height, processed_time
			FROM %s.processed_blocks
		ORDER BY processed_time DESC
		LIMIT 1`, qf.chainID)
}

func (qf QueryFactory) BlocksQuery() string {
	return fmt.Sprintf(`
		SELECT height, block_hash, time
			FROM %s.blocks
			WHERE ($1::bigint IS NULL OR height >= $1::bigint) AND
						($2::bigint IS NULL OR height <= $2::bigint) AND
						($3::timestamptz IS NULL OR time >= $3::timestamptz) AND
						($4::timestamptz IS NULL OR time <= $4::timestamptz)
		ORDER BY height DESC
		LIMIT $5::bigint
		OFFSET $6::bigint`, qf.chainID)
}

func (qf QueryFactory) BlockQuery() string {
	return fmt.Sprintf(`
		SELECT height, block_hash, time
			FROM %s.blocks
			WHERE height = $1::bigint`, qf.chainID)
}

func (qf QueryFactory) TransactionsQuery() string {
	return fmt.Sprintf(`
		SELECT 
				%[1]s.transactions.block as block,
				%[1]s.transactions.tx_hash as tx_hash,
				%[1]s.transactions.sender as sender,
				%[1]s.transactions.nonce as nonce,
				%[1]s.transactions.fee_amount as fee_amount,
				%[1]s.transactions.method as method,
				%[1]s.transactions.body as body,
				%[1]s.transactions.code as code
			FROM %[1]s.transactions
			LEFT JOIN %[1]s.accounts_related_transactions ON %[1]s.transactions.block = %[1]s.accounts_related_transactions.tx_block 
				AND %[1]s.transactions.tx_index = %[1]s.accounts_related_transactions.tx_index
				AND $4::text IS NOT NULL
				AND %[1]s.accounts_related_transactions.account_address = $4::text
			WHERE ($1::bigint IS NULL OR %[1]s.transactions.block = $1::bigint) AND
					($2::text IS NULL OR %[1]s.transactions.method = $2::text) AND
					($3::text IS NULL OR %[1]s.transactions.sender = $3::text) AND
					($4::text IS NULL OR %[1]s.accounts_related_transactions.account_address = $4::text) AND
					($5::bigint IS NULL OR %[1]s.transactions.fee_amount >= $5::bigint) AND
					($6::bigint IS NULL OR %[1]s.transactions.fee_amount <= $6::bigint) AND
					($7::bigint IS NULL OR %[1]s.transactions.code = $7::bigint)
			ORDER BY %[1]s.transactions.block DESC, %[1]s.transactions.tx_index
			LIMIT $8::bigint
			OFFSET $9::bigint`, qf.chainID)
}

func (qf QueryFactory) TransactionQuery() string {
	return fmt.Sprintf(`
		SELECT block, tx_hash, sender, nonce, fee_amount, method, body, code
			FROM %s.transactions
			WHERE tx_hash = $1::text`, qf.chainID)
}

func (qf QueryFactory) EventsQuery() string {
	return fmt.Sprintf(`
		SELECT tx_block, tx_index, tx_hash, type, body
			FROM %s.events
			WHERE ($1::bigint IS NULL OR tx_block = $1::bigint) AND
					($2::integer IS NULL OR tx_index = $2::integer) AND
					($3::text IS NULL OR tx_hash = $3::text) AND
					($4::text IS NULL OR type = $4::text)
			ORDER BY tx_block DESC, tx_index
			LIMIT $5::bigint
			OFFSET $6::bigint`, qf.chainID)
}

func (qf QueryFactory) EventsRelAccountsQuery() string {
	return fmt.Sprintf(`
		SELECT event_block, tx_index, tx_hash, type, body
			FROM %s.accounts_related_events
			WHERE (account_address = $1::text) AND
					($2::bigint IS NULL OR event_block = $1::bigint) AND
					($3::integer IS NULL OR tx_index = $3::integer) AND
					($4::text IS NULL OR tx_hash = $4::text) AND
					($5::text IS NULL OR type = $5::text)
			ORDER BY event_block DESC, tx_index
			LIMIT $6::bigint
			OFFSET $7::bigint`, qf.chainID)
}

func (qf QueryFactory) EntitiesQuery() string {
	return fmt.Sprintf(`
		SELECT id, address
			FROM %s.entities
		LIMIT $1::bigint
		OFFSET $2::bigint`, qf.chainID)
}

func (qf QueryFactory) EntityQuery() string {
	return fmt.Sprintf(`
		SELECT id, address
			FROM %s.entities
			WHERE id = $1::text`, qf.chainID)
}

func (qf QueryFactory) EntityNodeIdsQuery() string {
	return fmt.Sprintf(`
		SELECT id
			FROM %s.nodes
			WHERE entity_id = $1::text`, qf.chainID)
}

func (qf QueryFactory) EntityNodesQuery() string {
	return fmt.Sprintf(`
		SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM %s.nodes
			WHERE entity_id = $1::text
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
}

func (qf QueryFactory) EntityNodeQuery() string {
	return fmt.Sprintf(`
		SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM %s.nodes
			WHERE entity_id = $1::text AND id = $2::text`, qf.chainID)
}

func (qf QueryFactory) AccountsQuery() string {
	return fmt.Sprintf(`
		SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
			FROM %s.accounts
			WHERE ($1::bigint IS NULL OR general_balance >= $1::bigint) AND
						($2::bigint IS NULL OR general_balance <= $2::bigint) AND
						($3::bigint IS NULL OR escrow_balance_active >= $3::bigint) AND
						($4::bigint IS NULL OR escrow_balance_active <= $4::bigint) AND
						($5::bigint IS NULL OR escrow_balance_debonding >= $5::bigint) AND
						($6::bigint IS NULL OR escrow_balance_debonding <= $6::bigint) AND
						($7::bigint IS NULL OR general_balance + escrow_balance_active + escrow_balance_debonding >= $7::bigint) AND
						($8::bigint IS NULL OR general_balance + escrow_balance_active + escrow_balance_debonding <= $8::bigint)
		LIMIT $9::bigint
		OFFSET $10::bigint`, qf.chainID)
}

func (qf QueryFactory) AccountQuery() string {
	return fmt.Sprintf(`
		SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding,
			COALESCE (
				(SELECT COALESCE(ROUND(SUM(shares * escrow_balance_active / escrow_total_shares_active)), 0) AS delegations_balance
				FROM %[1]s.delegations
				JOIN %[1]s.accounts ON %[1]s.accounts.address = %[1]s.delegations.delegatee
				WHERE delegator = $1::text AND escrow_total_shares_active != 0)
			, 0) AS delegations_balance,
			COALESCE (
				(SELECT COALESCE(ROUND(SUM(shares * escrow_balance_debonding / escrow_total_shares_debonding)), 0) AS debonding_delegations_balance
				FROM %[1]s.debonding_delegations
				JOIN %[1]s.accounts ON %[1]s.accounts.address = %[1]s.debonding_delegations.delegatee
				WHERE delegator = $1::text AND escrow_total_shares_debonding != 0)
			, 0) AS debonding_delegations_balance
			FROM %[1]s.accounts
			WHERE address = $1::text`, qf.chainID)
}

func (qf QueryFactory) AccountAllowancesQuery() string {
	return fmt.Sprintf(`
		SELECT beneficiary, allowance
			FROM %s.allowances
			WHERE owner = $1::text`, qf.chainID)
}

func (qf QueryFactory) DelegationsQuery() string {
	return fmt.Sprintf(`
		SELECT delegatee, shares, escrow_balance_active, escrow_total_shares_active
			FROM %[1]s.delegations
			JOIN %[1]s.accounts ON %[1]s.delegations.delegatee = %[1]s.accounts.address
			WHERE delegator = $1::text
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
}

func (qf QueryFactory) DebondingDelegationsQuery() string {
	return fmt.Sprintf(`
		SELECT delegatee, shares, debond_end, escrow_balance_debonding, escrow_total_shares_debonding
			FROM %[1]s.debonding_delegations
			JOIN %[1]s.accounts ON %[1]s.debonding_delegations.delegatee = %[1]s.accounts.address
			WHERE delegator = $1::text
		ORDER BY debond_end
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
}

func (qf QueryFactory) EpochsQuery() string {
	return fmt.Sprintf(`
		SELECT id, start_height, end_height
			FROM %s.epochs
		ORDER BY id DESC
		LIMIT $1::bigint
		OFFSET $2::bigint`, qf.chainID)
}

func (qf QueryFactory) EpochQuery() string {
	return fmt.Sprintf(`
		SELECT id, start_height, end_height
			FROM %s.epochs
			WHERE id = $1::bigint`, qf.chainID)
}

func (qf QueryFactory) ProposalsQuery() string {
	return fmt.Sprintf(`
		SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM %s.proposals
			WHERE ($1::text IS NULL OR submitter = $1::text) AND
						($2::text IS NULL OR state = $2::text)
		ORDER BY id DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, qf.chainID)
}

func (qf QueryFactory) ProposalQuery() string {
	return fmt.Sprintf(`
		SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM %s.proposals
			WHERE id = $1::bigint`, qf.chainID)
}

func (qf QueryFactory) ProposalVotesQuery() string {
	return fmt.Sprintf(`
		SELECT voter, vote
			FROM %s.votes
			WHERE proposal = $1::bigint
		ORDER BY proposal DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
}

func (qf QueryFactory) ValidatorQuery() string {
	return fmt.Sprintf(`
		SELECT id, start_height
			FROM %s.epochs
		ORDER BY id DESC
		LIMIT 1`, qf.chainID)
}

func (qf QueryFactory) ValidatorDataQuery() string {
	return fmt.Sprintf(`
		SELECT
				%[1]s.entities.id AS entity_id,
				%[1]s.entities.address AS entity_address,
				%[1]s.nodes.id AS node_address,
				%[1]s.accounts.escrow_balance_active AS escrow,
				%[1]s.commissions.schedule AS commissions_schedule,
				CASE WHEN EXISTS(SELECT null FROM %[1]s.nodes WHERE %[1]s.entities.id = %[1]s.nodes.entity_id AND voting_power > 0) THEN true ELSE false END AS active,
				CASE WHEN EXISTS(SELECT null FROM %[1]s.nodes WHERE %[1]s.entities.id = %[1]s.nodes.entity_id AND %[1]s.nodes.roles like '%%validator%%') THEN true ELSE false END AS status,
				%[1]s.entities.meta AS meta
			FROM %[1]s.entities
			JOIN %[1]s.accounts ON %[1]s.entities.address = %[1]s.accounts.address
			LEFT JOIN %[1]s.commissions ON %[1]s.entities.address = %[1]s.commissions.address
			JOIN %[1]s.nodes ON %[1]s.entities.id = %[1]s.nodes.entity_id
				AND %[1]s.nodes.roles like '%%validator%%'
				AND %[1]s.nodes.voting_power = (
					SELECT max(voting_power)
					FROM %[1]s.nodes
					WHERE %[1]s.entities.id = %[1]s.nodes.entity_id
						AND %[1]s.nodes.roles like '%%validator%%'
				)
			WHERE %[1]s.entities.address = $1::text`, qf.chainID)
}

func (qf QueryFactory) ValidatorsQuery() string {
	return fmt.Sprintf(`
		SELECT id, start_height
			FROM %s.epochs
			ORDER BY id DESC`, qf.chainID)
}

func (qf QueryFactory) ValidatorsDataQuery() string {
	return fmt.Sprintf(`
		SELECT
				%[1]s.entities.id AS entity_id,
				%[1]s.entities.address AS entity_address,
				%[1]s.nodes.id AS node_address,
				%[1]s.accounts.escrow_balance_active AS escrow,
				%[1]s.commissions.schedule AS commissions_schedule,
				CASE WHEN EXISTS(SELECT NULL FROM %[1]s.nodes WHERE %[1]s.entities.id = %[1]s.nodes.entity_id AND voting_power > 0) THEN true ELSE false END AS active,
				CASE WHEN EXISTS(SELECT NULL FROM %[1]s.nodes WHERE %[1]s.entities.id = %[1]s.nodes.entity_id AND %[1]s.nodes.roles like '%%validator%%') THEN true ELSE false END AS status,
				%[1]s.entities.meta AS meta
			FROM %[1]s.entities
			JOIN %[1]s.accounts ON %[1]s.entities.address = %[1]s.accounts.address
			LEFT JOIN %[1]s.commissions ON %[1]s.entities.address = %[1]s.commissions.address
			JOIN %[1]s.nodes ON %[1]s.entities.id = %[1]s.nodes.entity_id
				AND %[1]s.nodes.roles like '%%validator%%'
				AND %[1]s.nodes.voting_power = (
					SELECT max(voting_power)
					FROM %[1]s.nodes
					WHERE %[1]s.entities.id = %[1]s.nodes.entity_id
						AND %[1]s.nodes.roles like '%%validator%%'
				)
		ORDER BY escrow_balance_active DESC
		LIMIT $1::bigint
		OFFSET $2::bigint`, qf.chainID)
}

func (qf QueryFactory) RuntimeBlocksQuery() string {
	return fmt.Sprintf(`
		SELECT round, block_hash, timestamp, num_transactions, size, gas_used
			FROM %s.%s_rounds
			WHERE ($1::bigint IS NULL OR round >= $1::bigint) AND
						($2::bigint IS NULL OR round <= $2::bigint) AND
						($3::timestamptz IS NULL OR timestamp >= $3::timestamptz) AND
						($4::timestamptz IS NULL OR timestamp <= $4::timestamptz)
		ORDER BY round DESC
		LIMIT $5::bigint
		OFFSET $6::bigint`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeTransactionsQuery() string {
	return fmt.Sprintf(`
		SELECT round, tx_index, tx_hash, tx_eth_hash, raw, result_raw
			FROM %s.%s_transactions
			WHERE $1::bigint IS NULL OR round = $1::bigint
		ORDER BY round DESC, tx_index DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeTokensQuery() string {
	return fmt.Sprintf(`
		SELECT token_address, COUNT(*) AS num_holders
			FROM %s.%s_token_balances
		GROUP BY token_address
		ORDER BY num_holders DESC
		LIMIT $1::bigint
		OFFSET $2::bigint`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) FineTxVolumesQuery() string {
	return `
		SELECT window_start, tx_volume
			FROM stats.min5_tx_volume
		ORDER BY
			window_start DESC
		LIMIT $1::bigint
		OFFSET $2::bigint
	`
}

func (qf QueryFactory) TxVolumesQuery() string {
	return `
		SELECT window_start, tx_volume
			FROM stats.daily_tx_volume
		ORDER BY
			window_start DESC
		LIMIT $1::bigint
		OFFSET $2::bigint
	`
}
