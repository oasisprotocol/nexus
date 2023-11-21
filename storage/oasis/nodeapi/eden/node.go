package eden

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	// nexus-internal data types.
	"github.com/oasisprotocol/oasis-core/go/common"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"
	consensusTx "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"

	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/connections"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Eden gRPC APIs.
	beaconEden "github.com/oasisprotocol/nexus/coreapi/v23.0/beacon/api"
	consensusEden "github.com/oasisprotocol/nexus/coreapi/v23.0/consensus/api"
	txResultsEden "github.com/oasisprotocol/nexus/coreapi/v23.0/consensus/api/transaction/results"
	genesisEden "github.com/oasisprotocol/nexus/coreapi/v23.0/genesis/api"
	governanceEden "github.com/oasisprotocol/nexus/coreapi/v23.0/governance/api"
	registryEden "github.com/oasisprotocol/nexus/coreapi/v23.0/registry/api"
	roothashEden "github.com/oasisprotocol/nexus/coreapi/v23.0/roothash/api"
	schedulerEden "github.com/oasisprotocol/nexus/coreapi/v23.0/scheduler/api"
	stakingEden "github.com/oasisprotocol/nexus/coreapi/v23.0/staking/api"
)

// ConsensusApiLite provides low-level access to the consensus API of a
// Eden node. To be able to use the old gRPC API, this struct uses gRPC
// directly, skipping the convenience wrappers provided by oasis-core.
type ConsensusApiLite struct {
	grpcConn connections.GrpcConn
}

var _ nodeapi.ConsensusApiLite = (*ConsensusApiLite)(nil)

func NewConsensusApiLite(grpcConn connections.GrpcConn) *ConsensusApiLite {
	return &ConsensusApiLite{
		grpcConn: grpcConn,
	}
}

func (c *ConsensusApiLite) Close() error {
	return c.grpcConn.Close()
}

func (c *ConsensusApiLite) GetGenesisDocument(ctx context.Context, chainContext string) (*genesis.Document, error) {
	var rsp genesisEden.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetGenesisDocument", nil, &rsp); err != nil {
		return nil, fmt.Errorf("GetGenesisDocument(eden): %w", err)
	}
	return ConvertGenesis(rsp)
}

func (c *ConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	var rsp genesisEden.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/StateToGenesis", height, &rsp); err != nil {
		return nil, fmt.Errorf("StateToGenesis(%d): %w", height, err)
	}
	return ConvertGenesis(rsp)
}

func (c *ConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	var rsp consensusEden.Block
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetBlock", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetBlock(%d): %w", height, err)
	}
	return (*consensus.Block)(&rsp), nil
}

func (c *ConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	var rsp consensusEden.TransactionsWithResults
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetTransactionsWithResults", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetTransactionsWithResults(%d): %w", height, err)
	}
	txrs := make([]nodeapi.TransactionWithResults, len(rsp.Transactions))

	// convert the response to the nexus-internal data type
	for i, txBytes := range rsp.Transactions {
		var tx consensusTx.SignedTransaction
		if err := cbor.Unmarshal(txBytes, &tx); err != nil {
			log.NewDefaultLogger("eden-consensus-api-lite").Error("malformed consensus transaction, leaving empty",
				"height", height,
				"index", i,
				"tx_bytes", txBytes,
				"err", err,
			)
			tx = consensusTx.SignedTransaction{}
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

func (c *ConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	var rsp beaconEden.EpochTime
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Beacon/GetEpoch", height, &rsp); err != nil {
		return beacon.EpochInvalid, fmt.Errorf("GetEpoch(%d): %w", height, err)
	}
	return rsp, nil
}

func (c *ConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*registryEden.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Registry/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("RegistryEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsEden.Event{Registry: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*stakingEden.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Staking/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("StakingEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsEden.Event{Staking: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*governanceEden.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Governance/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("GovernanceEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsEden.Event{Governance: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*roothashEden.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.RootHash/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("RoothashEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsEden.Event{RootHash: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) GetNodes(ctx context.Context, height int64) ([]nodeapi.Node, error) {
	var rsp []*nodeapi.Node // ABI is stable across Eden and Damask
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Registry/GetNodes", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetNodes(%d): %w", height, err)
	}
	nodes := make([]nodeapi.Node, len(rsp))
	for i, n := range rsp {
		nodes[i] = *n
	}
	return nodes, nil
}

func (c *ConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	var rsp []*schedulerEden.Validator
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Scheduler/GetValidators", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetValidators(%d): %w", height, err)
	}
	validators := make([]nodeapi.Validator, len(rsp))
	for i, v := range rsp {
		validators[i] = nodeapi.Validator{
			ID:          v.ID,
			VotingPower: v.VotingPower,
			// Eden introduces v.EntityID but we have it in the DB already, so no need to track it here also.
		}
	}
	return validators, nil
}

func (c *ConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]nodeapi.Committee, error) {
	var rsp []*schedulerEden.Committee
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

func (c *ConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	var rsp *governanceEden.Proposal
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Governance/Proposal", &governance.ProposalQuery{
		Height:     height,
		ProposalID: proposalID,
	}, &rsp); err != nil {
		return nil, fmt.Errorf("GetProposal(%d, %d): %w", height, proposalID, err)
	}
	return (*nodeapi.Proposal)(convertProposal(rsp)), nil
}

func (c *ConsensusApiLite) GrpcConn() connections.GrpcConn {
	return c.grpcConn
}