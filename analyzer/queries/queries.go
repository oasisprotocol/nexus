package queries

import (
	"fmt"

	"github.com/oasisprotocol/oasis-indexer/analyzer/runtime/evm"
)

var (
	FirstUnprocessedBlock = `
    SELECT LEAST( -- LEAST() ignores NULLs
      (
        -- The first explicitly tracked unprocessed block (locked but not yet processed).
        SELECT min(height) FROM chain.processed_blocks
        WHERE analyzer = $1 AND processed_time IS NULL
      ),
      (
        -- First block after all tracked processed blocks.
        -- This branch handles the case where there is no unprocessed blocks.
        SELECT max(height)+1 FROM chain.processed_blocks
        WHERE analyzer = $1
      )
    )`

	UnlockBlockForProcessing = `
    UPDATE chain.processed_blocks
    SET locked_time = '-infinity'
    WHERE analyzer = $1 AND height = $2`

	// TakeXactLock acquires an exclusive lock (with lock ID $1), with custom semantics.
	// The lock is automatically unlocked at the end of the db transaction.
	TakeXactLock = `
    SELECT pg_advisory_xact_lock($1)`

	PickBlocksForProcessing = `
    -- Locks a range of unprocessed blocks for an analyzer, handling expired locks and updating the lock status.
    -- Returns the locked blocks.
    -- Also referred to as the "mega-query".
    --
    -- Parameters:
    -- $1 = analyzer name (text)
    -- $2 = minimum block height (integer)
    -- $3 = maximum block height (integer)
    -- $4 = lock timeout in minutes (integer)
    -- $5 = number of blocks to lock (integer)

    WITH

    -- Find highest block that is processed (= processed_time set), or being processed (= locked_time set (and not expired)).
    -- We'll grab the next blocks from this height on.
    highest_done_block AS (
      SELECT COALESCE(max(height), -1) as height
      FROM chain.processed_blocks
      WHERE analyzer = $1 AND (processed_time IS NOT NULL OR locked_time >= CURRENT_TIMESTAMP - ($4::integer * INTERVAL '1 minute'))
    ),

    -- Find unprocessed blocks with expired locks (should be few and far between).
    expired_locks AS (
        SELECT height
        FROM chain.processed_blocks
        WHERE analyzer = $1 AND processed_time IS NULL AND (locked_time < CURRENT_TIMESTAMP - ($4::integer * INTERVAL '1 minute')) AND height >= $2 AND height <= $3
        ORDER BY height
        LIMIT $5
    ),

    -- The next $5 blocks from what is already processed or being processed.
    next_blocks AS (
        SELECT series_height.height
        FROM highest_done_block,
          generate_series(
            GREATEST(highest_done_block.height+1, $2), -- Don't go below $2
            LEAST( -- Don't go above $3.
              $3,
              GREATEST(highest_done_block.height+1, $2)+$5
            )
          ) AS series_height (height)
    ),

    -- Find the lowest eligible blocks to lock.
    blocks_to_lock AS (
      SELECT * FROM expired_locks
      UNION
      SELECT * FROM next_blocks
      ORDER BY height
      LIMIT $5
    )

    -- Lock the blocks. Try to insert a new row into processed_blocks; if a row already exists, update the lock.
    INSERT INTO chain.processed_blocks (analyzer, height, locked_time)
    SELECT $1, height, CURRENT_TIMESTAMP FROM blocks_to_lock
    ON CONFLICT (analyzer, height) DO UPDATE SET locked_time = excluded.locked_time
    RETURNING height`

	IsGenesisProcessed = `
    SELECT EXISTS (
      SELECT 1 FROM chain.processed_geneses
      WHERE chain_context = $1
    )`

	IndexingProgress = `
    UPDATE chain.processed_blocks
      SET height = $1, analyzer = $2, processed_time = CURRENT_TIMESTAMP
      WHERE height = $1 AND analyzer = $2`

	GenesisIndexingProgress = `
    INSERT INTO chain.processed_geneses (chain_context, processed_time)
      VALUES
        ($1, CURRENT_TIMESTAMP)`

	ConsensusBlockInsert = `
    INSERT INTO chain.blocks (height, block_hash, time, num_txs, namespace, version, type, root_hash)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	ConsensusEpochUpsert = `
    INSERT INTO chain.epochs AS epochs (id, start_height, end_height)
      VALUES ($1, $2, $2)
    ON CONFLICT (id) DO
    UPDATE SET
      start_height = LEAST(excluded.start_height, epochs.start_height),
      end_height = GREATEST(excluded.end_height, epochs.end_height)`

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
    INSERT INTO chain.entities(id, address, meta)
      VALUES ($1, $2, $3)
    ON CONFLICT (id) DO UPDATE SET
      meta = excluded.meta`

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
    INSERT INTO chain.runtime_transactions (runtime, round, tx_index, tx_hash, tx_eth_hash, fee, gas_limit, gas_used, size, timestamp, method, body, "to", amount, evm_encrypted_format, evm_encrypted_public_key, evm_encrypted_data_nonce, evm_encrypted_data_data, evm_encrypted_result_nonce, evm_encrypted_result_data, success, error_module, error_code, error_message)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24)`

	RuntimeEventInsert = `
    INSERT INTO chain.runtime_events (runtime, round, tx_index, tx_hash, type, body, related_accounts, evm_log_name, evm_log_params, evm_log_signature)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

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

	RuntimeEVMContractInsert = `
    INSERT INTO chain.evm_contracts
      (runtime, contract_address, creation_tx, creation_bytecode)
    VALUES ($1, $2, $3, $4)`

	RuntimeEVMTokenBalanceUpdate = `
    INSERT INTO chain.evm_token_balances (runtime, token_address, account_address, balance)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, token_address, account_address) DO
      UPDATE SET balance = chain.evm_token_balances.balance + $4`

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
      evm_token_analysis.last_download_round,
      evm_tokens.token_type,
      address_preimages.context_identifier,
      address_preimages.context_version,
      address_preimages.address_data,
      (SELECT MAX(height) FROM chain.processed_blocks WHERE analyzer = evm_token_analysis.runtime::text AND processed_time IS NOT NULL) AS download_round
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

	RuntimeEVMTokenBalanceAnalysisStale = fmt.Sprintf(`
    WITH
    max_processed_round AS (
      SELECT MAX(height) AS height
      FROM chain.processed_blocks
      WHERE analyzer = ($1::runtime)::text AND processed_time IS NOT NULL
    ),

    stale_evm_tokens AS (
      SELECT
        analysis.token_address,
        analysis.account_address,
        evm_tokens.token_type,
        evm_token_balances.balance,
        token_preimage.context_identifier,
        token_preimage.context_version,
        token_preimage.address_data,
        account_preimage.context_identifier,
        account_preimage.context_version,
        account_preimage.address_data,
        max_processed_round.height AS download_round
      FROM max_processed_round,
      chain.evm_token_balance_analysis AS analysis
      -- No LEFT JOIN; we need to know the token's type to query its balance.
      -- We do not exclude tokens with type=0 (unsupported) so that we can move them off the DB index of stale tokens.
      JOIN chain.evm_tokens USING (runtime, token_address)
      LEFT JOIN chain.evm_token_balances USING (runtime, token_address, account_address)
      LEFT JOIN chain.address_preimages AS token_preimage ON
        token_preimage.address = analysis.token_address
      LEFT JOIN chain.address_preimages AS account_preimage ON
        account_preimage.address = analysis.account_address
      WHERE
        analysis.runtime = $1 AND
        (
          analysis.last_download_round IS NULL OR
          analysis.last_mutate_round > analysis.last_download_round
        )
    ),
    
    stale_native_tokens AS (
      SELECT
        analysis.token_address,
        analysis.account_address,
        %d AS token_type,
        COALESCE(balances.balance, 0) AS balance,
        '' AS token_context_identifier,
        -1 AS token_context_version,
        ''::BYTEA AS token_address_data,
        account_preimage.context_identifier,
        account_preimage.context_version,
        account_preimage.address_data,
        max_processed_round.height AS download_round
      FROM max_processed_round,
      chain.evm_token_balance_analysis AS analysis
      LEFT JOIN chain.runtime_sdk_balances AS balances ON (
        balances.runtime = analysis.runtime AND
        balances.account_address = analysis.account_address AND
        balances.symbol = $2
      )
      LEFT JOIN chain.address_preimages AS account_preimage ON
        account_preimage.address = analysis.account_address
      WHERE
        analysis.runtime = $1 AND
        analysis.token_address = '%s' AND  -- Native token "address"
        (
          analysis.last_download_round IS NULL OR
          analysis.last_mutate_round > analysis.last_download_round
        )
    )

    SELECT * FROM (
      SELECT * FROM stale_evm_tokens
      UNION ALL
      SELECT * FROM stale_native_tokens
    ) foo LIMIT $3`,
		evm.EVMTokenTypeNative,
		evm.NativeRuntimeTokenAddress,
	)

	RuntimeEVMTokenBalanceAnalysisUpdate = `
    UPDATE chain.evm_token_balance_analysis
    SET
      last_download_round = $4
    WHERE
      runtime = $1 AND
      token_address = $2 AND
      account_address = $3`

	RefreshDailyTxVolume = `
    REFRESH MATERIALIZED VIEW CONCURRENTLY stats.daily_tx_volume
  `

	RefreshMin5TxVolume = `
    REFRESH MATERIALIZED VIEW CONCURRENTLY stats.min5_tx_volume
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
