// Package instancetransactions implements an analyzer that extracts
// transactions submitted by ROFL instances.
package instancetransactions

import (
	"context"
	"encoding/base64"
	"fmt"
	"reflect"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/ed25519"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/secp256k1"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/crypto/signature/sr25519"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer"
	"github.com/oasisprotocol/nexus/analyzer/item"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/config"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	analyzerPrefix = "rofl_instance_transactions_"
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
	AppID     string
	RAK       string
	ExtraKeys []string

	LastProcessedRound uint64
}

var _ item.ItemProcessor[*Item] = (*processor)(nil)

func (p *processor) GetItems(ctx context.Context, limit uint64) ([]*Item, error) {
	// Get all instances which are not expired, or are expired and were not yet processed.
	rows, err := p.target.Query(ctx, RuntimeRoflInstancesToProcess, p.runtime, limit)
	if err != nil {
		return nil, fmt.Errorf("querying stale rofl apps: %w", err)
	}
	defer rows.Close()

	var items []*Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.AppID, &item.RAK, &item.ExtraKeys, &item.LastProcessedRound); err != nil {
			return nil, fmt.Errorf("scanning stale rofl app: %w", err)
		}
		items = append(items, &item)
	}
	return items, nil
}

func (p *processor) ProcessItem(ctx context.Context, batch *storage.QueryBatch, item *Item) error {
	instanceAddresses := []string{}
	// Compute address from RAK.
	func() {
		// RAK is an Ed25519 key.
		var rak ed25519.PublicKey
		brak, err := base64.StdEncoding.DecodeString(item.RAK)
		if err != nil {
			p.logger.Error("failed to decode RAK, skipping key", "key", item.RAK, "error", err)
			return
		}
		if err := rak.UnmarshalBinary(brak); err != nil {
			p.logger.Error("failed to unmarshal RAK, skipping key", "key", item.RAK, "error", err)
			return
		}
		rakAddr := sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecEd25519(rak))
		instanceAddresses = append(instanceAddresses, rakAddr.String())
	}()

	// Compute addresses from extra keys.
	for _, ek := range item.ExtraKeys {
		var key sdkTypes.PublicKey
		if err := key.UnmarshalJSON([]byte(ek)); err != nil {
			p.logger.Error("failed to unmarshal extra key, skipping", "key", ek, "error", err)
			continue
		}
		// XXX: Should probably implement this in oasis-sdk and expose it.
		var extraAddress sdkTypes.Address
		switch pk := key.PublicKey.(type) {
		case ed25519.PublicKey:
			extraAddress = sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecEd25519(pk))
		case *ed25519.PublicKey:
			extraAddress = sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecEd25519(*pk))
		case secp256k1.PublicKey:
			extraAddress = sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecSecp256k1Eth(pk))
		case *secp256k1.PublicKey:
			extraAddress = sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecSecp256k1Eth(*pk))
		case sr25519.PublicKey:
			extraAddress = sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecSr25519(pk))
		case *sr25519.PublicKey:
			extraAddress = sdkTypes.NewAddress(sdkTypes.NewSignatureAddressSpecSr25519(*pk))
		default:
			p.logger.Error("unsupported extra key type", "key", ek, "type", reflect.TypeOf(pk))
			continue
		}
		instanceAddresses = append(instanceAddresses, extraAddress.String())
	}

	// Query the latest round here to use it.
	var latestRound uint64
	if err := p.target.QueryRow(ctx, RuntimeLatestRound, p.runtime).Scan(&latestRound); err != nil {
		return fmt.Errorf("querying latest round: %w", err)
	}

	// Find transactions that were signed by any of these addresses.
	rows, err := p.target.Query(
		ctx,
		RuntimeRoflInstanceTransactions,
		p.runtime,
		instanceAddresses,
		item.LastProcessedRound,
		latestRound,
	)
	if err != nil {
		return fmt.Errorf("querying rofl instance transactions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var round uint64
		var txIndex uint32
		var method string
		var isNativeTransfer bool
		if err := rows.Scan(&round, &txIndex, &method, &isNativeTransfer); err != nil {
			return fmt.Errorf("scanning rofl instance transaction: %w", err)
		}

		// Insert found transactions into the database.
		batch.Queue(
			RoflInstanceTransactionsUpsert,
			p.runtime,
			item.AppID,
			item.RAK,
			round,
			txIndex,
			method,
			isNativeTransfer,
		)
	}

	// Update last processed round.
	batch.Queue(
		RoflInstanceUpdateLastProcessedRound,
		p.runtime,
		item.AppID,
		item.RAK,
		latestRound,
	)

	return nil
}

func (p *processor) QueueLength(ctx context.Context) (int, error) {
	var count int
	if err := p.target.QueryRow(ctx, RuntimeRoflInstancesToProcessCount, p.runtime).Scan(&count); err != nil {
		return 0, fmt.Errorf("querying stale rofl apps: %w", err)
	}
	return count, nil
}
