package analyzer

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/log"
)

// A trivial analyzer that runs for `duration`, then appends its `name` to `finishLog`.
type DummyAnalyzer struct {
	name      string
	duration  time.Duration
	finishLog *[]string
}

var _ analyzer.Analyzer = (*DummyAnalyzer)(nil)

var finishLogLock = &sync.Mutex{} // for use by all DummyAnalyzer instances

func (a *DummyAnalyzer) PreWork(ctx context.Context) error {
	return nil
}

func (a *DummyAnalyzer) Start(ctx context.Context) {
	time.Sleep(a.duration)
	finishLogLock.Lock()
	defer finishLogLock.Unlock()
	*a.finishLog = append(*a.finishLog, a.name)
}

func (a *DummyAnalyzer) Name() string {
	return a.name
}

func dummyAnalyzer(syncTag string, name string, duration time.Duration, finishLog *[]string) SyncedAnalyzer {
	return SyncedAnalyzer{
		SyncTag: syncTag,
		Analyzer: &DummyAnalyzer{
			name:      name,
			duration:  duration,
			finishLog: finishLog,
		},
	}
}

func TestSequencing(t *testing.T) {
	// Log of analyzer completions. Each analyzer, when it completes, will append its names to this list.
	finishLog := []string{}
	// Fast analyzers: Tag "a" finishes after 1 second, tag "b" finishes after 3 seconds.
	fastA := dummyAnalyzer("a", "fastA", 1*time.Second, &finishLog)
	fastB1 := dummyAnalyzer("b", "fastB1", 500*time.Millisecond, &finishLog)
	fastB2 := dummyAnalyzer("b", "fastB2", 3*time.Second, &finishLog)
	// Slow analyzers
	slowA := dummyAnalyzer("a", "slowA", 1*time.Second, &finishLog)
	slowB := dummyAnalyzer("b", "slowB", 1*time.Second, &finishLog)
	slowC := dummyAnalyzer("c", "slowC", 1500*time.Millisecond, &finishLog) // depends on fastC, which does not exist
	slowX := dummyAnalyzer("", "slowX", 0*time.Second, &finishLog)          // depends on no fast analyzers

	s := Service{
		fastSyncAnalyzers: []SyncedAnalyzer{fastA, fastB1, fastB2},
		analyzers:         []SyncedAnalyzer{slowA, slowB, slowC, slowX},
		logger:            log.NewDefaultLogger("analyzer"),
	}

	s.Start()
	require.Equal(t, []string{
		"slowX",  // finishes immediately, at t=0s, because it depends on no fast analyzers
		"fastB1", // finishes at t=0.5s
		"fastA",  // finishes at t=1s
		"slowC",  // finishes at t=1.5s because it doesn't need to wait for any fast analyzers; we're making sure it starts at all
		"slowA",  // finishes at t=2s because it waits for fastA (1s), then runs for 1s
		"fastB2", // finishes at t=3s
		"slowB",  // finishes at t=4s because it waits for fastB1+fastB2 (3s), then runs for 1s
	}, finishLog, "analyzers did not finish in the expected order")
}
