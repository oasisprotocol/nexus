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
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"

	// data types for Cobalt gRPC APIs.
	consensusCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/consensus/api"
	genesisCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/genesis/api"
)

// CobaltConsensusApiLite provides low-level access to the consensus API of a
// Cobalt node. To be able to use the old gRPC API, this struct uses gRPC
// directly, skipping the convenience wrappers provided by oasis-core.
type CobaltConsensusApiLite struct {
	grpcConn *grpc.ClientConn
	// Used as a convenience for calling methods that are ABI-compatible between Cobalt and Damask.
	damaskClient consensus.ClientBackend
}

var _ nodeapi.ConsensusApiLite = (*CobaltConsensusApiLite)(nil)

func NewCobaltConsensusApiLite(grpcConn *grpc.ClientConn, damaskClient consensus.ClientBackend) *CobaltConsensusApiLite {
	return &CobaltConsensusApiLite{
		grpcConn:     grpcConn,
		damaskClient: damaskClient,
	}
}

func (c *CobaltConsensusApiLite) GetGenesisDocument(ctx context.Context) (*genesis.Document, error) {
	var rsp genesisCobalt.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetGenesisDocument", nil, &rsp); err != nil {
		return nil, err
	}
	return ConvertGenesis(rsp), nil
}

func (c *CobaltConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	var rsp genesisCobalt.Document
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/StateToGenesis", height, &rsp); err != nil {
		return nil, err
	}
	return ConvertGenesis(rsp), nil
}

func (c *CobaltConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	rsp, err := c.damaskClient.GetBlock(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("calling GetBlock() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]*nodeapi.TransactionWithResults, error) {
	var rsp consensusCobalt.TransactionsWithResults
	if err := c.grpcConn.Invoke(ctx, "/oasis-core.Consensus/GetTransactionsWithResults", nil, &rsp); err != nil {
		return nil, err
	}
	txrs := make([]*nodeapi.TransactionWithResults, len(rsp.Transactions))

	// convert the response to the indexer-internal data type
	for i, txBytes := range rsp.Transactions {
		var tx consensusTx.SignedTransaction
		if err := cbor.Unmarshal(txBytes, &tx); err != nil {
			return nil, err
		}
		if rsp.Results[i] == nil {
			return nil, fmt.Errorf("transaction %d (%s) has no result", i, tx.Hash())
		}
		txrs[i] = &nodeapi.TransactionWithResults{
			Transaction: tx,
			Result:      convertTxResult(*rsp.Results[i]),
		}
	}
	return txrs, nil
}

func (c *CobaltConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	rsp, err := c.damaskClient.Beacon().GetEpoch(ctx, height)
	if err != nil {
		return beacon.EpochInvalid, fmt.Errorf("calling GetEpoch() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]*registry.Event, error) {
	return []*registry.Event{}, nil
	// TODO: the currently-deployed node does not support this call yet (forbidden via its config)
	rsp, err := c.damaskClient.Registry().GetEvents(ctx, height)
	if err != nil {
		fmt.Printf("error in Registry: %s\n", err)
		return nil, err
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]*staking.Event, error) {
	rsp, err := c.damaskClient.Staking().GetEvents(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("calling StakingEvents() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]*governance.Event, error) {
	rsp, err := c.damaskClient.Governance().GetEvents(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("calling GovernanceEvents() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]*roothash.Event, error) {
	rsp, err := c.damaskClient.RootHash().GetEvents(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("calling RoothashEvents() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]*scheduler.Validator, error) {
	rsp, err := c.damaskClient.Scheduler().GetValidators(ctx, height)
	if err != nil {
		return nil, fmt.Errorf("calling GetValidators() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]*scheduler.Committee, error) {
	rsp, err := c.damaskClient.Scheduler().GetCommittees(ctx, &scheduler.GetCommitteesRequest{
		Height:    height,
		RuntimeID: runtimeID,
	})
	if err != nil {
		return nil, fmt.Errorf("calling GetCommittees() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}

func (c *CobaltConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*governance.Proposal, error) {
	rsp, err := c.damaskClient.Governance().Proposal(ctx, &governance.ProposalQuery{
		Height:     height,
		ProposalID: proposalID,
	})
	if err != nil {
		return nil, fmt.Errorf("calling GetProposal() on Cobalt node using Damask ABI: %w", err)
	}
	return rsp, nil
}
