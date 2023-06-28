package evmverifier

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/evmverifier/sourcify"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/client"
)

const evmContractsVerifierName = "evmcontracts-verifier"

var evmRuntimes = []common.Runtime{common.RuntimeEmerald, common.RuntimeSapphire}

// evmContractsVerifier is an analyzer that verifies EVM contracts.
type evmContractsVerifier struct {
	interval time.Duration
	target   storage.TargetStorage
	logger   *log.Logger
	chain    common.ChainName
	client   *sourcify.SourcifyClient
}

var _ analyzer.Analyzer = (*evmContractsVerifier)(nil)

func (a *evmContractsVerifier) Name() string {
	return evmContractsVerifierName
}

type unverifiedContract struct {
	Addr                  string
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
	EthAddr               ethCommon.Address
}

func (a *evmContractsVerifier) getUnverifiedContracts(ctx context.Context, runtime common.Runtime) ([]*unverifiedContract, error) {
	var unverifiedContracts []*unverifiedContract
	rows, err := a.target.Query(ctx, queries.RuntimeEVMUnverfiedContracts, runtime)
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
			a.logger.Warn("Unexpectedly high number of unverified contracts. Consider refactoring the analyzer", "runtime", runtime)
		}
	}
	return unverifiedContracts, nil
}

func (a *evmContractsVerifier) Start(ctx context.Context) {
	if a.chain != common.ChainNameMainnet && a.chain != common.ChainNameTestnet {
		a.logger.Warn("EVM contracts verifier only supports testnet/mainnet, stopping", "chain_name", a.chain)
		return
	}

	for {
		select {
		case <-time.After(a.interval):
		case <-ctx.Done():
			a.logger.Warn("shutting down EVM contracts verifier", "reason", ctx.Err())
			return
		}

		a.logger.Info("EVM contracts verifier iteration")

		for _, runtime := range evmRuntimes {
			// Load all non-verified contracts from the DB.
			unverified, err := a.getUnverifiedContracts(ctx, runtime)
			if err != nil {
				a.logger.Error("failed to get unverified contracts", "err", err, "runtime", runtime)
				continue
			}
			if len(unverified) == 0 {
				a.logger.Debug("no unverified contracts", "runtime", runtime)
				continue
			}

			// Query Sourcify for list of all verified contracts.
			addresses, err := a.client.GetVerifiedContractAddresses(ctx, runtime)
			if err != nil {
				a.logger.Error("failed to get verified contract addresses", "err", err, "runtime", runtime)
				continue
			}
			a.logger.Debug("got verified contract addresses", "runtime", runtime, "addresses", addresses)
			if len(addresses) == 0 {
				continue
			}
			// Create a lookup map of verified contract addresses.
			sourcifyAddresses := make(map[ethCommon.Address]bool, len(addresses))
			for _, address := range addresses {
				sourcifyAddresses[address] = true
			}

			// Pick currently unverified contracts that are present in sourcify.
			canBeVerified := []*unverifiedContract{}
			for _, contract := range unverified {
				if _, ok := sourcifyAddresses[contract.EthAddr]; ok {
					canBeVerified = append(canBeVerified, contract)
				}
			}

			// Verify the contracts.
			for _, contract := range canBeVerified {
				a.logger.Debug("verifying contract", "runtime", runtime, "address", contract.Addr, "eth_address", contract.EthAddr)

				// Timeout a bit between API calls.
				select {
				case <-time.After(1 * time.Second):
				case <-ctx.Done():
					a.logger.Warn("shutting down EVM contracts verifier", "reason", ctx.Err())
					return
				}

				// Load contract source files.
				sourceFiles, metadata, err := a.client.GetContractSourceFiles(ctx, runtime, contract.EthAddr)
				if err != nil {
					a.logger.Error("failed to get contract source files", "err", err, "runtime", runtime, "eth_address", contract.EthAddr, "address", contract.Addr)
					continue
				}

				// Parse ABI from the metadata.
				type abiStruct struct {
					Output struct {
						ABI json.RawMessage `json:"abi"`
					} `json:"output"`
				}
				var abi abiStruct
				if err = json.Unmarshal(metadata, &abi); err != nil {
					a.logger.Warn("failed to parse ABI from metadata", "err", err, "runtime", runtime, "eth_address", contract.EthAddr, "address", contract.Addr)
				}

				// Update the verified contract in the DB.
				rows, err := a.target.Query(
					ctx,
					queries.RuntimeEVMVerifyContractUpdate,
					runtime,
					contract.Addr,
					abi.Output.ABI,
					metadata,
					sourceFiles,
				)
				if err != nil {
					a.logger.Error("failed to update verified contract", "err", err, "runtime", runtime, "eth_address", contract.EthAddr, "address", contract.Addr)
					continue
				}
				rows.Close()
			}
		}
	}
}

func NewEVMVerifierAnalyzer(cfg *config.EVMContractsVerifierConfig, target storage.TargetStorage, logger *log.Logger) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmContractsVerifierName)

	client, err := sourcify.NewClient(cfg.SourcifyServerUrl, cfg.ChainName, logger)
	if err != nil {
		return nil, err
	}

	return &evmContractsVerifier{
		interval: cfg.Interval,
		target:   target,
		logger:   logger,
		chain:    cfg.ChainName,
		client:   client,
	}, nil
}
