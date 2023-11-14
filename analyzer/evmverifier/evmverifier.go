package evmverifier

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/evmverifier/sourcify"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/client"
)

const evmContractsVerifierAnalyzerPrefix = "evm_contract_verifier_"

type processor struct {
	chain   common.ChainName
	runtime common.Runtime
	source  *sourcify.SourcifyClient
	target  storage.TargetStorage
	logger  *log.Logger
}

var _ item.ItemProcessor[*unverifiedContract] = (*processor)(nil)

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

	return item.NewAnalyzer[*unverifiedContract](
		evmContractsVerifierAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type VerificationState uint8

const (
	VerificationStateUnverified VerificationState = iota
	VerificationStatePartial
	VerificationStateFull
)

type unverifiedContract struct {
	Addr                  string
	VerificationState     VerificationState
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
	EthAddr               ethCommon.Address
}

func (p *processor) getUnverifiedContracts(ctx context.Context) ([]*unverifiedContract, error) {
	var unverifiedContracts []*unverifiedContract
	rows, err := p.target.Query(ctx, queries.RuntimeEVMUnverfiedContracts, p.runtime)
	if err != nil {
		return nil, fmt.Errorf("querying unverified contracts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contract unverifiedContract
		if err = rows.Scan(
			&contract.Addr,
			&contract.AddrContextIdentifier,
			&contract.AddrContextVersion,
			&contract.AddrData,
		); err != nil {
			return nil, fmt.Errorf("scanning unverified contracts: %w", err)
		}

		// Compute the eth address from the preimage.
		ethAddress, err := client.EVMEthAddrFromPreimage(contract.AddrContextIdentifier, contract.AddrContextVersion, contract.AddrData)
		if err != nil {
			return nil, fmt.Errorf("contract eth address: %w", err)
		}
		contract.EthAddr = ethCommon.BytesToAddress(ethAddress)

		unverifiedContracts = append(unverifiedContracts, &contract)

		// The analyzer is not overly optimized to handle a large amount of unverified contracts.
		// Log a warning in case this happens to refactor the analyzer if that ever happens.
		if len(unverifiedContracts) == 100_000 {
			p.logger.Warn("Unexpectedly high number of unverified contracts. Consider refactoring the analyzer", "runtime", p.runtime)
		}
	}
	return unverifiedContracts, nil
}

func (p *processor) getVerifiableContracts(ctx context.Context) ([]*unverifiedContract, error) {
	// Load all non-verified contracts from the DB.
	unverified, err := p.getUnverifiedContracts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get unverified contracts: %w", err)
	}
	if len(unverified) == 0 {
		p.logger.Debug("no unverified contracts in Nexus database")
		return nil, nil
	}

	// Query Sourcify for list of all verified contracts.
	response, err := p.source.GetVerifiedContractAddresses(ctx, p.runtime)
	if err != nil {
		return nil, fmt.Errorf("failed to get verified contract addresses: %w", err)
	}
	p.logger.Debug("got verified contract addresses", "response", response)
	if len(response.Full) == 0 && len(response.Partial) == 0 {
		p.logger.Debug("no verified contracts found in Sourcify")
		return nil, nil
	}
	// Create a lookup map of verified contract addresses.
	sourcifyFullAddresses := make(map[ethCommon.Address]bool, len(response.Full))
	for _, address := range response.Full {
		sourcifyFullAddresses[address] = true
	}

	// Pick currently unverified contracts that are present in sourcify.
	var canBeVerified []*unverifiedContract
	for _, contract := range unverified {
		if _, ok := sourcifyFullAddresses[contract.EthAddr]; ok {
			canBeVerified = append(canBeVerified, contract)
		}
	}

	return canBeVerified, nil
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*unverifiedContract, error) {
	verifiableContracts, err := p.getVerifiableContracts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get verifiable contracts: %w", err)
	}
	if len(verifiableContracts) > int(limit) {
		verifiableContracts = verifiableContracts[:limit]
	}
	return verifiableContracts, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *unverifiedContract) error {
	p.logger.Debug("verifying contract", "address", item.Addr, "eth_address", item.EthAddr)

	// Load contract source files.
	sourceFiles, metadata, err := p.source.GetContractSourceFiles(ctx, p.runtime, item.EthAddr)
	if err != nil {
		return fmt.Errorf("failed to get contract source files: %w, eth_address: %s, address: %s", err, item.EthAddr, item.Addr)
	}

	// Parse ABI from the metadata.
	type abiStruct struct {
		Output struct {
			ABI json.RawMessage `json:"abi"`
		} `json:"output"`
	}
	var abi abiStruct
	if err = json.Unmarshal(metadata, &abi); err != nil {
		p.logger.Warn("failed to parse ABI from metadata", "err", err, "eth_address", item.EthAddr, "address", item.Addr)
	}

	batch.Queue(
		queries.RuntimeEVMVerifyContractUpdate,
		p.runtime,
		item.Addr,
		abi.Output.ABI,
		metadata,
		sourceFiles,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	verifiableContracts, err := p.getVerifiableContracts(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to get verifiable contracts: %w", err)
	}
	return len(verifiableContracts), nil
}
