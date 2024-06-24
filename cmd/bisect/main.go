// Runs bisection on a range of heights to find the height at which the DB and the node
// diverge wrt to a specific value (e.g. an account's escrow balance).
//
// This is a one-off debugging tool and is intended to be configured partially via code.
// Config comes from:
//   - The config file (passed as a command-line argument): the DB connection string and the node connection string.
//     This is the same YAML config file that the analyzer uses.
//   - The hardcoded values and functions below. They determine which value to compare,
//     and at which heights. See also docstring for bisect().
//
// To connect to the relevant Oasis-internal DB locally, see internal documentation:
//    https://app.clickup.com/24394368/v/dc/q8em0-18570/q8em0-15212
//
// To run:
//	go run ./cmd/bisect <YAML_CONFIG_FILE>

package main

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi/history"
	"github.com/oasisprotocol/nexus/storage/postgres"

	staking "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
)

var logger = log.NewDefaultLogger("cmd/bisect")

func main() {
	ctx := context.Background()
	nodeApi, db := initBackends(ctx)

	// CHANGEME: Job-specific config.
	delegator := addressFromBech32("oasis1qryftd0e9588mrd93pznmmuhulu6rzzgngmtkl8x")
	delegatee := addressFromBech32("oasis1qqekv2ymgzmd8j2s2u7g0hhc7e77e654kvwqtjwm")
	bisect(
		3027601, // minHeight; the DB matches the node at this height
		4743434, // maxHeight; the DB does NOT match the node at this height
		func(h int64) common.BigInt { return delegationViaDeadReckon(ctx, db, h, delegator, delegatee) },
		func(h int64) common.BigInt { return delegationViaNodeApi(ctx, nodeApi, h, delegator, delegatee) },
	)
}

// Performs bisection on a range of heights to find the height at which `dbFetch(height)` and `nodeFetch(height)` diverge.
// Prints results in human-readable form to stdout.
//
// Inputs:
//   - A min and max height between which to bisect, inclusive.
//   - A function that recreates the DB value at a given height h. We don't have _actual_ old dead-reckoned values
//     readily stored in the DB, but we do store all events in the DB, so we can recreate the dead-reckoned value
//     with a DB SUM() or similar.
//   - A function that fetches the expected value from the node.
//
// Assumptions:
//   - The DB matches the node at `minHeight`
//   - The DB does not match the node at `maxHeight`
//   - The correctness of the DB switches only once between `minHeight` and `maxHeight`
func bisect(minHeight, maxHeight int64, dbFetch, nodeFetch func(h int64) common.BigInt) {
	// Validate assumptions about endpoints.
	comparisons := []Comparison{
		{Height: minHeight, NodeVal: nodeFetch(minHeight), DbVal: dbFetch(minHeight)},
		{Height: maxHeight, NodeVal: nodeFetch(maxHeight), DbVal: dbFetch(maxHeight)},
	}
	printResults(comparisons)
	if !Eq(comparisons[0].NodeVal, comparisons[0].DbVal) || Eq(comparisons[1].NodeVal, comparisons[1].DbVal) {
		logger.Error("DB and the node should agree at minHeight and disagree at maxHeight")
		os.Exit(1)
	}

	// Perform bisection.
	for minHeight < maxHeight {
		h := (minHeight + maxHeight) / 2
		dbVal := dbFetch(h)
		nodeVal := nodeFetch(h)
		comparisons = append(comparisons, Comparison{Height: h, NodeVal: nodeVal, DbVal: dbVal})
		printResults(comparisons)
		if Eq(dbVal, nodeVal) {
			minHeight = h + 1
		} else {
			maxHeight = h
		}
	}
}

type Comparison struct {
	Height  int64
	NodeVal common.BigInt
	DbVal   common.BigInt
}

// Returns a sorted copy of `lst`.
func sortedResults(lst []Comparison) []Comparison {
	lst = append([]Comparison{}, lst...) // create a copy
	sort.Slice(lst, func(i, j int) bool { return lst[i].Height < lst[j].Height })
	return lst
}

// Prints the results of a bisection-in-progress.
func printResults(lst []Comparison) {
	fmt.Println("----------------------------------")
	mostRecentHeight := lst[len(lst)-1].Height
	lst = sortedResults(lst)
	for _, r := range lst {
		goodOrBad := "BAD "
		if Eq(r.NodeVal, r.DbVal) {
			goodOrBad = "GOOD"
		}

		recencyIndicator := " "
		if r.Height == mostRecentHeight {
			recencyIndicator = "*"
		}

		fmt.Printf("%s%s height %d:   node %-15s   db %-15s\n", recencyIndicator, goodOrBad, r.Height, r.NodeVal.String(), r.DbVal.String())
	}
}

func Eq(a, b common.BigInt) bool {
	return a.Cmp(&b.Int) == 0
}

func initBackends(ctx context.Context) (*history.HistoryConsensusApiLite, *postgres.Client) {
	if len(os.Args) < 2 {
		fmt.Println("usage: bisect <YAML_CONFIG_FILE>\n\nThe config file is in the same YAML format that the analyzer uses; it determines the connection string for the DB and the node.")
		os.Exit(1)
	}

	cfg, err := config.InitConfig(os.Args[1])
	if err != nil {
		logger.Error("config init failed", "error", err)
		os.Exit(1)
	}

	nodeApi, err := history.NewHistoryConsensusApiLite(ctx, cfg.Analysis.Source.History(), cfg.Analysis.Source.Nodes, true /*fastStartup*/)
	if err != nil {
		logger.Error("cannot instantiate consensus API", "error", err)
		os.Exit(1)
	}

	db, err := postgres.NewClient(cfg.Analysis.Storage.Endpoint, logger)
	if err != nil {
		logger.Error("cannot connect to DB", "error", err)
		os.Exit(1)
	}

	return nodeApi, db
}

func addressFromBech32(addr string) staking.Address {
	var a staking.Address
	if err := a.UnmarshalText([]byte(addr)); err != nil {
		panic(err)
	}
	return a
}
