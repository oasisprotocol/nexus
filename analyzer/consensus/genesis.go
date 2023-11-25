// Conversion of the genesis state into SQL statements.

package consensus

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"
	"github.com/oasisprotocol/oasis-core/go/common/entity"

	beacon "github.com/oasisprotocol/nexus/coreapi/v22.2.11/beacon/api"
	genesis "github.com/oasisprotocol/nexus/coreapi/v22.2.11/genesis/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"

	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

const bulkInsertBatchSize = 1000

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
func (mg *GenesisProcessor) Process(document *genesis.Document, nodesOverride []nodeapi.Node) ([]string, error) {
	var queries []string

	qs, err := mg.addRegistryBackendMigrations(document, nodesOverride)
	if err != nil {
		return nil, err
	}
	queries = append(queries, qs...)

	qs, err = mg.addStakingBackendMigrations(document)
	if err != nil {
		return nil, err
	}
	queries = append(queries, qs...)

	qs = mg.addGovernanceBackendMigrations(document)
	queries = append(queries, qs...)

	mg.logger.Info("generated genesis queries", "count", len(queries))

	return queries, nil
}

func (mg *GenesisProcessor) addRegistryBackendMigrations(document *genesis.Document, nodesOverride []nodeapi.Node) (queries []string, err error) {
	// Populate entities.
	queries = append(queries, "-- Registry Backend Data\n")
	query := `INSERT INTO chain.entities (id, address)
VALUES
`
	for i, signedEntity := range document.Registry.Entities {
		var entity entity.Entity
		if err := signedEntity.Open(registry.RegisterEntitySignatureContext, &entity); err != nil {
			return nil, err
		}

		query += fmt.Sprintf(
			"\t('%s', '%s')",
			entity.ID.String(),
			staking.NewAddress(entity.ID).String(),
		)
		if i != len(document.Registry.Entities)-1 {
			query += ",\n"
		}
	}
	query += `
ON CONFLICT (id) DO UPDATE SET address = EXCLUDED.address;`
	queries = append(queries, query)

	// Populate runtimes.
	if len(document.Registry.Runtimes) > 0 {
		query = `INSERT INTO chain.runtimes (id, suspended, kind, tee_hardware, key_manager)
VALUES
`
		for i, runtime := range document.Registry.Runtimes {
			keyManager := "NULL"
			if runtime.KeyManager != nil {
				keyManager = fmt.Sprintf("'%s'", runtime.KeyManager.String())
			}
			query += fmt.Sprintf(
				"\t('%s', %t, '%s', '%s', %s)",
				runtime.ID.String(),
				false,
				runtime.Kind.String(),
				runtime.TEEHardware.String(),
				keyManager,
			)

			if i != len(document.Registry.Runtimes)-1 {
				query += ",\n"
			}
		}
		query += `
ON CONFLICT (id) DO UPDATE SET
	suspended = EXCLUDED.suspended,
	kind = EXCLUDED.kind,
	tee_hardware = EXCLUDED.tee_hardware,
	key_manager = EXCLUDED.key_manager;`
		queries = append(queries, query)
	}

	if len(document.Registry.SuspendedRuntimes) > 0 {
		query = `INSERT INTO chain.runtimes (id, suspended, kind, tee_hardware, key_manager)
VALUES
`

		for i, runtime := range document.Registry.SuspendedRuntimes {
			keyManager := "NULL"
			if runtime.KeyManager != nil {
				keyManager = fmt.Sprintf("'%s'", runtime.KeyManager.String())
			}
			query += fmt.Sprintf(
				"\t('%s', %t, '%s', '%s', %s)",
				runtime.ID.String(),
				true,
				runtime.Kind.String(),
				runtime.TEEHardware.String(),
				keyManager,
			)

			if i != len(document.Registry.SuspendedRuntimes)-1 {
				query += ",\n"
			}
		}
		query += `
ON CONFLICT (id) DO UPDATE SET
	suspended = EXCLUDED.suspended,
	kind = EXCLUDED.kind,
	tee_hardware = EXCLUDED.tee_hardware,
	key_manager = EXCLUDED.key_manager;`
		queries = append(queries, query)
	}

	// Populate nodes.
	queries = append(queries, `DELETE FROM chain.nodes;`)
	queries = append(queries, `DELETE FROM chain.runtime_nodes;`)
	query = `INSERT INTO chain.nodes (id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles)
VALUES
`
	queryRt := "" // Query for populating the chain.runtime_nodes table.

	var nodes []nodeapi.Node // What we'll work with; either `overrideNodes` or the nodes from the genesis document.
	if nodesOverride != nil {
		nodes = nodesOverride
	} else {
		for _, signedNode := range document.Registry.Nodes {
			var node nodeapi.Node
			if err := cbor.Unmarshal(signedNode.Blob, &node); err != nil {
				// ^ We do not verify the signatures on the Blob; we trust the node that provided the genesis document.
				//   Also, nexus performs internal lossy data conversions where signatures are lost.
				return nil, err
			}
			if beacon.EpochTime(node.Expiration) < document.Beacon.Base {
				// Node expired before the genesis epoch, skip.
				continue
			}
			nodes = append(nodes, node)
		}
	}

	for i, node := range nodes {
		query += fmt.Sprintf(
			"\t('%s', '%s', %d, '%s', '%s', '%s', '%s', '%s')",
			node.ID.String(),
			node.EntityID.String(),
			node.Expiration,
			node.TLS.PubKey.String(),
			node.TLS.NextPubKey.String(),
			node.P2P.ID.String(),
			node.Consensus.ID.String(),
			node.Roles.String(),
		)
		if i != len(nodes)-1 {
			query += ",\n"
		}

		for _, runtime := range node.Runtimes {
			if queryRt != "" {
				// There's already a values tuple in the query.
				queryRt += ",\n"
			}
			queryRt += fmt.Sprintf(
				"\t('%s', '%s')",
				runtime.ID.String(),
				node.ID.String(),
			)
		}
	}
	query += ";"
	queries = append(queries, query)

	// There might be no runtime_nodes to insert; create a query only if there are.
	if queryRt != "" {
		queryRt = `INSERT INTO chain.runtime_nodes(runtime_id, node_id)
VALUES
` + queryRt + ";"
		queries = append(queries, queryRt)
	}

	return queries, nil
}

//nolint:gocyclo
func (mg *GenesisProcessor) addStakingBackendMigrations(document *genesis.Document) (queries []string, err error) {
	// Populate accounts.
	queries = append(queries, "-- Staking Backend Data\n")

	// Populate special accounts with reserved addresses.
	query := `-- Reserved addresses
INSERT INTO chain.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
VALUES
`

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

	for i, address := range sortedAddressKeys(reservedAccounts) {
		account := reservedAccounts[address]
		query += fmt.Sprintf(
			"\t('%s', %d, %d, %d, %d, %d, %d)",
			address.String(),
			account.General.Balance.ToBigInt(),
			account.General.Nonce,
			account.Escrow.Active.Balance.ToBigInt(),
			account.Escrow.Active.TotalShares.ToBigInt(),
			account.Escrow.Debonding.Balance.ToBigInt(),
			account.Escrow.Debonding.TotalShares.ToBigInt(),
		)

		if i != len(reservedAccounts)-1 {
			query += ",\n"
		}
	}
	query += `
ON CONFLICT (address) DO UPDATE SET
	general_balance = EXCLUDED.general_balance,
	nonce = EXCLUDED.nonce,
	escrow_balance_active = EXCLUDED.escrow_balance_active,
	escrow_total_shares_active = EXCLUDED.escrow_total_shares_active,
	escrow_balance_debonding = EXCLUDED.escrow_balance_debonding,
	escrow_total_shares_debonding = EXCLUDED.escrow_total_shares_debonding;`
	queries = append(queries, query)

	query = `INSERT INTO chain.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
VALUES
`

	for i, address := range sortedAddressKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[address]
		query += fmt.Sprintf(
			"\t('%s', %d, %d, %d, %d, %d, %d)",
			address.String(),
			account.General.Balance.ToBigInt(),
			account.General.Nonce,
			account.Escrow.Active.Balance.ToBigInt(),
			account.Escrow.Active.TotalShares.ToBigInt(),
			account.Escrow.Debonding.Balance.ToBigInt(),
			account.Escrow.Debonding.TotalShares.ToBigInt(),
		)

		if (i+1)%bulkInsertBatchSize == 0 {
			query += `
ON CONFLICT (address) DO UPDATE SET
	general_balance = EXCLUDED.general_balance,
	nonce = EXCLUDED.nonce,
	escrow_balance_active = EXCLUDED.escrow_balance_active,
	escrow_total_shares_active = EXCLUDED.escrow_total_shares_active,
	escrow_balance_debonding = EXCLUDED.escrow_balance_debonding,
	escrow_total_shares_debonding = EXCLUDED.escrow_total_shares_debonding;
`
			queries = append(queries, query)
			query = `INSERT INTO chain.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
VALUES
`
		} else if i != len(document.Staking.Ledger)-1 {
			query += ",\n"
		}
	}
	query += `
ON CONFLICT (address) DO UPDATE SET
	general_balance = EXCLUDED.general_balance,
	nonce = EXCLUDED.nonce,
	escrow_balance_active = EXCLUDED.escrow_balance_active,
	escrow_total_shares_active = EXCLUDED.escrow_total_shares_active,
	escrow_balance_debonding = EXCLUDED.escrow_balance_debonding,
	escrow_total_shares_debonding = EXCLUDED.escrow_total_shares_debonding;`
	if len(document.Staking.Ledger) > 0 {
		queries = append(queries, query)
	}

	// Populate commissions.
	// This likely won't overflow batch limit.
	query = `INSERT INTO chain.commissions (address, schedule) VALUES
`

	commissions := make([]string, 0)

	for _, address := range sortedAddressKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[address]
		if len(account.Escrow.CommissionSchedule.Rates) > 0 || len(account.Escrow.CommissionSchedule.Bounds) > 0 {
			schedule, err := json.Marshal(account.Escrow.CommissionSchedule)
			if err != nil {
				return nil, err
			}

			commissions = append(commissions, fmt.Sprintf(
				"\t('%s', '%s')",
				address.String(),
				string(schedule),
			))
		}
	}

	for index, commission := range commissions {
		query += commission

		if index != len(commissions)-1 {
			query += ",\n"
		} else {
			query += `
ON CONFLICT (address) DO UPDATE SET
	schedule = EXCLUDED.schedule;`
		}
	}
	if len(commissions) > 0 {
		queries = append(queries, query)
	}

	// Populate allowances.
	queries = append(queries, `DELETE FROM chain.allowances;`)
	foundAllowances := false // in case allowances are empty

	query = ""
	for _, owner := range sortedAddressKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[owner]
		if len(account.General.Allowances) > 0 && foundAllowances {
			query += ",\n"
		}

		ownerAllowances := make([]string, len(account.General.Allowances))
		for j, beneficiary := range sortedAddressKeys(account.General.Allowances) {
			allowance := account.General.Allowances[beneficiary]
			ownerAllowances[j] = fmt.Sprintf(
				"\t('%s', '%s', %d)",
				owner.String(),
				beneficiary.String(),
				allowance.ToBigInt(),
			)
		}
		if len(account.General.Allowances) > 0 && !foundAllowances {
			query += `INSERT INTO chain.allowances (owner, beneficiary, allowance)
VALUES
`
			foundAllowances = true
		}

		query += strings.Join(ownerAllowances, ",\n")
	}
	if foundAllowances {
		query += ";"
		queries = append(queries, query)
	}

	// Populate delegations.
	query = `INSERT INTO chain.delegations (delegatee, delegator, shares)
VALUES
`
	i := 0
	for j, delegatee := range sortedAddressKeys(document.Staking.Delegations) {
		escrows := document.Staking.Delegations[delegatee]
		for k, delegator := range sortedAddressKeys(escrows) {
			delegation := escrows[delegator]
			query += fmt.Sprintf(
				"\t('%s', '%s', %d)",
				delegatee.String(),
				delegator.String(),
				delegation.Shares.ToBigInt(),
			)
			i++

			if i%bulkInsertBatchSize == 0 {
				query += `
ON CONFLICT (delegatee, delegator) DO UPDATE SET
	shares = EXCLUDED.shares;`
				queries = append(queries, query)
				query = `INSERT INTO chain.delegations (delegatee, delegator, shares)
VALUES
`
			} else if !(k == len(escrows)-1 && j == len(document.Staking.Delegations)-1) {
				query += ",\n"
			}
		}
	}
	query += `
ON CONFLICT (delegatee, delegator) DO UPDATE SET
	shares = EXCLUDED.shares;`
	queries = append(queries, query)

	// Populate debonding delegations.
	queries = append(queries, `DELETE FROM chain.debonding_delegations;`)
	query = `INSERT INTO chain.debonding_delegations (delegatee, delegator, shares, debond_end)
VALUES
`
	for i, delegatee := range sortedAddressKeys(document.Staking.DebondingDelegations) {
		escrows := document.Staking.DebondingDelegations[delegatee]
		delegateeDebondingDelegations := make([]string, 0)
		for _, delegator := range sortedAddressKeys(escrows) {
			debondingDelegations := escrows[delegator]
			delegatorDebondingDelegations := make([]string, len(debondingDelegations))
			for k, debondingDelegation := range debondingDelegations {
				delegatorDebondingDelegations[k] = fmt.Sprintf(
					"\t('%s', '%s', %d, %d)",
					delegatee.String(),
					delegator.String(),
					debondingDelegation.Shares.ToBigInt(),
					debondingDelegation.DebondEndTime,
				)
			}
			delegateeDebondingDelegations = append(delegateeDebondingDelegations, delegatorDebondingDelegations...)
		}
		query += strings.Join(delegateeDebondingDelegations, ",\n")

		if i != len(document.Staking.DebondingDelegations)-1 && len(escrows) > 0 {
			query += ",\n"
		}
	}
	query += ";\n"
	if len(document.Staking.DebondingDelegations) > 0 {
		queries = append(queries, query)
	}

	return queries, nil
}

func (mg *GenesisProcessor) addGovernanceBackendMigrations(document *genesis.Document) (queries []string) {
	// Populate proposals.
	queries = append(queries, "-- Governance Backend Data\n")

	if len(document.Governance.Proposals) > 0 {
		// TODO: Extract `executed` for proposal.
		query := `INSERT INTO chain.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, cancels, parameters_change_module, parameters_change, created_at, closes_at, invalid_votes)
VALUES
`

		for i, proposal := range document.Governance.Proposals {
			switch {
			case proposal.Content.Upgrade != nil:
				query += fmt.Sprintf(
					"\t(%d, '%s', '%s', %d, '%s', '%s', '%s', '%s', %d, NULL, NULL, NULL, %d, %d, %d)",
					proposal.ID,
					proposal.Submitter.String(),
					proposal.State.String(),
					proposal.Deposit.ToBigInt(),
					proposal.Content.Upgrade.Handler,
					proposal.Content.Upgrade.Target.ConsensusProtocol.String(),
					proposal.Content.Upgrade.Target.RuntimeHostProtocol.String(),
					proposal.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
					proposal.Content.Upgrade.Epoch,
					// 1 hardcoded NULL for the proposal.Content.CancelUpgrade.ProposalID field.
					// 2 hardcoded NULLs for the proposal.Content.ChangeParameters fields.
					proposal.CreatedAt,
					proposal.ClosesAt,
					proposal.InvalidVotes,
				)
			case proposal.Content.CancelUpgrade != nil:
				query += fmt.Sprintf(
					"\t(%d, '%s', '%s', %d, NULL, NULL, NULL, NULL, NULL, %d, NULL, NULL, %d, %d, %d)",
					proposal.ID,
					proposal.Submitter.String(),
					proposal.State.String(),
					proposal.Deposit.ToBigInt(),
					// 5 hardcoded NULLs for the proposal.Content.Upgrade fields.
					proposal.Content.CancelUpgrade.ProposalID,
					// 2 hardcoded NULLs for the proposal.Content.ChangeParameters fields.
					proposal.CreatedAt,
					proposal.ClosesAt,
					proposal.InvalidVotes,
				)
			case proposal.Content.ChangeParameters != nil:
				query += fmt.Sprintf(
					"\t(%d, '%s', '%s', %d, NULL, NULL, NULL, NULL, NULL, NULL, '%s', decode('%s', 'hex'), %d, %d, %d)",
					proposal.ID,
					proposal.Submitter.String(),
					proposal.State.String(),
					proposal.Deposit.ToBigInt(),
					// 5 hardcoded NULLs for the proposal.Content.Upgrade fields.
					// 1 hardocded NULL for the proposal.Content.CancelUpgrade.ProposalID field.
					proposal.Content.ChangeParameters.Module,
					hex.EncodeToString(proposal.Content.ChangeParameters.Changes),
					proposal.CreatedAt,
					proposal.ClosesAt,
					proposal.InvalidVotes,
				)
			default:
				mg.logger.Warn("unknown proposal content type", "proposal_id", proposal.ID, "content", proposal.Content)
			}

			if i != len(document.Governance.Proposals)-1 {
				query += ",\n"
			}
		}
		query += `
ON CONFLICT (id) DO UPDATE SET
	submitter = EXCLUDED.submitter,
	state = EXCLUDED.state,
	deposit = EXCLUDED.deposit,
	handler = EXCLUDED.handler,
	cp_target_version = EXCLUDED.cp_target_version,
	rhp_target_version = EXCLUDED.rhp_target_version,
	rcp_target_version = EXCLUDED.rcp_target_version,
	upgrade_epoch = EXCLUDED.upgrade_epoch,
	cancels = EXCLUDED.cancels,
	created_at = EXCLUDED.created_at,
	closes_at = EXCLUDED.closes_at,
	invalid_votes = EXCLUDED.invalid_votes;`
		queries = append(queries, query)
	}

	// Populate votes.
	foundVotes := false // in case votes are empty

	var query string
	for i, proposalID := range sortedIntKeys(document.Governance.VoteEntries) {
		voteEntries := document.Governance.VoteEntries[proposalID]
		if len(voteEntries) > 0 && !foundVotes {
			query = `INSERT INTO chain.votes (proposal, voter, vote)
VALUES
`
			foundVotes = true
		}
		votes := make([]string, len(voteEntries))
		for j, voteEntry := range voteEntries {
			votes[j] = fmt.Sprintf(
				"\t(%d, '%s', '%s')",
				proposalID,
				voteEntry.Voter.String(),
				voteEntry.Vote.String(),
			)
		}
		query += strings.Join(votes, ",\n")

		if i != len(document.Governance.VoteEntries)-1 && len(voteEntries) > 0 {
			query += ",\n"
		}
	}
	if foundVotes {
		query += `
ON CONFLICT (proposal, voter) DO UPDATE SET
	vote = EXCLUDED.vote;`
		queries = append(queries, query)
	}

	return queries
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
