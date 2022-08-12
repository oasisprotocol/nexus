package consensus

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

func (qf QueryFactory) LatestBlockQuery() string {
	return fmt.Sprintf(`
		SELECT height FROM %s.processed_blocks
			WHERE analyzer = $1
		ORDER BY height DESC
		LIMIT 1`, qf.chainID)
}

func (qf QueryFactory) IndexingProgressQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP)`, qf.chainID)
}

func (qf QueryFactory) BlockInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.blocks (height, block_hash, time, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, qf.chainID)
}

func (qf QueryFactory) EpochInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.epochs (id, start_height)
			VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING`, qf.chainID)
}

func (qf QueryFactory) EpochUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.epochs
		SET end_height = $2
			WHERE id = $1 AND end_height IS NULL`, qf.chainID)
}

func (qf QueryFactory) TransactionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.transactions (block, txn_hash, txn_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, qf.chainID)
}

func (qf QueryFactory) AccountNonceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			nonce = $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) CommissionsUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.commissions (address, schedule)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET
				schedule = excluded.schedule`, qf.chainID)
}

func (qf QueryFactory) EventInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.events (backend, type, body, txn_block, txn_hash, txn_index)
			VALUES ($1, $2, $3, $4, $5, $6)`, qf.chainID)
}

func (qf QueryFactory) RuntimeUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.runtimes (id, suspended, kind, tee_hardware, key_manager)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO
			UPDATE SET
				suspended = excluded.suspended,
				kind = excluded.kind,
				tee_hardware = excluded.tee_hardware,
				key_manager = excluded.key_manager`, qf.chainID)
}

func (qf QueryFactory) RuntimeSuspensionQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.runtimes
			SET suspended = true
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) RuntimeUnsuspensionQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.runtimes
			SET suspended = false
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ClaimedNodeInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.claimed_nodes (entity_id, node_id) VALUES ($1, $2)
			ON CONFLICT (entity_id, node_id) DO NOTHING`, qf.chainID)
}

func (qf QueryFactory) EntityUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.entities (id, address) VALUES ($1, $2)
			ON CONFLICT (id) DO
			UPDATE SET
				address = excluded.address`, qf.chainID)
}

func (qf QueryFactory) NodeUpsertQuery() string {
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
			voting_power = excluded.voting_power`, qf.chainID)
}

func (qf QueryFactory) NodeDeleteQuery() string {
	return fmt.Sprintf(`
		DELETE FROM %s.nodes WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) EntityMetaUpsertQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.entities
		SET meta = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) SenderUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ReceiverUpdateQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address, general_balance)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET general_balance = %[1]s.accounts.general_balance + $2;`, qf.chainID)
}

func (qf QueryFactory) BurnUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) AddGeneralBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) AddEscrowBalanceUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address, escrow_balance_active, escrow_total_shares_active)
			VALUES ($1, $2, $3)
		ON CONFLICT (address) DO
			UPDATE SET
				escrow_balance_active = %[1]s.accounts.escrow_balance_active + $2,
				escrow_total_shares_active = %[1]s.accounts.escrow_total_shares_active + $3`, qf.chainID)
}

func (qf QueryFactory) AddDelegationsUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.delegations (delegatee, delegator, shares)
			VALUES ($1, $2, $3)
		ON CONFLICT (delegatee, delegator) DO
			UPDATE SET shares = %[1]s.delegations.shares + $3`, qf.chainID)
}

func (qf QueryFactory) TakeEscrowUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_active = escrow_balance_active - ROUND($2 * escrow_balance_active / (escrow_balance_active + escrow_balance_debonding)),
				escrow_balance_debonding = escrow_balance_debonding - ROUND($2 * escrow_balance_debonding / (escrow_balance_active + escrow_balance_debonding))
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) DebondingStartEscrowBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_active = escrow_balance_active - $2,
				escrow_total_shares_active = escrow_total_shares_active - $3,
				escrow_balance_debonding = escrow_balance_debonding + $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding + $4
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) DebondingStartDelegationsUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.delegations
			SET shares = shares - $3
				WHERE delegatee = $1 AND delegator = $2`, qf.chainID)
}

func (qf QueryFactory) DebondingStartDebondingDelegationsInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.debonding_delegations (delegatee, delegator, shares, debond_end)
			VALUES ($1, $2, $3, $4)`, qf.chainID)
}

func (qf QueryFactory) ReclaimGeneralBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				general_balance = general_balance + $2
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ReclaimEscrowBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_debonding = escrow_balance_debonding - $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding - $3
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) DeleteDebondingDelegationsQuery() string {
	// Network upgrades delays debonding by 1 epoch
	return fmt.Sprintf(`
		DELETE FROM %[1]s.debonding_delegations
			WHERE (ctid) IN (
				SELECT ctid
				FROM %[1]s.debonding_delegations
				WHERE
					delegator = $1 AND delegatee = $2 AND shares = $3 AND debond_end IN (
					SELECT id
					FROM %[1]s.epochs
					ORDER BY id DESC
					LIMIT 2
				) LIMIT 1
			)`, qf.chainID)
}

func (qf QueryFactory) AllowanceChangeDeleteQuery() string {
	return fmt.Sprintf(`
		DELETE FROM %s.allowances
			WHERE owner = $1 AND beneficiary = $2`, qf.chainID)
}

func (qf QueryFactory) AllowanceChangeUpdateQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.allowances (owner, beneficiary, allowance)
			VALUES ($1, $2, $3)
		ON CONFLICT (owner, beneficiary) DO
			UPDATE SET allowance = excluded.allowance`, qf.chainID)
}

func (qf QueryFactory) ValidatorNodeUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.nodes SET voting_power = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) CommitteeMemberInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.committee_members (node, valid_for, runtime, kind, role)
			VALUES ($1, $2, $3, $4, $5)`, qf.chainID)
}

func (qf QueryFactory) CommitteeMembersTruncateQuery() string {
	return fmt.Sprintf(`
		TRUNCATE %s.committee_members`, qf.chainID)
}

func (qf QueryFactory) ProposalSubmissionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, qf.chainID)
}

func (qf QueryFactory) ProposalSubmissionCancelInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, qf.chainID)
}

func (qf QueryFactory) ProposalExecutionsUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET executed = true
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ProposalUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET state = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ProposalInvalidVotesUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET invalid_votes = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) VoteInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.votes (proposal, voter, vote)
			VALUES ($1, $2, $3)`, qf.chainID)
}
