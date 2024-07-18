package config

import (
	"fmt"

	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"
	runtimeClient "github.com/oasisprotocol/nexus/coreapi/v22.2.11/runtime/client/api"

	"github.com/oasisprotocol/nexus/common"
)

type Record struct {
	ArchiveName   string `koanf:"archive_name"`
	GenesisHeight int64  `koanf:"genesis_height"`
	// RuntimeStartRounds has entries for runtimes that already exist at the
	// genesis of this network. Look these up in the genesis document's
	// .roothash.runtime_states[runtime_id_hex].round. For clarity, add an
	// entry stating round 0 in the first network where a runtime is available
	// (although code does not differentiate between the presence or absence
	// of a zero entry).
	RuntimeStartRounds map[common.Runtime]uint64 `koanf:"runtime_start_rounds"`
	ChainContext       string                    `koanf:"chain_context"`
}

type History struct {
	ChainName     common.ChainName    `koanf:"chain_name"`
	Records       []*Record           `koanf:"records"`
	MissingBlocks map[uint64]struct{} `koanf:"missing_blocks"`
}

func (h *History) CurrentRecord() *Record {
	return h.Records[0]
}

func (h *History) EarliestRecord() *Record {
	return h.Records[len(h.Records)-1]
}

func (h *History) RecordForHeight(height int64) (*Record, error) {
	if height == consensus.HeightLatest {
		return h.CurrentRecord(), nil
	}
	for _, r := range h.Records {
		if height >= r.GenesisHeight {
			return r, nil
		}
	}
	earliestRecord := h.EarliestRecord()
	return nil, fmt.Errorf(
		"height %d earlier than earliest history record %s genesis height %d",
		height,
		earliestRecord.ArchiveName,
		earliestRecord.GenesisHeight,
	)
}

func (h *History) RecordForChainContext(chainContext string) (*Record, error) {
	for _, r := range h.Records {
		if r.ChainContext == chainContext {
			return r, nil
		}
	}
	return nil, fmt.Errorf("chain context %s absent from history", chainContext)
}

func (h *History) RecordForRuntimeRound(runtime common.Runtime, round uint64) (*Record, error) {
	if round == runtimeClient.RoundLatest {
		return h.CurrentRecord(), nil
	}
	for _, r := range h.Records {
		if round >= r.RuntimeStartRounds[runtime] {
			return r, nil
		}
	}
	earliestRecord := h.EarliestRecord()
	return nil, fmt.Errorf(
		"runtime %s round %d earlier than earliest history record %s start round %d",
		runtime,
		round,
		earliestRecord.ArchiveName,
		earliestRecord.RuntimeStartRounds[runtime],
	)
}

var DefaultChains = map[common.ChainName]*History{
	common.ChainNameMainnet: {
		ChainName: common.ChainNameMainnet,
		// Block does not exist due to mishap during upgrade to Damask.
		MissingBlocks: map[uint64]struct{}{8048955: {}},
		Records: []*Record{
			{
				// https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2023-11-29
				ArchiveName:   "eden",
				GenesisHeight: 16817956,
				RuntimeStartRounds: map[common.Runtime]uint64{
					common.RuntimeCipher:   44054,
					common.RuntimeEmerald:  7875129,
					common.RuntimeSapphire: 1357486,
				},
				ChainContext: "bb3d748def55bdfb797a2ac53ee6ee141e54cd2ab2dc2375f4a0703a178e6e55",
			},
			{
				// https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2022-04-11
				ArchiveName:   "damask",
				GenesisHeight: 8048956,
				RuntimeStartRounds: map[common.Runtime]uint64{
					common.RuntimeCipher:   8284,
					common.RuntimeEmerald:  1003298,
					common.RuntimeSapphire: 0,
				},
				ChainContext: "b11b369e0da5bb230b220127f5e7b242d385ef8c6f54906243f30af63c815535",
			},
			{
				// https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2021-04-28
				ArchiveName:   "cobalt",
				GenesisHeight: 3027601,
				RuntimeStartRounds: map[common.Runtime]uint64{
					common.RuntimeCipher:  0,
					common.RuntimeEmerald: 0,
				},
				ChainContext: "53852332637bacb61b91b6411ab4095168ba02a50be4c3f82448438826f23898",
			},
			{
				// https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2020-11-18
				// This archive name coincides with the chain name "mainnet,"
				// but this "mainnet" refers to the consensus version that was
				// first used on the mainnet after mainnet beta. The testnet
				// chain also had a network running this consensus version
				// nicknamed "mainnet."
				ArchiveName:   "mainnet",
				GenesisHeight: 702000,
				ChainContext:  "a4dc2c4537992d6d2908c9779927ccfee105830250d903fd1abdfaf42cb45631",
			},
			{
				// https://github.com/oasisprotocol/mainnet-artifacts/releases/tag/2020-10-01
				ArchiveName:   "beta",
				GenesisHeight: 1,
				ChainContext:  "a245619497e580dd3bc1aa3256c07f68b8dcc13f92da115eadc3b231b083d3c4",
			},
		},
	},
	common.ChainNameTestnet: {
		ChainName:     common.ChainNameTestnet,
		MissingBlocks: map[uint64]struct{}{},
		Records: []*Record{
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2023-10-12
				ArchiveName:   "2023-10-12",
				GenesisHeight: 17751681,
				ChainContext:  "0b91b8e4e44b2003a7c5e23ddadb5e14ef5345c0ebcb3ddcae07fa2f244cab76",
				RuntimeStartRounds: map[common.Runtime]uint64{
					common.RuntimeCipher:      1730319,
					common.RuntimeEmerald:     2627790,
					common.RuntimeSapphire:    2995927,
					common.RuntimePontusxTest: 0,
					common.RuntimePontusxDev:  0,
				},
			},
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2022-03-03
				ArchiveName:   "2022-03-03",
				GenesisHeight: 8535081,
				ChainContext:  "50304f98ddb656620ea817cc1446c401752a05a249b36c9b90dba4616829977a",
				RuntimeStartRounds: map[common.Runtime]uint64{
					common.RuntimeCipher:   1675996,
					common.RuntimeEmerald:  398623,
					common.RuntimeSapphire: 0,
				},
			},
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2021-04-13
				ArchiveName:   "2021-04-13",
				GenesisHeight: 3398334,
				ChainContext:  "5ba68bc5e01e06f755c4c044dd11ec508e4c17f1faf40c0e67874388437a9e55",
				RuntimeStartRounds: map[common.Runtime]uint64{
					common.RuntimeCipher:  0,
					common.RuntimeEmerald: 0,
				},
			},
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2021-03-24
				ArchiveName:   "2021-03-24",
				GenesisHeight: 3076800,
				ChainContext:  "TODO-UNKNOWN",
			},
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2021-02-03
				ArchiveName:   "2021-02-03",
				GenesisHeight: 2284801,
				ChainContext:  "TODO-UNKNOWN",
			},
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2020-11-04
				ArchiveName:   "2020-11-04",
				GenesisHeight: 811055,
				ChainContext:  "TODO-UNKNOWN",
			},
			{
				// https://github.com/oasisprotocol/testnet-artifacts/releases/tag/2020-09-15
				ArchiveName:   "2020-09-15",
				GenesisHeight: 1,
				ChainContext:  "TODO-UNKNOWN",
			},
		},
	},
}
