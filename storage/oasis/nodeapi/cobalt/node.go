package cobalt

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	// indexer-internal data types.
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	consensusTx "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"

	// data types for Cobalt gRPC APIs.
	beaconCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/beacon/api"
	consensusCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/consensus/api"
	txResultsCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/consensus/api/transaction/results"
	genesisCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/genesis/api"
	governanceCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/governance/api"
	registryCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/registry/api"
	roothashCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/roothash/api"
	schedulerCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/scheduler/api"
	stakingCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/staking/api"
)

// CobaltConsensusApiLite provides low-level access to the consensus API of a
// Cobalt node. To be able to use the old gRPC API, this struct uses gRPC
// directly, skipping the convenience wrappers provided by oasis-core.
type CobaltConsensusApiLite struct {
	grpcConn *grpc.ClientConn
}

var _ nodeapi.ConsensusApiLite = (*CobaltConsensusApiLite)(nil)

func NewCobaltConsensusApiLite(grpcConn *grpc.ClientConn) *CobaltConsensusApiLite {
	return &CobaltConsensusApiLite{
		grpcConn: grpcConn,
	}
}

func (c *CobaltConsensusApiLite) Close() error {
	return c.grpcConn.Close()
}

func (c *CobaltConsensusApiLite) GetGenesisDocument(ctx context.Context) (*genesis.Document, error) {
	var rsp genesisCobalt.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetGenesisDocument", nil, &rsp); err != nil {
		return nil, fmt.Errorf("GetGenesisDocument(cobalt): %w", err)
	}
	return ConvertGenesis(rsp), nil
}

func (c *CobaltConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	var rsp genesisCobalt.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/StateToGenesis", height, &rsp); err != nil {
		return nil, fmt.Errorf("StateToGenesis(%d): %w", height, err)
	}
	return ConvertGenesis(rsp), nil
}

func (c *CobaltConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	var rsp consensusCobalt.Block
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetBlock", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetBlock(%d): %w", height, err)
	}
	return convertBlock(rsp), nil
}

func (c *CobaltConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	var rsp consensusCobalt.TransactionsWithResults
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetTransactionsWithResults", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetTransactionsWithResults(%d): %w", height, err)
	}
	txrs := make([]nodeapi.TransactionWithResults, len(rsp.Transactions))

	// convert the response to the indexer-internal data type
	for i, txBytes := range rsp.Transactions {
		var tx consensusTx.SignedTransaction
		if err := cbor.Unmarshal(txBytes, &tx); err != nil {
			return nil, fmt.Errorf("GetTransactionsWithResults(%d): transaction %d (%x): %w", height, i, txBytes, err)
		}
		if rsp.Results[i] == nil {
			return nil, fmt.Errorf("GetTransactionsWithResults(%d): transaction %d (%x): has no result", height, i, txBytes)
		}
		txrs[i] = nodeapi.TransactionWithResults{
			Transaction: tx,
			Result:      convertTxResult(*rsp.Results[i]),
		}
	}
	return txrs, nil
}

func (c *CobaltConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	var rsp beaconCobalt.EpochTime
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Beacon/GetEpoch", height, &rsp); err != nil {
		return beacon.EpochInvalid, fmt.Errorf("GetEpoch(%d): %w", height, err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*registryCobalt.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Registry/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("RegistryEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsCobalt.Event{Registry: e})
	}
	return events, nil
}

func (c *CobaltConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*stakingCobalt.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Staking/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("StakingEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsCobalt.Event{Staking: e})
	}
	return events, nil
}

func (c *CobaltConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*governanceCobalt.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Governance/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("GovernanceEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsCobalt.Event{Governance: e})
	}
	return events, nil
}

func (c *CobaltConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*roothashCobalt.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.RootHash/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("RoothashEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsCobalt.Event{RootHash: e})
	}
	return events, nil
}

func (c *CobaltConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	var rsp []*schedulerCobalt.Validator
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Scheduler/GetValidators", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetValidators(%d): %w", height, err)
	}
	validators := make([]nodeapi.Validator, len(rsp))
	for i, v := range rsp {
		validators[i] = nodeapi.Validator(*v)
	}
	return validators, nil
}

func (c *CobaltConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]nodeapi.Committee, error) {
	var rsp []*schedulerCobalt.Committee
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Scheduler/GetCommittees", &scheduler.GetCommitteesRequest{
		Height:    height,
		RuntimeID: runtimeID,
	}, &rsp); err != nil {
		return nil, fmt.Errorf("GetCommittees(%d): %w", height, err)
	}
	committees := make([]nodeapi.Committee, len(rsp))
	for i, c := range rsp {
		committees[i] = convertCommittee(*c)
	}
	return committees, nil
}

func (c *CobaltConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	var rsp *governanceCobalt.Proposal
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Governance/Proposal", &governance.ProposalQuery{
		Height:     height,
		ProposalID: proposalID,
	}, &rsp); err != nil {
		return nil, fmt.Errorf("GetProposal(%d, %d): %w", height, proposalID, err)
	}
	return (*nodeapi.Proposal)(convertProposal(rsp)), nil
}
