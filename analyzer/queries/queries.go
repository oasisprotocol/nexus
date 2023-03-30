package queries

const (
	LatestBlock = `
    SELECT height FROM chain.processed_blocks
      WHERE analyzer = $1
    ORDER BY height DESC
    LIMIT 1`

	IsGenesisProcessed = `
    SELECT EXISTS (
      SELECT 1 FROM chain.processed_geneses
      WHERE chain_context = $1
    )`

	IndexingProgress = `
    INSERT INTO chain.processed_blocks (height, analyzer, processed_time)
      VALUES
        ($1, $2, CURRENT_TIMESTAMP)`

	GenesisIndexingProgress = `
    INSERT INTO chain.processed_geneses (chain_context, processed_time)
      VALUES
        ($1, CURRENT_TIMESTAMP)`

	ConsensusBlockInsert = `
    INSERT INTO chain.blocks (height, block_hash, time, num_txs, namespace, version, type, root_hash)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	ConsensusEpochInsert = `
    INSERT INTO chain.epochs (id, start_height)
      VALUES ($1, $2)
    ON CONFLICT (id) DO NOTHING`

	ConsensusEpochUpdate = `
    UPDATE chain.epochs
    SET end_height = $2
      WHERE id = $1 AND end_height IS NULL`

	ConsensusTransactionInsert = `
    INSERT INTO chain.transactions (block, tx_hash, tx_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	ConsensusAccountNonceUpsert = `
    INSERT INTO chain.accounts(address, nonce)
    VALUES ($1, $2)
    ON CONFLICT (address) DO UPDATE
      SET nonce = $2`

	ConsensusCommissionsUpsert = `
    INSERT INTO chain.commissions (address, schedule)
      VALUES ($1, $2)
    ON CONFLICT (address) DO
      UPDATE SET
        schedule = excluded.schedule`

	ConsensusEventInsert = `
    INSERT INTO chain.events (type, body, tx_block, tx_hash, tx_index, related_accounts)
      VALUES ($1, $2, $3, $4, $5, $6)`

	ConsensusAccountRelatedTransactionInsert = `
    INSERT INTO chain.accounts_related_transactions (account_address, tx_block, tx_index)
      VALUES ($1, $2, $3)`

	ConsensusAccountRelatedEventInsert = `
    INSERT INTO chain.accounts_related_events (account_address, event_block, tx_index, tx_hash, type, body)
      VALUES ($1, $2, $3, $4, $5, $6)`

	ConsensusRuntimeUpsert = `
    INSERT INTO chain.runtimes (id, suspended, kind, tee_hardware, key_manager)
      VALUES ($1, $2, $3, $4, $5)
      ON CONFLICT (id) DO
      UPDATE SET
        suspended = excluded.suspended,
        kind = excluded.kind,
        tee_hardware = excluded.tee_hardware,
        key_manager = excluded.key_manager`

	ConsensusClaimedNodeInsert = `
    INSERT INTO chain.claimed_nodes (entity_id, node_id) VALUES ($1, $2)
      ON CONFLICT (entity_id, node_id) DO NOTHING`

	ConsensusEntityUpsert = `
    INSERT INTO chain.entities (id, address) VALUES ($1, $2)
      ON CONFLICT (id) DO
      UPDATE SET
        address = excluded.address`

	ConsensusNodeUpsert = `
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

	ConsensusRuntimeNodesUpsert = `
    INSERT INTO chain.runtime_nodes (runtime_id, node_id) VALUES ($1, $2)
      ON CONFLICT (runtime_id, node_id) DO NOTHING`

	ConsensusRuntimeNodesDelete = `
    DELETE FROM chain.runtime_nodes WHERE node_id = $1`

	ConsensusNodeDelete = `
    DELETE FROM chain.nodes WHERE id = $1`

	ConsensusEntityMetaUpsert = `
    INSERT INTO chain.entities(id, meta)
      VALUES ($1, $2)
    ON CONFLICT (id) DO UPDATE
      SET meta = $2`

	ConsensusIncreaseGeneralBalanceUpsert = `
    INSERT INTO chain.accounts (address, general_balance)
      VALUES ($1, $2)
    ON CONFLICT (address) DO
      UPDATE SET general_balance = chain.accounts.general_balance + $2`

	ConsensusDecreaseGeneralBalanceUpsert = `
    UPDATE chain.accounts
    SET
      general_balance = general_balance - $2
    WHERE address = $1`

	ConsensusAddEscrowBalanceUpsert = `
    INSERT INTO chain.accounts (address, escrow_balance_active, escrow_total_shares_active)
      VALUES ($1, $2, $3)
    ON CONFLICT (address) DO
      UPDATE SET
        escrow_balance_active = chain.accounts.escrow_balance_active + $2,
        escrow_total_shares_active = chain.accounts.escrow_total_shares_active + $3`

	ConsensusAddDelegationsUpsert = `
    INSERT INTO chain.delegations (delegatee, delegator, shares)
      VALUES ($1, $2, $3)
    ON CONFLICT (delegatee, delegator) DO
      UPDATE SET shares = chain.delegations.shares + $3`

	ConsensusTakeEscrowUpdate = `
    UPDATE chain.accounts
      SET
        escrow_balance_active = escrow_balance_active - FLOOR($2 * escrow_balance_active / (escrow_balance_active + escrow_balance_debonding)),
        escrow_balance_debonding = escrow_balance_debonding - FLOOR($2 * escrow_balance_debonding / (escrow_balance_active + escrow_balance_debonding))
      WHERE address = $1`

	ConsensusDebondingStartEscrowBalanceUpdate = `
    UPDATE chain.accounts
      SET
        escrow_balance_active = escrow_balance_active - $2,
        escrow_total_shares_active = escrow_total_shares_active - $3,
        escrow_balance_debonding = escrow_balance_debonding + $2,
        escrow_total_shares_debonding = escrow_total_shares_debonding + $4
      WHERE address = $1`

	ConsensusDebondingStartDelegationsUpdate = `
    UPDATE chain.delegations
      SET shares = shares - $3
        WHERE delegatee = $1 AND delegator = $2`

	ConsensusDebondingStartDebondingDelegationsInsert = `
    INSERT INTO chain.debonding_delegations (delegatee, delegator, shares, debond_end)
      VALUES ($1, $2, $3, $4)`

	ConsensusReclaimEscrowBalanceUpdate = `
    UPDATE chain.accounts
      SET
        escrow_balance_debonding = escrow_balance_debonding - $2,
        escrow_total_shares_debonding = escrow_total_shares_debonding - $3
      WHERE address = $1`

	// Network upgrades delays debonding by 1 epoch.
	ConsensusDeleteDebondingDelegations = `
    DELETE FROM chain.debonding_delegations
      WHERE id = (
        SELECT id
        FROM chain.debonding_delegations
        WHERE
          delegator = $1 AND delegatee = $2 AND shares = $3 AND debond_end IN ($4::bigint, $4::bigint - 1)
        LIMIT 1
      )`

	ConsensusAllowanceChangeDelete = `
    DELETE FROM chain.allowances
      WHERE owner = $1 AND beneficiary = $2`

	ConsensusAllowanceOwnerUpsert = `
    INSERT INTO chain.accounts (address)
      VALUES ($1)
    ON CONFLICT (address) DO NOTHING`

	ConsensusAllowanceChangeUpdate = `
    INSERT INTO chain.allowances (owner, beneficiary, allowance)
      VALUES ($1, $2, $3)
    ON CONFLICT (owner, beneficiary) DO
      UPDATE SET allowance = excluded.allowance`

	ConsensusValidatorNodeUpdate = `
    UPDATE chain.nodes SET voting_power = $2
      WHERE id = $1`

	ConsensusCommitteeMemberInsert = `
    INSERT INTO chain.committee_members (node, valid_for, runtime, kind, role)
      VALUES ($1, $2, $3, $4, $5)`

	ConsensusCommitteeMembersTruncate = `
    TRUNCATE chain.committee_members`

	ConsensusProposalSubmissionInsert = `
    INSERT INTO chain.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	ConsensusProposalSubmissionCancelInsert = `
    INSERT INTO chain.proposals (id, submitter, state, deposit, cancels, created_at, closes_at)
      VALUES ($1, $2, $3, $4, $5, $6, $7)`

	ConsensusProposalExecutionsUpdate = `
    UPDATE chain.proposals
    SET executed = true
      WHERE id = $1`

	ConsensusProposalUpdate = `
    UPDATE chain.proposals
    SET state = $2
      WHERE id = $1`

	ConsensusProposalInvalidVotesUpdate = `
    UPDATE chain.proposals
    SET invalid_votes = $2
      WHERE id = $1`

	ConsensusVoteInsert = `
    INSERT INTO chain.votes (proposal, voter, vote)
      VALUES ($1, $2, $3)`

	RuntimeBlockInsert = `
    INSERT INTO chain.runtime_blocks (runtime, round, version, timestamp, block_hash, prev_block_hash, io_root, state_root, messages_hash, in_messages_hash, num_transactions, gas_used, size)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	RuntimeTransactionSignerInsert = `
    INSERT INTO chain.runtime_transaction_signers (runtime, round, tx_index, signer_index, signer_address, nonce)
      VALUES ($1, $2, $3, $4, $5, $6)`

	RuntimeRelatedTransactionInsert = `
    INSERT INTO chain.runtime_related_transactions (runtime, account_address, tx_round, tx_index)
      VALUES ($1, $2, $3, $4)`

	RuntimeTransactionInsert = `
    INSERT INTO chain.runtime_transactions (runtime, round, tx_index, tx_hash, tx_eth_hash, fee, gas_limit, gas_used, size, timestamp, method, body, "to", amount, success, error_module, error_code, error_message)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`

	RuntimeEventInsert = `
    INSERT INTO chain.runtime_events (runtime, round, tx_index, tx_hash, type, body, evm_log_name, evm_log_params, related_accounts)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	RuntimeMintInsert = `
    INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
      VALUES ($1, $2, NULL, $3, $4, $5)`

	RuntimeBurnInsert = `
    INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
      VALUES ($1, $2, $3, NULL, $4, $5)`

	RuntimeTransferInsert = `
    INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
      VALUES ($1, $2, $3, $4, $5, $6)`

	RuntimeDepositInsert = `
    INSERT INTO chain.runtime_deposits (runtime, round, sender, receiver, amount, nonce, module, code)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	RuntimeWithdrawInsert = `
    INSERT INTO chain.runtime_withdraws (runtime, round, sender, receiver, amount, nonce, module, code)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	RuntimeNativeBalanceUpdate = `
    INSERT INTO chain.runtime_sdk_balances (runtime, account_address, symbol, balance)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, account_address, symbol) DO
    UPDATE SET balance = chain.runtime_sdk_balances.balance + $4`

	AddressPreimageInsert = `
    INSERT INTO chain.address_preimages (address, context_identifier, context_version, address_data)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT DO NOTHING`

	RuntimeEVMTokenBalanceUpdate = `
    INSERT INTO chain.evm_token_balances (runtime, token_address, account_address, balance)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, token_address, account_address) DO
      UPDATE SET balance = chain.evm_token_balances.balance + $4`

	//nolint:gosec // thinks this is an authentication "token"
	RuntimeEVMTokenBalanceAnalysisInsert = `
	INSERT INTO chain.evm_token_balance_analysis
		(runtime, token_address, account_address, last_mutate_round)
	VALUES
		($1, $2, $3, $4)
	ON CONFLICT (runtime, token_address, account_address) DO UPDATE
	SET
		last_mutate_round = excluded.last_mutate_round`

	RuntimeEVMTokenAnalysisStale = `
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

	RuntimeEVMTokenAnalysisInsert = `
    INSERT INTO chain.evm_token_analysis (runtime, token_address, last_mutate_round)
      VALUES ($1, $2, $3)
    ON CONFLICT (runtime, token_address) DO NOTHING`

	RuntimeEVMTokenAnalysisMutateInsert = `
    INSERT INTO chain.evm_token_analysis (runtime, token_address, last_mutate_round)
      VALUES ($1, $2, $3)
    ON CONFLICT (runtime, token_address) DO UPDATE
      SET last_mutate_round = excluded.last_mutate_round`

	RuntimeEVMTokenAnalysisUpdate = `
    UPDATE chain.evm_token_analysis
    SET
      last_download_round = $3
    WHERE
      runtime = $1 AND
      token_address = $2`

	RuntimeEVMTokenInsert = `
    INSERT INTO chain.evm_tokens (runtime, token_address, token_type, token_name, symbol, decimals, total_supply)
      VALUES ($1, $2, $3, $4, $5, $6, $7)`

	RuntimeEVMTokenUpdate = `
    UPDATE chain.evm_tokens
    SET
      total_supply = $3
    WHERE
      runtime = $1 AND
      token_address = $2`

	//nolint:gosec // thinks this is an authentication "token"
	RuntimeEVMTokenBalanceAnalysisStale = `
	SELECT
		evm_token_balance_analysis.token_address,
		evm_token_balance_analysis.account_address,
		evm_token_balance_analysis.last_mutate_round,
		evm_tokens.token_type,
		evm_token_balances.balance,
		token_address_preimage.context_identifier,
		token_address_preimage.context_version,
		token_address_preimage.address_data,
		account_address_preimage.context_identifier,
		account_address_preimage.context_version,
		account_address_preimage.address_data
	FROM chain.evm_token_balance_analysis
	JOIN chain.evm_token_analysis USING (runtime, token_address)
	LEFT JOIN chain.evm_tokens USING (runtime, token_address)
	LEFT JOIN chain.evm_token_balances USING (runtime, token_address, account_address)
	LEFT JOIN chain.address_preimages AS token_address_preimage ON
		token_address_preimage.address = evm_token_balance_analysis.token_address
	LEFT JOIN chain.address_preimages AS account_address_preimage ON
		account_address_preimage.address = evm_token_balance_analysis.account_address
	WHERE
		evm_token_balance_analysis.runtime = $1 AND
		(
			evm_token_balance_analysis.last_download_round IS NULL OR
			evm_token_balance_analysis.last_mutate_round > evm_token_balance_analysis.last_download_round
		) AND
		evm_token_analysis.last_download_round IS NOT NULL
	LIMIT $2`

	//nolint:gosec // thinks this is an authentication "token"
	RuntimeEVMTokenBalanceAnalysisUpdate = `
	UPDATE chain.evm_token_balance_analysis
	SET
		last_download_round = $4
	WHERE
		runtime = $1 AND
		token_address = $2 AND
		account_address = $3`

	RefreshDailyTxVolume = `
    REFRESH MATERIALIZED VIEW stats.daily_tx_volume
  `

	RefreshMin5TxVolume = `
    REFRESH MATERIALIZED VIEW stats.min5_tx_volume
  `

	// LatestDailyAccountStats is the query to get the timestamp of the latest daily active accounts stat.
	LatestDailyAccountStats = `
    SELECT window_end
    FROM stats.daily_active_accounts
    WHERE layer = $1
    ORDER BY window_end DESC
    LIMIT 1
  `

	// InsertDailyAccountStats is the query to insert the daily active accounts stat.
	InsertDailyAccountStats = `
    INSERT INTO stats.daily_active_accounts (layer, window_end, active_accounts)
    VALUES ($1, $2, $3)
  `

	// EarliestConsensusBlockTime is the query to get the timestamp of the earliest
	// indexed consensus block.
	EarliestConsensusBlockTime = `
    SELECT time
    FROM chain.blocks
    ORDER BY height
    LIMIT 1
  `

	// LatestConsensusBlockTime is the query to get the timestamp of the latest
	// indexed consensus block.
	LatestConsensusBlockTime = `
    SELECT time
    FROM chain.blocks
    ORDER BY height DESC
    LIMIT 1
  `

	// EarliestRuntimeBlockTime is the query to get the timestamp of the earliest
	// indexed runtime block.
	EarliestRuntimeBlockTime = `
    SELECT timestamp
    FROM chain.runtime_blocks
    WHERE (runtime = $1)
    ORDER BY round
    LIMIT 1
  `

	// LatestRuntimeBlockTime is the query to get the timestamp of the latest
	// indexed runtime block.
	LatestRuntimeBlockTime = `
    SELECT timestamp
    FROM chain.runtime_blocks
    WHERE (runtime = $1)
    ORDER BY round DESC
    LIMIT 1
  `

	// ConsensusActiveAccounts is the query to get the number of
	// active accounts in the consensus layer within the given time range.
	ConsensusActiveAccounts = `
    SELECT COUNT(DISTINCT account_address)
    FROM chain.accounts_related_transactions AS art
    JOIN chain.blocks AS b ON art.tx_block = b.height
    WHERE (b.time >= $1::timestamptz AND b.time < $2::timestamptz)
    `

	// RuntimeActiveAccounts is the query to get the number of
	// active accounts in the runtime layer within the given time range.
	RuntimeActiveAccounts = `
    SELECT COUNT(DISTINCT account_address)
    FROM chain.runtime_related_transactions AS rt
    JOIN chain.runtime_blocks AS b ON (rt.runtime = b.runtime AND rt.tx_round = b.round)
    WHERE (rt.runtime = $1 AND b.timestamp >= $2::timestamptz AND b.timestamp < $3::timestamptz)
  `
)
