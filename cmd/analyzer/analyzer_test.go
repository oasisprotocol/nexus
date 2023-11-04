package analyzer_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer"
	cmdAnalyzer "github.com/oasisprotocol/nexus/cmd/analyzer"
	"github.com/oasisprotocol/nexus/storage/postgres/testutil"
	"github.com/oasisprotocol/nexus/tests"
)

// Relative path to the migrations directory when running tests in this file.
// When running go tests, the working directory is always set to the package directory of the test being run.
const migrationsPath = "file://../../storage/migrations"

func TestMigrations(t *testing.T) {
	tests.SkipIfShort(t)
	client := testutil.NewTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Ensure database is empty before running migrations.
	require.NoError(t, client.Wipe(ctx), "failed to wipe database")

	// Run migrations.
	require.NoError(t, cmdAnalyzer.RunMigrations(migrationsPath, os.Getenv("CI_TEST_CONN_STRING")), "failed to run migrations")
}

// A trivial analyzer that runs for `duration`, then appends its `name` to `finishLog`.
type DummyAnalyzer struct {
	name      string
	duration  time.Duration
	finishLog *[]string
}

var finishLogLock = &sync.Mutex{} // for use by all DummyAnalyzer instances

var _ analyzer.Analyzer = (*DummyAnalyzer)(nil)

func (a *DummyAnalyzer) Start(ctx context.Context) {
	time.Sleep(a.duration)
	finishLogLock.Lock()
	defer finishLogLock.Unlock()
	*a.finishLog = append(*a.finishLog, a.name)
}

func (a *DummyAnalyzer) Name() string {
	return a.name
}

func dummyAnalyzer(syncTag string, name string, duration time.Duration, finishLog *[]string) cmdAnalyzer.SyncedAnalyzer {
	return cmdAnalyzer.SyncedAnalyzer{
		SyncTag: syncTag,
		Analyzer: &DummyAnalyzer{
			name:      name,
			duration:  duration,
			finishLog: finishLog,
		},
	}
}

func TestSequencing(t *testing.T) {
	s := cmdAnalyzer.ServiceTester{}
	finishLog := []string{}
	// Fast analyzers: Tag "a" finishes after 1 second, tag "b" finishes after 3 seconds.
	fastA := dummyAnalyzer("a", "fastA", 1*time.Second, &finishLog)
	fastB1 := dummyAnalyzer("b", "fastB1", 500*time.Millisecond, &finishLog)
	fastB2 := dummyAnalyzer("b", "fastB2", 3*time.Second, &finishLog)
	// Slow analyzers
	slowA := dummyAnalyzer("a", "slowA", 1*time.Second, &finishLog)
	slowB := dummyAnalyzer("b", "slowB", 1*time.Second, &finishLog)
	slowX := dummyAnalyzer("", "slowX", 0*time.Second, &finishLog)
	s.SetAnalyzers(
		[]cmdAnalyzer.SyncedAnalyzer{fastA, fastB1, fastB2},
		[]cmdAnalyzer.SyncedAnalyzer{slowA, slowB, slowX},
	)

	s.Start()
	require.Equal(t, []string{
		"slowX",  // finishes immediately, at t=0s, because it depends on no fast analyzers
		"fastB1", // finishes at t=0.5s
		"fastA",  // finishes at t=1s
		"slowA",  // finishes at t=2s because it waits for fastA (1s), then runs for 1s
		"fastB2", // finishes at t=3s
		"slowB",  // finishes at t=4s because it waits for fastB1+fastB2 (3s), then runs for 1s
	}, finishLog, "analyzers did not finish in the expected order")
}
