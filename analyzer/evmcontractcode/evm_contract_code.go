package evmcontractcode

import (
	"context"
	"fmt"

	ethCommon "github.com/ethereum/go-ethereum/common"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// This analyzer checks the list of addresses with an unknown is_contract status,
// and determines it by calling `getCode()` on the address.
// If the code is returned, it is also stored in the DB.
// Every address that is the recipient of a call is a potential contract address.
// Candidate addresses are inserted into the DB by the block analyzer.
// Each candidate address only needs to be checked once.

const (
	evmContractCodeAnalyzerPrefix = "evm_contract_code_"
)

type oasisAddress string

type processor struct {
	runtime common.Runtime
	source  nodeapi.RuntimeApiLite
	target  storage.TargetStorage
	logger  *log.Logger
}

var _ item.ItemProcessor[*ContractCandidate] = (*processor)(nil)

func NewAnalyzer(
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmContractCodeAnalyzerPrefix+runtime)
	p := &processor{
		runtime: runtime,
		source:  sourceClient,
		target:  target,
		logger:  logger,
	}
	return item.NewAnalyzer[*ContractCandidate](
		evmContractCodeAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type ContractCandidate struct {
	Addr          oasisAddress
	EthAddr       ethCommon.Address
	DownloadRound uint64
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*ContractCandidate, error) {
	var candidates []*ContractCandidate
	rows, err := p.target.Query(ctx, queries.RuntimeEVMContractCodeAnalysisStale, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying contract candidates: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cc ContractCandidate
		if err = rows.Scan(
			&cc.Addr,
			&cc.EthAddr,
			&cc.DownloadRound,
		); err != nil {
			return nil, fmt.Errorf("scanning contract candidate: %w", err)
		}
		candidates = append(candidates, &cc)
	}
	return candidates, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, candidate *ContractCandidate) error {
	p.logger.Info("downloading code", "addr", candidate.Addr, "eth_addr", candidate.EthAddr.Hex(), "round", candidate.DownloadRound)
	code, err := p.source.EVMGetCode(ctx, candidate.DownloadRound, candidate.EthAddr.Bytes())
	if err != nil {
		// Write nothing into the DB; we'll try again later.
		return fmt.Errorf("downloading code for %x: %w", candidate.EthAddr, err)
	}
	if len(code) == 0 {
		batch.Queue(
			queries.RuntimeEVMContractCodeAnalysisSetIsContract,
			p.runtime,
			candidate.Addr,
			false, // is_contract
		)
	} else {
		batch.Queue(
			queries.RuntimeEVMContractCodeAnalysisSetIsContract,
			p.runtime,
			candidate.Addr,
			true, // is_contract
		)
		batch.Queue(
			queries.RuntimeEVMContractRuntimeBytecodeUpsert,
			p.runtime,
			candidate.Addr,
			code,
		)
	}
	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var queueLength int
	if err := p.target.QueryRow(ctx, queries.RuntimeEVMContractCodeAnalysisStaleCount, p.runtime).Scan(&queueLength); err != nil {
		return 0, fmt.Errorf("querying number of stale contract code entries: %w", err)
	}
	return queueLength, nil
}
