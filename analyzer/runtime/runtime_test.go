package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
)

const RoundStart uint64 = 643746
const RoundEnd uint64 = 644246
const NumWorkers = 8

var SourceConfig = &config.SourceConfig{
	Cache: &config.CacheConfig{
		CacheDir:         "../../cache-cobalt",
		QueryOnCacheMiss: true,
	},
	ChainName: common.ChainNameMainnet,
	Nodes: map[string]*config.NodeConfig{
		"cobalt": {
			RPC: "(redacted)",
		},
	},
	FastStartup: true,
}

func TestCobalt(t *testing.T) {
	emeraldClient, err := oasis.NewRuntimeClient(context.Background(), SourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	totalTxs := 0
	for round := RoundStart; round < RoundEnd; round++ {
		fmt.Println(round)
		allData, err := emeraldClient.AllData(context.Background(), round)
		require.NoError(t, err)
		totalTxs += len(allData.TransactionsWithResults)
	}
	fmt.Println(totalTxs, "total txs")
}

func TestCobaltParallel(t *testing.T) {
	emeraldClient, err := oasis.NewRuntimeClient(context.Background(), SourceConfig, common.RuntimeEmerald)
	require.NoError(t, err)
	type Job struct {
		Round  uint64
		Result chan *storage.RuntimeAllData
	}
	jobs := make(chan Job, NumWorkers)
	results := make(chan Job, NumWorkers)
	go func() {
		for round := RoundStart; round < RoundEnd; round++ {
			job := Job{
				Round:  round,
				Result: make(chan *storage.RuntimeAllData, 1),
			}
			jobs <- job
			results <- job
		}
		close(jobs)
		close(results)
	}()
	for i := 0; i < NumWorkers; i++ {
		go func() {
			for {
				job, ok := <-jobs
				if !ok {
					break
				}
				fmt.Println(job.Round)
				allData, err := emeraldClient.AllData(context.Background(), job.Round)
				require.NoError(t, err)
				job.Result <- allData
			}
		}()
	}
	totalTxs := 0
	expectedRound := RoundStart
	for job := range results {
		require.Equal(t, expectedRound, job.Round)
		allData, ok := <-job.Result
		require.Equal(t, expectedRound, allData.Round)
		require.True(t, ok)
		totalTxs += len(allData.TransactionsWithResults)
		expectedRound++
	}
	require.Equal(t, RoundEnd, expectedRound)
	fmt.Println(totalTxs, "total txs")
}
