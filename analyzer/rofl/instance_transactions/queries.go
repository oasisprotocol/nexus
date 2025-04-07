package instancetransactions

const (
	RuntimeRoflInstancesToProcess = `
        WITH
            current_epoch AS (
                SELECT id, end_height FROM chain.epochs ORDER BY id DESC LIMIT 1
            ),
            latest_round AS (
                SELECT round FROM chain.runtime_blocks WHERE runtime = $1 ORDER BY round DESC LIMIT 1
            )
        SELECT
            app_id, rak, extra_keys, last_processed_round
        FROM
            chain.rofl_instances, current_epoch, latest_round
        WHERE
            runtime = $1 AND
            -- Process instances which are active OR have not yet been processed.
            -- Note: we consider instances which have expired in the previous epoch as active, hence the >=,
            -- so that we don't miss any transactions at the end of the last active epoch.
            (expiration_epoch >= current_epoch.id AND latest_round.round > last_processed_round) OR registration_round > last_processed_round
        ORDER BY expiration_epoch DESC
        LIMIT $2`

	RuntimeRoflInstancesToProcessCount = `
        WITH
            current_epoch AS (
                SELECT id, end_height FROM chain.epochs ORDER BY id DESC LIMIT 1
            ),
            latest_round AS (
                SELECT round FROM chain.runtime_blocks WHERE runtime = $1 ORDER BY round DESC LIMIT 1
            )
        SELECT COUNT(*) AS cnt
        FROM chain.rofl_instances, current_epoch, latest_round
        WHERE
            runtime = $1 AND
            (expiration_epoch >= current_epoch.id AND latest_round.round > last_processed_round) OR registration_round > last_processed_round`

	RuntimeLatestRound = `
        SELECT round
        FROM chain.runtime_transactions
        WHERE runtime = $1
        ORDER BY round DESC
        LIMIT 1`

	RuntimeRoflInstanceTransactions = `
        SELECT DISTINCT ON (s.runtime, s.round, s.tx_index)
            s.round,
            s.tx_index,
            t.method,
            t.likely_native_transfer
        FROM chain.runtime_transaction_signers s
        JOIN chain.runtime_transactions t ON
            t.runtime = s.runtime AND
            t.round = s.round AND
            t.tx_index = s.tx_index
        WHERE
            s.runtime = $1
            AND s.signer_address = ANY($2)
            AND s.round > $3
            AND s.round <= $4
        ORDER BY s.round DESC
        -- Sanity limit. Only way this is exceeded is if there are some old instances
        -- which have been active for long before the analyzer has started. In that case
        -- it seems fine to skip/miss some old transactions.
        LIMIT 10000`

	RoflInstanceTransactionsUpsert = `
        INSERT INTO chain.rofl_instance_transactions (runtime, app_id, rak, tx_round, tx_index, method, likely_native_transfer)
        VALUES ($1, $2, $3, $4, $5, $6, $7)`

	RoflInstanceUpdateLastProcessedRound = `
        UPDATE chain.rofl_instances
        SET last_processed_round = $4
        WHERE runtime = $1 AND app_id = $2 AND rak = $3`
)
