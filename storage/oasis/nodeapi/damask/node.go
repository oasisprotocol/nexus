package damask

import (
	"context"

	"github.com/oasisprotocol/oasis-core/go/common"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"

	// indexer-internal data types
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	roothash "github.com/oasisprotocol/oasis-core/go/roothash/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
)

// DamaskConsensusApiLite provides low-level access to the consensus API of a
// Damask node. Since the indexer is linked against a version of oasis-core that is
// compatible with Damask gRPC API, this struct just trivially wraps the
// convenience methods provided by oasis-core.
type DamaskConsensusApiLite struct {
	client consensus.ClientBackend
}

var _ nodeapi.ConsensusApiLite = (*DamaskConsensusApiLite)(nil)

func NewDamaskConsensusApiLite(client consensus.ClientBackend) *DamaskConsensusApiLite {
	return &DamaskConsensusApiLite{
		client: client,
	}
}

func (c *DamaskConsensusApiLite) GetGenesisDocument(ctx context.Context) (*genesis.Document, error) {
	return c.client.GetGenesisDocument(ctx)
}

func (c *DamaskConsensusApiLite) StateToGenesis(ctx context.Context, height int64) (*genesis.Document, error) {
	return c.client.StateToGenesis(ctx, height)
}

func (c *DamaskConsensusApiLite) GetBlock(ctx context.Context, height int64) (*consensus.Block, error) {
	return c.client.GetBlock(ctx, height)
}

func (c *DamaskConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) (*consensus.TransactionsWithResults, error) {
	return c.client.GetTransactionsWithResults(ctx, height)
}

func (c *DamaskConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	return c.client.Beacon().GetEpoch(ctx, height)
}

func (c *DamaskConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]*registry.Event, error) {
	return c.client.Registry().GetEvents(ctx, height)
}

func (c *DamaskConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]*staking.Event, error) {
	return c.client.Staking().GetEvents(ctx, height)
}

func (c *DamaskConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]*governance.Event, error) {
	return c.client.Governance().GetEvents(ctx, height)
}

func (c *DamaskConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]*roothash.Event, error) {
	return c.client.RootHash().GetEvents(ctx, height)
}

func (c *DamaskConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]*scheduler.Validator, error) {
	return c.client.Scheduler().GetValidators(ctx, height)
}

func (c *DamaskConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]*scheduler.Committee, error) {
	return c.client.Scheduler().GetCommittees(ctx, &scheduler.GetCommitteesRequest{
		Height:    height,
		RuntimeID: runtimeID,
	})
}

func (c *DamaskConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*governance.Proposal, error) {
	return c.client.Governance().Proposal(ctx, &governance.ProposalQuery{
		Height:     height,
		ProposalID: proposalID,
	})
}
