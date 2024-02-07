package item_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
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

const (
	testTableCreate = `
		CREATE TABLE analysis.item_analyzer_test (
			id uint63
		)`

	grantSelectOnAnalysis = `
		GRANT SELECT ON ALL TABLES IN SCHEMA analysis TO PUBLIC`

	grantExecuteOnAnalysis = `
		GRANT SELECT ON ALL TABLES IN SCHEMA analysis TO PUBLIC`

	testItemInsert = `
		INSERT INTO analysis.item_analyzer_test(id) 
			VALUES ($1)`

	testItemCounts = `
		SELECT id, count(*) from analysis.item_analyzer_test GROUP BY id
	`
)

// Default item based config.
var testItemBasedConfig = &config.ItemBasedAnalyzerConfig{
	BatchSize:           3,
	StopIfQueueEmptyFor: time.Second,
	Interval:            0, // use backoff
	InterItemDelay:      0,
}

type mockItem struct {
	id         uint64
	canProcess bool // whether or not the item should return an error during processing.
}

type mockProcessor struct {
	name string
	// An array of batches to be processed.
	workQueue [][]*mockItem
	// the index of the next batch in workQueue to return.
	nextBatch int
	// Tracks which items have been successfully processed by ProcessItem. Note that
	// this is distinct from whether the db updates returned by ProcessItem were
	// successfully applied to the database.
	processedItems map[uint64]struct{}
	lock           sync.Mutex
}

var _ item.ItemProcessor[*mockItem] = (*mockProcessor)(nil)

func (p *mockProcessor) GetItems(ctx context.Context, limit uint64) ([]*mockItem, error) {
	if p.nextBatch == len(p.workQueue) {
		return []*mockItem{}, nil
	}
	b := p.workQueue[p.nextBatch]
	p.nextBatch++
	return b, nil
}

func (p *mockProcessor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *mockItem) error {
	if !item.canProcess {
		return fmt.Errorf("error processing item %d", item.id)
	}
	batch.Queue(testItemInsert, item.id)
	p.lock.Lock()
	defer p.lock.Unlock()
	p.processedItems[item.id] = struct{}{}
	return nil
}

func (p *mockProcessor) QueueLength(ctx context.Context) (int, error) {
	qLen := 0
	for i := p.nextBatch; i < len(p.workQueue); i++ {
		qLen += len(p.workQueue[i])
	}
	return qLen, nil
}

func setupDB(t *testing.T) *postgres.Client {
	ctx := context.Background()

	// Initialize the test database.
	testDB := pgTestUtil.NewTestClient(t)
	// Ensure the test database is empty.
	require.NoError(t, testDB.Wipe(ctx), "testDb.Wipe")
	// Run DB migrations.
	require.NoError(t, analyzerCmd.RunMigrations(migrationsPath, os.Getenv("CI_TEST_CONN_STRING")), "failed to run migrations")
	// Initialize test table.
	batch := &storage.QueryBatch{}
	batch.Queue(testTableCreate)
	batch.Queue(grantSelectOnAnalysis)
	batch.Queue(grantExecuteOnAnalysis)
	require.NoError(t, testDB.SendBatch(ctx, batch), "failed to create test table")

	return testDB
}

func setupAnalyzer(t *testing.T, testDb *postgres.Client, p *mockProcessor, cfg *config.ItemBasedAnalyzerConfig) analyzer.Analyzer {
	logger := log.NewDefaultLogger(fmt.Sprintf("test-analyzer-%s", p.name))
	analyzer, err := item.NewAnalyzer[*mockItem](p.name, *cfg, p, testDb, logger)
	require.NoError(t, err, "item.NewAnalyzer")

	return analyzer
}

// getDbProcessedItems queries the test db and returns a mapping of item_id:frequency
// where item_id refers to the item whose database updates were committed, and
// frequency is the count of how many times those updates were applied.
func getDbProcessedItems(t *testing.T, ctx context.Context, storage storage.TargetStorage) map[uint64]uint64 { //nolint:revive
	rows, err := storage.Query(ctx, testItemCounts)
	require.NoError(t, err, "failed to fetch processed items from db")
	defer rows.Close()
	counts := make(map[uint64]uint64)
	for rows.Next() {
		var id, count uint64
		require.NoError(t, rows.Scan(&id, &count))
		counts[id] = count
	}

	return counts
}

func TestItemBasedAnalyzerAllItems(t *testing.T) {
	// Test that all items are processed across separate batches.
	ctx := context.Background()
	db := setupDB(t)
	workQueue := [][]*mockItem{
		{&mockItem{0, true}, &mockItem{1, true}, &mockItem{2, true}},
		{&mockItem{3, true}, &mockItem{4, true}, &mockItem{5, true}},
	}
	p := &mockProcessor{
		name:           "all_items",
		nextBatch:      0,
		workQueue:      workQueue,
		processedItems: make(map[uint64]struct{}),
		lock:           sync.Mutex{},
	}
	analyzer := setupAnalyzer(t, db, p, testItemBasedConfig)

	// Run the analyzer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for analyzer to finish
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that processItem was called on all items.
	var i uint64
	for i = 0; i < 6; i++ {
		_, ok := p.processedItems[i]
		require.True(t, ok, "item %d was not processed", i)
	}
	require.Len(t, p.processedItems, 6, "more items processed than expected")

	// Check that all db updates were applied.
	counts := getDbProcessedItems(t, ctx, db)
	require.Len(t, counts, 6, "more unique items found in db than expected")
	for i = 0; i < 6; i++ {
		_, ok := counts[i]
		require.True(t, ok, "item %d was not applied to db")
	}
	for id, count := range counts {
		require.Equal(t, uint64(1), count, "item %d was applied to the database more than once", id)
	}
}

func TestItemBasedAnalyzerNoStopOnEmpty(t *testing.T) {
	// Test that the analyzer stops on seeing an empty batch.
	ctx := context.Background()
	db := setupDB(t)
	workQueue := [][]*mockItem{
		{&mockItem{0, true}, &mockItem{1, true}, &mockItem{2, true}},
		{&mockItem{3, true}, &mockItem{4, true}, &mockItem{5, true}},
		{},
		{&mockItem{6, true}, &mockItem{7, true}, &mockItem{8, true}},
	}
	p := &mockProcessor{
		name:           "stop_on_empty",
		nextBatch:      0,
		workQueue:      workQueue,
		processedItems: make(map[uint64]struct{}),
		lock:           sync.Mutex{},
	}
	analyzer := setupAnalyzer(t, db, p, testItemBasedConfig)

	// Run the analyzer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for analyzer to finish
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that all blocks were processed.
	var i uint64
	for i = 0; i < 9; i++ {
		_, ok := p.processedItems[i]
		require.True(t, ok, "item %d was not processed", i)
	}
	require.Len(t, p.processedItems, 9, "more items processed than expected")
}

func TestItemBasedAnalyzerBadItem(t *testing.T) {
	// Test that if a batch has a bad item, db updates from other items
	// in the batch are applied.
	ctx := context.Background()
	db := setupDB(t)
	workQueue := [][]*mockItem{
		{&mockItem{0, false}, &mockItem{1, true}, &mockItem{2, true}},
		{&mockItem{3, true}, &mockItem{4, true}, &mockItem{5, true}},
	}
	p := &mockProcessor{
		name:           "bad_item",
		nextBatch:      0,
		workQueue:      workQueue,
		processedItems: make(map[uint64]struct{}),
		lock:           sync.Mutex{},
	}
	analyzer := setupAnalyzer(t, db, p, testItemBasedConfig)

	// Run the analyzer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for analyzer to finish
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that processItem() was called on all valid items.
	var i uint64
	for i = 1; i < 6; i++ {
		_, ok := p.processedItems[i]
		require.True(t, ok, "item %d was not processed", i)
	}
	require.Len(t, p.processedItems, 5, "more items processed than expected")

	// Check that no updates from the failing batch were applied to the database
	dbCounts := getDbProcessedItems(t, ctx, db)
	require.Len(t, dbCounts, 5, "more unique items in db than expected")
	for i = 1; i < 6; i++ {
		_, ok := dbCounts[i]
		require.True(t, ok, "item %d not found in database")
	}
	for id, count := range dbCounts {
		require.Equal(t, uint64(1), count, "item %d processed more than once", id)
	}
}

func TestItemBasedAnalyzerFailingBatch(t *testing.T) {
	// Test that a batch with all failures does not prevent the next batch from being processed.
	ctx := context.Background()
	db := setupDB(t)
	workQueue := [][]*mockItem{
		{&mockItem{0, false}, &mockItem{1, false}, &mockItem{2, false}},
		{&mockItem{3, true}, &mockItem{4, true}},
	}
	p := &mockProcessor{
		name:           "bad_batch_no_halt",
		nextBatch:      0,
		workQueue:      workQueue,
		processedItems: make(map[uint64]struct{}),
		lock:           sync.Mutex{},
	}
	analyzer := setupAnalyzer(t, db, p, testItemBasedConfig)

	// Run the analyzer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		analyzer.Start(ctx)
	}()

	// Wait for analyzer to finish
	analyzersDone := util.ClosingChannel(&wg)
	select {
	case <-time.After(testsTimeout):
		t.Fatal("timed out waiting for analyzer to finish")
	case <-analyzersDone:
	}

	// Check that all blocks were processed.
	var i uint64
	for i = 3; i < 5; i++ {
		_, ok := p.processedItems[i]
		require.True(t, ok, "item %d was not processed", i)
	}
	require.Len(t, p.processedItems, 2, "more items processed than expected")
}
