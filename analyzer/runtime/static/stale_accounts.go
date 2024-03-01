package static

import (
	"embed"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage"
)

//go:embed pre_eden
var preEdenStaleAcctsFS embed.FS
var preEdenStaleAccts map[staleAcctsKey][]string // Parsed version of embed.FS

type staleAcctsKey struct {
	net     common.ChainName
	runtime common.Runtime
	height  int64
}

// Parses a file with a list of stale accounts. The accounts are parsed from the file (one per line);
// the metadata is parsed from the filename, which should be of the form {chainName}_{runtime}_{height}_*
func readStaleAccts(path string) (chain common.ChainName, runtime common.Runtime, height int64, accts []string) {
	content, err := preEdenStaleAcctsFS.ReadFile(path)
	if err != nil {
		panic(err)
	}
	accts = strings.Split(string(content), "\n")

	nameParts := strings.Split(filepath.Base(path), "_")
	chain = common.ChainName(nameParts[0])
	runtime = common.Runtime(nameParts[1])
	height, err = strconv.ParseInt(nameParts[2], 10, 64)
	if err != nil {
		panic(err)
	}
	return
}

func init() {
	files, err := preEdenStaleAcctsFS.ReadDir(".")
	if err != nil {
		panic(err)
	}
	for _, file := range files {
		chainName, runtime, height, accts := readStaleAccts(file.Name())
		if err != nil {
			panic(err)
		}
		preEdenStaleAccts[staleAcctsKey{chainName, runtime, height}] = accts
	}
}

func staleAccountsList(chainName common.ChainName, runtime common.Runtime) (uint64, []string) {
	// TODO: probably use embed.FS to keep these long list in files, but
	// also include them in the binary.
	// Also name the files something like: {chainName}_{runtime}_{height}_stale_accounts.txt
	// So that the switch below is not needed.
	switch chainName {
	case common.ChainNameMainnet:
		switch runtime {
		case common.RuntimeEmerald:
			return 7875129, []string{}
		case common.RuntimeSapphire:
			// For Sapphire mainnet the number of all account at Eden genesis is ~3k, so we could likely just include all?
			return 1357486, []string{}
		default:
			// No known accounts for this runtime.
			return 0, nil
		}
	case common.ChainNameTestnet:
		switch runtime {
		case common.RuntimeEmerald:
			return 2627790, []string{}
		case common.RuntimeSapphire:
			return 2995927, []string{}
		default:
			// No known accounts for this runtime.
			return 0, nil
		}
	default:
		// No known accounts for this chain.
		return 0, nil
	}
}

// QueueEVMKnownStaleAccounts queues (known-to-be stale) account lists at specific heights for native token balance update.
//
// At the moment, these lists were manually obtained at Eden genesis heights, to mitigate the following issues:
// - runtimes used to not emit Transfer events for FEE payments
//   - therefore we are unable to dead-reckon these balances, as fee payment was missed
//
// - oasis-sdk runtimes used to not emit Transfer events for Reward and Fee disbursements
//   - therefore there exist accounts that have received native balance, but if they never submitted any transactions the Nexus is unaware of them
//
// These were fixed sometime during Damask, but for simplicity we take the list of accounts at Eden genesis, which at worst contains additional
// accounts that were not effected by the above issues.
//
// This list also helps partially mitigating the issue of accounts that have only ever interacted within internal EVM transactions (e.g. transfers within evm.Call).
// This is an ongoing issue where Nexus cannot known about the existence of these accounts as Nexus is unable to simulate EVM transactions.
// With these lists we at least know about all such accounts at Eden genesis time.
func QueueEVMKnownStaleAccounts(batch *storage.QueryBatch, chainName common.ChainName, runtime common.Runtime, height int64) error {
	accounts := preEdenStaleAccts[staleAcctsKey{chainName, runtime, height}]
	if len(accounts) == 0 {
		return nil
	}

	for _, account := range accounts {
		// Depending on fast-sync configuration different scenarios can happen:
		// - The analyzer has already indexed past the height of these accounts:
		//   - The account might have been mutated recently, therefore the below will be a no-op.
		//   - The account might have never been mutated (or not in a long time), below will queue it to be required.
		// - The analyzer is behind the height of these accounts:
		//   - The account will have `last_mutated_round` set in future and it will periodically be required
		//     until the round is reached so that `download_round` >= `last_mutated_round`.
		batch.Queue(
			queries.RuntimeEVMTokenBalanceAnalysisMutateRoundUpsert,
			runtime, evm.NativeRuntimeTokenAddress, apiTypes.Address(account), round,
		)
	}

	return nil
}
