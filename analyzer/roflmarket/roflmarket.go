// Package roflmarket implements an analyzer that tracks ROFL market providers,
// their instances and their offers.
package roflmarket

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"time"

	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	analyzerPrefix = "roflmarket_"
)

type processor struct {
	runtime common.Runtime
	source  nodeapi.RuntimeApiLite
	target  storage.TargetStorage
	logger  *log.Logger
}

func NewAnalyzer(
	runtime common.Runtime,
	cfg config.ItemBasedAnalyzerConfig,
	sourceClient nodeapi.RuntimeApiLite,
	target storage.TargetStorage,
	logger *log.Logger,
) (analyzer.Analyzer, error) {
	logger = logger.With("analyzer", analyzerPrefix+runtime)
	p := &processor{
		runtime: runtime,
		source:  sourceClient,
		target:  target,
		logger:  logger,
	}
	return item.NewAnalyzer(
		analyzerPrefix+string(runtime),
		cfg,
		p,
		target,
		logger,
	)
}

type Item struct {
	ProviderAddress sdkTypes.Address
	LastQueuedRound uint64
}

var _ item.ItemProcessor[*Item] = (*processor)(nil)

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*Item, error) {
	rows, err := p.target.Query(ctx, queries.RuntimeRoflmarketStaleProviders, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying stale rofl market providers: %w", err)
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		var item Item
		var providerAddress string
		if err := rows.Scan(&providerAddress, &item.LastQueuedRound); err != nil {
			return nil, fmt.Errorf("scanning stale rofl app: %w", err)
		}
		if err := item.ProviderAddress.UnmarshalText([]byte(providerAddress)); err != nil {
			return nil, fmt.Errorf("unmarshalling rofl market provider address: %w", err)
		}
		items = append(items, &item)
	}
	return items, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *Item) error {
	if err := p.queueRoflmarketProviderRefresh(ctx, batch, item.LastQueuedRound, item.ProviderAddress); err != nil {
		return fmt.Errorf("rofl market provider refresh failed (providerAddress: %s, round: %d): %w", item.ProviderAddress, item.LastQueuedRound, err)
	}
	if err := p.queueRoflmarketOffersRefresh(ctx, batch, item.LastQueuedRound, item.ProviderAddress); err != nil {
		return fmt.Errorf("rofl market offers refresh failed (providerAddress: %s, round: %d): %w", item.ProviderAddress, item.LastQueuedRound, err)
	}
	if err := p.queueRoflmarketInstancesRefresh(ctx, batch, item.LastQueuedRound, item.ProviderAddress); err != nil {
		return fmt.Errorf("rofl market instances refresh failed (providerAddress: %s, round: %d): %w", item.ProviderAddress, item.LastQueuedRound, err)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var count int
	if err := p.target.QueryRow(ctx, queries.RuntimeRoflmarketStaleProvidersCount, p.runtime).Scan(&count); err != nil {
		return 0, fmt.Errorf("querying stale rofl market providers: %w", err)
	}
	return count, nil
}

func (p *processor) queueRoflmarketProviderRefresh(ctx context.Context, batch *storage.QueryBatch, round uint64, providerAddress sdkTypes.Address) error {
	provider, err := p.source.RoflMarketProvider(ctx, round, providerAddress)
	if err != nil {
		return err
	}
	var emptyAddress sdkTypes.Address
	if provider.Address.Equal(emptyAddress) {
		// Provider was removed.
		batch.Queue(
			queries.RuntimeRoflmarketProviderRemoved,
			p.runtime,
			providerAddress.String(),
			round,
		)
		// Remove provider's offers and instances.
		batch.Queue(
			queries.RuntimeRoflmarketRemoveProviderOffers,
			p.runtime,
			providerAddress.String(),
		)
		batch.Queue(
			queries.RuntimeRoflmarketRemoveProviderInstances,
			p.runtime,
			providerAddress.String(),
		)
		return nil
	}
	nodes := make([]string, 0, len(provider.Nodes))
	for _, node := range provider.Nodes {
		nodes = append(nodes, node.String())
	}
	// Convert uint64 timestamp to time.Time.
	createdAt := time.Unix(int64(provider.CreatedAt), 0)
	updatedAt := time.Unix(int64(provider.UpdatedAt), 0)

	batch.Queue(
		queries.RuntimeRoflmarketProviderUpdate,
		p.runtime,
		provider.Address.String(),
		nodes,
		provider.SchedulerApp.String(),
		provider.PaymentAddress,
		provider.Metadata,
		provider.Stake.Amount,
		provider.OffersNextID[:],
		provider.OffersCount,
		provider.InstancesNextID[:],
		provider.InstancesCount,
		createdAt,
		updatedAt,
		round,
	)
	return nil
}

func (p *processor) queueRoflmarketOffersRefresh(ctx context.Context, batch *storage.QueryBatch, round uint64, providerAddress sdkTypes.Address) error {
	// Fetch offers from the database.
	rows, err := p.target.Query(ctx, queries.RuntimeRoflMarketProviderOfferIds, p.runtime, providerAddress)
	if err != nil {
		return fmt.Errorf("querying rofl market provider offers: %w", err)
	}
	defer rows.Close()
	var existingOffers [][]byte
	for rows.Next() {
		var offerID []byte
		if err := rows.Scan(&offerID); err != nil {
			return fmt.Errorf("scanning rofl market provider offer: %w", err)
		}
		existingOffers = append(existingOffers, offerID)
	}

	// Fetch offers from the node.
	offers, err := p.source.RoflMarketOffers(ctx, round, providerAddress)
	if err != nil {
		return err
	}

	// Update offers.
	for _, offer := range offers {
		existingOffers = slices.DeleteFunc(existingOffers, func(id []byte) bool {
			return bytes.Equal(id, offer.ID[:])
		})

		batch.Queue(
			queries.RuntimeRoflmarketOfferUpsert,
			p.runtime,
			offer.ID[:],
			providerAddress.String(),
			offer.Resources,
			offer.Payment,
			offer.Capacity,
			offer.Metadata,
		)
	}

	// Mark deleted offers.
	for _, offerID := range existingOffers {
		batch.Queue(
			queries.RuntimeRoflmarketOfferRemoved,
			p.runtime,
			providerAddress.String(),
			offerID,
		)
	}

	return nil
}

func (p *processor) queueRoflmarketInstancesRefresh(ctx context.Context, batch *storage.QueryBatch, round uint64, providerAddress sdkTypes.Address) error {
	// Fetch instances from the database.
	rows, err := p.target.Query(ctx, queries.RuntimeRoflMarketProviderInstanceIds, p.runtime, providerAddress)
	if err != nil {
		return fmt.Errorf("querying rofl market provider instances: %w", err)
	}
	defer rows.Close()
	var existingInstances [][]byte
	for rows.Next() {
		var instanceID []byte
		if err := rows.Scan(&instanceID); err != nil {
			return fmt.Errorf("scanning rofl market provider instance: %w", err)
		}
		existingInstances = append(existingInstances, instanceID)
	}

	// Fetch instances from the node.
	instances, err := p.source.RoflMarketInstances(ctx, round, providerAddress)
	if err != nil {
		return err
	}

	// Update instances.
	for _, instance := range instances {
		existingInstances = slices.DeleteFunc(existingInstances, func(id []byte) bool {
			return bytes.Equal(id, instance.ID[:])
		})

		// Query instance commands.
		cmds, err := p.source.RoflMarketInstanceCommands(ctx, round, providerAddress, instance.ID)
		if err != nil {
			return fmt.Errorf("querying rofl market instance commands: %w", err)
		}

		var nodeID *string
		if instance.NodeID != nil {
			nodeID = common.Ptr(instance.NodeID.String())
		}
		// Convert uint64 timestamp to time.Time.
		createdAt := time.Unix(int64(instance.CreatedAt), 0)
		updatedAt := time.Unix(int64(instance.UpdatedAt), 0)
		paidFrom := time.Unix(int64(instance.PaidFrom), 0)
		paidUntil := time.Unix(int64(instance.PaidUntil), 0)
		batch.Queue(
			queries.RuntimeRoflmarketInstanceUpsert,
			p.runtime,
			instance.ID[:],
			instance.Provider.String(),
			instance.Offer[:],
			instance.Status,
			instance.Creator.String(),
			instance.Admin.String(),
			nodeID,
			instance.Metadata,
			instance.Resources,
			instance.Deployment,
			createdAt,
			updatedAt,
			paidFrom,
			paidUntil,
			instance.Payment,
			instance.PaymentAddress[:],
			instance.RefundData,
			instance.CmdNextID[:],
			instance.CmdCount,
			cmds,
		)
	}

	// Mark deleted instances.
	for _, instanceID := range existingInstances {
		batch.Queue(
			queries.RuntimeRoflmarketInstanceRemoved,
			p.runtime,
			providerAddress.String(),
			instanceID,
		)
	}

	return nil
}
