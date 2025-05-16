package client

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/dgraph-io/ristretto"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"

	"github.com/oasisprotocol/oasis-core/go/common/crypto/hash"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
	oasisErrors "github.com/oasisprotocol/oasis-core/go/common/errors"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	sdkTypes "github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"

	"github.com/oasisprotocol/nexus/analyzer/evmabi"
	"github.com/oasisprotocol/nexus/config"
	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	"github.com/oasisprotocol/nexus/analyzer/util"
	apiCommon "github.com/oasisprotocol/nexus/api"
	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	common "github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/client/queries"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const (
	blockCost = 1

	maxTotalCount = 1000
)

// StorageClient is a wrapper around a storage.TargetStorage
// with knowledge of network semantics.
type StorageClient struct {
	sourceCfg      config.SourceConfig
	db             storage.TargetStorage
	referenceSwaps map[common.Runtime]config.ReferenceSwap
	runtimeClients map[common.Runtime]nodeapi.RuntimeApiLite
	networkConfig  *oasisConfig.Network

	evmTokensCustomOrderAddresses map[common.Runtime][]*apiTypes.StakingAddress
	evmTokensCustomOrderGroups    map[common.Runtime][]int

	disableCirculatingSupply    bool
	circulatingSupplyExclusions []apiTypes.Address

	blockCache *ristretto.Cache[int64, *Block]

	logger *log.Logger
}

func translateTokenType(tokenType common.TokenType) apiTypes.EvmTokenType {
	switch tokenType {
	case common.TokenTypeERC20:
		return apiTypes.EvmTokenTypeERC20
	case common.TokenTypeERC721:
		return apiTypes.EvmTokenTypeERC721
	default:
		return "unexpected_other_type"
	}
}

// The apiTypes Layers may be named differently from Nexus-internal Layers
// to make the api more ergonomic.
func translateLayer(layer apiTypes.Layer) common.Layer {
	switch layer {
	case apiTypes.LayerConsensus:
		return common.LayerConsensus
	case apiTypes.LayerCipher:
		return common.LayerCipher
	case apiTypes.LayerEmerald:
		return common.LayerEmerald
	case apiTypes.LayerSapphire:
		return common.LayerSapphire
	case apiTypes.LayerPontusxtest:
		return common.LayerPontusxTest
	case apiTypes.LayerPontusxdev:
		return common.LayerPontusxDev
	default:
		return "unexpected_layer"
	}
}

type rowsWithCount struct {
	rows                pgx.Rows
	totalCount          uint64
	isTotalCountClipped bool
}

// NewStorageClient creates a new storage client.
func NewStorageClient(
	cfg *config.ServerConfig,
	db storage.TargetStorage,
	referenceSwaps map[common.Runtime]config.ReferenceSwap,
	runtimeClients map[common.Runtime]nodeapi.RuntimeApiLite,
	networkConfig *oasisConfig.Network,
	l *log.Logger,
) (*StorageClient, error) {
	// The API currently uses an in-memory block cache for a specific endpoint and no other cases.
	// This somewhat arbitrary choice seems to have been made historically.
	// Instead, we should review common queries and responses to implement a more consistent and general caching strategy.
	// https://github.com/oasisprotocol/nexus/issues/887
	blockCache, err := ristretto.NewCache(&ristretto.Config[int64, *Block]{
		NumCounters:        1024 * 10,
		MaxCost:            1024,
		BufferItems:        64,
		IgnoreInternalCost: true,
	})
	if err != nil {
		l.Error("api client: failed to create block cache: %w", err)
		return nil, err
	}

	// Parse the provided custom EVM token ordering config into a suitable format for the EVM token query.
	// Per runtime list of token addresses.
	evmTokensCustomOrderAddresses := make(map[common.Runtime][]*apiTypes.StakingAddress)
	// Per runtime order weight of each token address.
	evmTokensCustomOrderGroups := make(map[common.Runtime][]int)
	for rt, tokenGroups := range cfg.EVMTokensCustomOrdering {
		var customOrderAddresses []*apiTypes.StakingAddress
		var customOrderGroups []int
		for i, tokenGroup := range tokenGroups {
			for _, tokenAddr := range tokenGroup {
				addr, err := apiTypes.UnmarshalToOcAddress(&tokenAddr)
				if err != nil {
					return nil, fmt.Errorf("provided custom EVM token ordering config is malformed, runtime: %s, token group: %d, token address: %s: %w", rt, i, tokenAddr, err)
				}
				customOrderAddresses = append(customOrderAddresses, addr)
				customOrderGroups = append(customOrderGroups, i)
			}
		}
		evmTokensCustomOrderAddresses[rt] = customOrderAddresses
		evmTokensCustomOrderGroups[rt] = customOrderGroups
	}

	// Prepare the list of reserved addresses for the circulating supply query.
	circulatingSupplyExclusions := make([]string, 0, len(cfg.ConsensusCirculatingSupplyExclusions))
	var foundCommonPool bool
	for _, address := range cfg.ConsensusCirculatingSupplyExclusions {
		if address == common.ConsensusPoolAddress.String() {
			foundCommonPool = true
		}
		circulatingSupplyExclusions = append(circulatingSupplyExclusions, address)
	}
	if !foundCommonPool {
		// Include the consensus common pool address in the list of reserved address, if it's not already present.
		circulatingSupplyExclusions = append(circulatingSupplyExclusions, common.ConsensusPoolAddress.String())
	}

	return &StorageClient{
		*cfg.Source,
		db,
		referenceSwaps,
		runtimeClients,
		networkConfig,
		evmTokensCustomOrderAddresses,
		evmTokensCustomOrderGroups,
		cfg.DisableCirculatingSupplyEndpoint,
		circulatingSupplyExclusions,
		blockCache,
		l,
	}, nil
}

// Shutdown closes the backing TargetStorage.
func (c *StorageClient) Shutdown() {
	c.db.Close()
}

// Returns the native token symbol of the specified runtime in the network
// specified by the networkConfig.
func (c *StorageClient) nativeTokenSymbol(runtime common.Runtime) string {
	if c.networkConfig == nil {
		c.logger.Warn("no network config available; unable to determine native token symbol", "runtime", runtime)
		return ""
	} else if c.networkConfig.ParaTimes.All[string(runtime)] == nil || c.networkConfig.ParaTimes.All[string(runtime)].Denominations[oasisConfig.NativeDenominationKey] == nil {
		c.logger.Warn("unknown runtime or denomination", "runtime", runtime, "denomination", oasisConfig.NativeDenominationKey)
		return ""
	}
	return c.networkConfig.ParaTimes.All[string(runtime)].Denominations[oasisConfig.NativeDenominationKey].Symbol
}

func (c *StorageClient) tokenDecimals(runtime common.Runtime, denom string) int {
	if c.networkConfig == nil {
		c.logger.Warn("no network config available; unable to determine native token decimals", "runtime", runtime)
		return 0
	} else if c.networkConfig.ParaTimes.All[string(runtime)] == nil || c.networkConfig.ParaTimes.All[string(runtime)].Denominations[denom] == nil {
		c.logger.Warn("unknown runtime denomination", "runtime", runtime, "denomination", denom)
		return 0
	}
	return int(c.networkConfig.ParaTimes.All[string(runtime)].Denominations[denom].Decimals)
}

func (c *StorageClient) baseToTokenUnits(baseUnits common.BigInt) (*common.BigInt, error) {
	if c.networkConfig == nil {
		return nil, fmt.Errorf("no network config available")
	}
	tenPow := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(c.networkConfig.Denomination.Decimals)), nil)
	if tenPow.Cmp(big.NewInt(0)) <= 0 {
		return nil, fmt.Errorf("invalid network config (denomination decimals: %d)", c.networkConfig.Denomination.Decimals)
	}
	tokenUnits := baseUnits.Div(common.BigInt{Int: *tenPow})
	return &tokenUnits, nil
}

// Wraps an error into one of the error types defined by the `common` package, if applicable.
func wrapError(err error) error {
	if err == pgx.ErrNoRows {
		return apiCommon.ErrNotFound
	}
	return apiCommon.ErrStorageError{Err: err}
}

// For queries that return multiple rows, returns the rows for a given query, as well as
// the total count of matching records, i.e. the number of rows the query would return
// with limit=infinity.
// Assumes that the last two query parameters are limit and offset.
// The total count is capped by an internal limit for performance reasons.
func (c *StorageClient) withTotalCount(ctx context.Context, sql string, args ...interface{}) (*rowsWithCount, error) {
	var totalCount uint64
	if len(args) < 2 {
		return nil, fmt.Errorf("list queries must have at least two params (limit and offset)")
	}

	// A note on ordering: We query the totalCount before querying for the rows in order to
	// avoid deadlocks. The row returned by the totalCount query is `Scan`-ed immediately
	// and thus the underlying db connection is also released. However, the `rows` are
	// `Scan`-ed in the calling function, which means that the underlying db connection is
	// held (and unavailable to other goroutines) in the meantime.
	origLimit := args[len(args)-2]
	// Temporarily set limit to just high enough to learn
	// if there are >maxTotalCount matching items in the DB.
	args[len(args)-2] = maxTotalCount + 1
	if err := c.db.QueryRow(
		ctx,
		queries.TotalCountQuery(sql),
		args...,
	).Scan(&totalCount); err != nil {
		return nil, wrapError(err)
	}
	clipped := totalCount == maxTotalCount+1
	if clipped {
		totalCount = maxTotalCount
	}

	args[len(args)-2] = origLimit
	rows, err := c.db.Query(
		ctx,
		sql,
		args...,
	)
	if err != nil {
		return nil, err
	}

	return &rowsWithCount{
		rows:                rows,
		totalCount:          totalCount,
		isTotalCountClipped: clipped,
	}, nil
}

// Status returns status information for Oasis Nexus.
func (c *StorageClient) Status(ctx context.Context) (*Status, error) {
	var s Status
	var latestBlockUpdate time.Time
	err := c.db.QueryRow(
		ctx,
		queries.Status,
		"consensus",
	).Scan(&s.LatestBlock, &latestBlockUpdate)
	switch err {
	case nil:
	case pgx.ErrNoRows:
		s.LatestBlock = -1
	default:
		return nil, wrapError(err)
	}
	// Calculate the elapsed time since the last block was processed. We assume that the analyzer and api server
	// are running on VMs with synced clocks.
	s.LatestUpdateAgeMs = time.Since(latestBlockUpdate).Milliseconds()

	// Query latest indexed block for info.
	err = c.db.QueryRow(
		ctx,
		queries.Blocks,
		s.LatestBlock,
		s.LatestBlock,
		nil,
		nil,
		nil,
		nil,
		1,
		0,
	).Scan(nil, nil, &s.LatestBlockTime, nil, nil, nil, nil, nil, nil, nil)
	switch err {
	case nil:
	case pgx.ErrNoRows:
		s.LatestBlockTime = time.Time{}
	default:
		return nil, wrapError(err)
	}

	// Fetch latest node height.
	err = c.db.QueryRow(
		ctx,
		queries.NodeHeight,
		"consensus",
	).Scan(&s.LatestNodeBlock)
	switch err {
	case nil:
		// Current node height is fetched in a separate goroutine, so it's possible for it
		// to be more out of date than the height of the most recently processed block.
		if s.LatestNodeBlock < s.LatestBlock {
			s.LatestNodeBlock = s.LatestBlock
		}
	case pgx.ErrNoRows:
		s.LatestNodeBlock = -1
	default:
		return nil, wrapError(err)
	}

	return &s, nil
}

func (c *StorageClient) TotalSupply(ctx context.Context) (*common.BigInt, error) {
	var totalSupply common.BigInt
	err := c.db.QueryRow(
		ctx,
		queries.TotalSupply,
	).Scan(&totalSupply)
	if err != nil {
		return nil, wrapError(err)
	}

	// The endpoint returns supply in token units.
	tokenUnits, err := c.baseToTokenUnits(totalSupply)
	if err != nil {
		return nil, wrapError(err)
	}
	return tokenUnits, nil
}

func (c *StorageClient) CirculatingSupply(ctx context.Context) (*common.BigInt, error) {
	if c.disableCirculatingSupply {
		return nil, apiCommon.ErrUnavailable
	}

	var totalSupply *common.BigInt
	err := c.db.QueryRow(
		ctx,
		queries.TotalSupply,
	).Scan(&totalSupply)
	if err != nil {
		return nil, wrapError(err)
	}
	if totalSupply == nil {
		return nil, wrapError(fmt.Errorf("total supply not available"))
	}

	// Subtract the balances of the provided addresses from the total supply.
	var subtractAmount *common.BigInt
	err = c.db.QueryRow(
		ctx,
		queries.AddressesTotalBalance,
		c.circulatingSupplyExclusions,
	).Scan(&subtractAmount)
	if err != nil {
		return nil, wrapError(err)
	}
	circulatingSupply := totalSupply.Minus(*subtractAmount)

	// The endpoint returns supply in token units.
	tokenUnits, err := c.baseToTokenUnits(circulatingSupply)
	if err != nil {
		return nil, wrapError(err)
	}
	return tokenUnits, nil
}

type entityInfoRow struct {
	EntityID       *string
	EntityAddress  *string
	EntityMetadata *json.RawMessage
}

func entityInfoFromRow(r entityInfoRow) apiTypes.EntityInfo {
	var entityMetadataAny any
	if r.EntityMetadata != nil {
		entityMetadataAny = *r.EntityMetadata
	}
	return apiTypes.EntityInfo{
		EntityAddress:  r.EntityAddress,
		EntityId:       r.EntityID,
		EntityMetadata: &entityMetadataAny,
	}
}

// Blocks returns a list of consensus blocks.
func (c *StorageClient) Blocks(ctx context.Context, r apiTypes.GetConsensusBlocksParams, height *int64) (*BlockList, error) {
	if height != nil {
		// Querying a single block by height, check cache.
		// XXX: This cache is somewhat arbitrary and likely not very useful in practice.
		// It has been kept for now to avoid regressions: https://github.com/oasisprotocol/nexus/issues/887
		block, ok := c.blockCache.Get(*height)
		if ok {
			return &BlockList{
				Blocks: []Block{*block},
			}, nil
		}

		// Otherwise continue with the query below.
		r.From = height
		r.To = height
		r.Limit = common.Ptr(uint64(1))
		r.Offset = common.Ptr(uint64(0))
	}

	hash, err := canonicalizedHash(r.Hash)
	if err != nil {
		return nil, wrapError(err)
	}
	res, err := c.withTotalCount(
		ctx,
		queries.Blocks,
		r.From,
		r.To,
		r.After,
		r.Before,
		hash,
		r.ProposedBy,
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	bs := BlockList{
		Blocks:              []Block{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var b Block
		var proposerRow entityInfoRow
		var signerRows []entityInfoRow
		if err := res.rows.Scan(
			&b.Height,
			&b.Hash,
			&b.Timestamp,
			&b.NumTransactions,
			&b.GasLimit,
			&b.SizeLimit,
			&b.Epoch,
			&b.StateRoot,
			&proposerRow,
			&signerRows,
		); err != nil {
			return nil, wrapError(err)
		}
		b.Timestamp = b.Timestamp.UTC()
		proposer := entityInfoFromRow(proposerRow)
		b.Proposer = proposer
		signers := make([]apiTypes.EntityInfo, 0, len(signerRows))
		for _, signerRow := range signerRows {
			signer := entityInfoFromRow(signerRow)
			signers = append(signers, signer)
		}
		b.Signers = &signers

		bs.Blocks = append(bs.Blocks, b)
	}

	// Cache the block if we queried a single block.
	if height != nil && len(bs.Blocks) > 0 {
		c.blockCache.Set(*height, &bs.Blocks[0], blockCost)
	}

	return &bs, nil
}

func canonicalizedHash(input *string) (*string, error) {
	if input == nil {
		return nil, nil
	}
	sanitized, _ := strings.CutPrefix(*input, "0x")
	var h hash.Hash
	if err := h.UnmarshalHex(sanitized); err != nil {
		return nil, fmt.Errorf("invalid hash: %w", err)
	}
	s := h.String()
	return &s, nil
}

// Transactions returns a list of consensus transactions.
func (c *StorageClient) Transactions(ctx context.Context, p apiTypes.GetConsensusTransactionsParams, txHash *string) (*TransactionList, error) {
	if p.Rel != nil && p.Sender != nil {
		return nil, fmt.Errorf("cannot filter on both related account and sender")
	}
	if p.Rel != nil && (p.After != nil || p.Before != nil) {
		return nil, fmt.Errorf("cannot use after/before with related transactions")
	}
	if p.Rel != nil && txHash != nil {
		return nil, fmt.Errorf("cannot use tx_hash with related transactions")
	}

	// Decide on the query to use, based on whether we are filtering on related account.
	transactionsQuery := queries.TransactionsNoRelated
	var addr *string
	if p.Sender != nil {
		addr = common.Ptr(p.Sender.String())
	}
	if p.Rel != nil {
		transactionsQuery = queries.TransactionsWithRelated
		addr = p.Rel
	}
	res, err := c.withTotalCount(
		ctx,
		transactionsQuery,
		txHash, // used for /consensus/transactions/{tx_hash}.
		p.Block,
		p.Method,
		addr,
		p.After,
		p.Before,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := TransactionList{
		Transactions:        []Transaction{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var t Transaction
		var code uint32
		var module *string
		var message *string
		if err := res.rows.Scan(
			&t.Block,
			&t.Index,
			&t.Hash,
			&t.Sender,
			&t.Nonce,
			&t.Fee,
			&t.GasLimit,
			&t.Method,
			&t.Body,
			&code,
			&module,
			&message,
			&t.Timestamp,
		); err != nil {
			return nil, wrapError(err)
		}
		if code == oasisErrors.CodeNoError {
			t.Success = true
		} else {
			t.Error = &apiTypes.TxError{
				Code:    code,
				Module:  module,
				Message: message,
			}
		}

		ts.Transactions = append(ts.Transactions, t)
	}

	return &ts, nil
}

// Events returns a list of events.
func (c *StorageClient) Events(ctx context.Context, p apiTypes.GetConsensusEventsParams) (*EventList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Events,
		p.Block,
		p.TxIndex,
		p.TxHash,
		p.Type,
		p.Rel,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	es := EventList{
		Events:              []Event{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}

	for res.rows.Next() {
		var e Event
		if err := res.rows.Scan(
			&e.Block,
			&e.TxIndex,
			&e.TxHash,
			&e.RoothashRuntimeId,
			&e.RoothashRuntime,
			&e.RoothashRuntimeRound,
			&e.Type,
			&e.Body,
			&e.Timestamp,
		); err != nil {
			return nil, wrapError(err)
		}
		es.Events = append(es.Events, e)
	}

	return &es, nil
}

func (c *StorageClient) RoothashMessages(ctx context.Context, p apiTypes.GetConsensusRoothashMessagesParams) (*apiTypes.RoothashMessageList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RoothashMessages,
		p.Runtime,
		p.Round,
		p.Type,
		p.Rel,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ms := RoothashMessageList{
		RoothashMessages:    []RoothashMessage{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}

	for res.rows.Next() {
		var m RoothashMessage
		var resultCBOR *[]byte
		if err := res.rows.Scan(
			&m.Runtime,
			&m.Round,
			&m.Index,
			&m.Type,
			&m.Body,
			&m.ErrorModule,
			&m.ErrorCode,
			&resultCBOR,
		); err != nil {
			return nil, wrapError(err)
		}
		if resultCBOR != nil && m.Type != nil {
			result, err := extractMessageResult(*resultCBOR, *m.Type)
			if err != nil {
				return nil, wrapError(err)
			}
			m.Result = &result
		}
		ms.RoothashMessages = append(ms.RoothashMessages, m)
	}

	return &ms, nil
}

// Entities returns a list of registered entities.
func (c *StorageClient) Entities(ctx context.Context, p apiTypes.GetConsensusEntitiesParams) (*EntityList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Entities,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	es := EntityList{
		Entities:            []Entity{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var e Entity
		if err := res.rows.Scan(&e.ID, &e.Address); err != nil {
			return nil, wrapError(err)
		}

		es.Entities = append(es.Entities, e)
	}

	return &es, nil
}

// Entity returns a registered entity.
func (c *StorageClient) Entity(ctx context.Context, address staking.Address) (*Entity, error) {
	var e Entity
	if err := c.db.QueryRow(
		ctx,
		queries.Entity,
		address.String(),
	).Scan(&e.ID, &e.Address); err != nil {
		return nil, wrapError(err)
	}

	nodeRows, err := c.db.Query(
		ctx,
		queries.EntityNodeIds,
		address.String(),
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var nid string
		if err := nodeRows.Scan(&nid); err != nil {
			return nil, wrapError(err)
		}

		e.Nodes = append(e.Nodes, nid)
	}

	return &e, nil
}

// EntityNodes returns a list of nodes controlled by the provided entity.
func (c *StorageClient) EntityNodes(ctx context.Context, address staking.Address, r apiTypes.GetConsensusEntitiesAddressNodesParams) (*NodeList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.EntityNodes,
		address.String(),
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ns := NodeList{
		Nodes:               []Node{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var n Node
		if err := res.rows.Scan(
			&n.ID,
			&n.EntityID,
			&n.Expiration,
			&n.TLSPubkey,
			&n.TLSNextPubkey,
			&n.P2PPubkey,
			&n.ConsensusPubkey,
			&n.Roles,
		); err != nil {
			return nil, wrapError(err)
		}

		ns.Nodes = append(ns.Nodes, n)
	}

	if err := c.db.QueryRow(
		ctx,
		queries.Entity,
		address.String(),
	).Scan(&ns.EntityID, nil); err != nil {
		return nil, wrapError(err)
	}

	return &ns, nil
}

// EntityNode returns a node controlled by the provided entity.
func (c *StorageClient) EntityNode(ctx context.Context, entityAddress staking.Address, nodeID signature.PublicKey) (*Node, error) {
	var n Node
	if err := c.db.QueryRow(
		ctx,
		queries.EntityNode,
		entityAddress.String(),
		nodeID.String(),
	).Scan(
		&n.ID,
		&n.EntityID,
		&n.Expiration,
		&n.TLSPubkey,
		&n.TLSNextPubkey,
		&n.P2PPubkey,
		&n.ConsensusPubkey,
		&n.Roles,
	); err != nil {
		return nil, wrapError(err)
	}

	return &n, nil
}

// Accounts returns a list of consensus accounts.
func (c *StorageClient) Accounts(ctx context.Context, r apiTypes.GetConsensusAccountsParams) (*AccountList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Accounts,
		r.Limit,
		r.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	as := AccountList{
		Accounts:            []Account{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		a := Account{}
		if err = res.rows.Scan(
			&a.Address,
			&a.Nonce,
			&a.Available,
			&a.Escrow,
			&a.Debonding,
			&a.DelegationsBalance,
			&a.DebondingDelegationsBalance,
			&a.FirstActivity,
		); err != nil {
			return nil, wrapError(err)
		}
		as.Accounts = append(as.Accounts, a)
	}

	return &as, nil
}

// Account returns a consensus account.
func (c *StorageClient) Account(ctx context.Context, address staking.Address) (*Account, error) {
	// Get basic account info.
	a := Account{
		// Initialize optional fields to empty values to avoid null pointer dereferences
		// when filling them from the database.
		Allowances: []Allowance{},
	}
	err := c.db.QueryRow(
		ctx,
		queries.Account,
		address.String(),
	).Scan(
		&a.Address,
		&a.Nonce,
		&a.Available,
		&a.Escrow,
		&a.Debonding,
		&a.DelegationsBalance,
		&a.DebondingDelegationsBalance,
		&a.Stats.NumTxns,
		&a.FirstActivity,
	)
	switch {
	case err == nil:
		// Continues below.
	case err == pgx.ErrNoRows:
		// An address can have no entry in the `accounts` table, which means no analyzer
		// has seen any activity for this address before. However, the address itself is
		// still valid, with 0 balance. We rely on type-checking of the input `address` to
		// ensure that we do not return these responses for malformed oasis addresses.
		a.Address = address.String()
		// If we have no entry in the accounts table, the stats below is likely also empty,
		// so we can return early here.
		return &a, nil
	default:
		return nil, wrapError(err)
	}

	// Get allowances.
	allowanceRows, queryErr := c.db.Query(
		ctx,
		queries.AccountAllowances,
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(queryErr)
	}
	defer allowanceRows.Close()

	for allowanceRows.Next() {
		var al Allowance
		if err2 := allowanceRows.Scan(
			&al.Address,
			&al.Amount,
		); err2 != nil {
			return nil, wrapError(err2)
		}

		a.Allowances = append(a.Allowances, al)
	}

	return &a, nil
}

// Computes shares worth given total shares and total balance.
func amountFromShares(shares common.BigInt, totalShares common.BigInt, totalBalance common.BigInt) (common.BigInt, error) {
	if shares.IsZero() {
		return common.NewBigInt(0), nil
	}
	if totalShares.IsZero() {
		// Shouldn't happen unless there is an invalid DB state. Don't panic since this is exposed
		// in a public API.
		return common.NewBigInt(0), fmt.Errorf("total shares is zero")
	}

	amount := new(big.Int).Mul(&shares.Int, &totalBalance.Int)
	amount.Quo(amount, &totalShares.Int)
	return common.BigInt{Int: *amount}, nil
}

// Delegations returns a list of delegations.
func (c *StorageClient) Delegations(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDelegationsParams) (*DelegationList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Delegations,
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ds := DelegationList{
		Delegations:         []Delegation{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		d := Delegation{
			Delegator: address.String(),
		}
		var shares, escrowBalanceActive, escrowTotalSharesActive common.BigInt
		if err = res.rows.Scan(
			&d.Validator,
			&shares,
			&escrowBalanceActive,
			&escrowTotalSharesActive,
		); err != nil {
			return nil, wrapError(err)
		}
		d.Amount, err = amountFromShares(shares, escrowTotalSharesActive, escrowBalanceActive)
		if err != nil {
			c.logger.Error("failed to compute delegated amount from shares (inconsistent DB state?)",
				"delegator", address.String(),
				"delegatee", d.Validator,
				"shares", shares,
				"total_shares", escrowTotalSharesActive,
				"total_balance", escrowBalanceActive,
				"err", err,
			)
		}
		d.Shares = shares

		ds.Delegations = append(ds.Delegations, d)
	}

	return &ds, nil
}

// DelegationsTo returns a list of delegations to an address.
func (c *StorageClient) DelegationsTo(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDelegationsToParams) (*DelegationList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.DelegationsTo,
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ds := DelegationList{
		Delegations:         []Delegation{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		d := Delegation{
			Validator: address.String(),
		}
		var shares, escrowBalanceActive, escrowTotalSharesActive common.BigInt
		if err = res.rows.Scan(
			&d.Delegator,
			&shares,
			&escrowBalanceActive,
			&escrowTotalSharesActive,
		); err != nil {
			return nil, wrapError(err)
		}
		d.Amount, err = amountFromShares(shares, escrowTotalSharesActive, escrowBalanceActive)
		if err != nil {
			c.logger.Error("failed to compute delegated amount from shares (inconsistent DB state?)",
				"delegator", address.String(),
				"delegatee", d.Validator,
				"shares", shares,
				"total_shares", escrowTotalSharesActive,
				"total_balance", escrowBalanceActive,
				"err", err,
			)
		}
		d.Shares = shares

		ds.Delegations = append(ds.Delegations, d)
	}

	return &ds, nil
}

// DebondingDelegations returns a list of debonding delegations.
func (c *StorageClient) DebondingDelegations(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDebondingDelegationsParams) (*DebondingDelegationList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.DebondingDelegations,
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ds := DebondingDelegationList{
		DebondingDelegations: []DebondingDelegation{},
		TotalCount:           res.totalCount,
		IsTotalCountClipped:  res.isTotalCountClipped,
	}
	for res.rows.Next() {
		d := DebondingDelegation{
			Delegator: address.String(),
		}
		var shares, escrowBalanceDebonding, escrowTotalSharesDebonding common.BigInt
		if err = res.rows.Scan(
			&d.Validator,
			&shares,
			&d.DebondEnd,
			&escrowBalanceDebonding,
			&escrowTotalSharesDebonding,
		); err != nil {
			return nil, wrapError(err)
		}
		d.Amount, err = amountFromShares(shares, escrowTotalSharesDebonding, escrowBalanceDebonding)
		if err != nil {
			c.logger.Error("failed to compute debonding delegated amount from shares (inconsistent DB state?)",
				"delegator", address.String(),
				"delegatee", d.Validator,
				"shares", shares,
				"total_shares_debonding", escrowTotalSharesDebonding,
				"total_balance_debonding", escrowBalanceDebonding,
				"err", err,
			)
		}
		d.Shares = shares

		ds.DebondingDelegations = append(ds.DebondingDelegations, d)
	}

	return &ds, nil
}

// DebondingDelegationsTo returns a list of debonding delegations to an address.
func (c *StorageClient) DebondingDelegationsTo(ctx context.Context, address staking.Address, p apiTypes.GetConsensusAccountsAddressDebondingDelegationsToParams) (*DebondingDelegationList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.DebondingDelegationsTo,
		address.String(),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ds := DebondingDelegationList{
		DebondingDelegations: []DebondingDelegation{},
		TotalCount:           res.totalCount,
		IsTotalCountClipped:  res.isTotalCountClipped,
	}
	for res.rows.Next() {
		d := DebondingDelegation{
			Validator: address.String(),
		}
		var shares, escrowBalanceDebonding, escrowTotalSharesDebonding common.BigInt
		if err = res.rows.Scan(
			&d.Delegator,
			&shares,
			&d.DebondEnd,
			&escrowBalanceDebonding,
			&escrowTotalSharesDebonding,
		); err != nil {
			return nil, wrapError(err)
		}
		d.Amount, err = amountFromShares(shares, escrowTotalSharesDebonding, escrowBalanceDebonding)
		if err != nil {
			c.logger.Error("failed to compute debonding delegated amount from shares (inconsistent DB state?)",
				"delegator", address.String(),
				"delegatee", d.Validator,
				"shares", shares,
				"total_shares_debonding", escrowTotalSharesDebonding,
				"total_balance_debonding", escrowBalanceDebonding,
				"err", err,
			)
		}
		d.Shares = shares

		ds.DebondingDelegations = append(ds.DebondingDelegations, d)
	}

	return &ds, nil
}

// Epochs returns a list of consensus epochs.
func (c *StorageClient) Epochs(ctx context.Context, p apiTypes.GetConsensusEpochsParams) (*EpochList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Epochs,
		nil,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}

	es := EpochList{
		Epochs:              []Epoch{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var e Epoch
		if err := res.rows.Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
			return nil, wrapError(err)
		}

		es.Epochs = append(es.Epochs, e)
	}

	return &es, nil
}

// Epoch returns a consensus epoch.
func (c *StorageClient) Epoch(ctx context.Context, epoch int64) (*Epoch, error) {
	var e Epoch
	if err := c.db.QueryRow(
		ctx,
		queries.Epochs,
		epoch,
		1,
		0,
	).Scan(&e.ID, &e.StartHeight, &e.EndHeight); err != nil {
		return nil, wrapError(err)
	}

	return &e, nil
}

// Proposals returns a list of governance proposals.
func (c *StorageClient) Proposals(ctx context.Context, p apiTypes.GetConsensusProposalsParams) (*ProposalList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.Proposals,
		p.Submitter,
		p.State,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ps := ProposalList{
		Proposals:           []Proposal{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		p := Proposal{Target: &ProposalTarget{}}
		var parametersChangeCBOR *[]byte
		if err := res.rows.Scan(
			&p.ID,
			&p.Submitter,
			&p.State,
			&p.Deposit,
			&p.Title,
			&p.Description,
			&p.Handler,
			&p.Target.ConsensusProtocol,
			&p.Target.RuntimeHostProtocol,
			&p.Target.RuntimeCommitteeProtocol,
			&p.Epoch,
			&p.Cancels,
			&p.ParametersChangeModule,
			&parametersChangeCBOR,
			&p.CreatedAt,
			&p.ClosesAt,
			&p.InvalidVotes,
		); err != nil {
			return nil, wrapError(err)
		}
		if parametersChangeCBOR != nil && p.ParametersChangeModule != nil {
			res, err := extractProposalParametersChange(*parametersChangeCBOR, *p.ParametersChangeModule)
			if err != nil {
				return nil, wrapError(err)
			}
			p.ParametersChange = &res
		}

		ps.Proposals = append(ps.Proposals, p)
	}

	return &ps, nil
}

// Proposal returns a governance proposal.
func (c *StorageClient) Proposal(ctx context.Context, proposalID uint64) (*Proposal, error) {
	p := Proposal{Target: &ProposalTarget{}}
	var parametersChangeCBOR *[]byte
	if err := c.db.QueryRow(
		ctx,
		queries.Proposal,
		proposalID,
	).Scan(
		&p.ID,
		&p.Submitter,
		&p.State,
		&p.Deposit,
		&p.Title,
		&p.Description,
		&p.Handler,
		&p.Target.ConsensusProtocol,
		&p.Target.RuntimeHostProtocol,
		&p.Target.RuntimeCommitteeProtocol,
		&p.Epoch,
		&p.Cancels,
		&p.ParametersChangeModule,
		&parametersChangeCBOR,
		&p.CreatedAt,
		&p.ClosesAt,
		&p.InvalidVotes,
	); err != nil {
		return nil, wrapError(err)
	}
	if parametersChangeCBOR != nil && p.ParametersChangeModule != nil {
		res, err := extractProposalParametersChange(*parametersChangeCBOR, *p.ParametersChangeModule)
		if err != nil {
			return nil, wrapError(err)
		}
		p.ParametersChange = &res
	}

	return &p, nil
}

// ProposalVotes returns votes for a governance proposal.
func (c *StorageClient) ProposalVotes(ctx context.Context, proposalID uint64, p apiTypes.GetConsensusProposalsProposalIdVotesParams) (*ProposalVotes, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.ProposalVotes,
		proposalID,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	vs := ProposalVotes{
		Votes:               []ProposalVote{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var v ProposalVote
		if err := res.rows.Scan(
			&v.Address,
			&v.Vote,
			&v.Height,
			&v.Timestamp,
		); err != nil {
			return nil, wrapError(err)
		}

		vs.Votes = append(vs.Votes, v)
	}
	vs.ProposalID = proposalID

	return &vs, nil
}

// Validators returns a list of validators, or optionally the single validator matching `address`.
func (c *StorageClient) Validators(ctx context.Context, p apiTypes.GetConsensusValidatorsParams, address *staking.Address) (*ValidatorList, error) {
	var epoch Epoch
	if err := c.db.QueryRow(
		ctx,
		queries.LatestEpochStart,
	).Scan(&epoch.ID, &epoch.StartHeight); err != nil {
		return nil, wrapError(err)
	}

	var stats ValidatorAggStats
	if err := c.db.QueryRow(
		ctx,
		queries.ValidatorsAggStats,
	).Scan(
		&stats.TotalVotingPower,
		&stats.TotalDelegators,
		&stats.TotalStakedBalance,
	); err != nil {
		return nil, wrapError(err)
	}

	res, err := c.withTotalCount(
		ctx,
		queries.ValidatorsData,
		address,
		p.Name,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	vs := ValidatorList{
		Validators:          []Validator{},
		Stats:               stats,
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		v := Validator{
			Escrow: apiTypes.Escrow{},
		}
		var schedule staking.CommissionSchedule
		var logoUrl *string
		if err = res.rows.Scan(
			&v.EntityID,
			&v.EntityAddress,
			&v.NodeID,
			&v.Escrow.ActiveBalance,
			&v.Escrow.ActiveShares,
			&v.Escrow.DebondingBalance,
			&v.Escrow.DebondingShares,
			&v.Escrow.SelfDelegationBalance,
			&v.Escrow.SelfDelegationShares,
			&v.Escrow.ActiveBalance24,
			&v.Escrow.NumDelegators,
			&v.VotingPower,
			&v.VotingPowerCumulative,
			&schedule,
			&v.StartDate,
			&v.Rank,
			&v.Active,
			&v.InValidatorSet,
			&v.Media,
			&logoUrl,
		); err != nil {
			return nil, wrapError(err)
		}

		if logoUrl != nil && *logoUrl != "" {
			if v.Media == nil {
				v.Media = &ValidatorMedia{}
			}
			v.Media.LogoUrl = logoUrl
		}
		currentRate := schedule.CurrentRate(beacon.EpochTime(epoch.ID))
		if currentRate != nil {
			v.CurrentRate = currentRate.ToBigInt().Uint64()
		}
		bound, next := util.CurrentBound(schedule, beacon.EpochTime(epoch.ID))
		if bound != nil {
			v.CurrentCommissionBound = ValidatorCommissionBound{
				Lower:      bound.RateMin.ToBigInt().Uint64(),
				Upper:      bound.RateMax.ToBigInt().Uint64(),
				EpochStart: uint64(bound.Start),
			}
		}

		if next > 0 {
			v.CurrentCommissionBound.EpochEnd = next
		}

		vs.Validators = append(vs.Validators, v)
	}

	// When querying for a single validator, include the detailed block sign data for last 100 blocks.
	if address != nil && len(vs.Validators) == 1 {
		rows, err := c.db.Query(ctx, queries.ValidatorLast100BlocksSigned, vs.Validators[0].EntityID)
		if err != nil {
			return nil, wrapError(err)
		}
		defer rows.Close()

		signedBlocks := []ValidatorSignedBlock{}
		for rows.Next() {
			var height int64
			var signed bool
			if err = rows.Scan(
				&height,
				&signed,
			); err != nil {
				return nil, wrapError(err)
			}
			signedBlocks = append(signedBlocks, ValidatorSignedBlock{Height: height, Signed: signed})
		}
		vs.Validators[0].SignedBlocks = &signedBlocks
	}

	return &vs, nil
}

func (c *StorageClient) ValidatorHistory(ctx context.Context, address staking.Address, p apiTypes.GetConsensusValidatorsAddressHistoryParams) (*ValidatorHistory, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.ValidatorHistory,
		address.String(),
		p.From,
		p.To,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	h := ValidatorHistory{
		History:             []ValidatorHistoryPoint{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		b := ValidatorHistoryPoint{}
		if err = res.rows.Scan(
			&b.Epoch,
			&b.ActiveBalance,
			&b.ActiveShares,
			&b.DebondingBalance,
			&b.DebondingShares,
			&b.NumDelegators,
		); err != nil {
			return nil, wrapError(err)
		}
		h.History = append(h.History, b)
	}

	return &h, nil
}

// RuntimeBlocks returns a list of runtime blocks.
func (c *StorageClient) RuntimeBlocks(ctx context.Context, runtime common.Runtime, p apiTypes.GetRuntimeBlocksParams) (*RuntimeBlockList, error) {
	hash, err := canonicalizedHash(p.Hash)
	if err != nil {
		return nil, wrapError(err)
	}
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeBlocks,
		runtime,
		p.From,
		p.To,
		p.After,
		p.Before,
		hash,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	bs := RuntimeBlockList{
		Blocks:              []RuntimeBlock{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var b RuntimeBlock
		if err := res.rows.Scan(&b.Round, &b.Hash, &b.Timestamp, &b.NumTransactions, &b.Size, &b.GasUsed); err != nil {
			return nil, wrapError(err)
		}
		b.Timestamp = b.Timestamp.UTC()

		bs.Blocks = append(bs.Blocks, b)
	}

	return &bs, nil
}

func EVMEthAddrFromPreimage(contextIdentifier string, contextVersion int, data []byte) ([]byte, error) {
	if contextIdentifier != sdkTypes.AddressV0Secp256k1EthContext.Identifier {
		return nil, fmt.Errorf("preimage context identifier %q, expecting %q", contextIdentifier, sdkTypes.AddressV0Secp256k1EthContext.Identifier)
	}
	if contextVersion != int(sdkTypes.AddressV0Secp256k1EthContext.Version) {
		return nil, fmt.Errorf("preimage context version %d, expecting %d", contextVersion, sdkTypes.AddressV0Secp256k1EthContext.Version)
	}
	return data, nil
}

// EthChecksumAddrFromPreimage gives the friendly Ethereum-style mixed-case
// checksum address (see ERC-55) for an address preimage or nil if the
// preimage context is not AddressV0Secp256k1EthContext.
func EthChecksumAddrFromPreimage(contextIdentifier string, contextVersion int, data []byte) *string {
	ethAddr, err := EVMEthAddrFromPreimage(contextIdentifier, contextVersion, data)
	if err != nil {
		// Ignore error about the preimage not being AddressV0Secp256k1EthContext.
		return nil
	}
	ethChecksumAddr := ethCommon.BytesToAddress(ethAddr).String()
	return &ethChecksumAddr
}

// EthChecksumAddrFromBarePreimage gives the friendly Ethereum-style
// mixed-case checksum address (see ERC-55) for an address preimage without
// checking the preimage context.
func EthChecksumAddrFromBarePreimage(data []byte) string {
	return ethCommon.BytesToAddress(data).String()
}

// EthChecksumAddrPtrFromBarePreimage gives the friendly Ethereum-style
// mixed-case checksum address (see ERC-55) for an address preimage without
// checking the preimage context. This one returns nil if the input was nil.
func EthChecksumAddrPtrFromBarePreimage(data []byte) *string {
	if data == nil {
		return nil
	}
	ethChecksumAddr := ethCommon.BytesToAddress(data).String()
	return &ethChecksumAddr
}

func runtimeTransactionFromRow(rows pgx.Rows, logger *log.Logger) (*RuntimeTransaction, error) {
	t := RuntimeTransaction{
		Error:   &TxError{},
		Signers: []apiTypes.RuntimeTransactionSigner{},
	}
	var oasisEncryptionEnvelope RuntimeTransactionEncryptionEnvelope
	var oasisEncryptionEnvelopeFormat *common.CallFormat
	var evmEncryptionEnvelope RuntimeTransactionEncryptionEnvelope
	var evmEncryptionEnvelopeFormat *common.CallFormat
	var signersAddresses []string
	var signersPreimageContextIdentifiers []*string
	var signersPreimageContextVersions []*int
	var signersPreimageData [][]byte
	var signersNonces []uint64
	var toPreimageContextIdentifier *string
	var toPreimageContextVersion *int
	var toPreimageData []byte
	var errorCode *uint32
	if err := rows.Scan(
		&t.Round,
		&t.Index,
		&t.Timestamp,
		&t.Hash,
		&t.EthHash,
		&signersAddresses,
		&signersPreimageContextIdentifiers,
		&signersPreimageContextVersions,
		&signersPreimageData,
		&signersNonces,
		&t.Fee,
		&t.FeeSymbol,
		&t.FeeProxyModule,
		&t.FeeProxyId,
		&t.GasLimit,
		&t.GasUsed,
		&t.ChargedFee,
		&t.Size,
		&oasisEncryptionEnvelopeFormat,
		&oasisEncryptionEnvelope.PublicKey,
		&oasisEncryptionEnvelope.DataNonce,
		&oasisEncryptionEnvelope.Data,
		&oasisEncryptionEnvelope.ResultNonce,
		&oasisEncryptionEnvelope.Result,
		&t.Method,
		&t.IsLikelyNativeTokenTransfer,
		&t.Body,
		&t.To,
		&toPreimageContextIdentifier,
		&toPreimageContextVersion,
		&toPreimageData,
		&t.Amount,
		&t.AmountSymbol,
		&evmEncryptionEnvelopeFormat,
		&evmEncryptionEnvelope.PublicKey,
		&evmEncryptionEnvelope.DataNonce,
		&evmEncryptionEnvelope.Data,
		&evmEncryptionEnvelope.ResultNonce,
		&evmEncryptionEnvelope.Result,
		&t.Success,
		&t.EvmFnName,
		&t.EvmFnParams,
		&t.Error.Module,
		&errorCode,
		&t.Error.Message,
		&t.Error.RawMessage,
		&t.Error.RevertParams,
	); err != nil {
		return nil, err
	}
	// If success field is unset (i.e. encrypted "Unknown" result) or
	// successful, some database versions have non-null error module/code
	// from when the analyzer would insert ""/0 instead. There's no error
	// information, so empty this stuff out.
	if t.Success == nil || *t.Success {
		t.Error = nil
	} else if errorCode != nil {
		t.Error.Code = *errorCode
	}
	if oasisEncryptionEnvelopeFormat != nil { // a rudimentary check to determine if the tx was encrypted
		oasisEncryptionEnvelope.Format = *oasisEncryptionEnvelopeFormat
		t.OasisEncryptionEnvelope = &oasisEncryptionEnvelope
	}
	if evmEncryptionEnvelopeFormat != nil { // a rudimentary check to determine if the tx was encrypted
		evmEncryptionEnvelope.Format = *evmEncryptionEnvelopeFormat
		t.EncryptionEnvelope = &evmEncryptionEnvelope
	}

	for i := range signersAddresses {
		t.Signers = append(t.Signers, apiTypes.RuntimeTransactionSigner{
			Address: signersAddresses[i],
			Nonce:   signersNonces[i],
		})
		// Render Ethereum-compatible address preimage.
		if signersPreimageContextIdentifiers[i] != nil && signersPreimageContextVersions[i] != nil {
			t.Signers[i].AddressEth = EthChecksumAddrFromPreimage(*signersPreimageContextIdentifiers[i], *signersPreimageContextVersions[i], signersPreimageData[i])
		}

		// Deprecated sender_0 fields.
		if i == 0 {
			t.Sender0 = t.Signers[0].Address
			t.Nonce0 = t.Signers[0].Nonce
			t.Sender0Eth = t.Signers[0].AddressEth
		}
	}

	// Render Ethereum-compatible address preimages.
	if toPreimageContextIdentifier != nil && toPreimageContextVersion != nil {
		t.ToEth = EthChecksumAddrFromPreimage(*toPreimageContextIdentifier, *toPreimageContextVersion, toPreimageData)
	}

	// Try extracting parsed PCS quote from rofl.Register transaction body.
	if t.Method != nil && *t.Method == "rofl.Register" {
		nb, err := extractPCSQuote(t.Body)
		if err != nil {
			logger.Warn("failed to extract PCS quote from rofl.Register transaction body", "tx_hash", t.Hash, "err", err)
			// In case of errors, original body is returned.
		}
		t.Body = nb
	}

	return &t, nil
}

// RuntimeTransactions returns a list of runtime transactions.
func (c *StorageClient) RuntimeTransactions(ctx context.Context, runtime common.Runtime, p apiTypes.GetRuntimeTransactionsParams, txHash *string) (*RuntimeTransactionList, error) {
	ocAddrRel, err := apiTypes.UnmarshalToOcAddress(p.Rel)
	if err != nil {
		return nil, err
	}
	if p.Rel != nil && (p.After != nil || p.Before != nil) {
		return nil, fmt.Errorf("cannot use after/before with related transactions")
	}
	if p.Rel != nil && txHash != nil {
		return nil, fmt.Errorf("cannot use tx_hash with related transactions")
	}

	query := queries.RuntimeTransactionsNoRelated
	if p.Rel != nil {
		query = queries.RuntimeTransactionsRelatedAddr
	}

	res, err := c.withTotalCount(
		ctx,
		query,
		runtime,
		p.Block,
		nil,
		txHash, // tx_hash; used only by GetRuntimeTransactionsTxHash
		ocAddrRel,
		nil,
		p.Method,
		p.After,
		p.Before,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := RuntimeTransactionList{
		Transactions:        []RuntimeTransaction{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		t, err := runtimeTransactionFromRow(res.rows, c.logger)
		if err != nil {
			c.logger.Error("error converting transaction to API response", "err", err)
			return nil, wrapError(err)
		}
		ts.Transactions = append(ts.Transactions, *t)
	}

	return &ts, nil
}

// RuntimeEvents returns a list of runtime events.
func (c *StorageClient) RuntimeEvents(ctx context.Context, runtime common.Runtime, p apiTypes.GetRuntimeEventsParams) (*RuntimeEventList, error) {
	var evmLogSignature *ethCommon.Hash
	if p.EvmLogSignature != nil {
		h := ethCommon.HexToHash(*p.EvmLogSignature)
		evmLogSignature = &h
	}

	// Validate query parameter constraints.
	// Due to DB indexes setup, other query combinations are inefficient and not supported.
	switch {
	case p.NftId != nil && p.ContractAddress == nil:
		return nil, fmt.Errorf("'nft_id' must be used with 'contract_address'")
	case p.ContractAddress != nil && p.NftId == nil && p.EvmLogSignature == nil:
		return nil, fmt.Errorf("'contract_address' must be used with either 'nft_id' or 'evm_log_signature'")
	default:
	}

	ocAddrContract, err := apiTypes.UnmarshalToOcAddress(p.ContractAddress)
	if err != nil {
		return nil, err
	}
	ocAddrRel, err := apiTypes.UnmarshalToOcAddress(p.Rel)
	if err != nil {
		return nil, err
	}
	var NFTIdB64 *string
	if p.NftId != nil {
		erc721TransferTokenIdBI := &big.Int{}
		if err2 := erc721TransferTokenIdBI.UnmarshalText([]byte(*p.NftId)); err2 != nil {
			return nil, fmt.Errorf("unmarshalling erc721_transfer_token_id: %w", err2)
		}
		erc721TransferTokenIdBuf, err2 := abi.Arguments{evmabi.ERC721.Events["Transfer"].Inputs[2]}.Pack(erc721TransferTokenIdBI)
		if err2 != nil {
			return nil, fmt.Errorf("ABI-packing erc721_transfer_token_id: %w", err2)
		}
		NFTIdB64 = common.Ptr(base64.StdEncoding.EncodeToString(erc721TransferTokenIdBuf))
	}

	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeEvents,
		runtime,
		p.Block,
		p.TxIndex,
		p.TxHash,
		p.Type,
		evmLogSignature,
		ocAddrRel,
		ocAddrContract,
		NFTIdB64,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	es := RuntimeEventList{
		Events:              []RuntimeEvent{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var e RuntimeEvent
		var et apiTypes.EvmEventToken
		var tokenType sql.NullInt32
		var fromPreimageContextIdentifier *string
		var fromPreimageContextVersion *int
		var fromPreimageData []byte
		var toPreimageContextIdentifier *string
		var toPreimageContextVersion *int
		var toPreimageData []byte
		var ownerPreimageContextIdentifier *string
		var ownerPreimageContextVersion *int
		var ownerPreimageData []byte
		if err := res.rows.Scan(
			&e.Round,
			&e.TxIndex,
			&e.TxHash,
			&e.EthTxHash,
			&e.Timestamp,
			&e.Type,
			&e.Body,
			&e.EvmLogName,
			&e.EvmLogParams,
			&et.Symbol,
			&tokenType,
			&et.Decimals,
			&fromPreimageContextIdentifier,
			&fromPreimageContextVersion,
			&fromPreimageData,
			&toPreimageContextIdentifier,
			&toPreimageContextVersion,
			&toPreimageData,
			&ownerPreimageContextIdentifier,
			&ownerPreimageContextVersion,
			&ownerPreimageData,
		); err != nil {
			return nil, wrapError(err)
		}
		if tokenType.Valid {
			et.Type = common.Ptr(translateTokenType(common.TokenType(tokenType.Int32)))
		}
		if et != (apiTypes.EvmEventToken{}) {
			e.EvmToken = &et
		}
		// Render Ethereum-compatible address preimages.
		// TODO: That's a little odd to do in the database layer. Move this farther
		// out if we have the energy.
		if fromPreimageContextIdentifier != nil && fromPreimageContextVersion != nil {
			if from_eth := EthChecksumAddrFromPreimage(*fromPreimageContextIdentifier, *fromPreimageContextVersion, fromPreimageData); from_eth != nil {
				e.Body["from_eth"] = from_eth
			}
		}
		if toPreimageContextIdentifier != nil && toPreimageContextVersion != nil {
			if to_eth := EthChecksumAddrFromPreimage(*toPreimageContextIdentifier, *toPreimageContextVersion, toPreimageData); to_eth != nil {
				e.Body["to_eth"] = to_eth
			}
		}
		if ownerPreimageContextIdentifier != nil && ownerPreimageContextVersion != nil {
			if owner_eth := EthChecksumAddrFromPreimage(*ownerPreimageContextIdentifier, *ownerPreimageContextVersion, ownerPreimageData); owner_eth != nil {
				e.Body["owner_eth"] = owner_eth
			}
		}
		es.Events = append(es.Events, e)
	}

	return &es, nil
}

// Fetch account balances from the node and pass them back via the channel. Note that if the input context
// times out or we encounter an error, we explicitly close the channel and return. This enables the calling
// function to create a context with a timeout and then listen to the channel with the expectation that the
// channel will return values and/or close approximately within the timeout.
func (c *StorageClient) fetchAccountBalancesFromNode(ctx context.Context, ch chan *RuntimeSdkBalance, runtime common.Runtime, address staking.Address) {
	var latestIndexedHeight uint64
	// Query latest block.
	err := c.db.QueryRow(
		ctx,
		queries.Status,
		runtime,
	).Scan(&latestIndexedHeight, nil)
	switch err {
	case nil:
	case pgx.ErrNoRows:
		// No runtime blocks indexed yet; return a 0 native balance.
		ch <- &RuntimeSdkBalance{
			Balance:       common.NewBigInt(0),
			TokenDecimals: c.tokenDecimals(runtime, oasisConfig.NativeDenominationKey),
			TokenSymbol:   c.nativeTokenSymbol(runtime),
		}
		close(ch)
		return
	default:
		c.logger.Warn("error fetching latest indexed height", "err", err, "runtime", runtime)
		close(ch)
		return
	}

	runtimeApi, ok := c.runtimeClients[runtime]
	if !ok {
		c.logger.Warn("no runtime api configured to fetch account balances", "runtime", runtime)
		close(ch)
		return
	}
	balances, err := runtimeApi.GetBalances(ctx, latestIndexedHeight, address)
	if err != nil {
		c.logger.Warn("failed to fetch balances from node", "address", address.String(), "runtime", runtime, "round", latestIndexedHeight, "err", err)
		close(ch)
		return
	}
	foundNativeBalance := false
	for denom, amount := range balances {
		var balance RuntimeSdkBalance
		if denom.IsNative() {
			foundNativeBalance = true
			// Assumption: we have a native token symbol for every runtime in runtimeClients.
			balance = RuntimeSdkBalance{
				Balance:       amount,
				TokenDecimals: c.tokenDecimals(runtime, oasisConfig.NativeDenominationKey),
				TokenSymbol:   c.nativeTokenSymbol(runtime),
			}
		} else {
			balance = RuntimeSdkBalance{
				Balance:       amount,
				TokenDecimals: c.tokenDecimals(runtime, denom.String()),
				TokenSymbol:   denom.String(),
			}
		}

		ch <- &balance
	}
	// Default to 0 native balance if not specified by the node.
	if !foundNativeBalance {
		ch <- &RuntimeSdkBalance{
			Balance:       common.NewBigInt(0),
			TokenDecimals: c.tokenDecimals(runtime, oasisConfig.NativeDenominationKey),
			TokenSymbol:   c.nativeTokenSymbol(runtime),
		}
	}
	close(ch)
}

func (c *StorageClient) RuntimeAccount(ctx context.Context, runtime common.Runtime, address staking.Address) (*RuntimeAccount, error) {
	a := RuntimeAccount{
		Address:         address.String(),
		AddressPreimage: &AddressPreimage{},
		Balances:        []RuntimeSdkBalance{},
		EvmBalances:     []RuntimeEvmBalance{},
	}
	nodeFetchCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	ch := make(chan *RuntimeSdkBalance)
	go c.fetchAccountBalancesFromNode(nodeFetchCtx, ch, runtime, address)

	var preimageContext string
	err := c.db.QueryRow(
		ctx,
		queries.AddressPreimage,
		address,
	).Scan(
		&preimageContext,
		&a.AddressPreimage.ContextVersion,
		&a.AddressPreimage.AddressData,
	)
	if err == nil { //nolint:gocritic
		a.AddressPreimage.Context = AddressDerivationContext(preimageContext)
	} else if err == pgx.ErrNoRows {
		// An address can have no entry in the address preimage table, which means no analyzer
		// has seen any activity for this address before. However, the address itself is
		// still valid, with 0 balance. We rely on type-checking of the input `address` to
		// ensure that we do not return these responses for malformed oasis addresses.
		a.Address = address.String()
		a.AddressPreimage = nil
	} else {
		return nil, wrapError(err)
	}

	// Get paratime balances.
	runtimeSdkRows, queryErr := c.db.Query(
		ctx,
		queries.AccountRuntimeSdkBalances,
		runtime,
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(queryErr)
	}
	defer runtimeSdkRows.Close()

	for runtimeSdkRows.Next() {
		b := RuntimeSdkBalance{
			// HACK: 18 is accurate for Emerald and Sapphire, but Cipher has 9.
			// Once we add a non-18-decimals runtime, we'll need to query the runtime for this
			// at analysis time and store it in a table, similar to how we store the EVM token metadata.
			TokenDecimals: 18,
		}
		if err = runtimeSdkRows.Scan(
			&b.Balance,
			&b.TokenSymbol,
		); err != nil {
			return nil, wrapError(err)
		}
		a.Balances = append(a.Balances, b)
	}

	runtimeEvmRows, queryErr := c.db.Query(
		ctx,
		queries.AccountRuntimeEvmBalances,
		runtime,
		address.String(),
	)
	if queryErr != nil {
		return nil, wrapError(queryErr)
	}
	defer runtimeEvmRows.Close()

	for runtimeEvmRows.Next() {
		b := RuntimeEvmBalance{}
		var addrPreimage []byte
		var tokenType common.TokenType
		if err = runtimeEvmRows.Scan(
			&b.Balance,
			&b.TokenContractAddr,
			&addrPreimage,
			&b.TokenSymbol,
			&b.TokenName,
			&tokenType,
			&b.TokenDecimals,
		); err != nil {
			return nil, wrapError(err)
		}
		b.TokenContractAddrEth = EthChecksumAddrFromBarePreimage(addrPreimage)
		b.TokenType = translateTokenType(tokenType)
		a.EvmBalances = append(a.EvmBalances, b)
	}

	evmContract := RuntimeEvmContract{
		Verification: &RuntimeEvmContractVerification{},
	}
	err = c.db.QueryRow(
		ctx,
		queries.RuntimeEvmContract,
		runtime,
		address.String(),
	).Scan(
		&evmContract.CreationTx,
		&evmContract.EthCreationTx,
		&evmContract.CreationBytecode,
		&evmContract.RuntimeBytecode,
		&evmContract.GasUsed,
		&evmContract.Verification.CompilationMetadata,
		&evmContract.Verification.SourceFiles,
		&evmContract.Verification.VerificationLevel,
	)
	switch err {
	case nil:
		a.EvmContract = &evmContract
	case pgx.ErrNoRows:
		// If an account address does not represent a smart contract; skip.
		a.EvmContract = nil
	default:
		return nil, wrapError(err)
	}
	// Make verification data null if the contract is not verified.
	if evmContract.Verification.CompilationMetadata == nil && evmContract.Verification.SourceFiles == nil {
		evmContract.Verification = nil
	}

	err = c.db.QueryRow(
		ctx,
		queries.RuntimeAccountStats,
		runtime,
		address.String(),
	).Scan(
		&a.Stats.TotalSent,
		&a.Stats.TotalReceived,
		&a.Stats.NumTxns,
	)

	switch err {
	case nil:
	case pgx.ErrNoRows:
		// If an account address has no activity, default to 0.
		a.Stats.TotalSent = common.Ptr(common.NewBigInt(0))
		a.Stats.TotalReceived = common.Ptr(common.NewBigInt(0))
		a.Stats.NumTxns = 0
	default:
		return nil, wrapError(err)
	}

	// Reconcile sdk balance(s) from node with the balance(s) fetched from the db.
	c.upsertBalances(ch, &a)

	return &a, nil
}

// Reads node sdk balances from ch and upserts them into acct.Balances, logging a
// warning if the balances are mismatched.
func (c *StorageClient) upsertBalances(ch chan *RuntimeSdkBalance, acct *RuntimeAccount) {
	for nb := range ch {
		found := false
		// If the node balance differs from the balance in Nexus, defer to the node balance.
		for i, b := range acct.Balances {
			if b.TokenSymbol == nb.TokenSymbol {
				found = true
				if !b.Balance.Eq(nb.Balance) {
					c.logger.Warn("sdk balance from node differed from dead-reckoned balance; defaulting to node balance", "address", acct.Address, "height", roothash.RoundLatest, "node_balance", nb.Balance.String(), "db_balance", b.Balance.String())
					bPtr := &acct.Balances[i]
					bPtr.Balance = nb.Balance
				}
			}
		}
		// If the balance doesn't exist in Nexus, add it.
		if !found {
			c.logger.Warn("node returned balance that doesn't exist in nexus", "symbol", nb)
			acct.Balances = append(acct.Balances, *nb)
		}
	}
}

func fillInPriceFromReserves(t *EvmToken) {
	reserve0f, _ := t.RefSwap.Reserve0.Float64()
	reserve1f, _ := t.RefSwap.Reserve1.Float64()
	if reserve0f > 0 && reserve1f > 0 {
		if t.ContractAddr == *t.RefSwap.Token0Address {
			t.RelativePrice = common.Ptr(reserve1f / reserve0f)
		} else {
			t.RelativePrice = common.Ptr(reserve0f / reserve1f)
		}
	}
}

func fillInPrice(t *EvmToken, refSwapTokenAddr *apiTypes.Address) {
	if t.ContractAddr == *refSwapTokenAddr {
		t.RelativePrice = common.Ptr(1.0)
	} else if t.RefSwap.Token0Address != nil && t.RefSwap.Token1Address != nil && t.RefSwap.Reserve0 != nil && t.RefSwap.Reserve1 != nil {
		fillInPriceFromReserves(t)
	}
	if t.RelativePrice != nil {
		t.RelativeTokenAddress = refSwapTokenAddr
		if t.TotalSupply != nil {
			totalSuppplyF, _ := t.TotalSupply.Float64()
			t.RelativeTotalValue = common.Ptr(*t.RelativePrice * totalSuppplyF)
		}
	}
}

// If `address` is non-nil, it is used to filter the results to at most 1 token: the one
// with the correcponding contract address.
func (c *StorageClient) RuntimeTokens(ctx context.Context, runtime common.Runtime, p apiTypes.GetRuntimeEvmTokensParams, address *staking.Address) (*EvmTokenList, error) {
	var refSwapFactoryAddr *apiTypes.Address
	var refSwapTokenAddr *apiTypes.Address
	if rs, ok := c.referenceSwaps[runtime]; ok {
		refSwapFactoryAddr = &rs.FactoryAddr
		refSwapTokenAddr = &rs.ReferenceTokenAddr
	}
	var tokenType *int
	if p.Type != nil {
		switch *p.Type {
		case apiTypes.EvmTokenTypeERC20:
			tokenType = common.Ptr(20)
		case apiTypes.EvmTokenTypeERC721:
			tokenType = common.Ptr(721)
		default:
			return nil, fmt.Errorf("invalid token type: %s", *p.Type)
		}
	}

	res, err := c.withTotalCount(
		ctx,
		queries.EvmTokens,
		runtime,
		address,
		p.Name,
		tokenType,
		refSwapFactoryAddr,
		refSwapTokenAddr,
		p.SortBy,
		c.evmTokensCustomOrderAddresses[runtime],
		c.evmTokensCustomOrderGroups[runtime],
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := EvmTokenList{
		EvmTokens:           []EvmToken{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var t EvmToken
		var addrPreimage []byte
		var tokenType common.TokenType
		var refSwapPairAddr *string
		var refSwap apiTypes.EvmTokenSwap
		var refSwapPairEthAddr []byte
		var refSwapFactoryEthAddr []byte
		var refSwapToken0EthAddr []byte
		var refSwapToken1EthAddr []byte
		var refToken apiTypes.EvmRefToken
		var refTokenType *common.TokenType
		if err2 := res.rows.Scan(
			&t.ContractAddr,
			&addrPreimage,
			&t.Name,
			&t.Symbol,
			&t.Decimals,
			&t.TotalSupply,
			&t.NumTransfers,
			&tokenType,
			&t.NumHolders,
			&refSwapPairAddr,
			&refSwapPairEthAddr,
			&refSwap.FactoryAddress,
			&refSwapFactoryEthAddr,
			&refSwap.Token0Address,
			&refSwapToken0EthAddr,
			&refSwap.Token1Address,
			&refSwapToken1EthAddr,
			&refSwap.CreateRound,
			&refSwap.Reserve0,
			&refSwap.Reserve1,
			&refSwap.LastSyncRound,
			&refTokenType,
			&refToken.Name,
			&refToken.Symbol,
			&refToken.Decimals,
			&t.VerificationLevel,
			nil, // custom_sort_order
		); err2 != nil {
			return nil, wrapError(err2)
		}

		t.IsVerified = (t.VerificationLevel != nil)
		t.EthContractAddr = EthChecksumAddrFromBarePreimage(addrPreimage)
		t.Type = translateTokenType(tokenType)
		if refSwapPairAddr != nil {
			refSwap.PairAddress = *refSwapPairAddr
			t.RefSwap = &refSwap
			t.RefSwap.PairAddressEth = EthChecksumAddrPtrFromBarePreimage(refSwapPairEthAddr)
			t.RefSwap.FactoryAddressEth = EthChecksumAddrPtrFromBarePreimage(refSwapFactoryEthAddr)
			t.RefSwap.Token0AddressEth = EthChecksumAddrPtrFromBarePreimage(refSwapToken0EthAddr)
			t.RefSwap.Token1AddressEth = EthChecksumAddrPtrFromBarePreimage(refSwapToken1EthAddr)
			if refSwapTokenAddr != nil {
				fillInPrice(&t, refSwapTokenAddr)
			}
		}
		if refTokenType != nil {
			refToken.Type = translateTokenType(*refTokenType)
			t.RefToken = &refToken
		}
		ts.EvmTokens = append(ts.EvmTokens, t)
	}

	return &ts, nil
}

func (c *StorageClient) RuntimeTokenHolders(ctx context.Context, runtime common.Runtime, p apiTypes.GetRuntimeEvmTokensAddressHoldersParams, address staking.Address) (*TokenHolderList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.EvmTokenHolders,
		runtime,
		address,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	hs := TokenHolderList{
		Holders:             []BareTokenHolder{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var h BareTokenHolder
		var addrPreimage []byte
		if err2 := res.rows.Scan(
			&h.HolderAddress,
			&addrPreimage,
			&h.Balance,
		); err2 != nil {
			return nil, wrapError(err2)
		}
		h.EthHolderAddress = EthChecksumAddrPtrFromBarePreimage(addrPreimage)
		hs.Holders = append(hs.Holders, h)
	}

	return &hs, nil
}

func (c *StorageClient) RuntimeEVMNFTs(ctx context.Context, runtime common.Runtime, limit *uint64, offset *uint64, tokenAddress *staking.Address, id *common.BigInt, ownerAddress *staking.Address) (*EvmNftList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.EvmNfts,
		runtime,
		tokenAddress,
		id,
		ownerAddress,
		limit,
		offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	nfts := EvmNftList{
		EvmNfts:             []EvmNft{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var nft EvmNft
		var contractAddrContextIdentifier string
		var contractAddrContextVersion int
		var contractAddrData []byte
		var tokenType sql.NullInt32
		// Owner might not be known, so these preimage fields are also nilable.
		var ownerAddrContextIdentifier *string
		var ownerAddrContextVersion *int
		var ownerAddrData []byte
		var metadataAccessedN sql.NullTime
		if err = res.rows.Scan(
			&nft.Token.ContractAddr,
			&contractAddrContextIdentifier,
			&contractAddrContextVersion,
			&contractAddrData,
			&nft.Token.Name,
			&nft.Token.Symbol,
			&nft.Token.Decimals,
			&tokenType,
			&nft.Token.TotalSupply,
			&nft.Token.NumTransfers,
			&nft.Token.NumHolders,
			&nft.Token.VerificationLevel,
			&nft.Id,
			&nft.Owner,
			&ownerAddrContextIdentifier,
			&ownerAddrContextVersion,
			&ownerAddrData,
			&nft.NumTransfers,
			&nft.MetadataUri,
			&metadataAccessedN,
			&nft.Metadata,
			&nft.Name,
			&nft.Description,
			&nft.Image,
		); err != nil {
			return nil, wrapError(err)
		}
		nft.Token.IsVerified = (nft.Token.VerificationLevel != nil)
		contractEthChecksumAddrPtr := EthChecksumAddrFromPreimage(contractAddrContextIdentifier, contractAddrContextVersion, contractAddrData)
		// API says this is required, but this was refactored from some code
		// that doesn't crash if preimage context is wrong, and I'm keeping
		// that robustness in this version.
		if contractEthChecksumAddrPtr != nil {
			nft.Token.EthContractAddr = *contractEthChecksumAddrPtr
		}
		if contractEthAddr, err1 := EVMEthAddrFromPreimage(contractAddrContextIdentifier, contractAddrContextVersion, contractAddrData); err1 == nil {
			contractECAddr := ethCommon.BytesToAddress(contractEthAddr)
			nft.Token.EthContractAddr = contractECAddr.String()
		}
		if tokenType.Valid {
			nft.Token.Type = translateTokenType(common.TokenType(tokenType.Int32))
		}
		if nft.Owner != nil {
			nft.OwnerEth = EthChecksumAddrFromPreimage(*ownerAddrContextIdentifier, *ownerAddrContextVersion, ownerAddrData)
		}
		if metadataAccessedN.Valid {
			nft.MetadataAccessed = common.Ptr(metadataAccessedN.Time.String())
		}
		nfts.EvmNfts = append(nfts.EvmNfts, nft)
	}

	return &nfts, nil
}

// RuntimeStatus returns runtime status information.
func (c *StorageClient) RuntimeStatus(ctx context.Context, runtime common.Runtime) (*RuntimeStatus, error) {
	runtimeID, err := c.sourceCfg.ResolveRuntimeID(runtime)
	if err != nil {
		// Return a generic error here and log the detailed error. This is most likely a misconfiguration of the server.
		c.logger.Error("runtime name to ID failure", "runtime", runtime, "err", err)
		return nil, apiCommon.ErrBadRuntime
	}

	var s apiTypes.RuntimeStatus
	var latest_block_update time.Time
	// Query latest block and update time.
	err = c.db.QueryRow(
		ctx,
		queries.Status,
		runtime,
	).Scan(&s.LatestBlock, &latest_block_update)
	switch err {
	case nil:
	case pgx.ErrNoRows:
		// No runtime blocks indexed yet.
		s.LatestBlock = -1
	default:
		return nil, wrapError(err)
	}
	// Calculate the elapsed time since the last block was processed. We assume that the analyzer and api server
	// are running on VMs with synced clocks.
	s.LatestUpdateAgeMs = time.Since(latest_block_update).Milliseconds()

	// Query latest block for info.
	if err := c.db.QueryRow(
		ctx,
		queries.RuntimeBlock,
		runtime,
		s.LatestBlock,
	).Scan(nil, nil, &s.LatestBlockTime, nil, nil, nil); err != nil {
		return nil, wrapError(err)
	}

	// Query active nodes.
	if err := c.db.QueryRow(
		ctx,
		queries.RuntimeActiveNodes,
		runtimeID,
	).Scan(&s.ActiveNodes); err != nil {
		return nil, wrapError(err)
	}

	return &s, nil
}

// RuntimeRoflApps returns a list of ROFL apps.
func (c *StorageClient) RuntimeRoflApps(ctx context.Context, runtime common.Runtime, params apiTypes.GetRuntimeRoflAppsParams, id *string) (*RoflAppList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeRoflApps,
		runtime,
		id,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	apps := RoflAppList{
		RoflApps:            []RoflApp{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	var lastActivityRound *uint64
	var lastActivityTxIndex *uint64
	var contractAddrContextIdentifier *string
	var contractAddrContextVersion *int
	var contractAddrData []byte
	for res.rows.Next() {
		var app RoflApp
		if err := res.rows.Scan(
			&app.Id,
			&app.Admin,
			&contractAddrContextIdentifier,
			&contractAddrContextVersion,
			&contractAddrData,
			&app.Stake,
			&app.Policy,
			&app.Sek,
			&app.Metadata,
			&app.Secrets,
			&app.Removed,
			&app.DateCreated,
			&app.LastActivity,
			&lastActivityRound,
			&lastActivityTxIndex,
			&app.NumActiveInstances,
			&app.ActiveInstances,
		); err != nil {
			return nil, wrapError(err)
		}
		if contractAddrContextIdentifier != nil && contractAddrContextVersion != nil && contractAddrData != nil {
			app.AdminEth = EthChecksumAddrFromPreimage(*contractAddrContextIdentifier, *contractAddrContextVersion, contractAddrData)
		}
		apps.RoflApps = append(apps.RoflApps, app)
	}

	// When querying a single ROFL app, also fetch the latest activity transaction.
	if id != nil && len(apps.RoflApps) == 1 && lastActivityRound != nil && lastActivityTxIndex != nil {
		func() {
			res, err := c.db.Query(ctx, queries.RuntimeTransactionsNoRelated, runtime, lastActivityRound, lastActivityTxIndex, nil, nil, nil, nil, nil, nil, 1, 0)
			if err != nil {
				c.logger.Error("error fetching latest activity transaction", "err", err)
				return
			}
			defer res.Close()

			for res.Next() {
				tx, err := runtimeTransactionFromRow(res, c.logger)
				if err != nil {
					c.logger.Error("error converting transaction to API response", "err", err)
					return
				}
				if tx != nil {
					apps.RoflApps[0].LastActivityTx = tx
				}
			}
		}()
	}

	return &apps, nil
}

// RuntimeRoflAppInstances returns a list of ROFL app instances.
func (c *StorageClient) RuntimeRoflAppInstances(ctx context.Context, runtime common.Runtime, params apiTypes.GetRuntimeRoflAppsIdInstancesParams, id string, rak *string) (*RoflAppInstanceList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeRoflAppInstances,
		runtime,
		id,
		rak,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	instances := RoflAppInstanceList{
		Instances:           []apiTypes.RoflInstance{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var instance apiTypes.RoflInstance
		if err := res.rows.Scan(
			&instance.Rak,
			&instance.EndorsingNodeId,
			&instance.EndorsingEntityId,
			&instance.Rek,
			&instance.ExpirationEpoch,
			&instance.ExtraKeys,
		); err != nil {
			return nil, wrapError(err)
		}
		instances.Instances = append(instances.Instances, instance)
	}
	return &instances, nil
}

// RuntimeRoflAppTransactions returns a list of ROFL app transactions.
func (c *StorageClient) RuntimeRoflAppTransactions(ctx context.Context, runtime common.Runtime, params apiTypes.GetRuntimeRoflAppsIdTransactionsParams, id string) (*RuntimeTransactionList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeTransactionsRelatedRofl,
		runtime,
		nil,
		nil,
		nil,
		nil,
		id,
		params.Method,
		nil,
		nil,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := RuntimeTransactionList{
		Transactions:        []RuntimeTransaction{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		t, err := runtimeTransactionFromRow(res.rows, c.logger)
		if err != nil {
			c.logger.Error("error converting transaction to API response", "err", err)
			return nil, wrapError(err)
		}
		ts.Transactions = append(ts.Transactions, *t)
	}

	return &ts, nil
}

// RuntimeRoflAppInstanceTransactions returns a list of ROFL app instance transactions.
func (c *StorageClient) RuntimeRoflAppInstanceTransactions(ctx context.Context, runtime common.Runtime, method *string, limit *uint64, offset *uint64, appId string, rak *string) (*RuntimeTransactionList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeRoflAppInstanceTransactions,
		runtime,
		appId,
		rak,
		method,
		limit,
		offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	ts := RuntimeTransactionList{
		Transactions:        []RuntimeTransaction{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		t, err := runtimeTransactionFromRow(res.rows, c.logger)
		if err != nil {
			c.logger.Error("error converting transaction to API response", "err", err)
			return nil, wrapError(err)
		}
		ts.Transactions = append(ts.Transactions, *t)
	}

	return &ts, nil
}

// RuntimeRoflmarketProviders returns a list of ROFL market providers.
func (c *StorageClient) RuntimeRoflmarketProviders(ctx context.Context, runtime common.Runtime, params apiTypes.GetRuntimeRoflmarketProvidersParams, address *staking.Address) (*RoflMarketProviderList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeRoflmarketProviders,
		runtime,
		address,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	providers := RoflMarketProviderList{
		Providers:           []apiTypes.RoflMarketProvider{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var provider apiTypes.RoflMarketProvider

		var offersNextId []byte
		var instancesNextId []byte
		if err := res.rows.Scan(
			&provider.Address,
			&provider.Nodes,
			&provider.Scheduler,
			&provider.PaymentAddress,
			&provider.Metadata,
			&provider.Stake,
			&offersNextId,
			&provider.OffersCount,
			&instancesNextId,
			&provider.InstancesCount,
			&provider.CreatedAt,
			&provider.UpdatedAt,
			&provider.Removed,
		); err != nil {
			return nil, wrapError(err)
		}
		provider.OffersNextId = hex.EncodeToString(offersNextId)
		provider.InstancesNextId = hex.EncodeToString(instancesNextId)
		providers.Providers = append(providers.Providers, provider)
	}

	return &providers, nil
}

func (c *StorageClient) RuntimeRoflmarketOffers(ctx context.Context, runtime common.Runtime, params apiTypes.GetRuntimeRoflmarketProvidersAddressOffersParams, address *staking.Address) (*RoflMarketOfferList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeRoflmarketProviderOffers,
		runtime,
		address,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	offers := RoflMarketOfferList{
		Offers:              []apiTypes.RoflMarketOffer{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var offer apiTypes.RoflMarketOffer
		var offerID []byte
		if err := res.rows.Scan(
			&offerID,
			&offer.Provider,
			&offer.Resources,
			&offer.Payment,
			&offer.Capacity,
			&offer.Metadata,
			&offer.Removed,
		); err != nil {
			return nil, wrapError(err)
		}
		offer.Id = hex.EncodeToString(offerID)
		offers.Offers = append(offers.Offers, offer)
	}

	return &offers, nil
}

func (c *StorageClient) RuntimeRoflmarketInstances(ctx context.Context, runtime common.Runtime, params apiTypes.GetRuntimeRoflmarketProvidersAddressInstancesParams, address *staking.Address) (*RoflMarketInstanceList, error) {
	res, err := c.withTotalCount(
		ctx,
		queries.RuntimeRoflmarketProviderInstances,
		runtime,
		address,
		params.Limit,
		params.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer res.rows.Close()

	instances := RoflMarketInstanceList{
		Instances:           []apiTypes.RoflMarketInstance{},
		TotalCount:          res.totalCount,
		IsTotalCountClipped: res.isTotalCountClipped,
	}
	for res.rows.Next() {
		var instance apiTypes.RoflMarketInstance
		var instanceID []byte
		var offerID []byte
		var paymentAddress []byte
		var refundData []byte
		var cmdNextId []byte
		if err := res.rows.Scan(
			&instanceID,
			&instance.Provider,
			&offerID,
			&instance.Status,
			&instance.Creator,
			&instance.Admin,
			&instance.NodeId,
			&instance.Metadata,
			&instance.Resources,
			&instance.Deployment,
			&instance.CreatedAt,
			&instance.UpdatedAt,
			&instance.PaidFrom,
			&instance.PaidUntil,
			&instance.Payment,
			&paymentAddress,
			&refundData,
			&cmdNextId,
			&instance.CmdCount,
			&instance.Cmds,
			&instance.Removed,
		); err != nil {
			return nil, wrapError(err)
		}
		instance.Id = hex.EncodeToString(instanceID)
		instance.OfferId = hex.EncodeToString(offerID)
		instance.PaymentAddress = hex.EncodeToString(paymentAddress)
		instance.RefundData = hex.EncodeToString(refundData)
		instance.CmdNextId = hex.EncodeToString(cmdNextId)
		instances.Instances = append(instances.Instances, instance)
	}

	return &instances, nil
}

// TxVolumes returns a list of transaction volumes per time window.
func (c *StorageClient) TxVolumes(ctx context.Context, layer apiTypes.Layer, p apiTypes.GetLayerStatsTxVolumeParams) (*TxVolumeList, error) {
	var query string

	switch {
	case *p.WindowSizeSeconds == 300 && *p.WindowStepSeconds == 300:
		// 5 minute window, 5 minute step.
		query = queries.FineTxVolumes
	case *p.WindowSizeSeconds == 86400 && *p.WindowStepSeconds == 86400:
		// 1 day window, 1 day step.
		query = queries.DailyTxVolumes
	case *p.WindowSizeSeconds == 86400 && *p.WindowStepSeconds == 300:
		// 1 day window, 5 minute step.
		query = queries.FineDailyTxVolumes
	default:
		// Unsupported: case *p.WindowSizeSeconds == 300 && *p.WindowStepSeconds == 86400:
		return nil, fmt.Errorf("invalid window size parameters: %w", apiCommon.ErrBadRequest)
	}

	rows, err := c.db.Query(
		ctx,
		query,
		translateLayer(layer),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ts := TxVolumeList{
		WindowSizeSeconds: *p.WindowSizeSeconds,
		Windows:           []apiTypes.TxVolume{},
	}
	for rows.Next() {
		var t TxVolume
		if err := rows.Scan(
			&t.WindowEnd,
			&t.TxVolume,
		); err != nil {
			return nil, wrapError(err)
		}
		t.WindowEnd = t.WindowEnd.UTC() // Ensure UTC timestamp in response.
		ts.Windows = append(ts.Windows, t)
	}

	return &ts, nil
}

// DailyActiveAccounts returns a list of daily active accounts.
func (c *StorageClient) DailyActiveAccounts(ctx context.Context, layer apiTypes.Layer, p apiTypes.GetLayerStatsActiveAccountsParams) (*DailyActiveAccountsList, error) {
	var query string
	switch {
	case p.WindowStepSeconds != nil && *p.WindowStepSeconds == 300:
		query = queries.FineDailyActiveAccounts
	default:
		query = queries.DailyActiveAccounts
	}

	rows, err := c.db.Query(
		ctx,
		query,
		translateLayer(layer),
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return nil, wrapError(err)
	}
	defer rows.Close()

	ts := DailyActiveAccountsList{
		WindowSizeSeconds: 86400, // Day.
		Windows:           []apiTypes.ActiveAccounts{},
	}
	for rows.Next() {
		var t apiTypes.ActiveAccounts
		if err := rows.Scan(
			&t.WindowEnd,
			&t.ActiveAccounts,
		); err != nil {
			return nil, wrapError(err)
		}
		t.WindowEnd = t.WindowEnd.UTC() // Ensure UTC timestamp in response.
		ts.Windows = append(ts.Windows, t)
	}

	return &ts, nil
}
