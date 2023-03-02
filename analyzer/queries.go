package analyzer

import (
	"fmt"
)

// QueryFactory is a convenience type for creating database queries.
type QueryFactory struct {
	chainID string
	runtime string
}

func NewQueryFactory(_ string, runtime string) QueryFactory {
	return QueryFactory{"chain", runtime}
}

// NewWithRuntime returns a new QueryFactory with the runtime set.
func (qf QueryFactory) NewWithRuntime(runtime string) QueryFactory {
	return QueryFactory{qf.chainID, runtime}
}

func (qf QueryFactory) LatestBlockQuery() string {
	return fmt.Sprintf(`
		SELECT height FROM %s.processed_blocks
			WHERE analyzer = $1
		ORDER BY height DESC
		LIMIT 1`, qf.chainID)
}

func (qf QueryFactory) IsGenesisProcessedQuery() string {
	return `
		SELECT EXISTS (
			SELECT 1 FROM chain.processed_geneses
			WHERE chain_context = $1
		)`
}

func (qf QueryFactory) IndexingProgressQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP)`, qf.chainID)
}

func (qf QueryFactory) GenesisIndexingProgressQuery() string {
	return `
		INSERT INTO chain.processed_geneses (chain_context, processed_time)
			VALUES
				($1, CURRENT_TIMESTAMP)`
}

func (qf QueryFactory) ConsensusBlockInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.blocks (height, block_hash, time, num_txs, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`, qf.chainID)
}

func (qf QueryFactory) ConsensusEpochInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.epochs (id, start_height)
			VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING`, qf.chainID)
}

func (qf QueryFactory) ConsensusEpochUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.epochs
		SET end_height = $2
			WHERE id = $1 AND end_height IS NULL`, qf.chainID)
}

func (qf QueryFactory) ConsensusTransactionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.transactions (block, tx_hash, tx_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, qf.chainID)
}

func (qf QueryFactory) ConsensusAccountNonceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			nonce = $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusCommissionsUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.commissions (address, schedule)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET
				schedule = excluded.schedule`, qf.chainID)
}

func (qf QueryFactory) ConsensusEventInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.events (type, body, tx_block, tx_hash, tx_index, related_accounts)
			VALUES ($1, $2, $3, $4, $5, $6)`, qf.chainID)
}

func (qf QueryFactory) ConsensusAccountRelatedTransactionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.accounts_related_transactions (account_address, tx_block, tx_index)
			VALUES ($1, $2, $3)`, qf.chainID)
}

func (qf QueryFactory) ConsensusAccountRelatedEventInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.accounts_related_events (account_address, event_block, tx_index, tx_hash, type, body)
			VALUES ($1, $2, $3, $4, $5, $6)`, qf.chainID)
}

func (qf QueryFactory) ConsensusRuntimeUpsertQuery() string {
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

func (qf QueryFactory) ConsensusClaimedNodeInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.claimed_nodes (entity_id, node_id) VALUES ($1, $2)
			ON CONFLICT (entity_id, node_id) DO NOTHING`, qf.chainID)
}

func (qf QueryFactory) ConsensusEntityUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.entities (id, address) VALUES ($1, $2)
			ON CONFLICT (id) DO
			UPDATE SET
				address = excluded.address`, qf.chainID)
}

func (qf QueryFactory) ConsensusNodeUpsertQuery() string {
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

func (qf QueryFactory) ConsensusNodeDeleteQuery() string {
	return fmt.Sprintf(`
		DELETE FROM %s.nodes WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusEntityMetaUpsertQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.entities
		SET meta = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusSenderUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusReceiverUpdateQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address, general_balance)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET general_balance = %[1]s.accounts.general_balance + $2`, qf.chainID)
}

func (qf QueryFactory) ConsensusBurnUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, qf.chainID)
}

// Decreases the general balance because the given amount is being moved to escrow.
func (qf QueryFactory) ConsensusDecreaseGeneralBalanceForEscrowUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusAddEscrowBalanceUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address, escrow_balance_active, escrow_total_shares_active)
			VALUES ($1, $2, $3)
		ON CONFLICT (address) DO
			UPDATE SET
				escrow_balance_active = %[1]s.accounts.escrow_balance_active + $2,
				escrow_total_shares_active = %[1]s.accounts.escrow_total_shares_active + $3`, qf.chainID)
}

func (qf QueryFactory) ConsensusAddDelegationsUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.delegations (delegatee, delegator, shares)
			VALUES ($1, $2, $3)
		ON CONFLICT (delegatee, delegator) DO
			UPDATE SET shares = %[1]s.delegations.shares + $3`, qf.chainID)
}

func (qf QueryFactory) ConsensusTakeEscrowUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_active = escrow_balance_active - FLOOR($2 * escrow_balance_active / (escrow_balance_active + escrow_balance_debonding)),
				escrow_balance_debonding = escrow_balance_debonding - FLOOR($2 * escrow_balance_debonding / (escrow_balance_active + escrow_balance_debonding))
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusDebondingStartEscrowBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_active = escrow_balance_active - $2,
				escrow_total_shares_active = escrow_total_shares_active - $3,
				escrow_balance_debonding = escrow_balance_debonding + $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding + $4
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusDebondingStartDelegationsUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.delegations
			SET shares = shares - $3
				WHERE delegatee = $1 AND delegator = $2`, qf.chainID)
}

func (qf QueryFactory) ConsensusDebondingStartDebondingDelegationsInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.debonding_delegations (delegatee, delegator, shares, debond_end)
			VALUES ($1, $2, $3, $4)`, qf.chainID)
}

func (qf QueryFactory) ConsensusReclaimGeneralBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				general_balance = general_balance + $2
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusReclaimEscrowBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.accounts
			SET
				escrow_balance_debonding = escrow_balance_debonding - $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding - $3
			WHERE address = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusDeleteDebondingDelegationsQuery() string {
	// Network upgrades delays debonding by 1 epoch
	return fmt.Sprintf(`
		DELETE FROM %[1]s.debonding_delegations
			WHERE id = (
				SELECT id
				FROM %[1]s.debonding_delegations
				WHERE
					delegator = $1 AND delegatee = $2 AND shares = $3 AND debond_end IN ($4::bigint, $4::bigint - 1)
				LIMIT 1
			)`, qf.chainID)
}

func (qf QueryFactory) ConsensusAllowanceChangeDeleteQuery() string {
	return fmt.Sprintf(`
		DELETE FROM %s.allowances
			WHERE owner = $1 AND beneficiary = $2`, qf.chainID)
}

func (qf QueryFactory) ConsensusAllowanceOwnerUpsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.accounts (address)
			VALUES ($1)
		ON CONFLICT (address) DO NOTHING`, qf.chainID)
}

func (qf QueryFactory) ConsensusAllowanceChangeUpdateQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.allowances (owner, beneficiary, allowance)
			VALUES ($1, $2, $3)
		ON CONFLICT (owner, beneficiary) DO
			UPDATE SET allowance = excluded.allowance`, qf.chainID)
}

func (qf QueryFactory) ConsensusValidatorNodeUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.nodes SET voting_power = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusCommitteeMemberInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.committee_members (node, valid_for, runtime, kind, role)
			VALUES ($1, $2, $3, $4, $5)`, qf.chainID)
}

func (qf QueryFactory) ConsensusCommitteeMembersTruncateQuery() string {
	return fmt.Sprintf(`
		TRUNCATE %s.committee_members`, qf.chainID)
}

func (qf QueryFactory) ConsensusProposalSubmissionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`, qf.chainID)
}

func (qf QueryFactory) ConsensusProposalSubmissionCancelInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`, qf.chainID)
}

func (qf QueryFactory) ConsensusProposalExecutionsUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET executed = true
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusProposalUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET state = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusProposalInvalidVotesUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %s.proposals
		SET invalid_votes = $2
			WHERE id = $1`, qf.chainID)
}

func (qf QueryFactory) ConsensusVoteInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.votes (proposal, voter, vote)
			VALUES ($1, $2, $3)`, qf.chainID)
}

func (qf QueryFactory) RuntimeBlockInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.runtime_blocks (runtime, round, version, timestamp, block_hash, prev_block_hash, io_root, state_root, messages_hash, in_messages_hash, num_transactions, gas_used, size)
			VALUES ('%s', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeTransactionSignerInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_transaction_signers (runtime, round, tx_index, signer_index, signer_address, nonce)
			VALUES ('%[2]s', $1, $2, $3, $4, $5)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeRelatedTransactionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_related_transactions (runtime, account_address, tx_round, tx_index)
			VALUES ('%[2]s', $1, $2, $3)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeTransactionInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_transactions (runtime, round, tx_index, tx_hash, tx_eth_hash, gas_used, size, timestamp, raw, result_raw)
			VALUES ('%[2]s', $1, $2, $3, $4, $5, $6, $7, $8, $9)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEventInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_events (runtime, round, tx_index, tx_hash, type, body, evm_log_name, evm_log_params, related_accounts)
			VALUES ('%[2]s', $1, $2, $3, $4, $5, $6, $7, $8)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeMintInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
			VALUES ('%[2]s', $1, NULL, $2, $3, $4)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeBurnInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
			VALUES ('%[2]s', $1, $2, NULL, $3, $4)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeTransferInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
			VALUES ('%[2]s', $1, $2, $3, $4, $5)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeDepositInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_deposits (runtime, round, sender, receiver, amount, nonce, module, code)
			VALUES ('%[2]s', $1, $2, $3, $4, $5, $6, $7)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeWithdrawInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_withdraws (runtime, round, sender, receiver, amount, nonce, module, code)
			VALUES ('%[2]s', $1, $2, $3, $4, $5, $6, $7)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeNativeBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_sdk_balances (runtime, account_address, symbol, balance)
		  VALUES ('%[2]s', $1, $2, $3)
		ON CONFLICT (runtime, account_address, symbol) DO
		UPDATE SET balance = %[1]s.runtime_sdk_balances.balance + $3`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeGasUsedInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.runtime_gas_used (runtime, round, sender, amount)
			VALUES ('%[2]s', $1, $2, $3)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) AddressPreimageInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.address_preimages (address, context_identifier, context_version, address_data)
			VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`, qf.chainID)
}

func (qf QueryFactory) RuntimeEvmBalanceUpdateQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.evm_token_balances (runtime, token_address, account_address, balance)
			VALUES ('%[2]s', $1, $2, $3)
		ON CONFLICT (runtime, token_address, account_address) DO
			UPDATE SET balance = %[1]s.evm_token_balances.balance + $3`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEVMTokensAnalysisStaleQuery() string {
	return fmt.Sprintf(`
		SELECT
			evm_token_analysis.token_address,
			evm_token_analysis.last_mutate_round,
			evm_token_analysis.last_download_round,
			evm_tokens.token_type,
			address_preimages.context_identifier,
			address_preimages.context_version,
			address_preimages.address_data
		FROM %[1]s.evm_token_analysis
		LEFT JOIN %[1]s.evm_tokens USING (runtime, token_address)
		LEFT JOIN %[1]s.address_preimages ON
			address_preimages.address = evm_token_analysis.token_address
		WHERE
			evm_token_analysis.runtime = '%[2]s' AND
			(
				evm_token_analysis.last_download_round IS NULL OR
				evm_token_analysis.last_mutate_round > evm_token_analysis.last_download_round
			)
		LIMIT $1`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEVMTokenAnalysisInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.evm_token_analysis (runtime, token_address, last_mutate_round)
			VALUES ('%[2]s', $1, $2)
		ON CONFLICT (runtime, token_address) DO NOTHING`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEVMTokenAnalysisMutateInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.evm_token_analysis (runtime, token_address, last_mutate_round)
			VALUES ('%[2]s', $1, $2)
		ON CONFLICT (runtime, token_address) DO UPDATE
			SET last_mutate_round = excluded.last_mutate_round`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEVMTokenAnalysisUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %[1]s.evm_token_analysis
		SET
			last_download_round = $2
		WHERE
			runtime = '%[2]s' AND
			token_address = $1`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEVMTokenInsertQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %[1]s.evm_tokens (runtime, token_address, token_type, token_name, symbol, decimals, total_supply)
			VALUES ('%[2]s', $1, $2, $3, $4, $5, $6)`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RuntimeEVMTokenUpdateQuery() string {
	return fmt.Sprintf(`
		UPDATE %[1]s.evm_tokens
		SET
			total_supply = $2
		WHERE
			runtime = '%[2]s' AND
			token_address = $1`, qf.chainID, qf.runtime)
}

func (qf QueryFactory) RefreshDailyTxVolumeQuery() string {
	return `
		REFRESH MATERIALIZED VIEW stats.daily_tx_volume
	`
}

func (qf QueryFactory) RefreshMin5TxVolumeQuery() string {
	return `
		REFRESH MATERIALIZED VIEW stats.min5_tx_volume
	`
}

// LatestDailyAccountStatsQuery returns the query to get the timestamp of the latest daily active accounts stat.
func (qf QueryFactory) LatestDailyAccountStatsQuery() string {
	return `
		SELECT window_end
		FROM stats.daily_active_accounts
		WHERE layer = $1
		ORDER BY window_end DESC
		LIMIT 1
	`
}

// InsertDailyAccountStatsQuery returns the query to insert the daily active accounts stat.
func (qf QueryFactory) InsertDailyAccountStatsQuery() string {
	return `
		INSERT INTO stats.daily_active_accounts (layer, window_end, active_accounts)
		VALUES ($1, $2, $3)
	`
}

// EarliestConsensusBlockTimeQuery returns the query to get the timestamp of the earliest
// indexed consensus block.
func (qf QueryFactory) EarliestConsensusBlockTimeQuery() string {
	return fmt.Sprintf(`
		SELECT time
		FROM %[1]s.blocks
		ORDER BY height
		LIMIT 1
	`, qf.chainID)
}

// LatestConsensusBlockTimeQuery returns the query to get the timestamp of the latest
// indexed consensus block.
func (qf QueryFactory) LatestConsensusBlockTimeQuery() string {
	return fmt.Sprintf(`
		SELECT time
		FROM %[1]s.blocks
		ORDER BY height DESC
		LIMIT 1
	`, qf.chainID)
}

// EarliestRuntimeBlockTimeQuery returns the query to get the timestamp of the earliest
// indexed runtime block.
func (qf QueryFactory) EarliestRuntimeBlockTimeQuery() string {
	return fmt.Sprintf(`
		SELECT timestamp
		FROM %[1]s.runtime_blocks
		WHERE (runtime = '%[2]s')
		ORDER BY round
		LIMIT 1
	`, qf.chainID, qf.runtime)
}

// LatestRuntimeBlockTimeQuery returns the query to get the timestamp of the latest
// indexed runtime block.
func (qf QueryFactory) LatestRuntimeBlockTimeQuery() string {
	return fmt.Sprintf(`
		SELECT timestamp
		FROM %[1]s.runtime_blocks
		WHERE (runtime = '%[2]s')
		ORDER BY round DESC
		LIMIT 1
	`, qf.chainID, qf.runtime)
}

// ConsensusActiveAccountsQuery returns the query to get the number of
// active accounts in the consensus layer within the given time range.
func (qf QueryFactory) ConsensusActiveAccountsQuery() string {
	return fmt.Sprintf(`
		SELECT COUNT(DISTINCT account_address)
		FROM %[1]s.accounts_related_transactions AS art
		JOIN %[1]s.blocks AS b ON art.tx_block = b.height
		WHERE (b.time >= $1::timestamptz AND b.time < $2::timestamptz)
		`, qf.chainID)
}

// RuntimeActiveAccountsQuery returns the query to get the number of
// active accounts in the runtime layer within the given time range.
func (qf QueryFactory) RuntimeActiveAccountsQuery() string {
	return fmt.Sprintf(`
		SELECT COUNT(DISTINCT account_address)
		FROM %[1]s.runtime_related_transactions AS rt
		JOIN %[1]s.runtime_blocks AS b ON (rt.runtime = b.runtime AND rt.tx_round = b.round)
		WHERE (rt.runtime = '%[2]s' AND b.timestamp >= $1::timestamptz AND b.timestamp < $2::timestamptz)
		`, qf.chainID, qf.runtime)
}
