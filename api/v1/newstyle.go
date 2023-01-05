package v1

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"

	"github.com/google/uuid"
	apiCommon "github.com/oasisprotocol/oasis-indexer/api/common"
	apiTypes "github.com/oasisprotocol/oasis-indexer/api/v1/types"
	"github.com/oasisprotocol/oasis-indexer/common"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/metrics"
)

// Stub impl generated with:
//   sed -n '/type ServerInterface interface/,/^\}/p' api/v1/types/server.gen.go | grep -v // | head -n-1 | tail -n+2 | sed -E 's/^\s+/func (foo *Foo) /g; s/[a-zA-Z]+Params/apiTypes.\0/g; s/$/{}/'

type Foo struct {
	client  *storageClient
	logger  *log.Logger
	metrics metrics.RequestMetrics
}

var _ apiTypes.ServerInterface = (*Foo)(nil)

func NewFoo(client *storageClient, logger *log.Logger, metrics metrics.RequestMetrics) *Foo {
	return &Foo{
		client:  client,
		logger:  logger,
		metrics: metrics,
	}
}

func (h *Foo) logAndReply(ctx context.Context, msg string, w http.ResponseWriter, err error) {
	h.logger.Error(msg,
		"request_id", ctx.Value(common.RequestIDContextKey),
		"error", err,
	)
	if err = apiCommon.ReplyWithError(w, err); err != nil {
		h.logger.Error("failed to reply with error",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"error", err,
		)
	}
}

var defaultOffset = uint64(0)
var defaultLimit = uint64(100)
var maxLimit = uint64(1000)

// Sets a value for `Limit` and `Offset` fields of the given struct, if present and nil-valued.
// oapi-codegen ignores the specified default value in the openapi spec:
//
//	https://github.com/deepmap/oapi-codegen/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc++default+in%3Atitle
//
// Luckily our defaults are pretty simple (just limit and offset) so we can hardcode them here.
func fixDefaultsAndLimits(p any) {
	// check that p is a pointer to a struct
	if p == nil || reflect.TypeOf(p).Kind() != reflect.Ptr || reflect.TypeOf(p).Elem().Kind() != reflect.Struct {
		panic("fixDefaults: p is not a pointer to a struct")
	}
	// iterate through the struct fields. If the field name equals "Limit" or "Offset" and the value is nil, set it to 100 and 0 respectively.
	v := reflect.ValueOf(p).Elem()
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Ptr && f.IsNil() {
			switch v.Type().Field(i).Name {
			case "Limit":
				f.Set(reflect.ValueOf(&defaultLimit))
			case "Offset":
				f.Set(reflect.ValueOf(&defaultOffset))
			}
		}
	}

	// iterate through the struct fields. If the field name equals "Limit" and its type is *uint64 and the value is not nil, clamp it to between 0 and 1000.
	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Ptr && !f.IsNil() {
			switch v.Type().Field(i).Name {
			case "Limit":
				if v.Type().Field(i).Type == reflect.TypeOf(&maxLimit) && *f.Interface().(*uint64) > maxLimit {
					*f.Interface().(*uint64) = maxLimit
				}
			}
		}
	}
}

func (foo *Foo) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write([]byte("hello there")); err != nil {
		panic(err)
	}
	reqId := ctx.Value(common.RequestIDContextKey)
	rr := ""
	if reqId == nil {
		rr = "nil"
	} else {
		rr = reqId.(uuid.UUID).String()
	}
	if _, err := w.Write([]byte(rr)); err != nil {
		panic(err)
	}
}
func (h *Foo) GetConsensusAccounts(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusAccountsParams) {
	ctx := r.Context()
	fixDefaultsAndLimits(&params)
	//fmt.Fprintf(os.Stdout, "Limit is now %d\n", *params.Limit)

	//accounts, err := h.client.Accounts(ctx, r)  // obsolete
	accounts, err := h.client.Storage.Accounts(ctx, params)
	if err != nil {
		h.logAndReply(ctx, "failed to list accounts", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "database_error").Inc()
		return
	}

	resp, err := json.Marshal(accounts)
	if err != nil {
		h.logAndReply(ctx, "failed to marshal accounts", w, err)
		h.metrics.RequestCounter(r.URL.Path, "failure", "serde_error").Inc()
		return
	}

	w.Header().Set("content-type", "application/json")
	if _, err := w.Write(resp); err != nil {
		h.logger.Error("failed to write response",
			"request_id", ctx.Value(common.RequestIDContextKey),
			"error", err,
		)
		h.metrics.RequestCounter(r.URL.Path, "failure", "http_error").Inc()
	} else {
		h.metrics.RequestCounter(r.URL.Path, "success").Inc()
	}
}
func (foo *Foo) GetConsensusAccountsAddress(w http.ResponseWriter, r *http.Request, address string) {}
func (foo *Foo) GetConsensusAccountsAddressDebondingDelegations(w http.ResponseWriter, r *http.Request, address string) {
}
func (foo *Foo) GetConsensusAccountsAddressDelegations(w http.ResponseWriter, r *http.Request, address string) {
}
func (foo *Foo) GetConsensusBlocks(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusBlocksParams) {
}
func (foo *Foo) GetConsensusBlocksHeight(w http.ResponseWriter, r *http.Request, height int64) {}
func (foo *Foo) GetConsensusEntities(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusEntitiesParams) {
}
func (foo *Foo) GetConsensusEntitiesEntityId(w http.ResponseWriter, r *http.Request, entityId string) {
}
func (foo *Foo) GetConsensusEntitiesEntityIdNodes(w http.ResponseWriter, r *http.Request, entityId string, params apiTypes.GetConsensusEntitiesEntityIdNodesParams) {
}
func (foo *Foo) GetConsensusEntitiesEntityIdNodesNodeId(w http.ResponseWriter, r *http.Request, entityId string, nodeId string) {
}
func (foo *Foo) GetConsensusEpochs(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusEpochsParams) {
}
func (foo *Foo) GetConsensusEpochsEpoch(w http.ResponseWriter, r *http.Request, epoch int64) {}
func (foo *Foo) GetConsensusEvents(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusEventsParams) {
}
func (foo *Foo) GetConsensusProposals(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusProposalsParams) {
}
func (foo *Foo) GetConsensusProposalsProposalId(w http.ResponseWriter, r *http.Request, proposalId int64) {
}
func (foo *Foo) GetConsensusProposalsProposalIdVotes(w http.ResponseWriter, r *http.Request, proposalId int64, params apiTypes.GetConsensusProposalsProposalIdVotesParams) {
}
func (foo *Foo) GetConsensusStatsTxVolume(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusStatsTxVolumeParams) {
}
func (foo *Foo) GetConsensusTransactions(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusTransactionsParams) {
}
func (foo *Foo) GetConsensusTransactionsTxHash(w http.ResponseWriter, r *http.Request, txHash string) {
}
func (foo *Foo) GetConsensusValidators(w http.ResponseWriter, r *http.Request, params apiTypes.GetConsensusValidatorsParams) {
}
func (foo *Foo) GetConsensusValidatorsEntityId(w http.ResponseWriter, r *http.Request, entityId string) {
}
func (foo *Foo) GetEmeraldBlocks(w http.ResponseWriter, r *http.Request, params apiTypes.GetEmeraldBlocksParams) {
}
func (foo *Foo) GetEmeraldTokens(w http.ResponseWriter, r *http.Request, params apiTypes.GetEmeraldTokensParams) {
}
func (foo *Foo) GetEmeraldTransactions(w http.ResponseWriter, r *http.Request, params apiTypes.GetEmeraldTransactionsParams) {
}
