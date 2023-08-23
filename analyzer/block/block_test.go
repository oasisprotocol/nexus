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
	name            string
	processedBlocks map[uint64]struct{}
	processedOrder  []uint64
	storage         storage.TargetStorage

	fail func(uint64) error
}

// PreWork implements block.BlockProcessor.
func (*mockProcessor) PreWork(ctx context.Context) error {
	return nil
}

// PreWork implements block.BlockProcessor.
func (*mockProcessor) FinalizeFastSync(ctx context.Context, lastFastSyncHeight int64) error {
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
		false, // is_fast_sync
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

func setupAnalyzer(t *testing.T, testDb *postgres.Client, p *mockProcessor, cfg *config.BlockBasedAnalyzerConfig, mode analyzer.BlockAnalysisMode) analyzer.Analyzer {
	// Initialize the block analyzer.
	logger, err := log.NewLogger(fmt.Sprintf("test-analyzer-%s", p.name), os.Stdout, log.FmtJSON, log.LevelError)
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

// closingChannel returns a channel that closes when the wait group `wg` is done.
func closingChannel(wg *sync.WaitGroup) <-chan struct{} {
	c := make(chan struct{})
	go func() {
		wg.Wait()
		close(c)
	}()
	return c
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
	p := &mockProcessor{name: "test-analyzer", storage: db}
	analyzer := setupAnalyzer(t, db, p, testBlockBasedConfig, analyzer.SlowSyncMode)

	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for all analyzers to finish.
	analyzersDone := closingChannel(&wg)
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
		p := &mockProcessor{name: "test-analyzer", storage: db}
		analyzer := setupAnalyzer(t, db, p, testFastSyncBlockBasedConfig, analyzer.FastSyncMode)
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
	analyzersDone := closingChannel(&wg)
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
		p := &mockProcessor{name: "test-analyzer", storage: db, fail: fail}
		analyzer := setupAnalyzer(t, db, p, testFastSyncBlockBasedConfig, analyzer.FastSyncMode)
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
	analyzersDone := closingChannel(&wg)
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
		p := &mockProcessor{name: fmt.Sprintf("test-analyzer-%d", i), storage: db}
		analyzer := setupAnalyzer(t, db, p, testFastSyncBlockBasedConfig, analyzer.FastSyncMode)
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
	analyzersDone := closingChannel(&wg)
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
	p := &mockProcessor{name: "test-analyzer", storage: db}
	analyzer := setupAnalyzer(t, db, p, testBlockBasedConfig, analyzer.SlowSyncMode)

	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for all analyzers to finish.
	analyzersDone := closingChannel(&wg)
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
	p := &mockProcessor{name: "test-analyzer", storage: db, fail: func(height uint64) error {
		// Fail ~5% of the time.
		if rand.Float64() > 0.95 { // /nolint:gosec // G404: Use of weak random number generator (math/rand instead of crypto/rand).
			return fmt.Errorf("failed by chance")
		}
		return nil
	}}
	analyzer := setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 100, BatchSize: 100}, analyzer.SlowSyncMode)

	// Run the analyzer and ensure all blocks are processed.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for all analyzers to finish.
	analyzersDone := closingChannel(&wg)
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
		p := &mockProcessor{name: fmt.Sprintf("test-analyzer-%d", i), storage: db}
		analyzer := setupAnalyzer(t, db, p, testBlockBasedConfig, analyzer.SlowSyncMode)
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
	analyzersDone := closingChannel(&wg)
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
