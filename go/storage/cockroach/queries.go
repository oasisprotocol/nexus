package cockroach

import "fmt"

const (
	// TODO: Make this config somehow
	chainID = "oasis_2"
)

var (
	// Block Data Queries
	blocksInsertQuery = fmt.Sprintf(`
		INSERT INTO %s.blocks (height, block_hash, time, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7);
	`, chainID)

	transactionsInsertQuery = fmt.Sprintf(`
		INSERT INTO %s.transactions (block, txn_hash, txn_index, nonce, fee_amount, max_gas, method, body, module, code, message)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
	`, chainID)

	eventsInsertQuery = fmt.Sprintf(`
		INSERT INTO %s.events (backend, type, body, txn_block, txn_hash, txn_index)
			VALUES ($1, $2, $3, $4, $5, $6);
	`, chainID)

	// Beacon Data Queries
	// None, for now.

	// Registry Data Queries
	runtimeRegistrationQuery = fmt.Sprintf(`
		INSERT INTO %s.runtimes (id, extra_data)
			VALUES ($1, $2);
	`, chainID)

	// Staking Data Queries
	transferFromQuery = fmt.Sprintf(`
		UPDATE %s.accounts
		SET general_balance = general_balance - $2
			WHERE address = $1;
	`, chainID)
	transferToQuery = fmt.Sprintf(`
		INSERT INTO %s.accounts (address, general_balance)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET general_balance = general_balance - $2;
	`, chainID)

	burnQuery = fmt.Sprintf(`
		UPDATE %s.accounts
		SET general_balance = general_balance - $2
			WHERE address = $1;
	`, chainID)

	ownerBalanceQuery = fmt.Sprintf(`
		UPDATE %s.accounts
		SET general_balance = general_balance - $2
			WHERE address = $1;
	`, chainID)
	escrowBalanceQuery = fmt.Sprintf(`
		INSERT INTO %s.accounts (address, escrow_balance_active, escrow_total_shares_active)
			VALUES ($1, $2, $3)
		ON CONFLICT (address) DO
			UPDATE %s.accounts
			SET
				escrow_balance_active = escrow_balance_active + $2,
				escrow_total_shares_active = escrow_total_shares_active + $3;
	`, chainID)
	delegationsQuery = fmt.Sprintf(`
		INSERT INTO %s.delegations (delegatee, delegator, shares)
			VALUES ($1, $2, $3)
		ON CONFLICT (delegatee, delegator) DO
			UPDATE %s.delegations
			SET shares = shares + $3;
	`, chainID)

	takeEscrowQuery = fmt.Sprintf(`
		UPDATE %s.accounts
		SET escrow_balance_active = escrow_balance_active - $2
			WHERE address = $1;
	`, chainID)

	debondingStartRemoveDelegationQuery = fmt.Sprintf(`
		UPDATE %s.delegations
		SET shares = shares - $3
			WHERE delegatee = $1 AND delegator = $2;
	`, chainID)
	debondingStartAddDebondingDelegationQuery = fmt.Sprintf(`
		INSERT INTO %s.debonding_delegations (delegatee, delegator, shares, debond_end)
			VALUES ($1, $2, $3, $4);
	`, chainID)

	reclaimEscrowQuery = fmt.Sprintf(`
		
	`, chainID)

	allowanceChangeQuery = fmt.Sprintf(`
		INSERT INTO %s.allowances (owner, beneficiary, allowance)
			VALUES ($1, $2, $3) 
		ON CONFLICT (owner, beneficiary) DO
			UPDATE %s.allowances
			SET allowance = EXCLUDED.allowance;
	`, chainID)

	// Scheduler Data Queries
	updateVotingPowerQuery = fmt.Sprintf(`
		UPDATE %s.nodes
		SET voting_power = $2
			WHERE id = $1;
	`, chainID)

	truncateCommitteesQuery = fmt.Sprintf(`
		TRUNCATE %s.committee_members;
	`, chainID)
	addCommitteeMemberQuery = fmt.Sprintf(`
		INSERT INTO %s.committee_members (node, valid_for, runtime, kind, role)
			VALUES ($1, $2, $3, $4, $5);
	`, chainID)

	// Governance Data Queries
	submissionUpgradeQuery = fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);
	`, chainID)
	submissionCancelUpgradeQuery = fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7);
	`, chainID)

	executionQuery = fmt.Sprintf(`
		UPDATE %s.proposals
		SET executed = true
			WHERE id = $1;
	`, chainID)

	finalizationQuery = fmt.Sprintf(`
		UPDATE %s.proposals
		SET state = $2
			WHERE id = $1;
	`, chainID)
	invalidVotesQuery = fmt.Sprintf(`
		UPDATE %s.proposals
		SET invalid_votes = $2
			WHERE id = $1;
	`, chainID)

	voteQuery = fmt.Sprintf(`
		INSERT INTO %s.votes (proposal, voter, vote)
			VALUES ($1, $2, $3);
	`, chainID)
)
