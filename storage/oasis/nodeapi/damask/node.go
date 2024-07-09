package damask

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	cmdCommon "github.com/oasisprotocol/nexus/cmd/common"

	// nexus-internal data types.
	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	consensus "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api"
	consensusTx "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	roothash "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	scheduler "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	"github.com/oasisprotocol/nexus/storage/oasis/connections"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"

	// data types for Damask gRPC APIs.
	txResultsDamask "github.com/oasisprotocol/nexus/coreapi/v22.2.11/consensus/api/transaction/results"
)

// ConsensusApiLite provides low-level access to the consensus API of a
// Damask node. Since Nexus is linked against a version of oasis-core that is
// compatible with Damask gRPC API, this struct just trivially wraps the
// convenience methods provided by oasis-core.
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
	var rsp genesis.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetGenesisDocument", nil, &rsp); err != nil {
		return nil, fmt.Errorf("GetGenesisDocument(damask): %w", err)
	}
	return &rsp, nil
}

func (c *ConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	var rsp genesis.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/StateToGenesis", height, &rsp); err != nil {
		return nil, fmt.Errorf("StateToGenesis(%d): %w", height, err)
	}
	return &rsp, nil
}

func (c *ConsensusApiLite) GetConsensusParameters(ctx context.Context, height int64) (*nodeapi.ConsensusParameters, error) {
	// All needed consensus parameters were static in Damask, so avoid querying the node.
	return &nodeapi.ConsensusParameters{
		// Max block gas was 0 (unlimited), the network limited only on max block size in bytes.
		MaxBlockGas: uint64(0),
	}, nil
}

func (c *ConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	var rsp consensus.Block
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetBlock", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetBlock(%d): %w", height, err)
	}
	return &rsp, nil
}

func (c *ConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	var rsp consensus.TransactionsWithResults
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetTransactionsWithResults", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetTransactionsWithResults(%d): %w", height, err)
	}
	txrs := make([]nodeapi.TransactionWithResults, len(rsp.Transactions))

	// convert the response to the nexus-internal data type
	for i, txBytes := range rsp.Transactions {
		var tx consensusTx.SignedTransaction
		if err := cbor.Unmarshal(txBytes, &tx); err != nil {
			cmdCommon.RootLogger().WithModule("damask-consensus-api-lite").Error("malformed consensus transaction, leaving empty",
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
	var rsp beacon.EpochTime
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Beacon/GetEpoch", height, &rsp); err != nil {
		return beacon.EpochInvalid, fmt.Errorf("GetEpoch(%d): %w", height, err)
	}
	return rsp, nil
}

func (c *ConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*registry.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Registry/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("RegistryEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{Registry: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*staking.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Staking/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("StakingEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{Staking: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*governance.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Governance/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("GovernanceEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{Governance: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	var rsp []*roothash.Event
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.RootHash/GetEvents", height, &rsp); err != nil {
		return nil, fmt.Errorf("RoothashEvents(%d): %w", height, err)
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{RootHash: e})
	}
	return events, nil
}

func (c *ConsensusApiLite) RoothashLastRoundResults(ctx context.Context, height int64, runtimeID common.Namespace) (*roothash.RoundResults, error) {
	var rsp roothash.RoundResults
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.RootHash/GetLastRoundResults", &roothash.RuntimeRequest{
		Height:    height,
		RuntimeID: runtimeID,
	}, &rsp); err != nil {
		return nil, fmt.Errorf("RoothashLastRoundResults(%d, %v): %w", height, runtimeID, err)
	}
	return &rsp, nil
}

func (c *ConsensusApiLite) GetNodes(ctx context.Context, height int64) ([]nodeapi.Node, error) {
	var rsp []*nodeapi.Node
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
	var rsp []*scheduler.Validator
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Scheduler/GetValidators", height, &rsp); err != nil {
		return nil, fmt.Errorf("GetValidators(%d): %w", height, err)
	}
	validators := make([]nodeapi.Validator, len(rsp))
	for i, v := range rsp {
		validators[i] = nodeapi.Validator(*v)
	}
	return validators, nil
}

func (c *ConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]nodeapi.Committee, error) {
	var rsp []*scheduler.Committee
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Scheduler/GetCommittees", &scheduler.GetCommitteesRequest{
		Height:    height,
		RuntimeID: runtimeID,
	}, &rsp); err != nil {
		return nil, fmt.Errorf("GetCommittees(%d): %w", height, err)
	}
	committees := make([]nodeapi.Committee, len(rsp))
	for i, c := range rsp {
		committees[i] = nodeapi.Committee{
			Kind:      nodeapi.CommitteeKind(c.Kind), // The enum is compatible between Cobalt and Damask.
			Members:   c.Members,
			RuntimeID: c.RuntimeID,
			ValidFor:  c.ValidFor,
		}
	}
	return committees, nil
}

func (c *ConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	var rsp *governance.Proposal
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Governance/Proposal", &governance.ProposalQuery{
		Height:     height,
		ProposalID: proposalID,
	}, &rsp); err != nil {
		return nil, fmt.Errorf("GetProposal(%d, %d): %w", height, proposalID, err)
	}
	return (*nodeapi.Proposal)(rsp), nil
}

func (c *ConsensusApiLite) GrpcConn() connections.GrpcConn {
	return c.grpcConn
}
