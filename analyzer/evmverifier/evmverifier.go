package evmverifier

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/evmverifier/sourcify"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	uncategorized "github.com/oasisprotocol/nexus/analyzer/uncategorized"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
)

const evmContractsVerifierAnalyzerPrefix = "evm_contract_verifier_"

type processor struct {
	chain            common.ChainName
	runtime          common.Runtime
	source           *sourcify.SourcifyClient
	target           storage.TargetStorage
	logger           *log.Logger
	queueLengthCache int // Cache the queue length to avoid querying Sourcify too often.
}

var _ item.ItemProcessor[contract] = (*processor)(nil)

func NewAnalyzer(
	chain common.ChainName,
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	sourcifyServerUrl string,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmContractsVerifierAnalyzerPrefix+runtime)
	if chain != common.ChainNameMainnet && chain != common.ChainNameTestnet {
		logger.Warn("EVM contracts verifier only supports testnet/mainnet, stopping", "chain_name", chain)
		return nil, fmt.Errorf("invalid chainName %s, expected one of testnet/mainnet", chain)
	}

	// Assuming we're running against "real" Sourcify, impose conservative request rates so we don't get banned.
	// If we're running against localhost, assume it's backed by a local cache and proceed quickly.
	// NOTE: We might hit Sourcify at very high rates if the local cache is empty, and the default intervals (0) are used.
	//       In my experiment with 26 contracts, that was not a problem - Sourcify did not have time to ban me.
	if !(strings.Contains(sourcifyServerUrl, "localhost") || strings.Contains(sourcifyServerUrl, "127.0.0.1")) {
		// Default interval is 5 minutes.
		if cfg.Interval == 0 {
			cfg.Interval = 5 * time.Minute
		}
		if cfg.Interval < time.Minute {
			return nil, fmt.Errorf("invalid interval %s, evm contracts verifier interval must be at least 1 minute to meet Sourcify rate limits", cfg.Interval.String())
		}
		// interItemDelay should be at least 1 second to meet Sourcify rate limits.
		if cfg.InterItemDelay == 0 {
			cfg.InterItemDelay = time.Second
		}
		if cfg.InterItemDelay < time.Second {
			return nil, fmt.Errorf("invalid interItemDelay %s, evm contracts verifier inter item delay must be at least 1 second to meet sourcify rate limits", cfg.InterItemDelay.String())
		}
	}

	client, err := sourcify.NewClient(sourcifyServerUrl, chain, logger)
	if err != nil {
		return nil, err
	}
	p := &processor{
		chain:   chain,
		runtime: runtime,
		source:  client,
		target:  target,
		logger:  logger,
	}

	return item.NewAnalyzer[contract](
		evmContractsVerifierAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type oasisAddress string

// A smart contract, and info on verification progress.
type contract struct {
	Addr                  oasisAddress
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
	EthAddr               ethCommon.Address
	VerificationLevel     sourcify.VerificationLevel // Status on Sourcify OR in Nexus; context-dependent.
}

func (p *processor) getNexusVerifiedContracts(ctx context.Context) (map[oasisAddress]sourcify.VerificationLevel, error) {
	rows, err := p.target.Query(ctx, queries.RuntimeEVMVerifiedContracts, p.runtime)
	if err != nil {
		return nil, fmt.Errorf("querying verified contracts: %w", err)
	}
	defer rows.Close()

	nexusVerifiedContracts := map[oasisAddress]sourcify.VerificationLevel{}
	for rows.Next() {
		var addr oasisAddress
		var level sourcify.VerificationLevel
		if err = rows.Scan(&addr, &level); err != nil {
			return nil, fmt.Errorf("scanning verified contracts: %w", err)
		}
		nexusVerifiedContracts[addr] = level
	}
	return nexusVerifiedContracts, nil
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]contract, error) {
	// Load all nexus-verified contracts from the DB.
	nexusLevels, err := p.getNexusVerifiedContracts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nexus verified contracts: %w", err)
	}

	// Query Sourcify for list of all verified contracts.
	sourcifyLevels, err := p.source.GetVerifiedContractAddresses(ctx, p.runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get verified contract addresses: %w", err)
	}
	p.logger.Debug("got verified contract addresses", "addresses", sourcifyLevels)

	// Find contracts that are verified in Sourcify and not yet verified in Nexus.
	var items []contract
	for ethAddr, sourcifyLevel := range sourcifyLevels {
		oasisAddr, err := uncategorized.StringifyEthAddress(ethAddr.Bytes())
		if err != nil {
			p.logger.Warn("failed to stringify eth address from sourcify", "err", err, "eth_address", ethAddr)
			continue
		}

		nexusLevel, isKnownToNexus := nexusLevels[oasisAddress(oasisAddr)]
		if !isKnownToNexus || (nexusLevel == sourcify.VerificationLevelPartial && sourcifyLevel == sourcify.VerificationLevelFull) {
			items = append(items, contract{
				Addr:              oasisAddress(oasisAddr),
				EthAddr:           ethAddr,
				VerificationLevel: sourcifyLevel,
			})
		}
	}

	p.queueLengthCache = len(items)
	if uint64(len(items)) > limit {
		items = items[:limit]
	}
	return items, nil
}

func extractABI(metadataJSON json.RawMessage) (*json.RawMessage, error) {
	type metadataStruct struct { // A subset of the metadata type; focuses only on ABI.
		Output struct {
			ABI json.RawMessage `json:"abi"`
		} `json:"output"`
	}
	var parsedMeta metadataStruct
	if err := json.Unmarshal(metadataJSON, &parsedMeta); err != nil {
		return nil, err
	}
	return &parsedMeta.Output.ABI, nil
}

// In inputs, item.VerificationLevel indicates the level on *Sourcify*.
func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item contract) error {
	p.logger.Debug("verifying contract", "address", item.Addr, "eth_address", item.EthAddr)

	// Load contract source files.
	sourceFiles, metadata, err := p.source.GetContractSourceFiles(ctx, p.runtime, item.EthAddr)
	if err != nil {
		return fmt.Errorf("failed to get contract source files: %w, eth_address: %s, address: %s", err, item.EthAddr, item.Addr)
	}
	sourceFilesJSON, err := json.Marshal(sourceFiles)
	if err != nil {
		return fmt.Errorf("failed to marshal source files: %w, eth_address: %s, address: %s", err, item.EthAddr, item.Addr)
	}

	var abi *json.RawMessage
	if abi, err = extractABI(metadata); err != nil {
		p.logger.Warn("failed to parse ABI from metadata", "err", err, "eth_address", item.EthAddr, "address", item.Addr)
	}

	p.logger.Info("verified contract", "address", item.Addr, "eth_address", item.EthAddr, "verification_level", item.VerificationLevel)

	batch.Queue(
		// NOTE: This also updates `verification_info_downloaded_at`, causing the `evm_abi` to re-parse
		//       the contract's txs and events.
		// NOTE: We upsert rather than update; if the contract is not in the db yet, UPDATE would ignore the
		//       contract and this analyzer would keep retrying it on every iteration.
		queries.RuntimeEVMVerifyContractUpsert,
		p.runtime,
		item.Addr,
		abi,
		metadata,
		sourceFilesJSON,
		item.VerificationLevel,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	return p.queueLengthCache, nil
}
