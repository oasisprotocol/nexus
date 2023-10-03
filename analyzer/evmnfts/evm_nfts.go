package evmnfts

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/evmnfts/ipfsclient"
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

const (
	evmNFTsAnalyzerPrefix = "evm_nfts_"
)

type processor struct {
	runtime    common.Runtime
	source     nodeapi.RuntimeApiLite
	ipfsClient ipfsclient.Client
	target     storage.TargetStorage
	logger     *log.Logger
}

var _ item.ItemProcessor[*StaleNFT] = (*processor)(nil)

func NewAnalyzer(
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	sourceClient nodeapi.RuntimeApiLite,
	ipfsClient ipfsclient.Client,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", evmNFTsAnalyzerPrefix+runtime)
	p := &processor{
		runtime:    runtime,
		source:     sourceClient,
		ipfsClient: ipfsClient,
		target:     target,
		logger:     logger,
	}
	return item.NewAnalyzer[*StaleNFT](
		evmNFTsAnalyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type StaleNFT struct {
	Addr                  string
	ID                    common.BigInt
	Type                  *evm.EVMTokenType
	AddrContextIdentifier string
	AddrContextVersion    int
	AddrData              []byte
	DownloadRound         uint64
}

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*StaleNFT, error) {
	var staleNFTs []*StaleNFT
	rows, err := p.target.Query(ctx, queries.RuntimeEVMNFTAnalysisStale, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying discovered NFTs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var staleNFT StaleNFT
		if err = rows.Scan(
			&staleNFT.Addr,
			&staleNFT.ID,
			&staleNFT.Type,
			&staleNFT.AddrContextIdentifier,
			&staleNFT.AddrContextVersion,
			&staleNFT.AddrData,
			&staleNFT.DownloadRound,
		); err != nil {
			return nil, fmt.Errorf("scanning discovered nft: %w", err)
		}
		staleNFTs = append(staleNFTs, &staleNFT)
	}
	return staleNFTs, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, staleNFT *StaleNFT) error {
	p.logger.Info("downloading", "stale_nft", staleNFT)
	tokenEthAddr, err := client.EVMEthAddrFromPreimage(staleNFT.AddrContextIdentifier, staleNFT.AddrContextVersion, staleNFT.AddrData)
	if err != nil {
		return fmt.Errorf("token address: %w", err)
	}
	nftData, err := evm.EVMDownloadNewNFT(
		ctx,
		p.logger,
		p.source,
		p.ipfsClient,
		staleNFT.DownloadRound,
		tokenEthAddr,
		*staleNFT.Type,
		&staleNFT.ID.Int,
	)
	if err != nil {
		return fmt.Errorf("downloading NFT %s %v: %w", staleNFT.Addr, staleNFT.ID, err)
	}
	batch.Queue(
		queries.RuntimeEVMNFTUpdate,
		p.runtime,
		staleNFT.Addr,
		staleNFT.ID,
		staleNFT.DownloadRound,
		nftData.MetadataURI,
		nftData.MetadataAccessed,
		nftData.Name,
		nftData.Description,
		nftData.Image,
	)
	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var queueLength int
	if err := p.target.QueryRow(ctx, queries.RuntimeEVMNFTAnalysisStaleCount, p.runtime).Scan(&queueLength); err != nil {
		return 0, fmt.Errorf("querying number of stale NFTs: %w", err)
	}
	return queueLength, nil
}
