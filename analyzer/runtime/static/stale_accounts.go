// Package static contains static data for the analyzer.
package static

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

var (
	//go:embed accounts
	staleAccountsFS embed.FS
	staleAccounts   = make(map[staleAcctsKey][]string) // Parsed version of embed.FS
)

type staleAcctsKey struct {
	chain   common.ChainName
	runtime common.Runtime
	height  uint64
}

// Parses a file with a list of stale accounts. The accounts are parsed from the file (one per line);
// the metadata is parsed from the filename, which should be of the form "<chainName>_<runtime>_<height>_*".
func mustReadStaleAccts(path string) (chain common.ChainName, runtime common.Runtime, height uint64, accts []string) {
	content, err := staleAccountsFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	accts = strings.Split(strings.TrimSpace(string(content)), "\n")

	nameParts := strings.Split(filepath.Base(path), "_")
	chain = common.ChainName(nameParts[0])
	runtime = common.Runtime(nameParts[1])
	height, err = strconv.ParseUint(strings.Split(nameParts[2], ".")[0], 10, 64)
	if err != nil {
		panic(err)
	}
	return
}

func init() {
	readAccounts := func(files []fs.DirEntry, basePath string) {
		for _, file := range files {
			chainName, runtime, height, accts := mustReadStaleAccts(path.Join(basePath, file.Name()))
			staleAccounts[staleAcctsKey{chainName, runtime, height}] = accts
		}
	}

	preEden, err := staleAccountsFS.ReadDir("accounts/pre_eden")
	if err != nil {
		panic(err)
	}
	readAccounts(preEden, "accounts/pre_eden")

	eden, err := staleAccountsFS.ReadDir("accounts/eden")
	if err != nil {
		panic(err)
	}
	readAccounts(eden, "accounts/eden")
}

// Returns the list of known stale accounts for a given chain and runtime,
// and assumes there's only one such list. Returns such a list only once.
// TODO: Remove this function; see callsite for more info.
func knownStaleAccountsRegardlessOfHeight(chainName common.ChainName, runtime common.Runtime) []string {
	for key, accounts := range staleAccounts {
		if key.chain == chainName && key.runtime == runtime {
			accountsCopy := accounts
			delete(staleAccounts, key) // to avoid re-applying on next rounds
			return accountsCopy
		}
	}
	return nil
}

// QueueEVMKnownStaleAccounts queues (known-to-be stale) account lists at specific heights for native token balance update.
//
// At the moment, these lists were manually obtained at rounds soon after Eden genesis, to mitigate the following issues:
// - runtimes used to not emit Transfer events for FEE payments
//   - therefore we are unable to dead-reckon these balances, as fee payment was missed
//
// - oasis-sdk runtimes used to not emit Transfer events for Reward and Fee disbursements
//   - therefore there exist accounts that have received native balance, but if they never submitted any transactions then Nexus is unaware of them
//
// These were fixed sometime during Damask, but for simplicity we take the list of accounts at (or soon after) Eden genesis,
// which possibly contains a few additional accounts that were not effected by the above issues and are instead incorrect due to unknown Nexus bugs.
//
// This list also helps partially mitigating the issue of accounts that have only ever interacted within internal EVM transactions (e.g. transfers within evm.Call).
// This is an ongoing issue where Nexus cannot known about the existence of these accounts as Nexus is unable to simulate EVM transactions.
// With these lists we at least know about all such accounts at Eden genesis time.
func QueueEVMKnownStaleAccounts(batch *storage.QueryBatch, chainName common.ChainName, runtime common.Runtime, round uint64, logger *log.Logger) error {
	accounts, ok := staleAccounts[staleAcctsKey{chainName, runtime, round}]
	if !ok {
		// XXX: The whole "then" branch of this `if` is a temporary hack that allows us to
		//      mark known stale accounts in the DB even though we already passed the height at which
		//      we'd normally do this. This lets us avoid a reindex.
		// TODO: Remove the hack once stale accounts have been marked in all internal deploys.
		if os.Getenv("NEXUS_FORCE_MARK_STALE_ACCOUNTS") == "1" {
			accounts = knownStaleAccountsRegardlessOfHeight(chainName, runtime)
		} else {
			return nil
		}
	}
	if len(accounts) == 0 {
		return nil
	}

	logger.Info("Enqueueing known pre-Eden stale accounts for EVM native token balance update", "height", round, "n_accounts", len(accounts))
	for _, account := range accounts {
		// The (non-binding) assumption is that the block analyzer has already indexed past the height of these accounts. But:
		//   - If the block analyzer already scanned past the height and mutated the accounts more recently, the below will be a no-op.
		//   - If the analyzer is behind the height of these accounts, the account will have `last_mutated_round` set in future and it
		//     will periodically be re-queried until the round is reached so that `download_round` >= `last_mutated_round`.
		//     This periodic re-querying might starve other accounts of balance updates though.
		batch.Queue(
			queries.RuntimeEVMTokenBalanceAnalysisMutateRoundUpsert,
			runtime, evm.NativeRuntimeTokenAddress, account, round,
		)
	}

	return nil
}
