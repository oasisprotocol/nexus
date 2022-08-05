package consensus

import (
	"fmt"
)

func makeLatestBlockQuery(chainID string) string {
	return fmt.Sprintf(`
		SELECT height FROM %s.processed_blocks
			WHERE analyzer = $1
		ORDER BY height DESC
		LIMIT 1`, chainID)
}

func makeIndexingProgressQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP)`, chainID)
}

func makeBlockInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.blocks (height, block_hash, time, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, chainID)
}

func makeEpochInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.epochs (id, start_height)
			VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING`, chainID)
}

func makeEpochUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.epochs
		SET end_height = $2
			WHERE id = $1 AND end_height IS NULL`, chainID)
}

func makeTransactionInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.transactions (block, txn_hash, txn_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, chainID)
}

func makeAccountNonceUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			nonce = $2
		WHERE address = $1`, chainID)
}

func makeCommissionsUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.commissions (address, schedule)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET
				schedule = excluded.schedule`, chainID)
}

func makeEventInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.events (backend, type, body, txn_block, txn_hash, txn_index)
			VALUES ($1, $2, $3, $4, $5, $6)`, chainID)
}

func makeRuntimeUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.runtimes (id, suspended, kind, tee_hardware, key_manager)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO
			UPDATE SET
				suspended = excluded.suspended,
				kind = excluded.kind,
				tee_hardware = excluded.tee_hardware,
				key_manager = excluded.key_manager`, chainID)
}

func makeRuntimeSuspensionQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.runtimes
			SET suspended = true
			WHERE id = $1`, chainID)
}

func makeRuntimeUnsuspensionQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.runtimes
			SET suspended = false
			WHERE id = $1`, chainID)
}

func makeClaimedNodeInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.claimed_nodes (entity_id, node_id) VALUES ($1, $2)
			ON CONFLICT (entity_id, node_id) DO NOTHING`, chainID)
}

func makeEntityUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.entities (id, address) VALUES ($1, $2)
			ON CONFLICT (id) DO
			UPDATE SET
				address = excluded.address`, chainID)
}

func makeNodeUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.nodes (id, entity_id, expiration, tls_pubkey, tls_next_pubkey, tls_addresses, p2p_pubkey, p2p_addresses, consensus_pubkey, consensus_address, vrf_pubkey, roles, software_version, voting_power)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (id) DO UPDATE
		SET
			entity_id = excluded.entity_id,
			expiration = excluded.expiration,
			tls_pubkey = excluded.tls_pubkey,
			tls_next_pubkey = excluded.tls_next_pubkey,
			tls_addresses = excluded.tls_addresses,
			p2p_pubkey = excluded.p2p_pubkey,
			p2p_addresses = excluded.p2p_addresses,
			consensus_pubkey = excluded.consensus_pubkey,
			consensus_address = excluded.consensus_address,
			vrf_pubkey = excluded.vrf_pubkey,
			roles = excluded.roles,
			software_version = excluded.software_version,
			voting_power = excluded.voting_power`, chainID)
}

func makeNodeDeleteQuery(chainID string) string {
	return fmt.Sprintf(`
		DELETE FROM %s.nodes WHERE id = $1`, chainID)
}

func makeEntityMetaUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.entities
		SET meta = $2
			WHERE id = $1`, chainID)
}

func makeSenderUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, chainID)
}

func makeReceiverUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address, general_balance)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET general_balance = %[1]s.accounts.general_balance + $2;`, chainID)
}

func makeBurnUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, chainID)
}

func makeAddGeneralBalanceUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, chainID)
}

func makeAddEscrowBalanceUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address, escrow_balance_active, escrow_total_shares_active)
			VALUES ($1, $2, $3)
		ON CONFLICT (address) DO
			UPDATE SET
				escrow_balance_active = %[1]s.accounts.escrow_balance_active + $2,
				escrow_total_shares_active = %[1]s.accounts.escrow_total_shares_active + $3`, chainID)
}

func makeAddDelegationsUpsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.delegations (delegatee, delegator, shares)
			VALUES ($1, $2, $3)
		ON CONFLICT (delegatee, delegator) DO
			UPDATE SET shares = %[1]s.delegations.shares + $3`, chainID)
}

func makeTakeEscrowUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET escrow_balance_active = escrow_balance_active - $2
			WHERE address = $1`, chainID)
}

func makeDebondingStartEscrowBalanceUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_active = escrow_balance_active - $2,
				escrow_balance_debonding = escrow_balance_debonding + $2
			WHERE address = $1`, chainID)
}

func makeDebondingStartDelegationsUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.delegations
			SET shares = shares - $3
				WHERE delegatee = $1 AND delegator = $2`, chainID)
}

func makeDebondingStartDebondingDelegationsInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.debonding_delegations (delegatee, delegator, shares, debond_end)
			VALUES ($1, $2, $3, $4)`, chainID)
}

func makeReclaimGeneralBalanceUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				general_balance = general_balance + $2
			WHERE address = $1`, chainID)
}

func makeReclaimEscrowBalanceUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_debonding = escrow_balance_debonding - $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding - $3
			WHERE address = $1`, chainID)
}

func makeAllowanceChangeDeleteQuery(chainID string) string {
	return fmt.Sprintf(`
		DELETE FROM %s.allowances
			WHERE owner = $1 AND beneficiary = $2`, chainID)
}

func makeAllowanceChangeUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		DELETE FROM %s.allowances
			WHERE owner = $1 AND beneficiary = $2`, chainID)
}

func makeValidatorNodeUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.nodes SET voting_power = $2
			WHERE id = $1`, chainID)
}

func makeCommitteeMemberInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.committee_members (node, valid_for, runtime, kind, role)
			VALUES ($1, $2, $3, $4, $5)`, chainID)
}

func makeCommitteeMembersTruncateQuery(chainID string) string {
	return fmt.Sprintf(`
		TRUNCATE %s.committee_members`, chainID)
}

func makeProposalSubmissionInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, chainID)
}

func makeProposalSubmissionCancelInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, chainID)
}

func makeProposalExecutionsUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET executed = true
			WHERE id = $1`, chainID)
}

func makeProposalUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET state = $2
			WHERE id = $1`, chainID)
}

func makeProposalInvalidVotesUpdateQuery(chainID string) string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET invalid_votes = $2
			WHERE id = $1`, chainID)
}

func makeVoteInsertQuery(chainID string) string {
	return fmt.Sprintf(`
		INSERT INTO %s.votes (proposal, voter, vote)
			VALUES ($1, $2, $3)`, chainID)
}
