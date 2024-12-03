package block_test

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/block"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/util"
	analyzerCmd "github.com/oasisprotocol/nexus/cmd/analyzer"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/postgres"
	pgTestUtil "github.com/oasisprotocol/nexus/storage/postgres/testutil"
)

// Relative path to the migrations directory when running tests in this file.
// When running go tests, the working directory is always set to the package directory of the test being run.
const migrationsPath = "file://../../storage/migrations"

const testsTimeout = 10 * time.Second

// Default block based config used in slow-sync tests.
var testBlockBasedConfig = &config.BlockBasedAnalyzerConfig{From: 1, To: 1_000, BatchSize: 100}

// Default block based config used in fast-sync tests.
var testFastSyncBlockBasedConfig = &config.BlockBasedAnalyzerConfig{
	From: 1, To: 2_000,
	BatchSize: 100,
	FastSync:  &config.FastSyncConfig{To: 1_000, Parallelism: 3},
}

type mockProcessor struct {
	name       string
	storage    storage.TargetStorage
	isFastSync bool // set implicitly by setupAnalyzer()

	// If specified, can simulate a failure at a given block height.
	fail func(uint64) error

	// Fields for testing; these let us report back what blocks were processed.
	processedBlocks     map[uint64]struct{}
	processedOrder      []uint64
	fastSyncFinalizedAt *int64 // Height at which FinalizeFastSync was called, if any.
}

// PreWork implements block.BlockProcessor.
func (*mockProcessor) PreWork(ctx context.Context) error {
	return nil
}

// PreWork implements block.BlockProcessor.
func (m *mockProcessor) FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error {
	m.fastSyncFinalizedAt = &lastFastSyncHeight
	return nil
}

// ProcessBlock implements block.BlockProcessor.
func (m *mockProcessor) ProcessBlock(ctx context.Context, height uint64) error {
	if m.fail != nil {
		if err := m.fail(height); err != nil {
			return fmt.Errorf("mock processor failure: %w", err)
		}
	}
	if m.processedBlocks == nil {
		m.processedBlocks = make(map[uint64]struct{})
	}
	m.processedBlocks[height] = struct{}{}
	m.processedOrder = append(m.processedOrder, height)

	row, err := m.storage.Query(
		ctx,
		queries.IndexingProgress,
		height,
		m.name,
		m.isFastSync,
	)
	if err != nil {
		return err
	}
	row.Close()

	return nil
}

var _ block.BlockProcessor = (*mockProcessor)(nil)

func setupDB(t *testing.T) *postgres.Client {
	ctx := context.Background()

	// Initialize the test database.
	testDB := pgTestUtil.NewTestClient(t)
	// Ensure the test database is empty.
	require.NoError(t, testDB.Wipe(ctx), "testDb.Wipe")
	// Run DB migrations.
	require.NoError(t, analyzerCmd.RunMigrations(migrationsPath, os.Getenv("CI_TEST_CONN_STRING")), "failed to run migrations")

	return testDB
}

func setupAnalyzerWithPreWork(t *testing.T, testDb *postgres.Client, p *mockProcessor, cfg *config.BlockBasedAnalyzerConfig, mode analyzer.BlockAnalysisMode) analyzer.Analyzer {
	a := setupAnalyzer(t, testDb, p, cfg, mode)
	require.NoError(t, a.PreWork(context.Background()), "analyzer.PreWork")

	return a
}

func setupAnalyzer(t *testing.T, testDb *postgres.Client, p *mockProcessor, cfg *config.BlockBasedAnalyzerConfig, mode analyzer.BlockAnalysisMode) analyzer.Analyzer {
	// Modify the processor in-place (!): make sure the isFastSync field is in agreement with the analyzer's mode.
	p.isFastSync = (mode == analyzer.FastSyncMode)

	// Initialize the block analyzer.
	logger, err := log.NewLogger(fmt.Sprintf("test_analyzer-%s", p.name), os.Stdout, log.FmtJSON, log.LevelInfo)
	require.NoError(t, err, "log.NewLogger")
	var blockRange config.BlockRange
	switch mode {
	case analyzer.SlowSyncMode:
		blockRange = cfg.SlowSyncRange()
	case analyzer.FastSyncMode:
		blockRange = *cfg.FastSyncRange()
	default:
		t.Fatal("invalid block analysis mode")
	}
	analyzer, err := block.NewAnalyzer(blockRange, cfg.BatchSize, mode, p.name, p, testDb, logger)
	require.NoError(t, err, "block.NewAnalyzer")

	return analyzer
}

// exactlyOneTrue checks a list of boolean values and returns an error unless exactly one true.
func exactlyOneTrue(bools ...bool) error {
	count := 0
	for _, b := range bools {
		if b {
			count++
		}
	}

	if count < 1 {
		return fmt.Errorf("no true value found")
	}
	if count > 1 {
		return fmt.Errorf("more than one true value found")
	}
	return nil
}

func TestFastSyncBlockAnalyzer(t *testing.T) {
	// Test that the fast-sync block analyzer processes all blocks in the range.
	ctx := context.Background()

	db := setupDB(t)
	p := &mockProcessor{name: "test_analyzer", storage: db}
	analyzer := setupAnalyzer(t, db, p, testBlockBasedConfig, analyzer.SlowSyncMode)

	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that all blocks were processed.
	for i := uint64(1); i <= 1_000; i++ {
		_, ok := p.processedBlocks[i]
		require.True(t, ok, "block %d was not processed", i)
	}
	require.Len(t, p.processedBlocks, 1_000, "more blocks processed than expected")
}

func TestMultipleFastSyncBlockAnalyzers(t *testing.T) {
	// Test that multiple fast-sync block analyzers can run concurrently.
	numAnalyzers := 5
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		p := &mockProcessor{name: "test_analyzer", storage: db}
		analyzer := setupAnalyzerWithPreWork(t, db, p, testFastSyncBlockBasedConfig, analyzer.FastSyncMode)
		ps = append(ps, p)
		as = append(as, analyzer)
	}
	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	for _, a := range as {
		wg.Add(1)
		go func(a analyzer.Analyzer) {
			defer wg.Done()
			a.Start(ctx)
		}(a)
	}

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Ensure that every block in the fast-sync range was processed by exactly one analyzer.
	for i := uint64(1); i <= 1_000; i++ {
		oks := []bool{}
		for _, p := range ps {
			_, ok := p.processedBlocks[i]
			oks = append(oks, ok)
		}
		require.NoError(t, exactlyOneTrue(oks...), "block %d was not processed by exactly one analyzer", i)
	}
}

func TestFailingFastSyncBlockAnalyzers(t *testing.T) {
	// Test that a failing fast-sync block analyzer doesn't block other analyzers.
	numAnalyzers := 2
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		var fail func(uint64) error
		// The last analyzer fails all blocks.
		if i == numAnalyzers-1 {
			fail = func(height uint64) error {
				return fmt.Errorf("failing analyzer")
			}
		}
		p := &mockProcessor{name: "test_analyzer", storage: db, fail: fail}
		analyzer := setupAnalyzerWithPreWork(t, db, p, testFastSyncBlockBasedConfig, analyzer.FastSyncMode)
		ps = append(ps, p)
		as = append(as, analyzer)
	}
	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	for _, a := range as {
		wg.Add(1)
		go func(a analyzer.Analyzer) {
			defer wg.Done()
			a.Start(ctx)
		}(a)
	}

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Ensure that every block was processed by exactly one analyzer.
	for i := uint64(1); i <= 1_000; i++ {
		oks := []bool{}
		for _, p := range ps {
			_, ok := p.processedBlocks[i]
			oks = append(oks, ok)
		}
		require.NoError(t, exactlyOneTrue(oks...), "block %d was not processed by exactly one analyzer", i)
	}
}

func TestDistinctFastSyncBlockAnalyzers(t *testing.T) {
	// Test that multiple distinct fast-sync block analyzers can run concurrently.
	numAnalyzers := 5
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		p := &mockProcessor{name: fmt.Sprintf("test_analyzer_%d", i), storage: db}
		analyzer := setupAnalyzerWithPreWork(t, db, p, testFastSyncBlockBasedConfig, analyzer.FastSyncMode)
		ps = append(ps, p)
		as = append(as, analyzer)
	}
	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	for _, a := range as {
		wg.Add(1)
		go func(a analyzer.Analyzer) {
			defer wg.Done()
			a.Start(ctx)
		}(a)
	}

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Ensure that every block was processed by all analyzers.
	for i := uint64(1); i <= 1_000; i++ {
		for _, p := range ps {
			_, ok := p.processedBlocks[i]
			require.True(t, ok, "block %d was not processed by analyzer %s", i, p.name)
		}
	}
}

func TestSlowSyncBlockAnalyzer(t *testing.T) {
	// Test that the slow-sync block analyzer processes all blocks in the range.
	ctx := context.Background()

	db := setupDB(t)
	p := &mockProcessor{name: "test_analyzer", storage: db}
	analyzer := setupAnalyzerWithPreWork(t, db, p, testBlockBasedConfig, analyzer.SlowSyncMode)

	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that all blocks were processed and in order.
	for i := uint64(1); i <= 1_000; i++ {
		_, ok := p.processedBlocks[i]
		require.True(t, ok, "block %d was not processed", i)
		require.Equal(t, i, p.processedOrder[i-1], "block %d was not processed in order", i)
	}
	require.Len(t, p.processedBlocks, 1_000, "more blocks processed than expected")
}

func TestFailingSlowSyncBlockAnalyzer(t *testing.T) {
	// Test that the slow-sync block analyzer with some failures processes all blocks in the range.
	ctx := context.Background()

	db := setupDB(t)
	p := &mockProcessor{name: "test_analyzer", storage: db, fail: func(height uint64) error {
		// Fail ~5% of the time.
		if rand.Float64() > 0.95 { // /nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand).
			return fmt.Errorf("failed by chance")
		}
		return nil
	}}
	analyzer := setupAnalyzerWithPreWork(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 100, BatchSize: 100}, analyzer.SlowSyncMode)

	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that all blocks were processed and in order.
	for i := uint64(1); i <= 100; i++ {
		_, ok := p.processedBlocks[i]
		require.True(t, ok, "block %d was not processed", i)
		require.Equal(t, i, p.processedOrder[i-1], "block %d was not processed in order", i)
	}
	require.Len(t, p.processedBlocks, 100, "more blocks processed than expected")
}

func TestDistinctSlowSyncBlockAnalyzers(t *testing.T) {
	// Test that multiple distinct slow-sync block analyzers can run concurrently.
	numAnalyzers := 5
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		p := &mockProcessor{name: fmt.Sprintf("test_analyzer_%d", i), storage: db}
		analyzer := setupAnalyzerWithPreWork(t, db, p, testBlockBasedConfig, analyzer.SlowSyncMode)
		ps = append(ps, p)
		as = append(as, analyzer)
	}
	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	for _, a := range as {
		wg.Add(1)
		go func(a analyzer.Analyzer) {
			defer wg.Done()
			a.Start(ctx)
		}(a)
	}

	// Wait for all analyzers to finish.
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Ensure that every block was processed by all analyzers.
	for i := uint64(1); i <= 1_000; i++ {
		for _, p := range ps {
			_, ok := p.processedBlocks[i]
			require.True(t, ok, "block %d was not processed by analyzer %s", i, p.name)
			require.Equal(t, i, p.processedOrder[i-1], "block %d was not processed in order", i)
		}
	}
}

func TestFinalizeFastSync(t *testing.T) {
	// Test that a slow-sync analyzer finalizes the work of the preceding fast-sync analyzers, if any.
	ctx := context.Background()
	db := setupDB(t)

	// Run multiple analyzers, each on a separate block range, to simulate past Nexus invocations.
	// Note: The .Start() call blocks until the analyzer finishes.
	p := &mockProcessor{name: "consensus", storage: db}
	setupAnalyzerWithPreWork(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 10, FastSync: &config.FastSyncConfig{To: 10}}, analyzer.FastSyncMode).Start(ctx)
	require.Nil(t, p.fastSyncFinalizedAt,
		fmt.Sprintf("fast-sync analyzer should never finalize fast-sync, but it did at %d", p.fastSyncFinalizedAt))

	p = &mockProcessor{name: "consensus", storage: db}
	setupAnalyzerWithPreWork(t, db, p, &config.BlockBasedAnalyzerConfig{From: 5, To: 20}, analyzer.SlowSyncMode).Start(ctx)
	require.NotNil(t, p.fastSyncFinalizedAt,
		"slow-sync analyzer should have finalized fast sync because it's taking up work from a fast-sync analyzer")
	require.Equal(t, int64(10), *p.fastSyncFinalizedAt,
		"slow-sync analyzer should finalize fast-sync at the height of the last fast-processed block")

	p = &mockProcessor{name: "consensus", storage: db}
	setupAnalyzerWithPreWork(t, db, p, &config.BlockBasedAnalyzerConfig{From: 21, To: 30}, analyzer.SlowSyncMode).Start(ctx)
	require.Nil(t, p.fastSyncFinalizedAt,
		"second slow-sync analyzer should not finalize fast-sync because its range extends an existing slow-sync-analyzed range")
}

// Tests the `SoftEnqueueGapsInProcessedBlocks` query.
func TestSoftEqueueGaps(t *testing.T) {
	ctx := context.Background()
	db := setupDB(t)

	// Returns a sorted list of all heights that have an entry in the processed_blocks table (even if not completed).
	getHeights := func() []uint64 {
		rows, err := db.Query(ctx, "SELECT height FROM analysis.processed_blocks WHERE analyzer = $1 ORDER BY height", "consensus")
		require.NoError(t, err)
		heights := []uint64{}
		for rows.Next() {
			var height uint64
			require.NoError(t, rows.Scan(&height))
			heights = append(heights, height)
		}
		return heights
	}

	// Inserts a row into the processed_blocks table.
	markAsProcessed := func(analyzer string, height uint64) {
		batch := storage.QueryBatch{}
		batch.Queue("INSERT INTO analysis.processed_blocks (analyzer, height, locked_time) values ($1, $2, '-infinity')", analyzer, height)
		require.NoError(t, db.SendBatch(ctx, &batch))
	}

	// Runs the query that we're testing.
	enqueueGaps := func(from, to int64) {
		batch := &storage.QueryBatch{}
		batch.Queue(queries.SoftEnqueueGapsInProcessedBlocks, "consensus", from, to)
		require.NoError(t, db.SendBatch(ctx, batch))
	}

	// Sanity check our helper methods.
	markAsProcessed("some_other_analyzer", 10) // to test that queries are scoped by analyzer
	require.Equal(t, []uint64{}, getHeights())

	// Pretend some blocks are already processed.
	markAsProcessed("consensus", 3)
	markAsProcessed("consensus", 4)
	markAsProcessed("consensus", 6)
	require.Equal(t, []uint64{3, 4, 6}, getHeights())

	// Fill the gaps.
	// NOTE: Only the gaps. Heights above the highest-processed-so-far (i.e. 6) are not enqueued.
	enqueueGaps(1, 10)
	require.Equal(t, []uint64{1, 2, 3, 4, 5, 6}, getHeights())
}

func TestRefuseSlowSyncOnDirtyRange(t *testing.T) {
	// Test that slow-sync analyzer won't start if the already-analyzed block range is non-contiguous.
	ctx := context.Background()
	db := setupDB(t)

	// Run multiple analyzers, each on a separate block range, to simulate past Nexus invocations.
	// Note: The .Start() call blocks until the analyzer finishes.
	p := &mockProcessor{name: "consensus", storage: db}
	setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 3, To: 10}, analyzer.SlowSyncMode).Start(ctx)

	p = &mockProcessor{name: "consensus", storage: db}
	a := setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 15}, analyzer.SlowSyncMode)
	require.Error(t, a.PreWork(ctx), "slow-sync analyzer should refuse to process anything because the already-analyzed range is non-contiguous")

	// Patch up the holes with a fast-sync analyzer.
	fp := &mockProcessor{name: "consensus", storage: db}
	setupAnalyzerWithPreWork(t, db, fp, &config.BlockBasedAnalyzerConfig{From: 1, To: 10, FastSync: &config.FastSyncConfig{To: 10}}, analyzer.FastSyncMode).Start(ctx)
	require.Equal(t, fp.processedBlocks, map[uint64]struct{}{1: {}, 2: {}},
		"fast-sync analyzer should have processed the missing blocks")

	// Try a slow-sync analyzer again
	require.NoError(t, a.PreWork(ctx), "slow-sync analyzer should be able to process the missing blocks")
	a.Start(ctx)
	require.Equal(t, p.processedBlocks, map[uint64]struct{}{11: {}, 12: {}, 13: {}, 14: {}, 15: {}},
		"slow-sync analyzer should have processed the missing blocks")
}
