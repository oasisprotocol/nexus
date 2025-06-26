// Package rofl implements an analyzer that tracks ROFL apps and their
// instances.
package rofl

import (
	"context"
	"encoding/base64"
	"fmt"

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
	analyzerPrefix = "rofl_"
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
	AppID           nodeapi.AppID
	LastQueuedRound uint64

	// XXX: Could track instance refreshes separately, to avoid app refresh when only instances
	// are updated, if we end up with to many updates in future.
	// InstanceRAK *nodeapi.PublicKey
}

var _ item.ItemProcessor[*Item] = (*processor)(nil)

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*Item, error) {
	rows, err := p.target.Query(ctx, queries.RuntimeRoflStaleApps, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying stale rofl apps: %w", err)
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		var item Item
		var appId string
		if err := rows.Scan(&appId, &item.LastQueuedRound); err != nil {
			return nil, fmt.Errorf("scanning stale rofl app: %w", err)
		}
		if err := item.AppID.UnmarshalText([]byte(appId)); err != nil {
			return nil, fmt.Errorf("unmarshalling rofl app id: %w", err)
		}
		items = append(items, &item)
	}
	return items, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *Item) error {
	if err := p.queueRoflAppRefresh(ctx, batch, item.LastQueuedRound, item.AppID); err != nil {
		return fmt.Errorf("rofl app refresh failed (appID: %s, round: %d): %w", item.AppID, item.LastQueuedRound, err)
	}
	if err := p.queueRoflAppInstancesRefresh(ctx, batch, item.LastQueuedRound, item.AppID); err != nil {
		return fmt.Errorf("rofl app instances refresh failed (appID: %s, round: %d): %w", item.AppID, item.LastQueuedRound, err)
	}

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var count int
	if err := p.target.QueryRow(ctx, queries.RuntimeRoflStaleAppsCount, p.runtime).Scan(&count); err != nil {
		return 0, fmt.Errorf("querying stale rofl apps: %w", err)
	}
	return count, nil
}

func (p *processor) queueRoflAppRefresh(ctx context.Context, batch *storage.QueryBatch, round uint64, id nodeapi.AppID) error {
	app, err := p.source.RoflApp(ctx, round, id)
	if err != nil {
		return err
	}
	var appID nodeapi.AppID
	if app.ID == appID {
		// App was removed
		batch.Queue(
			queries.RuntimeRoflAppRemoved,
			p.runtime,
			id.String(),
			round,
		)
		return nil
	}
	var metadataName *string
	if app.Metadata != nil {
		if name, ok := app.Metadata["net.oasis.rofl.name"]; ok {
			metadataName = &name
		}
	}
	sek := base64.StdEncoding.EncodeToString(app.SEK[:]) // x25519.PublicKey doesn't implement String() method, this matches other public keys string marshalling.
	batch.Queue(
		queries.RuntimeRoflAppUpdate,
		p.runtime,
		app.ID.String(),
		app.Admin.String(),
		app.Stake.Amount,
		app.Policy,
		sek,
		app.Metadata,
		metadataName,
		app.Secrets,
		round,
	)
	return nil
}

func (p *processor) queueRoflAppInstancesRefresh(ctx context.Context, batch *storage.QueryBatch, round uint64, id nodeapi.AppID) error {
	instances, err := p.source.RoflAppInstances(ctx, round, id)
	if err != nil {
		return err
	}
	for _, instance := range instances {
		var entityID *string
		if instance.EntityID != nil {
			entityID = common.Ptr(instance.EntityID.String())
		}
		var extraKeys []string
		for _, key := range instance.ExtraKeys {
			// Extra keys can be Ed25519, Secp256k1, or Sr25519.
			// Store them as json to keep type information.
			jsonKey, err := key.MarshalJSON()
			if err != nil {
				return fmt.Errorf("marshalling extra key: %w", err)
			}
			extraKeys = append(extraKeys, string(jsonKey))
		}
		rek := base64.StdEncoding.EncodeToString(instance.REK[:]) // x25519.PublicKey doesn't implement String() method, this matches other public keys string marshalling.
		batch.Queue(
			queries.RuntimeRoflInstanceUpsert,
			p.runtime,
			instance.App.String(),
			instance.RAK.String(),
			instance.NodeID.String(),
			entityID,
			rek,
			instance.Expiration,
			instance.Metadata,
			extraKeys,
			// In case the ROFL analyzer would be lagging behind, this round is not necessary the registration round,
			// since there could have been more events in the meantime. This means we might miss some early instance
			// transactions, since we will have the wrong registration round in the DB and the rofl_instance_transactions
			// analyzer relies on it.
			round,
			round-1,
		)
	}
	return nil
}
