package v1

import (
	"fmt"
)

// QueryFactory is a convenience type for creating API queries.
type QueryFactory struct {
	chainID string
}

func NewQueryFactory(chainID string) QueryFactory {
	return QueryFactory{chainID}
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
		ORDER BY
			$5::text
		DESC
		LIMIT $6::bigint
		OFFSET $7::bigint`, qf.chainID)
}

func (qf QueryFactory) BlockQuery() string {
	return fmt.Sprintf(`
		SELECT height, block_hash, time
			FROM %s.blocks
			WHERE height = $1::bigint`, qf.chainID)
}

func (qf QueryFactory) TransactionsQuery() string {
	return fmt.Sprintf(`
		SELECT block, txn_hash, sender, nonce, fee_amount, method, body, code
			FROM %s.transactions
			WHERE ($1::bigint IS NULL OR block = $1::bigint) AND
						($2::text IS NULL OR method = $2::text) AND
						($3::text IS NULL OR sender = $3::text) AND
						($4::bigint IS NULL OR fee_amount >= $4::bigint) AND
						($5::bigint IS NULL OR fee_amount <= $5::bigint) AND
						($6::bigint IS NULL OR code = $6::bigint)
		ORDER BY
			$7::text
		DESC
		LIMIT $8::bigint
		OFFSET $9::bigint`, qf.chainID)
}

func (qf QueryFactory) TransactionQuery() string {
	return fmt.Sprintf(`
		SELECT block, txn_hash, sender, nonce, fee_amount, method, body, code
			FROM %s.transactions
			WHERE txn_hash = $1::text`, qf.chainID)
}

func (qf QueryFactory) EntitiesQuery() string {
	return fmt.Sprintf(`
		SELECT id, address
			FROM %s.entities
		ORDER BY
			$1::text
		DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
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
		ORDER BY
			$2::text
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, qf.chainID)
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
		ORDER BY
			$9::text
		DESC
		LIMIT $10::bigint
		OFFSET $11::bigint`, qf.chainID)
}

func (qf QueryFactory) AccountQuery() string {
	return fmt.Sprintf(`
		SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding,
			(
				SELECT COALESCE(ROUND(SUM(shares * escrow_balance_active / escrow_total_shares_active)), 0) AS delegations_balance
				FROM %[1]s.delegations
				JOIN %[1]s.accounts ON %[1]s.accounts.address = %[1]s.delegations.delegatee
				WHERE delegator = $1::text
			) AS delegations_balance,
			(
				SELECT COALESCE(ROUND(SUM(shares * escrow_balance_debonding / escrow_total_shares_debonding)), 0) AS debonding_delegations_balance
				FROM %[1]s.debonding_delegations
				JOIN %[1]s.accounts ON %[1]s.accounts.address = %[1]s.debonding_delegations.delegatee
				WHERE delegator = $1::text
			) AS debonding_delegations_balance
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
		ORDER BY
			$2::text
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, qf.chainID)
}

func (qf QueryFactory) DebondingDelegationsQuery() string {
	return fmt.Sprintf(`
		SELECT delegatee, shares, debond_end, escrow_balance_debonding, escrow_total_shares_debonding
			FROM %[1]s.debonding_delegations
			JOIN %[1]s.accounts ON %[1]s.debonding_delegations.delegatee = %[1]s.accounts.address
			WHERE delegator = $1::text
		ORDER BY
			$2::text
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, qf.chainID)
}

func (qf QueryFactory) EpochsQuery() string {
	return fmt.Sprintf(`
		SELECT id, start_height, end_height
			FROM %s.epochs
		ORDER BY
			$1::text
		DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
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
		ORDER BY
			$3::text
		DESC
		LIMIT $4::bigint
		OFFSET $5::bigint`, qf.chainID)
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
		ORDER BY
			$2::text
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, qf.chainID)
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
		ORDER BY
			$1::text
		DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, qf.chainID)
}
