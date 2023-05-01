package damask

import (
	"context"
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	// indexer-internal data types.
	beacon "github.com/oasisprotocol/oasis-core/go/beacon/api"
	consensus "github.com/oasisprotocol/oasis-core/go/consensus/api"
	consensusTx "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governance "github.com/oasisprotocol/oasis-core/go/governance/api"
	scheduler "github.com/oasisprotocol/oasis-core/go/scheduler/api"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis/nodeapi"

	// data types for Damask gRPC APIs.
	txResultsDamask "github.com/oasisprotocol/oasis-core/go/consensus/api/transaction/results"
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

func (c *DamaskConsensusApiLite) Close() error {
	return nil // Nothing to do; c.client does not expose a Close() method despite containing a gRPC connection.
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

func (c *DamaskConsensusApiLite) GetTransactionsWithResults(ctx context.Context, height int64) ([]nodeapi.TransactionWithResults, error) {
	rsp, err := c.client.GetTransactionsWithResults(ctx, height)
	if err != nil {
		return nil, err
	}
	txrs := make([]nodeapi.TransactionWithResults, len(rsp.Transactions))

	// convert the response to the indexer-internal data type
	for i, txBytes := range rsp.Transactions {
		var tx consensusTx.SignedTransaction
		if err := cbor.Unmarshal(txBytes, &tx); err != nil {
			return nil, err
		}
		if rsp.Results[i] == nil {
			return nil, fmt.Errorf("transaction %d (%s) has no result", i, tx.Hash())
		}
		txrs[i] = nodeapi.TransactionWithResults{
			Transaction: tx,
			Result:      convertTxResult(*rsp.Results[i]),
		}
	}
	return txrs, nil
}

func (c *DamaskConsensusApiLite) GetEpoch(ctx context.Context, height int64) (beacon.EpochTime, error) {
	return c.client.Beacon().GetEpoch(ctx, height)
}

func (c *DamaskConsensusApiLite) RegistryEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	rsp, err := c.client.Registry().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{Registry: e})
	}
	return events, nil
}

func (c *DamaskConsensusApiLite) StakingEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	rsp, err := c.client.Staking().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{Staking: e})
	}
	return events, nil
}

func (c *DamaskConsensusApiLite) GovernanceEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	rsp, err := c.client.Governance().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{Governance: e})
	}
	return events, nil
}

func (c *DamaskConsensusApiLite) RoothashEvents(ctx context.Context, height int64) ([]nodeapi.Event, error) {
	rsp, err := c.client.RootHash().GetEvents(ctx, height)
	if err != nil {
		return nil, err
	}
	events := make([]nodeapi.Event, len(rsp))
	for i, e := range rsp {
		events[i] = convertEvent(txResultsDamask.Event{RootHash: e})
	}
	return events, nil
}

func (c *DamaskConsensusApiLite) GetValidators(ctx context.Context, height int64) ([]nodeapi.Validator, error) {
	rsp, err := c.client.Scheduler().GetValidators(ctx, height)
	if err != nil {
		return nil, err
	}
	validators := make([]nodeapi.Validator, len(rsp))
	for i, v := range rsp {
		validators[i] = nodeapi.Validator(*v)
	}
	return validators, nil
}

func (c *DamaskConsensusApiLite) GetCommittees(ctx context.Context, height int64, runtimeID common.Namespace) ([]nodeapi.Committee, error) {
	rsp, err := c.client.Scheduler().GetCommittees(ctx, &scheduler.GetCommitteesRequest{
		Height:    height,
		RuntimeID: runtimeID,
	})
	if err != nil {
		return nil, err
	}
	committees := make([]nodeapi.Committee, len(rsp))
	for i, c := range rsp {
		committees[i] = nodeapi.Committee(*c)
	}
	return committees, nil
}

func (c *DamaskConsensusApiLite) GetProposal(ctx context.Context, height int64, proposalID uint64) (*nodeapi.Proposal, error) {
	rsp, err := c.client.Governance().Proposal(ctx, &governance.ProposalQuery{
		Height:     height,
		ProposalID: proposalID,
	})
	if err != nil {
		return nil, err
	}
	return (*nodeapi.Proposal)(rsp), nil
}
