package queries

import (
	"fmt"

	"github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	"github.com/oasisprotocol/nexus/common"
)

var (
	FirstUnprocessedBlock = `
    SELECT LEAST( -- LEAST() ignores NULLs
      (
        -- The first explicitly tracked unprocessed block (locked but not yet processed).
        SELECT min(height) FROM analysis.processed_blocks
        WHERE analyzer = $1 AND processed_time IS NULL
      ),
      (
        -- First block after all tracked processed blocks.
        -- This branch handles the case where there is no unprocessed blocks.
        SELECT max(height)+1 FROM analysis.processed_blocks
        WHERE analyzer = $1
      )
    )`

	UnlockBlocksForProcessing = `
    UPDATE analysis.processed_blocks
    SET locked_time = '-infinity'
    WHERE analyzer = $1 AND height = ANY($2::uint63[]) AND processed_time IS NULL`

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
      FROM analysis.processed_blocks
      WHERE analyzer = $1 AND (processed_time IS NOT NULL OR locked_time >= CURRENT_TIMESTAMP - ($4::integer * INTERVAL '1 minute'))
    ),

    -- Find unprocessed blocks with expired locks (should be few and far between).
    expired_locks AS (
        SELECT height
        FROM analysis.processed_blocks
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
    INSERT INTO analysis.processed_blocks (analyzer, height, locked_time)
    SELECT $1, height, CURRENT_TIMESTAMP FROM blocks_to_lock
    ON CONFLICT (analyzer, height) DO UPDATE SET locked_time = excluded.locked_time
    RETURNING height`

	ProcessedSubrangeInfo = `
    -- Returns info about already-processed blocks in the given range; see below for description.
    -- Parameters:
    --   $1 = analyzer name (text)
    --   $2 = minimum block height, inclusive (integer)
    --   $3 = maximum block height, inclusive (integer; can be 0 or -1 to mean unlimited)

    WITH completed_blocks_in_range AS (
      SELECT height, is_fast_sync FROM analysis.processed_blocks
      WHERE
        analyzer = $1 AND
        height >= $2 AND ($3 <= 0 OR height <= $3) AND
        processed_time IS NOT NULL
      )

    SELECT
      -- Whether the processed subrange is a contiguous range that starts at the input range.
      COALESCE(
        (COUNT(*) = MAX(height) - MIN(height) + 1) AND MIN(height) = $2,
        TRUE -- If there are no processed blocks, we consider the range contiguous.
      ) AS is_contiguous,

      COALESCE(MAX(height), $2 - 1) AS max_processed_height
    FROM completed_blocks_in_range`

	IsBlockProcessedBySlowSync = `
    SELECT EXISTS(
      SELECT 1 FROM analysis.processed_blocks
      WHERE
        analyzer = $1 AND
        height = $2 AND
        processed_time IS NOT NULL AND
        NOT is_fast_sync
    )`

	SoftEnqueueGapsInProcessedBlocks = `
    -- Soft-enqueues gaps in analysis.processed_blocks, i.e. adds entries with
    -- expired locks for all heights that are not present in the table but are
    -- inside the [$2, $3] range and are also lower than the max already-processed height.
    -- Parameters:
    --   $1 = analyzer name (text)
    --   $2, $3 = height range in which to search for gaps (inclusive)

    WITH
    highest_encountered_block AS ( -- Note: encountered, not necessarily completed
      SELECT COALESCE(max(height), -1) as height
      FROM analysis.processed_blocks
      WHERE analyzer = $1
    )

    INSERT INTO analysis.processed_blocks (analyzer, height, locked_time)
    SELECT $1, h, '-infinity'::timestamptz
    FROM highest_encountered_block, generate_series(GREATEST(1, $2::bigint), LEAST(highest_encountered_block.height, $3::bigint)) AS h
    ON CONFLICT (analyzer, height) DO NOTHING`

	IndexingProgress = `
    UPDATE analysis.processed_blocks
      SET processed_time = CURRENT_TIMESTAMP, is_fast_sync = $3
      WHERE height = $1 AND analyzer = $2`

	NodeHeight = `
    SELECT height
    FROM chain.latest_node_heights
    WHERE layer = $1`

	NodeHeightUpsert = `
    INSERT INTO chain.latest_node_heights (layer, height)
      VALUES
        ($1, $2)
      ON CONFLICT (layer)
      DO UPDATE
        SET
        height = excluded.height`

	ConsensusBlockInsert = `
    INSERT INTO chain.blocks (height, block_hash, time, num_txs, namespace, version, state_root, epoch, gas_limit, size_limit)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	ConsensusEpochUpsert = `
    INSERT INTO chain.epochs AS old (id, start_height, end_height)
      VALUES ($1, $2, $2)
    ON CONFLICT (id) DO
    UPDATE SET
      start_height = LEAST(old.start_height, excluded.start_height),
      end_height = GREATEST(old.end_height, excluded.end_height)`

	ConsensusFastSyncEpochHeightInsert = `
    INSERT INTO todo_updates.epochs (epoch, height)
      VALUES ($1, $2)`

	ConsensusEpochsRecompute = `
    INSERT INTO chain.epochs AS old (id, start_height, end_height)
    (
      SELECT epoch, MIN(height) AS start_height, MAX(height) AS end_height
      FROM todo_updates.epochs
      GROUP BY epoch
    )
    ON CONFLICT (id) DO UPDATE SET
      start_height = LEAST(old.start_height, excluded.start_height),
      end_height = GREATEST(old.end_height, excluded.end_height)`

	ConsensusTransactionInsert = `
    INSERT INTO chain.transactions (block, tx_hash, tx_index, nonce, fee_amount, max_gas, method, sender, body, module, code, message)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	ConsensusAccountUpsert = `
    INSERT INTO chain.accounts
      (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
    VALUES
      ($1, $2, $3, $4, $5, $6, $7)
    ON CONFLICT (address) DO UPDATE
    SET
      general_balance = excluded.general_balance,
      nonce = excluded.nonce,
      escrow_balance_active = excluded.escrow_balance_active,
      escrow_total_shares_active = excluded.escrow_total_shares_active,
      escrow_balance_debonding = excluded.escrow_balance_debonding,
      escrow_total_shares_debonding = excluded.escrow_total_shares_debonding`

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
    INSERT INTO chain.events (type, body, tx_block, tx_hash, tx_index, related_accounts, roothash_runtime_id, roothash_runtime, roothash_runtime_round)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	ConsensusRoothashMessageScheduleUpsert = `
    INSERT INTO chain.roothash_messages
      (runtime, round, message_index, type, body, related_accounts)
    VALUES
      ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (runtime, round, message_index) DO UPDATE
    SET
      type = excluded.type,
      body = excluded.body,
      related_accounts = excluded.related_accounts`

	ConsensusRoothashMessageFinalizeUpsert = `
    INSERT INTO chain.roothash_messages
      (runtime, round, message_index, error_module, error_code, result)
    VALUES
      ($1, $2, $3, $4, $5, $6)
    ON CONFLICT (runtime, round, message_index) DO UPDATE
    SET
      error_module = excluded.error_module,
      error_code = excluded.error_code,
      result = excluded.result`

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

	ConsensusRuntimeSuspendedUpdate = `
    UPDATE chain.runtimes
      SET suspended = $2
      WHERE id = $1`

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
    INSERT INTO chain.entities(id, address, meta, logo_url)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (id) DO UPDATE SET
      meta = excluded.meta,
      logo_url = excluded.logo_url`

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

	ConsensusTakeEscrowUpdateGuessRatio = `
    UPDATE chain.accounts
      SET
        escrow_balance_active = escrow_balance_active - FLOOR($2 * escrow_balance_active / (escrow_balance_active + escrow_balance_debonding)),
        escrow_balance_debonding = escrow_balance_debonding - FLOOR($2 * escrow_balance_debonding / (escrow_balance_active + escrow_balance_debonding))
      WHERE address = $1`

	ConsensusTakeEscrowUpdateExact = `
    UPDATE chain.accounts
      SET
        escrow_balance_active = escrow_balance_active - $2,
        escrow_balance_debonding = escrow_balance_debonding - $3
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

	ConsensusDelegationDeleteIfZeroShares = `
    DELETE FROM chain.delegations
      WHERE delegatee = $1 AND delegator = $2 AND shares = 0`

	ConsensusDebondingStartDebondingDelegationsUpsert = `
    INSERT INTO chain.debonding_delegations (delegatee, delegator, shares, debond_end)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (delegatee, delegator, debond_end) DO
      UPDATE SET shares = chain.debonding_delegations.shares + $3`

	ConsensusReclaimEscrowBalanceUpdate = `
    UPDATE chain.accounts
      SET
        escrow_balance_debonding = escrow_balance_debonding - $2,
        escrow_total_shares_debonding = escrow_total_shares_debonding - $3
      WHERE address = $1`

	// debond_end IN ($4::bigint, $4::bigint - 1, 0) is used because:
	// - Network upgrades delays debonding by 1 epoch.
	// - Some very old events might not have the debond_end set, so we have 0 in the Db.
	//   This should not be problematic in practice since nowadays we have fast-sync where
	//   we skip inserting debonding delegations for old epochs, so we should not encounter this.
	ConsensusDeleteDebondingDelegations = `
    DELETE FROM chain.debonding_delegations
      WHERE delegator = $1 AND delegatee = $2 AND shares = $3 AND debond_end IN ($4::bigint, $4::bigint - 1, 0)`

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

	ConsensusValidatorNodeResetVotingPowers = `
    UPDATE chain.nodes SET voting_power = 0`

	ConsensusValidatorNodeUpdateVotingPower = `
    UPDATE chain.nodes SET voting_power = $2
      WHERE id = $1`

	ConsensusCommitteeMemberInsert = `
    INSERT INTO chain.committee_members (node, valid_for, runtime, kind, role)
      VALUES ($1, $2, $3, $4, $5)`

	ConsensusCommitteeMembersTruncate = `
    TRUNCATE chain.committee_members`

	ConsensusProposalSubmissionInsert = `
    INSERT INTO chain.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, created_at, closes_at, invalid_votes)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	ConsensusProposalSubmissionCancelInsert = `
    INSERT INTO chain.proposals (id, submitter, state, deposit, cancels, created_at, closes_at, invalid_votes)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	ConsensusProposalSubmissionChangeParametersInsert = `
    INSERT INTO chain.proposals (id, submitter, state, deposit, parameters_change_module, parameters_change, created_at, closes_at, invalid_votes)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

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

	ConsensusVoteUpsert = `
    INSERT INTO chain.votes (proposal, voter, vote, height)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (proposal, voter) DO UPDATE SET
	    vote = excluded.vote,
      height = excluded.height;`

	RuntimeBlockInsert = `
    INSERT INTO chain.runtime_blocks (runtime, round, version, timestamp, block_hash, prev_block_hash, io_root, state_root, messages_hash, in_messages_hash, num_transactions, gas_used, size)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`

	RuntimeTransactionSignerInsert = `
    INSERT INTO chain.runtime_transaction_signers (runtime, round, tx_index, signer_index, signer_address, nonce)
      VALUES ($1, $2, $3, $4, $5, $6)`

	RuntimeRelatedTransactionInsert = `
    INSERT INTO chain.runtime_related_transactions (runtime, account_address, tx_round, tx_index)
      VALUES ($1, $2, $3, $4)`

	RuntimeAccountNumTxsUpsert = `
    INSERT INTO chain.runtime_accounts as accounts (runtime, address, num_txs)
      VALUES ($1, $2, $3)
    ON CONFLICT (runtime, address) DO UPDATE
      SET num_txs = accounts.num_txs + $3;`

	// Recomputes the number of related transactions for all runtime account in runtime $1.
	// Inteded for use after fast-sync that ran up to height $2 (inclusive).
	RuntimeAccountNumTxsRecompute = `
    WITH agg AS (
      SELECT runtime, account_address, count(*) AS num_txs
      FROM chain.runtime_related_transactions
      WHERE runtime = $1::runtime AND tx_round <= $2::bigint
      GROUP BY 1, 2
    )
    INSERT INTO chain.runtime_accounts AS accts (runtime, address, num_txs)
    SELECT runtime, account_address, num_txs FROM agg
    ON CONFLICT (runtime, address) DO UPDATE
      SET num_txs = EXCLUDED.num_txs`

	RuntimeAccountTotalSentUpsert = `
    INSERT INTO chain.runtime_accounts as accounts (runtime, address, total_sent)
      VALUES ($1, $2, $3)
    ON CONFLICT (runtime, address) DO UPDATE
      SET total_sent = accounts.total_sent + $3`

	RuntimeAccountTotalSentRecompute = `
    WITH agg AS (
      SELECT runtime, sender, sum(amount) AS total_sent
      FROM chain.runtime_transfers
      WHERE runtime = $1::runtime AND round <= $2::bigint AND sender IS NOT NULL
      GROUP BY 1, 2
    )
    INSERT INTO chain.runtime_accounts as accts (runtime, address, total_sent)
    SELECT runtime, sender, total_sent FROM agg
    ON CONFLICT (runtime, address) DO UPDATE
      SET total_sent = EXCLUDED.total_sent`

	RuntimeAccountTotalReceivedUpsert = `
    INSERT INTO chain.runtime_accounts as accounts (runtime, address, total_received)
      VALUES ($1, $2, $3)
    ON CONFLICT (runtime, address) DO UPDATE
      SET total_received = accounts.total_received + $3`

	// $3 should be the symbol of the _native_ token.
	RuntimeAccountTotalReceivedRecompute = `
    WITH agg AS (
      SELECT runtime, receiver, sum(amount) AS total_received
      FROM chain.runtime_transfers
      WHERE runtime = $1::runtime AND round <= $2::bigint AND receiver IS NOT NULL AND symbol = $3
      GROUP BY 1, 2
    )
    INSERT INTO chain.runtime_accounts as accts (runtime, address, total_received)
    SELECT runtime, receiver, total_received FROM agg
    ON CONFLICT (runtime, address) DO UPDATE
      SET total_received = EXCLUDED.total_received`

	RuntimeTransactionInsert = `
    INSERT INTO chain.runtime_transactions (runtime, round, tx_index, tx_hash, tx_eth_hash, fee, gas_limit, gas_used, size, timestamp, method, body, "to", amount, evm_encrypted_format, evm_encrypted_public_key, evm_encrypted_data_nonce, evm_encrypted_data_data, evm_encrypted_result_nonce, evm_encrypted_result_data, success, error_module, error_code, error_message_raw, error_message)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)`

	// We use COALESCE here to avoid overwriting existing data with null values.
	RuntimeTransactionEvmParsedFieldsUpdate = `
    UPDATE chain.runtime_transactions
    SET
      evm_fn_name = COALESCE($3, evm_fn_name),
      evm_fn_params = COALESCE($4, evm_fn_params),
      error_message = COALESCE($5, error_message),
      error_params = COALESCE($6, error_params),
      abi_parsed_at = CURRENT_TIMESTAMP
    WHERE
      runtime = $1 AND
      tx_hash = $2`

	RuntimeEventInsert = `
    INSERT INTO chain.runtime_events (runtime, round, tx_index, tx_hash, tx_eth_hash, timestamp, type, body, related_accounts, evm_log_name, evm_log_params, evm_log_signature)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	// We use COALESCE here to avoid overwriting existing data with null values.
	RuntimeEventEvmParsedFieldsUpdate = `
    UPDATE chain.runtime_events
    SET
      evm_log_name = COALESCE($5, evm_log_name),
      evm_log_params = COALESCE($6, evm_log_params),
      evm_log_signature = COALESCE($7, evm_log_signature),
      abi_parsed_at = CURRENT_TIMESTAMP
    WHERE
      runtime = $1 AND
      round = $2 AND
      tx_index = $3 AND
      body = $4`

	RuntimeMintInsert = `
    INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
      VALUES ($1, $2, NULL, $3, $4, $5)`

	RuntimeBurnInsert = `
    INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
      VALUES ($1, $2, $3, NULL, $4, $5)`

	RuntimeTransferInsert = `
    INSERT INTO chain.runtime_transfers (runtime, round, sender, receiver, symbol, amount)
      VALUES ($1, $2, $3, $4, $5, $6)`

	RuntimeNativeBalanceUpsert = `
    INSERT INTO chain.runtime_sdk_balances AS old (runtime, account_address, symbol, balance)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, account_address, symbol) DO
    UPDATE SET balance = old.balance + $4`

	RuntimeNativeBalanceAbsoluteUpsert = `
    INSERT INTO chain.runtime_sdk_balances (runtime, account_address, symbol, balance)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, account_address, symbol) DO
    UPDATE SET balance = $4`

	AddressPreimageInsert = `
    INSERT INTO chain.address_preimages (address, context_identifier, context_version, address_data)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT DO NOTHING`

	RuntimeEVMContractCreationUpsert = `
    INSERT INTO chain.evm_contracts
      (runtime, contract_address, creation_tx, creation_bytecode)
    VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, contract_address) DO UPDATE
    SET
      creation_tx = $3,
      creation_bytecode = $4`

	RuntimeEVMContractRuntimeBytecodeUpsert = `
    INSERT INTO chain.evm_contracts(runtime, contract_address, runtime_bytecode)
    VALUES ($1, $2, $3)
    ON CONFLICT (runtime, contract_address) DO UPDATE
    SET runtime_bytecode = $3`

	RuntimeAccountGasForCallingUpsert = `
    INSERT INTO chain.runtime_accounts AS old (runtime, address, num_txs, gas_for_calling)
    VALUES ($1, $2, 0, $3)
    ON CONFLICT (runtime, address) DO UPDATE
    SET gas_for_calling = old.gas_for_calling + $3`

	// Recomputes the total gas used for calling every runtime contract in runtime $1.
	// Inteded for use after fast-sync that ran up to height $2 (inclusive).
	RuntimeAccountGasForCallingRecompute = `
    WITH agg AS (
      SELECT runtime, "to" AS contract_address, SUM(gas_used) as gas_for_calling
      FROM chain.runtime_transactions
      WHERE runtime = $1::runtime AND round <= $2::bigint AND method IN ('evm.Call', 'evm.Create')
      GROUP BY runtime, "to"
      HAVING "to" IS NOT NULL
    )
    INSERT INTO chain.runtime_accounts AS accts (runtime, address, gas_for_calling)
    SELECT runtime, contract_address, gas_for_calling FROM agg
    ON CONFLICT (runtime, address) DO UPDATE
      SET gas_for_calling = EXCLUDED.gas_for_calling`

	RuntimeEVMContractCodeAnalysisInsert = `
    INSERT INTO analysis.evm_contract_code(runtime, contract_candidate)
    VALUES ($1, $2)
    ON CONFLICT (runtime, contract_candidate) DO NOTHING`

	RuntimeEVMContractCodeAnalysisSetIsContract = `
    UPDATE analysis.evm_contract_code
    SET is_contract = $3
    WHERE runtime = $1 AND contract_candidate = $2`

	RuntimeEVMContractCodeAnalysisStale = `
    SELECT
      code_analysis.contract_candidate,
      pre.address_data AS eth_contract_candidate,
      (SELECT MAX(height) FROM analysis.processed_blocks WHERE analyzer = $1::runtime::text AND processed_time IS NOT NULL) AS download_round
    FROM analysis.evm_contract_code AS code_analysis
    JOIN chain.address_preimages AS pre ON
      pre.address = code_analysis.contract_candidate AND
      pre.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
      pre.context_version = 0
    WHERE
      code_analysis.runtime = $1::runtime AND
      code_analysis.is_contract IS NULL
    LIMIT $2`

	RuntimeEVMContractCodeAnalysisStaleCount = `
    SELECT COUNT(*) AS cnt
    FROM analysis.evm_contract_code AS code_analysis
    WHERE
      code_analysis.runtime = $1::runtime AND
      code_analysis.is_contract IS NULL`

	RuntimeEVMTokenBalanceUpdate = `
    INSERT INTO chain.evm_token_balances (runtime, token_address, account_address, balance)
      VALUES ($1, $2, $3, $4)
    ON CONFLICT (runtime, token_address, account_address) DO
      UPDATE SET balance = chain.evm_token_balances.balance + $4`

	RuntimeEVMTokenBalanceAnalysisMutateRoundUpsert = `
    INSERT INTO analysis.evm_token_balances
      (runtime, token_address, account_address, last_mutate_round)
    VALUES
      ($1, $2, $3, $4)
    ON CONFLICT (runtime, token_address, account_address) DO UPDATE
    SET
      last_mutate_round = GREATEST(excluded.last_mutate_round, analysis.evm_token_balances.last_mutate_round)`

	RuntimeFastSyncEVMTokenBalanceAnalysisMutateRoundInsert = `
    INSERT INTO todo_updates.evm_token_balances
      (runtime, token_address, account_address, last_mutate_round)
    VALUES
      ($1, $2, $3, $4)`

	// Recomputes the last round at which a token balance was known to mutate.
	// Inteded for use after fast-sync.
	RuntimeEVMTokenBalanceAnalysisMutateRoundRecompute = `
    INSERT INTO analysis.evm_token_balances AS old
      (runtime, token_address, account_address, last_mutate_round)
    (
      SELECT runtime, token_address, account_address, MAX(last_mutate_round)
      FROM todo_updates.evm_token_balances
      WHERE runtime = $1
      GROUP BY runtime, token_address, account_address
    )
    ON CONFLICT (runtime, token_address, account_address) DO UPDATE
    SET
      last_mutate_round = GREATEST(old.last_mutate_round, excluded.last_mutate_round)`

	RuntimeEVMTokenAnalysisStale = `
    SELECT
      t.token_address,
      t.last_download_round,
      t.total_supply,
      t.num_transfers,
      t.token_type,
      ap.context_identifier,
      ap.context_version,
      ap.address_data,
      (SELECT MAX(height) FROM analysis.processed_blocks WHERE analyzer = t.runtime::text AND processed_time IS NOT NULL) AS download_round
    FROM chain.evm_tokens AS t
    LEFT JOIN chain.address_preimages AS ap ON
      ap.address = t.token_address
    WHERE
      t.runtime = $1 AND
      (
        t.last_download_round IS NULL OR
        t.last_mutate_round > t.last_download_round
      )
    LIMIT $2`

	RuntimeEVMTokenAnalysisStaleCount = `
    SELECT COUNT(*) AS cnt
    FROM chain.evm_tokens AS t
    WHERE
      t.runtime = $1 AND
      (
        t.last_download_round IS NULL OR
        t.last_mutate_round > t.last_download_round
      )`

	// Upserts a new EVM token, but the column values are treated as deltas (!) to the existing values.
	// NOTE: Passing a 0 for last_mutate round causes that field to not be updated, effectively signalling
	//       "this upsert does not create a need for a subsequent download of info from the EVM runtime".
	RuntimeEVMTokenDeltaUpsert = `
    INSERT INTO chain.evm_tokens AS old (runtime, token_address, total_supply, num_transfers, last_mutate_round)
      VALUES ($1, $2, $3, $4, $5)
    ON CONFLICT (runtime, token_address) DO UPDATE SET
      total_supply = old.total_supply + $3,
      num_transfers = old.num_transfers + $4,
      last_mutate_round = GREATEST(old.last_mutate_round, $5)`

	RuntimeFastSyncEVMTokenDeltaInsert = `
    INSERT INTO todo_updates.evm_tokens AS old (runtime, token_address, total_supply, num_transfers, last_mutate_round)
      VALUES ($1, $2, $3, $4, $5)`

	RuntimeEVMTokenRecompute = `
    INSERT INTO chain.evm_tokens AS old (runtime, token_address, total_supply, num_transfers, last_mutate_round)
    (
      SELECT runtime, token_address, SUM(total_supply) AS total_supply, SUM(num_transfers) AS num_transfers, MAX(last_mutate_round) AS last_mutate_round
      FROM todo_updates.evm_tokens
      WHERE runtime = $1
      GROUP BY runtime, token_address
    )
    ON CONFLICT (runtime, token_address) DO UPDATE SET
      total_supply = old.total_supply + excluded.total_supply,
      num_transfers = old.num_transfers + excluded.num_transfers,
      last_mutate_round = GREATEST(old.last_mutate_round, excluded.last_mutate_round)`

	// Upserts a new EVM token with information that was downloaded from the EVM runtime (as opposed to dead-reckoned).
	RuntimeEVMTokenDownloadedUpsert = `
    INSERT INTO chain.evm_tokens (runtime, token_address, token_type, token_name, symbol, decimals, total_supply, last_download_round)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    ON CONFLICT (runtime, token_address) DO
      UPDATE SET
        token_type = excluded.token_type,
        token_name = excluded.token_name,
        symbol = excluded.symbol,
        decimals = excluded.decimals,
        total_supply = excluded.total_supply,
        last_download_round = excluded.last_download_round`

	// Updates the total_supply of an EVM token with information that was downloaded from the EVM runtime (as opposed to dead-reckoned).
	RuntimeEVMTokenDownloadedTotalSupplyUpdate = `
    UPDATE chain.evm_tokens
    SET
      total_supply = $3
    WHERE
      runtime = $1 AND
      token_address = $2`

	// Updates the last_download_round of an EVM token.
	RuntimeEVMTokenDownloadRoundUpdate = `
    UPDATE chain.evm_tokens
    SET
      last_download_round = $3
    WHERE
      runtime = $1 AND
      token_address = $2`

	RuntimeEVMNFTUpdate = `
    UPDATE chain.evm_nfts SET
      last_download_round = $4,
      metadata_uri = $5,
      metadata_accessed = $6,
      metadata = $7,
      name = $8,
      description = $9,
      image = $10
    WHERE
      runtime = $1 AND
      token_address = $2 AND
      nft_id = $3`

	RuntimeEVMNFTUpsert = `
    INSERT INTO chain.evm_nfts AS old
      (runtime, token_address, nft_id, num_transfers, last_want_download_round)
    VALUES
      ($1, $2, $3, 0, $4)
    ON CONFLICT (runtime, token_address, nft_id) DO NOTHING`

	RuntimeEVMNFTUpdateTransfer = `
    UPDATE chain.evm_nfts SET
      num_transfers = num_transfers + $4,
      owner = $5
    WHERE
      runtime = $1 AND
      token_address = $2 AND
      nft_id = $3`

	RuntimeEVMNFTAnalysisStale = `
    SELECT
      chain.evm_nfts.token_address,
      chain.evm_nfts.nft_id,
      chain.evm_tokens.token_type,
      chain.address_preimages.context_identifier,
      chain.address_preimages.context_version,
      chain.address_preimages.address_data,
      (
        SELECT MAX(height)
        FROM analysis.processed_blocks
        WHERE
          analysis.processed_blocks.analyzer = chain.evm_nfts.runtime::TEXT AND
          processed_time IS NOT NULL
      ) AS download_round
    FROM chain.evm_nfts
    JOIN chain.evm_tokens USING
      (runtime, token_address)
    LEFT JOIN chain.address_preimages ON
      chain.address_preimages.address = chain.evm_nfts.token_address
    WHERE
      chain.evm_nfts.runtime = $1 AND
      (
          chain.evm_nfts.last_download_round IS NULL OR
          chain.evm_nfts.last_want_download_round > chain.evm_nfts.last_download_round
      ) AND
      chain.evm_tokens.token_type IS NOT NULL
    LIMIT $2`

	RuntimeEVMNFTAnalysisStaleCount = `
    SELECT COUNT(*) AS cnt
    FROM chain.evm_nfts
    WHERE
      runtime = $1 AND
      (last_download_round IS NULL OR last_want_download_round > last_download_round)`

	RuntimeEVMTokenBalanceAnalysisStale = fmt.Sprintf(`
    WITH
    max_processed_round AS (
      SELECT MAX(height) AS height
      FROM analysis.processed_blocks
      WHERE analyzer = ($1::runtime)::text AND processed_time IS NOT NULL
    ),

    stale_evm_tokens AS (
      SELECT
        balance_analysis.token_address,
        balance_analysis.account_address,
        evm_tokens.token_type,
        COALESCE(evm_token_balances.balance, 0) AS balance, -- evm_token_balances entry can be absent in fast-sync mode
        token_preimage.context_identifier,
        token_preimage.context_version,
        token_preimage.address_data,
        account_preimage.context_identifier,
        account_preimage.context_version,
        account_preimage.address_data,
        max_processed_round.height AS download_round
      FROM max_processed_round,
      analysis.evm_token_balances AS balance_analysis
      -- No LEFT JOIN; we need to know the token's type to query its balance.
      -- We do not exclude tokens with type=0 (unsupported) so that we can move them off the DB index of stale tokens.
      JOIN chain.evm_tokens USING (runtime, token_address)
      LEFT JOIN chain.evm_token_balances USING (runtime, token_address, account_address)
      LEFT JOIN chain.address_preimages AS token_preimage ON
        token_preimage.address = balance_analysis.token_address
      LEFT JOIN chain.address_preimages AS account_preimage ON
        account_preimage.address = balance_analysis.account_address
      WHERE
        evm_tokens.token_type IS NOT NULL AND
        balance_analysis.runtime = $1 AND
        (
          balance_analysis.last_download_round IS NULL OR
          balance_analysis.last_mutate_round > balance_analysis.last_download_round
        )
    ),

    stale_native_tokens AS (
      SELECT
        balance_analysis.token_address,
        balance_analysis.account_address,
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
      analysis.evm_token_balances AS balance_analysis
      LEFT JOIN chain.runtime_sdk_balances AS balances ON (
        balances.runtime = balance_analysis.runtime AND
        balances.account_address = balance_analysis.account_address AND
        balances.symbol = $2
      )
      LEFT JOIN chain.address_preimages AS account_preimage ON
        account_preimage.address = balance_analysis.account_address
      WHERE
        balance_analysis.runtime = $1 AND
        balance_analysis.token_address = '%s' AND  -- Native token "address"
        (
          balance_analysis.last_download_round IS NULL OR
          balance_analysis.last_mutate_round > balance_analysis.last_download_round
        )
    )

    SELECT * FROM (
      SELECT * FROM stale_evm_tokens
      UNION ALL
      SELECT * FROM stale_native_tokens
    ) foo LIMIT $3`,
		common.TokenTypeNative,
		evm.NativeRuntimeTokenAddress,
	)

	RuntimeEVMTokenBalanceAnalysisStaleCount = `
    SELECT COUNT(*) AS cnt
    FROM analysis.evm_token_balances AS balance_analysis
    WHERE
      balance_analysis.runtime = $1 AND
      (
        balance_analysis.last_download_round IS NULL OR
        balance_analysis.last_mutate_round > balance_analysis.last_download_round
      )`

	RuntimeEVMTokenBalanceAnalysisUpdate = `
    UPDATE analysis.evm_token_balances
    SET
      last_download_round = $4
    WHERE
      runtime = $1 AND
      token_address = $2 AND
      account_address = $3`

	RuntimeEVMUnverfiedContracts = `
    SELECT contracts.contract_address,
      address_preimages.context_identifier,
      address_preimages.context_version,
      address_preimages.address_data
    FROM chain.evm_contracts AS contracts
    LEFT JOIN chain.address_preimages AS address_preimages ON
      address_preimages.address = contracts.contract_address AND
      address_preimages.context_identifier = 'oasis-runtime-sdk/address: secp256k1eth' AND
      address_preimages.context_version = 0
    WHERE
      runtime = $1 AND verification_info_downloaded_at IS NULL`

	RuntimeEVMVerifiedContracts = `
    SELECT
      contracts.contract_address,
      contracts.verification_level
    FROM chain.evm_contracts AS contracts
    WHERE
      runtime = $1 AND verification_level IS NOT NULL`

	RuntimeEVMVerifyContractUpsert = `
    INSERT INTO chain.evm_contracts (runtime, contract_address, verification_info_downloaded_at, abi, compilation_metadata, source_files, verification_level)
    VALUES ($1, $2, CURRENT_TIMESTAMP, $3, $4, $5, $6)
    ON CONFLICT (runtime, contract_address) DO UPDATE
    SET
      verification_info_downloaded_at = CURRENT_TIMESTAMP,
      abi = EXCLUDED.abi,
      compilation_metadata = EXCLUDED.compilation_metadata,
      source_files = EXCLUDED.source_files,
      verification_level = EXCLUDED.verification_level`

	RuntimeEvmVerifiedContractTxs = `
    WITH abi_contracts AS (
      SELECT
        runtime,
        contract_address AS addr,
        abi,
        verification_info_downloaded_at
      FROM chain.evm_contracts
      WHERE
        runtime = $1 AND abi IS NOT NULL
    )
    SELECT
      abi_contracts.addr,
      abi_contracts.abi,
      txs.tx_hash,
      decode(txs.body->>'data', 'base64'),
      txs.error_message_raw
    FROM abi_contracts
    JOIN chain.runtime_transactions as txs ON
      txs.runtime = abi_contracts.runtime AND
      txs.to = abi_contracts.addr AND
      txs.method = 'evm.Call' -- note: does not include evm.Create txs; their payload is never encrypted.
    WHERE
      txs.body IS NOT NULL AND
      (txs.abi_parsed_at IS NULL OR txs.abi_parsed_at < abi_contracts.verification_info_downloaded_at)
    ORDER BY addr
    LIMIT $2`

	RuntimeEvmVerifiedContractEvents = `
    WITH abi_contracts AS (
      SELECT
        runtime,
        contract_address AS addr,
        abi,
        verification_info_downloaded_at
      FROM chain.evm_contracts
      WHERE
        runtime = $1 AND
        abi IS NOT NULL
    )
    SELECT
      abi_contracts.addr,
      abi_contracts.abi,
      evs.round,
      evs.tx_index,
      evs.body
    FROM abi_contracts
    JOIN chain.address_preimages as preimages ON
      abi_contracts.addr = preimages.address
    JOIN chain.runtime_events as evs ON
      evs.type = 'evm.log' AND
      evs.runtime = abi_contracts.runtime AND
      decode(body->>'address', 'base64') = preimages.address_data
    WHERE
      (evs.abi_parsed_at IS NULL OR evs.abi_parsed_at < abi_contracts.verification_info_downloaded_at)
    LIMIT $2`
)
