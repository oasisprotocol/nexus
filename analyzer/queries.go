package analyzer

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
	return `
		SELECT height FROM chain.processed_blocks
			WHERE analyzer = $1
		ORDER BY height DESC
		LIMIT 1`
}

func (qf QueryFactory) IsGenesisProcessedQuery() string {
	return `
		SELECT EXISTS (
			SELECT 1 FROM chain.processed_geneses
			WHERE chain_context = $1
		)`
}

func (qf QueryFactory) IndexingProgressQuery() string {
	return `
		INSERT INTO chain.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP)`
}

func (qf QueryFactory) GenesisIndexingProgressQuery() string {
	return `
		INSERT INTO chain.processed_geneses (chain_context, processed_time)
			VALUES
				($1, CURRENT_TIMESTAMP)`
}

func (qf QueryFactory) ConsensusBlockInsertQuery() string {
	return `
		INSERT INTO chain.blocks (height, block_hash, time, num_txs, namespace, version, type, root_hash)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
}

func (qf QueryFactory) ConsensusEpochInsertQuery() string {
	return `
		INSERT INTO chain.epochs (id, start_height)
			VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING`
}

func (qf QueryFactory) ConsensusEpochUpdateQuery() string {
	return `
		UPDATE chain.epochs
		SET end_height = $2
			WHERE id = $1 AND end_height IS NULL`
}

func (qf QueryFactory) ConsensusTransactionInsertQuery() string {
	return `
		INSERT INTO chain.transactions (block, tx_hash, tx_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`
}

func (qf QueryFactory) ConsensusAccountNonceUpdateQuery() string {
	return `
		UPDATE chain.accounts
		SET
			nonce = $2
		WHERE address = $1`
}

func (qf QueryFactory) ConsensusCommissionsUpsertQuery() string {
	return `
		INSERT INTO chain.commissions (address, schedule)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET
				schedule = excluded.schedule`
}

func (qf QueryFactory) ConsensusEventInsertQuery() string {
	return `
		INSERT INTO chain.events (type, body, tx_block, tx_hash, tx_index, related_accounts)
			VALUES ($1, $2, $3, $4, $5, $6)`
}

func (qf QueryFactory) ConsensusAccountRelatedTransactionInsertQuery() string {
	return `
		INSERT INTO chain.accounts_related_transactions (account_address, tx_block, tx_index)
			VALUES ($1, $2, $3)`
}

func (qf QueryFactory) ConsensusAccountRelatedEventInsertQuery() string {
	return `
		INSERT INTO chain.accounts_related_events (account_address, event_block, tx_index, tx_hash, type, body)
			VALUES ($1, $2, $3, $4, $5, $6)`
}

func (qf QueryFactory) ConsensusRuntimeUpsertQuery() string {
	return `
		INSERT INTO chain.runtimes (id, suspended, kind, tee_hardware, key_manager)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (id) DO
			UPDATE SET
				suspended = excluded.suspended,
				kind = excluded.kind,
				tee_hardware = excluded.tee_hardware,
				key_manager = excluded.key_manager`
}

func (qf QueryFactory) ConsensusClaimedNodeInsertQuery() string {
	return `
		INSERT INTO chain.claimed_nodes (entity_id, node_id) VALUES ($1, $2)
			ON CONFLICT (entity_id, node_id) DO NOTHING`
}

func (qf QueryFactory) ConsensusEntityUpsertQuery() string {
	return `
		INSERT INTO chain.entities (id, address) VALUES ($1, $2)
			ON CONFLICT (id) DO
			UPDATE SET
				address = excluded.address`
}

func (qf QueryFactory) ConsensusNodeUpsertQuery() string {
	return `
		INSERT INTO chain.nodes (id, entity_id, expiration, tls_pubkey, tls_next_pubkey, tls_addresses, p2p_pubkey, p2p_addresses, consensus_pubkey, consensus_address, vrf_pubkey, roles, software_version, voting_power)
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
			voting_power = excluded.voting_power`
}

func (qf QueryFactory) ConsensusNodeDeleteQuery() string {
	return `
		DELETE FROM chain.nodes WHERE id = $1`
}

func (qf QueryFactory) ConsensusEntityMetaUpsertQuery() string {
	return `
		UPDATE chain.entities
		SET meta = $2
			WHERE id = $1`
}

func (qf QueryFactory) ConsensusSenderUpdateQuery() string {
	return `
		UPDATE chain.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`
}

func (qf QueryFactory) ConsensusReceiverUpdateQuery() string {
	return `
		INSERT INTO chain.accounts (address, general_balance)
			VALUES ($1, $2)
		ON CONFLICT (address) DO
			UPDATE SET general_balance = chain.accounts.general_balance + $2`
}

func (qf QueryFactory) ConsensusBurnUpdateQuery() string {
	return `
		UPDATE chain.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`
}

// Decreases the general balance because the given amount is being moved to escrow.
func (qf QueryFactory) ConsensusDecreaseGeneralBalanceForEscrowUpdateQuery() string {
	return `
		UPDATE chain.accounts
		SET
			general_balance = general_balance - $2
		WHERE address = $1`
}

func (qf QueryFactory) ConsensusAddEscrowBalanceUpsertQuery() string {
	return `
		INSERT INTO chain.accounts (address, escrow_balance_active, escrow_total_shares_active)
			VALUES ($1, $2, $3)
		ON CONFLICT (address) DO
			UPDATE SET
				escrow_balance_active = chain.accounts.escrow_balance_active + $2,
				escrow_total_shares_active = chain.accounts.escrow_total_shares_active + $3`
}

func (qf QueryFactory) ConsensusAddDelegationsUpsertQuery() string {
	return `
		INSERT INTO chain.delegations (delegatee, delegator, shares)
			VALUES ($1, $2, $3)
		ON CONFLICT (delegatee, delegator) DO
			UPDATE SET shares = chain.delegations.shares + $3`
}

func (qf QueryFactory) ConsensusTakeEscrowUpdateQuery() string {
	return `
		UPDATE chain.accounts
			SET
				escrow_balance_active = escrow_balance_active - FLOOR($2 * escrow_balance_active / (escrow_balance_active + escrow_balance_debonding)),
				escrow_balance_debonding = escrow_balance_debonding - FLOOR($2 * escrow_balance_debonding / (escrow_balance_active + escrow_balance_debonding))
			WHERE address = $1`
}

func (qf QueryFactory) ConsensusDebondingStartEscrowBalanceUpdateQuery() string {
	return `
		UPDATE chain.accounts
			SET
				escrow_balance_active = escrow_balance_active - $2,
				escrow_total_shares_active = escrow_total_shares_active - $3,
				escrow_balance_debonding = escrow_balance_debonding + $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding + $4
			WHERE address = $1`
}

func (qf QueryFactory) ConsensusDebondingStartDelegationsUpdateQuery() string {
	return `
		UPDATE chain.delegations
			SET shares = shares - $3
				WHERE delegatee = $1 AND delegator = $2`
}

func (qf QueryFactory) ConsensusDebondingStartDebondingDelegationsInsertQuery() string {
	return `
		INSERT INTO chain.debonding_delegations (delegatee, delegator, shares, debond_end)
			VALUES ($1, $2, $3, $4)`
}

func (qf QueryFactory) ConsensusReclaimGeneralBalanceUpdateQuery() string {
	return `
		UPDATE chain.accounts
			SET
				general_balance = general_balance + $2
			WHERE address = $1`
}

func (qf QueryFactory) ConsensusReclaimEscrowBalanceUpdateQuery() string {
	return `
		UPDATE chain.accounts
			SET
				escrow_balance_debonding = escrow_balance_debonding - $2,
				escrow_total_shares_debonding = escrow_total_shares_debonding - $3
			WHERE address = $1`
}

func (qf QueryFactory) ConsensusDeleteDebondingDelegationsQuery() string {
	// Network upgrades delays debonding by 1 epoch
	return `
		DELETE FROM chain.debonding_delegations
			WHERE id = (
				SELECT id
				FROM chain.debonding_delegations
				WHERE
					delegator = $1 AND delegatee = $2 AND shares = $3 AND debond_end IN ($4::bigint, $4::bigint - 1)
				LIMIT 1
			)`
}

func (qf QueryFactory) ConsensusAllowanceChangeDeleteQuery() string {
	return `
		DELETE FROM chain.allowances
			WHERE owner = $1 AND beneficiary = $2`
}

func (qf QueryFactory) ConsensusAllowanceOwnerUpsertQuery() string {
	return `
		INSERT INTO chain.accounts (address)
			VALUES ($1)
		ON CONFLICT (address) DO NOTHING`
}

func (qf QueryFactory) ConsensusAllowanceChangeUpdateQuery() string {
	return `
		INSERT INTO chain.allowances (owner, beneficiary, allowance)
			VALUES ($1, $2, $3)
		ON CONFLICT (owner, beneficiary) DO
			UPDATE SET allowance = excluded.allowance`
}

func (qf QueryFactory) ConsensusValidatorNodeUpdateQuery() string {
	return `
		UPDATE chain.nodes SET voting_power = $2
			WHERE id = $1`
}

func (qf QueryFactory) ConsensusCommitteeMemberInsertQuery() string {
	return `
		INSERT INTO chain.committee_members (node, valid_for, runtime, kind, role)
			VALUES ($1, $2, $3, $4, $5)`
}

func (qf QueryFactory) ConsensusCommitteeMembersTruncateQuery() string {
	return `
		TRUNCATE chain.committee_members`
}

func (qf QueryFactory) ConsensusProposalSubmissionInsertQuery() string {
	return `
		INSERT INTO chain.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`
}

func (qf QueryFactory) ConsensusProposalSubmissionCancelInsertQuery() string {
	return `
		INSERT INTO chain.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
}

func (qf QueryFactory) ConsensusProposalExecutionsUpdateQuery() string {
	return `
		UPDATE chain.proposals
		SET executed = true
			WHERE id = $1`
}

func (qf QueryFactory) ConsensusProposalUpdateQuery() string {
	return `
		UPDATE chain.proposals
		SET state = $2
			WHERE id = $1`
}

func (qf QueryFactory) ConsensusProposalInvalidVotesUpdateQuery() string {
	return `
		UPDATE chain.proposals
		SET invalid_votes = $2
			WHERE id = $1`
}

func (qf QueryFactory) ConsensusVoteInsertQuery() string {
	return `
		INSERT INTO chain.votes (proposal, voter, vote)
			VALUES ($1, $2, $3)`
}

func (qf QueryFactory) RuntimeBlockInsertQuery() string {
	return `
		INSERT INTO chain.runtime_blocks (runtime, round, version, timestamp, block_hash, prev_block_hash, io_root, state_root, messages_hash, in_messages_hash, num_transactions, gas_used, size)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`
}

func (qf QueryFactory) RuntimeTransactionSignerInsertQuery() string {
	return `
		INSERT INTO chain.runtime_transaction_signers (runtime, round, tx_index, signer_index, signer_address, nonce)
			VALUES ($1, $2, $3, $4, $5, $6)`
}

func (qf QueryFactory) RuntimeRelatedTransactionInsertQuery() string {
	return `
		INSERT INTO chain.runtime_related_transactions (runtime, account_address, tx_round, tx_index)
			VALUES ($1, $2, $3, $4)`
}

func (qf QueryFactory) RuntimeTransactionInsertQuery() string {
	return `
		INSERT INTO chain.runtime_transactions (runtime, round, tx_index, tx_hash, tx_eth_hash, gas_used, size, timestamp, raw, result_raw)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
}

func (qf QueryFactory) RuntimeEventInsertQuery() string {
	return `
		INSERT INTO chain.runtime_events (runtime, round, tx_index, tx_hash, type, body, evm_log_name, evm_log_params, related_accounts)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
}

func (qf QueryFactory) RuntimeMintInsertQuery() string {
	return `
		INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
			VALUES ($1, $2, NULL, $3, $4, $5)`
}

func (qf QueryFactory) RuntimeBurnInsertQuery() string {
	return `
		INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
			VALUES ($1, $2, $3, NULL, $4, $5)`
}

func (qf QueryFactory) RuntimeTransferInsertQuery() string {
	return `
		INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
			VALUES ($1, $2, $3, $4, $5, $6)`
}

func (qf QueryFactory) RuntimeDepositInsertQuery() string {
	return `
		INSERT INTO chain.runtime_deposits (runtime, round, sender, receiver, amount, nonce, module, code)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
}

func (qf QueryFactory) RuntimeWithdrawInsertQuery() string {
	return `
		INSERT INTO chain.runtime_withdraws (runtime, round, sender, receiver, amount, nonce, module, code)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`
}

func (qf QueryFactory) RuntimeNativeBalanceUpdateQuery() string {
	return `
		INSERT INTO chain.runtime_sdk_balances (runtime, account_address, symbol, balance)
		  VALUES ($1, $2, $3, $4)
		ON CONFLICT (runtime, account_address, symbol) DO
		UPDATE SET balance = chain.runtime_sdk_balances.balance + $4`
}

func (qf QueryFactory) RuntimeGasUsedInsertQuery() string {
	return `
		INSERT INTO chain.runtime_gas_used (runtime, round, sender, amount)
			VALUES ($1, $2, $3, $4)`
}

func (qf QueryFactory) AddressPreimageInsertQuery() string {
	return `
		INSERT INTO chain.address_preimages (address, context_identifier, context_version, address_data)
			VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`
}

func (qf QueryFactory) RuntimeEvmBalanceUpdateQuery() string {
	return `
		INSERT INTO chain.evm_token_balances (runtime, token_address, account_address, balance)
			VALUES ($1, $2, $3, $4)
		ON CONFLICT (runtime, token_address, account_address) DO
			UPDATE SET balance = chain.evm_token_balances.balance + $4`
}

func (qf QueryFactory) RuntimeEVMTokensAnalysisStaleQuery() string {
	return `
		SELECT
			evm_token_analysis.token_address,
			evm_token_analysis.last_mutate_round,
			evm_token_analysis.last_download_round,
			evm_tokens.token_type,
			address_preimages.context_identifier,
			address_preimages.context_version,
			address_preimages.address_data
		FROM chain.evm_token_analysis
		LEFT JOIN chain.evm_tokens USING (runtime, token_address)
		LEFT JOIN chain.address_preimages ON
			address_preimages.address = evm_token_analysis.token_address
		WHERE
			evm_token_analysis.runtime = $1 AND
			(
				evm_token_analysis.last_download_round IS NULL OR
				evm_token_analysis.last_mutate_round > evm_token_analysis.last_download_round
			)
		LIMIT $2`
}

func (qf QueryFactory) RuntimeEVMTokenAnalysisInsertQuery() string {
	return `
		INSERT INTO chain.evm_token_analysis (runtime, token_address, last_mutate_round)
			VALUES ($1, $2, $3)
		ON CONFLICT (runtime, token_address) DO NOTHING`
}

func (qf QueryFactory) RuntimeEVMTokenAnalysisMutateInsertQuery() string {
	return `
		INSERT INTO chain.evm_token_analysis (runtime, token_address, last_mutate_round)
			VALUES ($1, $2, $3)
		ON CONFLICT (runtime, token_address) DO UPDATE
			SET last_mutate_round = excluded.last_mutate_round`
}

func (qf QueryFactory) RuntimeEVMTokenAnalysisUpdateQuery() string {
	return `
		UPDATE chain.evm_token_analysis
		SET
			last_download_round = $3
		WHERE
			runtime = $1 AND
			token_address = $2`
}

func (qf QueryFactory) RuntimeEVMTokenInsertQuery() string {
	return `
		INSERT INTO chain.evm_tokens (runtime, token_address, token_type, token_name, symbol, decimals, total_supply)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
}

func (qf QueryFactory) RuntimeEVMTokenUpdateQuery() string {
	return `
		UPDATE chain.evm_tokens
		SET
			total_supply = $3
		WHERE
			runtime = $1 AND
			token_address = $2`
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
	return `
		SELECT time
		FROM chain.blocks
		ORDER BY height
		LIMIT 1
	`
}

// LatestConsensusBlockTimeQuery returns the query to get the timestamp of the latest
// indexed consensus block.
func (qf QueryFactory) LatestConsensusBlockTimeQuery() string {
	return `
		SELECT time
		FROM chain.blocks
		ORDER BY height DESC
		LIMIT 1
	`
}

// EarliestRuntimeBlockTimeQuery returns the query to get the timestamp of the earliest
// indexed runtime block.
func (qf QueryFactory) EarliestRuntimeBlockTimeQuery() string {
	return `
		SELECT timestamp
		FROM chain.runtime_blocks
		WHERE (runtime = $1)
		ORDER BY round
		LIMIT 1
	`
}

// LatestRuntimeBlockTimeQuery returns the query to get the timestamp of the latest
// indexed runtime block.
func (qf QueryFactory) LatestRuntimeBlockTimeQuery() string {
	return `
		SELECT timestamp
		FROM chain.runtime_blocks
		WHERE (runtime = $1)
		ORDER BY round DESC
		LIMIT 1
	`
}

// ConsensusActiveAccountsQuery returns the query to get the number of
// active accounts in the consensus layer within the given time range.
func (qf QueryFactory) ConsensusActiveAccountsQuery() string {
	return `
		SELECT COUNT(DISTINCT account_address)
		FROM chain.accounts_related_transactions AS art
		JOIN chain.blocks AS b ON art.tx_block = b.height
		WHERE (b.time >= $1::timestamptz AND b.time < $2::timestamptz)
		`
}

// RuntimeActiveAccountsQuery returns the query to get the number of
// active accounts in the runtime layer within the given time range.
func (qf QueryFactory) RuntimeActiveAccountsQuery() string {
	return `
		SELECT COUNT(DISTINCT account_address)
		FROM chain.runtime_related_transactions AS rt
		JOIN chain.runtime_blocks AS b ON (rt.runtime = b.runtime AND rt.tx_round = b.round)
		WHERE (rt.runtime = $1 AND b.timestamp >= $2::timestamptz AND b.timestamp < $3::timestamptz)
	`
}
