package static

import (
	"embed"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

//go:embed accounts
var genesisAccountsFs embed.FS

//go:embed delegations
var genesisDelegationsFs embed.FS

var (
	mainnetAccounts = &accounts{}
	testnetAccounts = &accounts{}

	mainnetDelegations = &delegations{}
)

type accounts struct {
	tss   []time.Time
	accts [][]string
}

type delegations struct {
	height []int64
	dels   [][]delegation
}

type delegation struct {
	delegator string
	delegatee string
	shares    string
}

// Parses a file with a list of accounts. The accounts are parsed from the file (one per line);
// the metadata is parsed from the filename, which should be of the form "<name>_<timestamp>.*".
func mustReadAddrs(path string) (time.Time, []string) {
	content, err := genesisAccountsFs.ReadFile(path)
	if err != nil {
		panic(err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	accts := make([]string, 0, len(lines))
	for _, line := range lines {
		if !strings.HasPrefix(line, "//") {
			accts = append(accts, line)
		}
	}

	nameParts := strings.Split(filepath.Base(path), "_")
	ts, err := strconv.ParseInt(strings.Split(nameParts[1], ".")[0], 10, 64)
	if err != nil {
		panic(err)
	}

	return time.Unix(ts, 0), accts
}

func mustReadDelegations(path string) (int64, []delegation) {
	content, err := genesisDelegationsFs.ReadFile(path)
	if err != nil {
		panic(err)
	}
	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	dels := make([]delegation, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "//") {
			continue
		}
		fields := strings.Split(line, ",")
		d := delegation{
			delegatee: strings.TrimSpace(fields[0]),
			delegator: strings.TrimSpace(fields[1]),
			shares:    strings.TrimSpace(fields[2]),
		}
		dels = append(dels, d)
	}

	height, err := strconv.ParseInt(strings.Split(filepath.Base(path), ".")[0], 10, 64)
	if err != nil {
		panic(err)
	}

	return height, dels
}

func init() {
	// Accounts.
	mainnetAccts := "accounts/mainnet"
	testnetAccts := "accounts/testnet"

	// Mainnet.
	files, err := genesisAccountsFs.ReadDir(mainnetAccts)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		ts, accts := mustReadAddrs(path.Join(mainnetAccts, file.Name()))
		mainnetAccounts.tss = append(mainnetAccounts.tss, ts)
		mainnetAccounts.accts = append(mainnetAccounts.accts, accts)
	}

	// Testnet.
	files, err = genesisAccountsFs.ReadDir(testnetAccts)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		ts, accts := mustReadAddrs(path.Join(testnetAccts, file.Name()))
		testnetAccounts.tss = append(testnetAccounts.tss, ts)
		testnetAccounts.accts = append(testnetAccounts.accts, accts)
	}

	// Delegations.
	mainnetDels := "delegations/mainnet"
	files, err = genesisDelegationsFs.ReadDir(mainnetDels)
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		height, dels := mustReadDelegations(path.Join(mainnetDels, file.Name()))
		mainnetDelegations.height = append(mainnetDelegations.height, height)
		mainnetDelegations.dels = append(mainnetDelegations.dels, dels)
	}
}

// QueueConsensusAccountsFirstActivity queues upserts for the first activity of consensus accounts.
//
// We need these static lists because we do not have access to the transaction history all the way back
// to the initial chain (pre-Cobalt). Therefore we use static lists of accounts existing at the network
// dump-restore upgrades for each pre-Cobalt upgrade and insert their approximate first_activity date.
func QueueConsensusAccountsFirstActivity(batch *storage.QueryBatch, chainName common.ChainName, logger *log.Logger) error {
	var accounts *accounts
	switch chainName {
	case common.ChainNameMainnet:
		accounts = mainnetAccounts
	case common.ChainNameTestnet:
		accounts = testnetAccounts
	default:
		return nil
	}

	for i := range accounts.accts {
		for _, account := range accounts.accts[i] {
			batch.Queue(
				queries.ConsensusAccountFirstActivityUpsert,
				account,
				accounts.tss[i].UTC(),
			)
		}
	}

	return nil
}

func QueueConsensusDelegationsSnapshots(batch *storage.QueryBatch, chainName common.ChainName, logger *log.Logger) error {
	var delegations *delegations
	switch chainName {
	case common.ChainNameMainnet:
		delegations = mainnetDelegations
	default:
		return nil
	}

	for i := range delegations.dels {
		for _, d := range delegations.dels[i] {
			batch.Queue(
				queries.ConsensusDelegationsHistoryInsert,
				delegations.height[i],
				d.delegatee,
				d.delegator,
				d.shares,
			)
		}
	}

	return nil
}
