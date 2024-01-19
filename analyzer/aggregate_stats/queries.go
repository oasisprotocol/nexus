package aggregate_stats

const (
	// QueryEarliestConsensusBlockTime is the query to get the timestamp of the earliest
	// indexed consensus block.
	QueryEarliestConsensusBlockTime = `
		SELECT time
		FROM chain.blocks
		ORDER BY height
		LIMIT 1
	`

	// QueryEarliestRuntimeBlockTime is the query to get the timestamp of the earliest
	// indexed runtime block.
	QueryEarliestRuntimeBlockTime = `
		SELECT timestamp
		FROM chain.runtime_blocks
		WHERE (runtime = $1)
		ORDER BY round
		LIMIT 1
	`
	// QueryLatestConsensusBlockTime is the query to get the timestamp of the latest
	// indexed consensus block.
	QueryLatestConsensusBlockTime = `
		SELECT time
		FROM chain.blocks
		ORDER BY height DESC
		LIMIT 1
	`
	// QueryLatestRuntimeBlockTime is the query to get the timestamp of the latest
	// indexed runtime block.
	QueryLatestRuntimeBlockTime = `
		SELECT timestamp
		FROM chain.runtime_blocks
		WHERE (runtime = $1)
		ORDER BY round DESC
		LIMIT 1
	`

	// QueryConsensusTxVolume is the query to get the number the number of transactions
	// in the given window at the consensus layer.
	QueryConsensusTxVolume = `
		WITH dummy AS (SELECT $1::text) -- Dummy select that uses parameter $1 so that the query parameters are compatible with QueryRuntimeTxVolume.
		SELECT COUNT(*)
		FROM chain.blocks AS b
		JOIN chain.transactions AS t ON b.height = t.block
		WHERE (b.time >= $2::timestamptz AND b.time < $3::timestamptz)
	`

	// QueryRuntimeTxVolume is the query to get the number of of transactions
	// in the runtime layer within the given time range.
	QueryRuntimeTxVolume = `
		SELECT COUNT(*)
		FROM chain.runtime_transactions AS t
		WHERE (t.runtime = $1 AND t.timestamp >= $2::timestamptz AND t.timestamp < $3::timestamptz)
	`

	// QueryLatestStatsComputation is the query to get the timestamp of the latest computed stat window.
	QueryLatestStatsComputation = `
		SELECT window_end
		FROM %s
		WHERE layer = $1
		ORDER BY window_end DESC
		LIMIT 1
	`

	// QueryInsertStatsComputation is the query to insert the stats computation.
	QueryInsertStatsComputation = `
		INSERT INTO %s (layer, window_end, %s)
		VALUES ($1, $2, $3)
	`

	// QueryConsensusActiveAccounts is the query to get the number of
	// active accounts in the consensus layer within the given time range.
	QueryConsensusActiveAccounts = `
		WITH dummy AS (SELECT $1::text) -- Dummy select that uses parameter $1 so that the query parameters are compatible with QueryRuntimeActiveAccounts.
		SELECT COUNT(DISTINCT account_address)
		FROM chain.accounts_related_transactions AS art
		JOIN chain.blocks AS b ON art.tx_block = b.height
		WHERE (b.time >= $2::timestamptz AND b.time < $3::timestamptz)
	`

	// QueryRuntimeActiveAccounts is the query to get the number of
	// active accounts in the runtime layer within the given time range.
	//
	// Note: We explicitly convert the timestamp range to a round range
	// to force postgres to use the corresponding index on the
	// chain.runtime_related_transactions table. Without this conversion,
	// postgres does not realize that the timestamps correspond to a sequential
	// set of rounds and does a full table scan.
	QueryRuntimeActiveAccounts = `
		WITH relevant_rounds AS (
			SELECT min(round) as min_round, max(round) as max_round
			FROM chain.runtime_blocks AS b
			WHERE (b.runtime = $1 AND b.timestamp >= $2::timestamptz AND b.timestamp < $3::timestamptz)
		)
		SELECT COUNT(DISTINCT account_address)
		FROM chain.runtime_related_transactions AS rt, relevant_rounds
		WHERE (rt.runtime = $1 AND tx_round >= relevant_rounds.min_round AND tx_round <= relevant_rounds.max_round)
	`

	// QueryDailyTxVolume is the query to get the number of transactions within the given
	// time range, by using the already computed 5 minute windowed data.
	QueryDailyTxVolume = `
		SELECT COALESCE(SUM(t.tx_volume), 0)
		FROM stats.min5_tx_volume AS t
		WHERE (t.layer = $1 AND t.window_end >= $2::timestamptz AND t.window_end < $3::timestamptz)
	`

	// QueryLatestMin5TxVolume is the query to get the timestamp of the latest computed
	// 5 minute windowed data for the given layer.
	QueryLatestMin5TxVolume = `
		SELECT window_end
		FROM stats.min5_tx_volume as t
		WHERE t.layer = $1
		ORDER BY window_end DESC
		LIMIT 1
	`
)
