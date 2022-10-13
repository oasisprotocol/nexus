// Conversion of the genesis state into SQL statements.

package consensus

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/iancoleman/strcase"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	genesis "github.com/oasisprotocol/oasis-core/go/genesis/api"
	registry "github.com/oasisprotocol/oasis-core/go/registry/api"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"

	"github.com/oasisprotocol/oasis-indexer/log"
)

const bulkInsertBatchSize = 1000

// MigrationGenerator generates migrations for the Oasis Indexer
// target storage.
type MigrationGenerator struct {
	logger *log.Logger
}

// NewMigrationGenerator creates a new migration generator.
func NewMigrationGenerator(logger *log.Logger) *MigrationGenerator {
	return &MigrationGenerator{logger}
}

// WriteGenesisDocumentMigrationOasis3 creates a new migration that re-initializes all
// height-dependent state as per the provided genesis document.
func (mg *MigrationGenerator) WriteGenesisDocumentMigrationOasis3(w io.Writer, document *genesis.Document) error {

	queries, err := mg.ProcessGenesisDocumentOasis3(document)
	if err != nil {
		return err
	}

	for _, query := range queries {
		query += "\n"
		if _, err := io.WriteString(w, query); err != nil {
			return err
		}
	}

	return nil
}

func (mg *MigrationGenerator) ProcessGenesisDocumentOasis3(document *genesis.Document) ([]string, error) {
	var queries []string

	for _, f := range []func(*genesis.Document) ([]string, error){
		mg.addRegistryBackendMigrations,
		mg.addStakingBackendMigrations,
		mg.addGovernanceBackendMigrations,
	} {
		qs, err := f(document)
		if err != nil {
			return nil, err
		}
		queries = append(queries, qs...)
	}

	// Rudimentary templating.
	chainId := strcase.ToSnake(document.ChainID)
	for i, query := range queries {
		queries[i] = strings.ReplaceAll(query, "{{ChainId}}", chainId)
	}
	mg.logger.Info("generated genesis queries", "count", len(queries))

	return queries, nil
}

func (mg *MigrationGenerator) addRegistryBackendMigrations(document *genesis.Document) (queries []string, err error) {
	// Populate entities.
	queries = append(queries, `-- Registry Backend Data
TRUNCATE {{ChainId}}.entities CASCADE;`)
	query := `INSERT INTO {{ChainId}}.entities (id, address)
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
	query += ";"
	queries = append(queries, query)

	// Populate nodes.
	queries = append(queries, `TRUNCATE {{ChainId}}.nodes CASCADE;`)
	query = `INSERT INTO {{ChainId}}.nodes (id, entity_id, expiration, tls_pubkey, tls_next_pubkey, p2p_pubkey, consensus_pubkey, roles)
VALUES
`
	for i, signedNode := range document.Registry.Nodes {
		var node node.Node
		if err := signedNode.Open(registry.RegisterNodeSignatureContext, &node); err != nil {
			return nil, err
		}

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
		if i != len(document.Registry.Nodes)-1 {
			query += ",\n"
		}
	}
	query += ";"
	queries = append(queries, query)

	// Populate runtimes.
	queries = append(queries, `TRUNCATE {{ChainId}}.runtimes CASCADE;`)

	if len(document.Registry.Runtimes) > 0 {
		query = `INSERT INTO {{ChainId}}.runtimes (id, suspended, kind, tee_hardware, key_manager)
VALUES
`
		for i, runtime := range document.Registry.Runtimes {
			keyManager := "none"
			if runtime.KeyManager != nil {
				keyManager = runtime.KeyManager.String()
			}
			query += fmt.Sprintf(
				"\t('%s', %t, '%s', '%s', '%s')",
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
		query += ";"
		queries = append(queries, query)
	}

	if len(document.Registry.SuspendedRuntimes) > 0 {
		query = `INSERT INTO {{ChainId}}.runtimes (id, suspended, kind, tee_hardware, key_manager)
VALUES
`

		for i, runtime := range document.Registry.SuspendedRuntimes {
			keyManager := "none"
			if runtime.KeyManager != nil {
				keyManager = runtime.KeyManager.Hex()
			}
			query += fmt.Sprintf(
				"\t('%s', %t, '%s', '%s', '%s')",
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
		query += ";\n"
		queries = append(queries, query)
	}

	return queries, nil
}

func (mg *MigrationGenerator) addStakingBackendMigrations(document *genesis.Document) (queries []string, err error) {
	// Populate accounts.
	queries = append(queries, `-- Staking Backend Data
TRUNCATE {{ChainId}}.accounts CASCADE;`)

	// Populate special accounts with reserved addresses.
	query := `-- Reserved addresses
INSERT INTO {{ChainId}}.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
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

	i := 0
	for _, address := range sortedKeys(reservedAccounts) {
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
		} else {
			query += ";\n"
		}
		i++
	}
	queries = append(queries, query)

	query = `INSERT INTO {{ChainId}}.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
VALUES
`

	i = 0
	for _, address := range sortedKeys(document.Staking.Ledger) {
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
		i++

		if i%bulkInsertBatchSize == 0 {
			query += ";\n"
			queries = append(queries, query)
			query = `INSERT INTO {{ChainId}}.accounts (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding)
VALUES
`
		} else if i != len(document.Staking.Ledger) {
			query += ",\n"
		}
	}
	query += ";\n"
	queries = append(queries, query)

	// Populate commissions.
	// This likely won't overflow batch limit.
	queries = append(queries, `TRUNCATE {{ChainId}}.commissions CASCADE;`)

	query = `INSERT INTO {{ChainId}}.commissions (address, schedule) VALUES
`

	commissions := make([]string, 0)

	for _, address := range sortedKeys(document.Staking.Ledger) {
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
			query += ";\n"
		}
	}
	queries = append(queries, query)

	// Populate allowances.
	queries = append(queries, `TRUNCATE {{ChainId}}.allowances CASCADE;`)

	foundAllowances := false // in case allowances are empty

	i = 0
	query = ""
	for _, owner := range sortedKeys(document.Staking.Ledger) {
		account := document.Staking.Ledger[owner]
		if len(account.General.Allowances) > 0 && foundAllowances {
			query += ",\n"
		}

		ownerAllowances := make([]string, len(account.General.Allowances))
		j := 0
		for _, beneficiary := range sortedKeys(account.General.Allowances) {
			allowance := account.General.Allowances[beneficiary]
			ownerAllowances[j] = fmt.Sprintf(
				"\t('%s', '%s', %d)",
				owner.String(),
				beneficiary.String(),
				allowance.ToBigInt(),
			)
			j++
		}
		if len(account.General.Allowances) > 0 && !foundAllowances {
			query += `INSERT INTO {{ChainId}}.allowances (owner, beneficiary, allowance)
VALUES
`
			foundAllowances = true
		}

		query += strings.Join(ownerAllowances, ",\n")
		i++
	}
	if foundAllowances {
		query += ";\n"
		queries = append(queries, query)
		query = ""
	}

	// Populate delegations.
	queries = append(queries, `TRUNCATE {{ChainId}}.delegations CASCADE;`)
	query = `INSERT INTO {{ChainId}}.delegations (delegatee, delegator, shares)
VALUES
`
	i = 0
	j := 0
	for _, delegatee := range sortedKeys(document.Staking.Delegations) {
		escrows := document.Staking.Delegations[delegatee]
		k := 0
		for _, delegator := range sortedKeys(escrows) {
			delegation := escrows[delegator]
			query += fmt.Sprintf(
				"\t('%s', '%s', %d)",
				delegatee.String(),
				delegator.String(),
				delegation.Shares.ToBigInt(),
			)
			i++

			if i%bulkInsertBatchSize == 0 {
				query += ";\n"
				queries = append(queries, query)
				query = `INSERT INTO {{ChainId}}.delegations (delegatee, delegator, shares)
VALUES
`
			} else if !(k == len(escrows)-1 && j == len(document.Staking.Delegations)-1) {
				query += ",\n"
			}
			k++
		}
		j++
	}
	query += ";\n"
	queries = append(queries, query)

	// Populate debonding delegations.
	queries = append(queries, `TRUNCATE {{ChainId}}.debonding_delegations CASCADE;`)
	query = `INSERT INTO {{ChainId}}.debonding_delegations (delegatee, delegator, shares, debond_end)
VALUES
`
	i = 0
	for _, delegatee := range sortedKeys(document.Staking.DebondingDelegations) {
		escrows := document.Staking.DebondingDelegations[delegatee]
		delegateeDebondingDelegations := make([]string, 0)
		j := 0
		for _, delegator := range sortedKeys(escrows) {
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
			j++
		}
		query += strings.Join(delegateeDebondingDelegations, ",\n")
		i++

		if i != len(document.Staking.DebondingDelegations) && len(escrows) > 0 {
			query += ",\n"
		}
	}
	query += ";\n"
	queries = append(queries, query)

	return queries, nil
}

func (mg *MigrationGenerator) addGovernanceBackendMigrations(document *genesis.Document) (queries []string, err error) {
	// Populate proposals.
	queries = append(queries, `-- Governance Backend Data
TRUNCATE {{ChainId}}.proposals CASCADE;`)

	if len(document.Governance.Proposals) > 0 {

		// TODO(ennsharma): Extract `executed` for proposal.
		query := `INSERT INTO {{ChainId}}.proposals (id, submitter, state, deposit, handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, cancels, created_at, closes_at, invalid_votes)
VALUES
`

		for i, proposal := range document.Governance.Proposals {
			if proposal.Content.Upgrade != nil {
				query += fmt.Sprintf(
					"\t(%d, '%s', '%s', %d, '%s', '%s', '%s', '%s', %d, %s, %d, %d, %d)",
					proposal.ID,
					proposal.Submitter.String(),
					proposal.State.String(),
					proposal.Deposit.ToBigInt(),
					proposal.Content.Upgrade.Handler,
					proposal.Content.Upgrade.Target.ConsensusProtocol.String(),
					proposal.Content.Upgrade.Target.RuntimeHostProtocol.String(),
					proposal.Content.Upgrade.Target.RuntimeCommitteeProtocol.String(),
					proposal.Content.Upgrade.Epoch,
					"null",
					proposal.CreatedAt,
					proposal.ClosesAt,
					proposal.InvalidVotes,
				)
			} else if proposal.Content.CancelUpgrade != nil {
				query += fmt.Sprintf(
					"\t(%d, '%s', '%s', %d, '%s', '%s', '%s', '%s', '%s', %d, %d, %d, %d)",
					proposal.ID,
					proposal.Submitter.String(),
					proposal.State.String(),
					proposal.Deposit.ToBigInt(),
					"",
					"",
					"",
					"",
					"",
					proposal.Content.CancelUpgrade.ProposalID,
					proposal.CreatedAt,
					proposal.ClosesAt,
					proposal.InvalidVotes,
				)
			}

			if i != len(document.Governance.Proposals)-1 {
				query += ",\n"
			}
		}
		query += ";\n"
		queries = append(queries, query)
	}

	// Populate votes.
	queries = append(queries, `TRUNCATE {{ChainId}}.votes CASCADE;`)

	foundVotes := false // in case votes are empty

	i := 0
	var query string
	for _, proposalID := range sortedIntKeys(document.Governance.VoteEntries) {
		voteEntries := document.Governance.VoteEntries[proposalID]
		if len(voteEntries) > 0 && !foundVotes {
			query = `INSERT INTO {{ChainId}}.votes (proposal, voter, vote)
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
		i++

		if i != len(document.Governance.VoteEntries) && len(voteEntries) > 0 {
			query += ",\n"
		}
	}
	if foundVotes {
		query += ";\n"
		queries = append(queries, query)
	}

	return queries, nil
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

func sortedKeys[V any](m map[staking.Address]V) []staking.Address {
	keys := make([]staking.Address, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
	return keys
}
