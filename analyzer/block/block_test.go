package block_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/analyzer"
	"github.com/oasisprotocol/oasis-indexer/analyzer/block"
	"github.com/oasisprotocol/oasis-indexer/analyzer/queries"
	analyzerCmd "github.com/oasisprotocol/oasis-indexer/cmd/analyzer"
	"github.com/oasisprotocol/oasis-indexer/config"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/postgres"
	pgTestUtil "github.com/oasisprotocol/oasis-indexer/storage/postgres/testutil"
)

// Relative path to the migrations directory when running tests in this file.
// When running go tests, the working directory is always set to the package directory of the test being run.
const migrationsPath = "file://../../storage/migrations"

const testsTimeout = 10 * time.Second

type mockProcessor struct {
	name              string
	latestBlockHeight uint64
	processedBlocks   map[uint64]struct{}
	storage           storage.TargetStorage

	fail bool
}

// PreWork implements block.BlockProcessor.
func (*mockProcessor) PreWork(ctx context.Context) error {
	return nil
}

// ProcessBlock implements block.BlockProcessor.
func (m *mockProcessor) ProcessBlock(ctx context.Context, height uint64) error {
	if m.fail {
		return fmt.Errorf("mock processor failure")
	}
	if m.processedBlocks == nil {
		m.processedBlocks = make(map[uint64]struct{})
	}
	m.processedBlocks[height] = struct{}{}

	row, err := m.storage.Query(
		ctx,
		queries.IndexingProgress,
		height,
		m.name,
	)
	if err != nil {
		return err
	}
	row.Close()

	return nil
}

// SourceLatestBlockHeight implements block.BlockProcessor.
func (m *mockProcessor) SourceLatestBlockHeight(ctx context.Context) (uint64, error) {
	return m.latestBlockHeight, nil
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

func setupAnalyzer(t *testing.T, testDb *postgres.Client, p *mockProcessor, cfg *config.BlockBasedAnalyzerConfig) analyzer.Analyzer {
	// Initialize the block analyzer.
	logger, err := log.NewLogger(fmt.Sprintf("test-analyzer-%s", p.name), os.Stdout, log.FmtJSON, log.LevelError)
	require.NoError(t, err, "log.NewLogger")
	analyzer, err := block.NewAnalyzer(cfg, p.name, p, testDb, logger)
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

func TestBlockAnalyzer(t *testing.T) {
	// Test that the block analyzer processes all blocks in the range.
	ctx := context.Background()

	db := setupDB(t)
	p := &mockProcessor{name: "test-analyzer", latestBlockHeight: 10_000, storage: db}
	analyzer := setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 1_000})

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

func TestMultipleBlockAnalyzers(t *testing.T) {
	// Test that multiple block analyzers can run concurrently.
	numAnalyzers := 5
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		p := &mockProcessor{name: "test-analyzer", latestBlockHeight: 10_000, storage: db}
		analyzer := setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 1_000})
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

func TestFailingBlockAnalyzers(t *testing.T) {
	// Test that a failing block analyzer doesn't block other analyzers.
	numAnalyzers := 2
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		p := &mockProcessor{name: "test-analyzer", latestBlockHeight: 10_000, storage: db, fail: i == numAnalyzers-1}
		analyzer := setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 1_000})
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

func TestDistinctBlockAnalyzers(t *testing.T) {
	// Test that multiple distinct block analyzers can run concurrently.
	numAnalyzers := 5
	ctx := context.Background()

	db := setupDB(t)
	ps := []*mockProcessor{}
	as := []analyzer.Analyzer{}
	for i := 0; i < numAnalyzers; i++ {
		p := &mockProcessor{name: fmt.Sprintf("test-analyzer-%d", i), latestBlockHeight: 1_000, storage: db}
		analyzer := setupAnalyzer(t, db, p, &config.BlockBasedAnalyzerConfig{From: 1, To: 1_000})
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
