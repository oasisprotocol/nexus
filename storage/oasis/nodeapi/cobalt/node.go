package cobalt

import (
	"context"

	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"
	"google.golang.org/grpc"

	// indexer-internal data types
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	// data types for Cobalt gRPC APIs
	genesisCobalt "github.com/oasisprotocol/oasis-indexer/coreapi/v21.1.1/genesis/api"
)

// CobaltConsensusApiLite provides low-level access to the consensus API of a
// Cobalt node. To be able to use the old gRPC API, this struct uses gRPC
// directly, skipping the convenience wrappers provided by oasis-core.
type CobaltConsensusApiLite struct {
	grpcConn grpc.ClientConn
}

var _ nodeapi.ConsensusApiLite = (*CobaltConsensusApiLite)(nil)

func NewCobaltConsensusApiLite(grpcConn grpc.ClientConn) *CobaltConsensusApiLite {
	return &CobaltConsensusApiLite{
		grpcConn: grpcConn,
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
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) (*consensus.TransactionsWithResults, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]*registry.Event, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]*staking.Event, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]*governance.Event, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]*roothash.Event, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]*scheduler.Validator, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]*scheduler.Committee, error) {
	panic("not implemented") // TODO: Implement
}

func (c *CobaltConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*governance.Proposal, error) {
	panic("not implemented") // TODO: Implement
}
