package evmtokens

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/analyzer/runtime/evm"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// The token analyzer (1) gets a list from the database of tokens to download
// the info for, (2) downloads that info, and (3) saves the info in the
// database. Note that steps (1) and (3) don't happen in a single transaction,
// so that we don't hold the tables locked and disrupt the block scanner. So
// be careful of block analyzer potentially making further updates in between.
// That's why, for example, there are separate last_mutate_round and
// last_download_round columns. The block analyzer updates last_mutate_round,
// this token analyzer updates last_download_round.

const (
	//nolint:gosec // thinks this is a hardcoded credential
	evmTokensAnalyzerPrefix = "evm_tokens_"
)

type processor struct {
	runtime common.Runtime
	source  nodeapi.RuntimeApiLite
	target  storage.TargetStorage
	logger  *log.Logger
}

var _ item.ItemProcessor[*StaleToken] = (*processor)(nil)

func NewAnalyzer(
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmTokensAnalyzerPrefix+runtime)
	p := &processor{
		runtime: runtime,
		source:  sourceClient,
		target:  target,
		logger:  logger,
	}

	return item.NewAnalyzer[*StaleToken](
		evmTokensAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type StaleToken struct {
	Addr                  string
	LastDownloadRound     *uint64
	TotalSupply           common.BigInt
	NumTransfers          uint64
	Type                  *common.TokenType
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
	DownloadRound         uint64
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*StaleToken, error) {
	var staleTokens []*StaleToken
	rows, err := p.target.Query(ctx, queries.RuntimeEVMTokenAnalysisStale, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying discovered tokens: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var staleToken StaleToken
		var totalSupply pgtype.Numeric
		if err = rows.Scan(
			&staleToken.Addr,
			&staleToken.LastDownloadRound,
			&totalSupply,
			&staleToken.NumTransfers,
			&staleToken.Type,
			&staleToken.AddrContextIdentifier,
			&staleToken.AddrContextVersion,
			&staleToken.AddrData,
			&staleToken.DownloadRound,
		); err != nil {
			return nil, fmt.Errorf("scanning discovered token: %w", err)
		}
		staleToken.TotalSupply, err = common.NumericToBigInt(totalSupply)
		if err != nil {
			return nil, fmt.Errorf("unmarshalling token totalSupply: %w", err)
		}
		staleTokens = append(staleTokens, &staleToken)
	}
	return staleTokens, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, staleToken *StaleToken) error {
	p.logger.Info("downloading", "stale_token", staleToken)
	tokenEthAddr, err := client.EVMEthAddrFromPreimage(staleToken.AddrContextIdentifier, staleToken.AddrContextVersion, staleToken.AddrData)
	if err != nil {
		return fmt.Errorf("token address: %w", err)
	}
	// These two conditions should be equivalent; only a never-before-downloaded token
	// can have a nil type, because downloading a token always sets the type.
	// We check both just in case, because we later dereference the .Type pointer.
	if staleToken.LastDownloadRound == nil || staleToken.Type == nil { //nolint:nestif // has complex nested blocks (complexity: 5)
		tokenData, err := evm.EVMDownloadNewToken(
			ctx,
			p.logger,
			p.source,
			staleToken.DownloadRound,
			tokenEthAddr,
		)
		if err != nil {
			return fmt.Errorf("downloading new token %s: %w", staleToken.Addr, err)
		}
		// Use the totalSupply downloaded directly from the chain if it exists,
		// else keep the dead-reckoned value.
		totalSupply := staleToken.TotalSupply.String()
		if tokenData.EVMTokenMutableData != nil && tokenData.TotalSupply != nil {
			totalSupply = tokenData.TotalSupply.String()
		}
		batch.Queue(queries.RuntimeEVMTokenDownloadedUpsert,
			p.runtime,
			staleToken.Addr,
			tokenData.Type,
			tokenData.Name,
			tokenData.Symbol,
			tokenData.Decimals,
			totalSupply,
			staleToken.DownloadRound,
		)
	} else if *staleToken.Type != common.TokenTypeUnsupported {
		mutable, err := evm.EVMDownloadMutatedToken(
			ctx,
			p.logger,
			p.source,
			staleToken.DownloadRound,
			tokenEthAddr,
			*staleToken.Type,
		)
		if err != nil {
			return fmt.Errorf("downloading mutated token %s: %w", staleToken.Addr, err)
		}
		// We may be unable to download the totalSupply because
		// of a deterministic error or because the contract may
		// not provide the data.
		if mutable != nil && mutable.TotalSupply != nil {
			batch.Queue(queries.RuntimeEVMTokenDownloadedTotalSupplyUpdate,
				p.runtime,
				staleToken.Addr,
				mutable.TotalSupply.String(),
			)
		}
	}
	batch.Queue(queries.RuntimeEVMTokenDownloadRoundUpdate,
		p.runtime,
		staleToken.Addr,
		staleToken.DownloadRound,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var queueLength int
	if err := p.target.QueryRow(ctx, queries.RuntimeEVMTokenAnalysisStaleCount, p.runtime).Scan(&queueLength); err != nil {
		return 0, fmt.Errorf("querying number of stale tokens: %w", err)
	}
	return queueLength, nil
}
