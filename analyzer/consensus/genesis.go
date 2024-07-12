// Conversion of the genesis state into SQL statements.

package consensus

import (
	"encoding/json"
	"sort"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/entity"

	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	"github.com/oasisprotocol/nexus/storage"

	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// GenesisProcessor generates sql statements for indexing the genesis state.
type GenesisProcessor struct {
	logger *log.Logger
}

// NewGenesisProcessor creates a new GenesisProcessor.
func NewGenesisProcessor(logger *log.Logger) *GenesisProcessor {
	return &GenesisProcessor{logger}
}

// Process generates SQL statements for indexing the genesis state.
// `nodesOverride` can be nil; if non-nil, the behavior is as if the genesis document contained that set of nodes instead of whatever it contains.
func (mg *GenesisProcessor) Process(document *genesis.Document, nodesOverride []nodeapi.Node) (*storage.QueryBatch, error) {
	batch := &storage.QueryBatch{}

	if err := mg.addRegistryBackendMigrations(batch, document, nodesOverride); err != nil {
		return nil, err
	}

	if err := mg.addStakingBackendMigrations(batch, document); err != nil {
		return nil, err
	}

	mg.addGovernanceBackendMigrations(batch, document)

	mg.logger.Info("generated genesis queries", "count", batch.Len())

	return batch, nil
}

func (mg *GenesisProcessor) addRegistryBackendMigrations(batch *storage.QueryBatch, document *genesis.Document, nodesOverride []nodeapi.Node) error {
	// Populate entities.
	for _, signedEntity := range document.Registry.Entities {
		var entity entity.Entity
		if err := signedEntity.Open(registry.RegisterEntitySignatureContext, &entity); err != nil {
			return err
		}

		batch.Queue(queries.ConsensusEntityUpsert,
			entity.ID.String(),
			staking.NewAddress(entity.ID).String(),
			document.Height,
		)
	}

	// Populate runtimes.
	for _, runtime := range document.Registry.Runtimes {
		batch.Queue(queries.ConsensusRuntimeUpsert,
			runtime.ID.String(),
			false,
			runtime.Kind.String(),
			runtime.TEEHardware.String(),
			common.StringOrNil(runtime.KeyManager),
		)
	}

	for _, runtime := range document.Registry.SuspendedRuntimes {
		batch.Queue(queries.ConsensusRuntimeUpsert,
			runtime.ID.String(),
			true,
			runtime.Kind.String(),
			runtime.TEEHardware.String(),
			common.StringOrNil(runtime.KeyManager),
		)
	}

	// Populate nodes.
	batch.Queue(`DELETE FROM chain.nodes`)
	batch.Queue(`DELETE FROM chain.runtime_nodes`)

	var nodes []nodeapi.Node // What we'll work with; either `overrideNodes` or the nodes from the genesis document.
	if nodesOverride != nil {
		nodes = nodesOverride
	} else {
		for _, signedNode := range document.Registry.Nodes {
			var node nodeapi.Node
			if err := cbor.Unmarshal(signedNode.Blob, &node); err != nil {
				// ^ We do not verify the signatures on the Blob; we trust the node that provided the genesis document.
				//   Also, nexus performs internal lossy data conversions where signatures are lost.
				return err
			}
			if beacon.EpochTime(node.Expiration) < document.Beacon.Base {
				// Node expired before the genesis epoch, skip.
				continue
			}
			nodes = append(nodes, node)
		}
	}

	for _, node := range nodes {
		batch.Queue(queries.ConsensusNodeUpsert,
			node.ID.String(),
			node.EntityID.String(),
			node.Expiration,
			node.TLS.PubKey.String(),
			node.TLS.NextPubKey.String(),
			nil,
			node.P2P.ID.String(),
			nil,
			node.Consensus.ID.String(),
			nil,
			nil,
			node.Roles.String(),
			nil,
			nil,
		)

		for _, runtime := range node.Runtimes {
			batch.Queue(queries.ConsensusRuntimeNodesUpsert,
				runtime.ID.String(),
				node.ID.String(),
			)
		}
	}

	return nil
}

func (mg *GenesisProcessor) addStakingBackendMigrations(batch *storage.QueryBatch, document *genesis.Document) error {
	// Populate accounts.

	// Populate special accounts with reserved addresses.
	reservedAccounts := make(map[staking.Address]*staking.Account)

	commonPoolAccount := staking.Account{
		General: staking.GeneralAccount{
			Balance: document.Staking.CommonPool,
		},
	}
	feeAccumulatorAccount := staking.Account{
		General: staking.GeneralAccount{
			Balance: document.Staking.LastBlockFees,
		},
	}
	governanceDepositsAccount := staking.Account{
		General: staking.GeneralAccount{
			Balance: document.Staking.GovernanceDeposits,
		},
	}

	reservedAccounts[staking.CommonPoolAddress] = &commonPoolAccount
	reservedAccounts[staking.FeeAccumulatorAddress] = &feeAccumulatorAccount
	reservedAccounts[staking.GovernanceDepositsAddress] = &governanceDepositsAccount

	for _, address := range sortedAddressKeys(reservedAccounts) {
		account := reservedAccounts[address]
		batch.Queue(queries.ConsensusAccountUpsert,
			address.String(),
			account.General.Balance.ToBigInt(),
			account.General.Nonce,
			account.Escrow.Active.Balance.ToBigInt(),
			account.Escrow.Active.TotalShares.ToBigInt(),
			account.Escrow.Debonding.Balance.ToBigInt(),
			account.Escrow.Debonding.TotalShares.ToBigInt(),
			document.Time.UTC(),
		)
	}

	for _, address := range sortedAddressKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[address]
		batch.Queue(queries.ConsensusAccountUpsert,
			address.String(),
			account.General.Balance.ToBigInt(),
			account.General.Nonce,
			account.Escrow.Active.Balance.ToBigInt(),
			account.Escrow.Active.TotalShares.ToBigInt(),
			account.Escrow.Debonding.Balance.ToBigInt(),
			account.Escrow.Debonding.TotalShares.ToBigInt(),
			document.Time.UTC(),
		)
	}

	// Populate commissions.
	for _, address := range sortedAddressKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[address]
		if len(account.Escrow.CommissionSchedule.Rates) > 0 || len(account.Escrow.CommissionSchedule.Bounds) > 0 {
			schedule, err := json.Marshal(account.Escrow.CommissionSchedule)
			if err != nil {
				return err
			}

			batch.Queue(queries.ConsensusCommissionsUpsert,
				address.String(),
				string(schedule),
			)
		}
	}

	// Populate allowances.
	batch.Queue(`DELETE FROM chain.allowances`)

	for _, owner := range sortedAddressKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[owner]

		for _, beneficiary := range sortedAddressKeys(account.General.Allowances) {
			allowance := account.General.Allowances[beneficiary]
			batch.Queue(queries.ConsensusAllowanceChangeUpdate,
				owner.String(),
				beneficiary.String(),
				allowance.ToBigInt(),
			)
		}
	}

	// Populate delegations.

	for _, delegatee := range sortedAddressKeys(document.Staking.Delegations) {
		escrows := document.Staking.Delegations[delegatee]
		for _, delegator := range sortedAddressKeys(escrows) {
			delegation := escrows[delegator]
			batch.Queue(queries.ConsensusAddDelegationsUpsert,
				delegatee.String(),
				delegator.String(),
				delegation.Shares.ToBigInt(),
			)
		}
	}

	// Populate debonding delegations.
	batch.Queue(`DELETE FROM chain.debonding_delegations`)
	for _, delegatee := range sortedAddressKeys(document.Staking.DebondingDelegations) {
		escrows := document.Staking.DebondingDelegations[delegatee]
		for _, delegator := range sortedAddressKeys(escrows) {
			debondingDelegations := escrows[delegator]
			for _, debondingDelegation := range debondingDelegations {
				batch.Queue(queries.ConsensusDebondingStartDebondingDelegationsUpsert,
					delegatee.String(),
					delegator.String(),
					debondingDelegation.Shares.ToBigInt(),
					debondingDelegation.DebondEndTime,
				)
			}
		}
	}

	return nil
}

func (mg *GenesisProcessor) addGovernanceBackendMigrations(batch *storage.QueryBatch, document *genesis.Document) {
	// Populate proposals.

	// TODO: Extract `executed` for proposal.
	for _, proposal := range document.Governance.Proposals {
		switch {
		case proposal.Content.Upgrade != nil:
			batch.Queue(queries.ConsensusProposalSubmissionInsert,
				proposal.ID,
				proposal.Submitter.String(),
				proposal.State.String(),
				proposal.Deposit.ToBigInt(),
				proposal.Content.Upgrade.Handler,
				proposal.Content.Upgrade.Target.ConsensusProtocol.String(),
				proposal.Content.Upgrade.Target.RuntimeHostProtocol.String(),
				proposal.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
				proposal.Content.Upgrade.Epoch,
				proposal.CreatedAt,
				proposal.ClosesAt,
				proposal.InvalidVotes,
			)
		case proposal.Content.CancelUpgrade != nil:
			batch.Queue(queries.ConsensusProposalSubmissionCancelInsert,
				proposal.ID,
				proposal.Submitter.String(),
				proposal.State.String(),
				proposal.Deposit.ToBigInt(),
				proposal.Content.CancelUpgrade.ProposalID,
				proposal.CreatedAt,
				proposal.ClosesAt,
				proposal.InvalidVotes,
			)
		case proposal.Content.ChangeParameters != nil:
			batch.Queue(queries.ConsensusProposalSubmissionChangeParametersInsert,
				proposal.ID,
				proposal.Submitter.String(),
				proposal.State.String(),
				proposal.Deposit.ToBigInt(),
				proposal.Content.ChangeParameters.Module,
				proposal.Content.ChangeParameters.Changes,
				proposal.CreatedAt,
				proposal.ClosesAt,
				proposal.InvalidVotes,
			)
		default:
			mg.logger.Warn("unknown proposal content type", "proposal_id", proposal.ID, "content", proposal.Content)
		}
	}

	// Populate votes.
	for _, proposalID := range sortedIntKeys(document.Governance.VoteEntries) {
		voteEntries := document.Governance.VoteEntries[proposalID]
		for _, voteEntry := range voteEntries {
			batch.Queue(queries.ConsensusVoteUpsert,
				proposalID,
				voteEntry.Voter.String(),
				voteEntry.Vote.String(),
				nil,
			)
		}
	}
}

func sortedIntKeys[V any](m map[uint64]V) []uint64 {
	keys := make([]uint64, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func sortedAddressKeys[V any](m map[staking.Address]V) []staking.Address {
	keys := make([]staking.Address, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	return keys
}
