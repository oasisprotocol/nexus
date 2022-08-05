package v1

import (
	"fmt"

	"github.com/iancoleman/strcase"
)

func makeStatusQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT height, processed_time
			FROM %s.processed_blocks
		ORDER BY processed_time DESC
		LIMIT 1`, strcase.ToSnake(LatestChainID))
}

func makeBlocksQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT height, block_hash, time
			FROM %s.blocks
			WHERE ($1::bigint IS NULL OR height >= $1::bigint) AND
						($2::bigint IS NULL OR height <= $2::bigint) AND
						($3::timestamptz IS NULL OR time >= $3::timestamptz) AND
						($4::timestampz IS NULL OR time <= $4::timestamptz)
		ORDER BY
			CASE
				WHEN $5::text IS NULL THEN 1
				ELSE $5::text
			END
		DESC
		LIMIT $6::bigint
		OFFSET $7::bigint`, chainID)
}

func makeBlockQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT height, block_hash, time
			FROM %s.blocks
			WHERE height = $1::bigint`, chainID)
}

func makeTransactionsQuery(chainID string) string {
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
			CASE
				WHEN $7::text IS NULL THEN 1
				ELSE $7::text
			END
		DESC
		LIMIT $8::bigint
		OFFSET $9::bigint`, chainID)
}

func makeTransactionQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT block, txn_hash, sender, nonce, fee_amount, method, body, code
			FROM %s.transactions
			WHERE txn_hash = $1::text`, chainID)
}

func makeEntitiesQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, address
			FROM %s.entities
		ORDER BY
			CASE
				WHEN $1::text IS NULL THEN 1
				ELSE $1::text
			END
		DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, chainID)
}

func makeEntityQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, address
			FROM %s.entities
			WHERE id = $1::text`, chainID)
}

func makeEntityNodeIdsQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id
			FROM %s.nodes
			WHERE entity_id = $1::text`, chainID)
}

func makeEntityNodesQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM %s.nodes
			WHERE entity_id = $1::text
		ORDER BY
			CASE
				WHEN $2::text IS NULL THEN 1
				ELSE $2::text
			END
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, chainID)
}

func makeEntityNodeQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles
			FROM %s.nodes
			WHERE entity_id = $1::text AND id = $2::text`, chainID)
}

func makeAccountsQuery(chainID string) string {
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
			CASE
				WHEN $9::text IS NULL THEN 1
				ELSE $9::text
			END
		DESC
		LIMIT $10::bigint
		OFFSET $11::bigint`, chainID)
}

func makeAccountQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
			FROM %s.accounts
			WHERE address = $1::text`, chainID)
}

func makeAccountAllowancesQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT beneficiary, allowance
			FROM %s.allowances
			WHERE owner = $1::text`, chainID)
}

func makeDelegationsQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT delegatee, shares, escrow_balance_active, escrow_total_shares_active
			FROM %[1]s.delegations
			JOIN %[1]s.accounts ON %[1]s.delegations.delegatee = %[1]s.accounts.address
			WHERE delegator = $1::text
		ORDER BY
			CASE
				WHEN $2::text IS NULL THEN 1
				ELSE $2::text
			END
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, chainID)
}

func makeDebondingDelegationsQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT delegatee, shares, debond_end, escrow_balance_debonding, escrow_total_shares_debonding
			FROM %[1]s.debonding_delegations
			JOIN %[1]s.accounts ON %[1]s.debonding_delegations.delegatee = %[1]s.accounts.address
			WHERE delegator = $1::text
		ORDER BY
			CASE
				WHEN $2::text IS NULL THEN 1
				ELSE $2::text
			END
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, chainID)
}

func makeEpochsQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, start_height, end_height
			FROM %s.epochs
		ORDER BY
			CASE
				WHEN $1::text IS NULL THEN 1
				ELSE $1::text
			END
		DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, chainID)
}

func makeEpochQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, start_height, end_height
			FROM %s.epochs
			WHERE id = $1::bigint`, chainID)
}

func makeProposalsQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM %s.proposals
			WHERE ($1::text IS NULL OR submitter = $1::text) AND
						($2::text IS NULL OR state = $2::text)
		ORDER BY
			CASE
				WHEN $3::text IS NULL THEN 1
				ELSE $3::text
			END
		DESC
		LIMIT $4::bigint
		OFFSET $5::bigint`, chainID)
}

func makeProposalQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version,
				upgrade_epoch, cancels, created_at, closes_at, invalid_votes
			FROM %s.proposals
			WHERE id = $1::bigint`, chainID)
}

func makeProposalVotesQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT voter, vote
			FROM %s.votes
			WHERE proposal = $1::bigint
		ORDER BY
			CASE
				WHEN $2::text IS NULL THEN 1
				ELSE $2::text
			END
		DESC
		LIMIT $3::bigint
		OFFSET $4::bigint`, chainID)
}

func makeValidatorQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, start_height
			FROM %s.epochs
			ORDER BY id DESC
			LIMIT 1`, chainID)
}

func makeValidatorDataQuery(chainID string) string {
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
			WHERE %[1]s.entities.address = $1::text`, chainID)
}

func makeValidatorsQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT id, start_height
			FROM %s.epochs
			ORDER BY id DESC`, chainID)
}

func makeValidatorsDataQuery(chainID string) string {
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
			CASE
				WHEN $1::text IS NULL THEN 1
				ELSE $1::text
			END
		DESC
		LIMIT $2::bigint
		OFFSET $3::bigint`, chainID)
}
