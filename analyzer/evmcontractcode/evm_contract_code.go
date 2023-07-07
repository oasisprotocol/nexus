package evmcontractcode

import (
	"context"
	"fmt"
	"time"

	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/util"
	"github.com/oasisprotocol/nexus/common"
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
	maxDownloadBatch              = 20
	downloadTimeout               = 61 * time.Second
)

type oasisAddress string

type main struct {
	runtime           common.Runtime
	source            nodeapi.RuntimeApiLite
	target            storage.TargetStorage
	logger            *log.Logger
	queueLengthMetric prometheus.Gauge
}

var _ analyzer.Analyzer = (*main)(nil)

func NewMain(
	runtime common.Runtime,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	m := &main{
		runtime: runtime,
		source:  sourceClient,
		target:  target,
		logger:  logger.With("analyzer", evmContractCodeAnalyzerPrefix+runtime),
		queueLengthMetric: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: fmt.Sprintf("%s%s_queue_length", evmContractCodeAnalyzerPrefix, runtime),
			Help: "count of stale analysis.evm_contract_code rows",
		}),
	}
	prometheus.MustRegister(m.queueLengthMetric)
	return m, nil
}

type ContractCandidate struct {
	Addr          oasisAddress
	EthAddr       ethCommon.Address
	DownloadRound uint64
}

func (m main) getContractCandidates(ctx context.Context, limit int) ([]ContractCandidate, error) {
	var candidates []ContractCandidate
	rows, err := m.target.Query(ctx, queries.RuntimeEVMContractCodeAnalysisStale, m.runtime, limit)
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
		candidates = append(candidates, cc)
	}
	return candidates, nil
}

func (m main) processContractCandidate(ctx context.Context, batch *storage.QueryBatch, candidate ContractCandidate) error {
	m.logger.Info("downloading code", "addr", candidate.Addr, "eth_addr", candidate.EthAddr.Hex())
	code, err := m.source.EVMGetCode(ctx, candidate.DownloadRound, candidate.EthAddr.Bytes())
	if err != nil {
		// Write nothing into the DB; we'll try again later.
		return fmt.Errorf("downloading code for %x: %w", candidate.EthAddr, err)
	}
	if len(code) == 0 {
		batch.Queue(
			queries.RuntimeEVMContractCodeAnalysisSetIsContract,
			m.runtime,
			candidate.Addr,
			false, // is_contract
		)
	} else {
		batch.Queue(
			queries.RuntimeEVMContractCodeAnalysisSetIsContract,
			m.runtime,
			candidate.Addr,
			true, // is_contract
		)
		batch.Queue(
			queries.RuntimeEVMContractRuntimeBytecodeUpsert,
			m.runtime,
			candidate.Addr,
			code,
		)
	}
	return nil
}

func (m main) sendQueueLengthMetric(ctx context.Context) error {
	var queueLength int
	if err := m.target.QueryRow(ctx, queries.RuntimeEVMContractCodeAnalysisStaleCount, m.runtime).Scan(&queueLength); err != nil {
		return fmt.Errorf("querying number of stale contract code entries: %w", err)
	}
	m.queueLengthMetric.Set(float64(queueLength))
	return nil
}

func (m main) processBatch(ctx context.Context) (int, error) {
	contractCandidates, err := m.getContractCandidates(ctx, maxDownloadBatch)
	if err != nil {
		return 0, fmt.Errorf("getting contract candidates: %w", err)
	}
	m.logger.Info("processing", "num_contract_candidates", len(contractCandidates))
	if len(contractCandidates) == 0 {
		return 0, nil
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	group, groupCtx := errgroup.WithContext(ctxWithTimeout)

	batches := make([]*storage.QueryBatch, 0, len(contractCandidates))

	for _, cc := range contractCandidates {
		cc := cc // Redeclare for unclobbered use within goroutine.
		batch := &storage.QueryBatch{}
		batches = append(batches, batch)
		group.Go(func() error {
			return m.processContractCandidate(groupCtx, batch, cc)
		})
	}

	if err := group.Wait(); err != nil {
		return 0, err
	}

	batch := &storage.QueryBatch{}
	for _, b := range batches {
		batch.Extend(b)
	}
	if err := m.target.SendBatch(ctx, batch); err != nil {
		return 0, fmt.Errorf("sending batch: %w", err)
	}
	return len(contractCandidates), nil
}

func (m main) Start(ctx context.Context) {
	backoff, err := util.NewBackoff(
		100*time.Millisecond,
		// Cap the timeout at the expected round time. All runtimes currently have the same round time.
		6*time.Second,
	)
	if err != nil {
		m.logger.Error("error configuring backoff policy",
			"err", err,
		)
		return
	}

	for {
		select {
		case <-time.After(backoff.Timeout()):
			// Process next block.
		case <-ctx.Done():
			m.logger.Warn("shutting down evm_contract_code analyzer", "reason", ctx.Err())
			return
		}

		if err := m.sendQueueLengthMetric(ctx); err != nil {
			m.logger.Warn("error sending queue length", "err", err)
		}

		numProcessed, err := m.processBatch(ctx)
		if err != nil {
			m.logger.Error("error processing batch", "err", err)
			backoff.Failure()
			continue
		}

		if numProcessed == 0 {
			// Count this as a failure to reduce the polling when we are
			// running faster than the block analyzer can find new contract candidates.
			backoff.Failure()
			continue
		}

		backoff.Success()
	}
}

func (m main) Name() string {
	return evmContractCodeAnalyzerPrefix + string(m.runtime)
}
