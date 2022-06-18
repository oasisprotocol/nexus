package genesis

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"testing"

	"github.com/iancoleman/strcase"
	staking "github.com/oasisprotocol/oasis-core/go/staking/api"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oasislabs/oasis-indexer/log"
	"github.com/oasislabs/oasis-indexer/storage"
	"github.com/oasislabs/oasis-indexer/storage/oasis"
	"github.com/oasislabs/oasis-indexer/storage/postgres"
	"github.com/oasislabs/oasis-indexer/tests"
)

type TestAccount struct {
	Address   string
	Nonce     uint64
	Available uint64
	Escrow    uint64
	Debonding uint64
}

type TestProposal struct {
	ID               uint64
	Submitter        string
	State            string
	Executed         bool
	Deposit          uint64
	Handler          *string
	CpTargetVersion  *string
	RhpTargetVersion *string
	RcpTargetVersion *string
	UpgradeEpoch     *uint64
	Cancels          *uint64
	CreatedAt        uint64
	ClosesAt         uint64
	InvalidVotes     uint64
}

type TestVote struct {
	Proposal uint64
	Voter    string
	Vote     string
}

func newTargetClient(t *testing.T) (*postgres.Client, error) {
	connString := os.Getenv("CI_TEST_CONN_STRING")
	logger, err := log.NewLogger("cockroach-test", ioutil.Discard, log.FmtJSON, log.LevelInfo)
	assert.Nil(t, err)

	return postgres.NewClient(connString, logger)
}

func newSourceClient(t *testing.T) (*oasis.Client, error) {
	network := &oasisConfig.Network{
		ChainContext: os.Getenv("CI_TEST_CHAIN_CONTEXT"),
		RPC:          os.Getenv("CI_TEST_NODE_RPC"),
	}
	return oasis.NewClient(context.Background(), network)
}

func checkpointRegistryBackend(t *testing.T, source *oasis.Client, target *postgres.Client) (int64, error) {
	return 0, nil
}

func checkpointStakingBackend(t *testing.T, source *oasis.Client, target *postgres.Client) (int64, error) {
	ctx := context.Background()

	doc, err := source.GenesisDocument(ctx)
	assert.Nil(t, err)
	chainID := strcase.ToSnake(doc.ChainID)

	// Prepare checkpoint queries.
	batch := &storage.QueryBatch{}

	batch.Queue("BEGIN WORK;")

	// Lock staking backend tables.
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.accounts IN ROW EXCLUSIVE MODE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.allowances IN ROW EXCLUSIVE MODE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.delegations IN ROW EXCLUSIVE MODE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.debonding_delegations IN ROW EXCLUSIVE MODE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.processed_blocks IN ROW EXCLUSIVE MODE;
	`, chainID))

	// Create checkpoints.
	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.accounts_checkpoint CASCADE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.accounts_checkpoint (address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding, extra_data)
		SELECT address, general_balance, nonce, escrow_balance_active, escrow_total_shares_active, escrow_balance_debonding, escrow_total_shares_debonding, extra_data FROM %s.accounts;
	`, chainID, chainID))

	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.allowances_checkpoint CASCADE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.allowances_checkpoint (owner, beneficiary, allowance)
		SELECT owner, beneficiary, allowance FROM %s.allowances;
	`, chainID, chainID))

	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.delegations_checkpoint CASCADE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.delegations_checkpoint (delegatee, delegator, shares)
		SELECT delegatee, delegator, shares FROM %s.delegations;
	`, chainID, chainID))

	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.debonding_delegations_checkpoint CASCADE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.debonding_delegations_checkpoint (delegatee, delegator, shares, debond_end)
		SELECT delegatee, delegator, shares, debond_end FROM %s.debonding_delegations;
	`, chainID, chainID))

	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.checkpointed_heights (height)
		SELECT height FROM %s.processed_blocks ORDER BY height DESC, processed_time DESC LIMIT 1;
	`, chainID, chainID))

	// Release locks and commit!
	batch.Queue("COMMIT WORK;")

	if err := target.SendBatch(ctx, batch); err != nil {
		return 0, err
	}

	var checkpointHeight int64
	if err := target.QueryRow(ctx, fmt.Sprintf(`
		SELECT height FROM %s.checkpointed_heights
		ORDER BY height DESC LIMIT 1;
	`, chainID)).Scan(&checkpointHeight); err != nil {
		return 0, nil
	}

	return checkpointHeight, nil
}

func checkpointSchedulerBackend(t *testing.T, source *oasis.Client, target *postgres.Client) (int64, error) {
	return 0, nil
}

func checkpointGovernanceBackend(t *testing.T, source *oasis.Client, target *postgres.Client) (int64, error) {
	ctx := context.Background()

	doc, err := source.GenesisDocument(ctx)
	assert.Nil(t, err)
	chainID := strcase.ToSnake(doc.ChainID)

	// Prepare checkpoint queries.
	batch := &storage.QueryBatch{}

	batch.Queue("BEGIN WORK;")

	// Lock governance backend tables.
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.proposals IN ROW EXCLUSIVE MODE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.votes IN ROW EXCLUSIVE MODE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		LOCK TABLE %s.processed_blocks IN ROW EXCLUSIVE MODE;
	`, chainID))

	// Create checkpoints.
	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.proposals_checkpoint CASCADE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.proposals_checkpoint (
			id, submitter, state,
			executed, deposit, handler,
			cp_target_version, rhp_target_version, rcp_target_version,
			upgrade_epoch, cancels, created_at,
			closes_at, invalid_votes, extra_data
		)
		SELECT 
			id, submitter, state,
			executed, deposit, handler,
			cp_target_version, rhp_target_version, rcp_target_version,
			upgrade_epoch, cancels, created_at,
			closes_at, invalid_votes, extra_data
		FROM %s.proposals;
	`, chainID, chainID))

	batch.Queue(fmt.Sprintf(`
		TRUNCATE %s.votes_checkpoint CASCADE;
	`, chainID))
	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.votes_checkpoint (proposal, voter, vote)
		SELECT proposal, voter, vote FROM %s.votes;
	`, chainID, chainID))

	batch.Queue(fmt.Sprintf(`
		INSERT INTO %s.checkpointed_heights (height)
		SELECT height FROM %s.processed_blocks ORDER BY height DESC, processed_time DESC LIMIT 1;
	`, chainID, chainID))

	// Release locks and commit!
	batch.Queue("COMMIT WORK;")

	if err := target.SendBatch(ctx, batch); err != nil {
		return 0, err
	}

	var checkpointHeight int64
	if err := target.QueryRow(ctx, fmt.Sprintf(`
		SELECT height FROM %s.checkpointed_heights
		ORDER BY height DESC LIMIT 1;
	`, chainID)).Scan(&checkpointHeight); err != nil {
		return 0, nil
	}

	return checkpointHeight, nil
}

func TestBlocksSanityCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	ctx := context.Background()

	oasisClient, err := newSourceClient(t)
	require.Nil(t, err)

	postgresClient, err := newTargetClient(t)
	require.Nil(t, err)

	doc, err := oasisClient.GenesisDocument(ctx)
	require.Nil(t, err)
	chainID := strcase.ToSnake(doc.ChainID)

	var latestHeight int64
	err = postgresClient.QueryRow(ctx, fmt.Sprintf(
		`SELECT height FROM %s.blocks ORDER BY height DESC LIMIT 1;`,
		chainID,
	)).Scan(&latestHeight)
	require.Nil(t, err)

	var actualHeightSum int64
	err = postgresClient.QueryRow(ctx, fmt.Sprintf(
		`SELECT SUM(height) FROM %s.blocks WHERE height <= $1;`,
		chainID,
	), latestHeight).Scan(&actualHeightSum)
	require.Nil(t, err)

	// Using formula for sum of first k natural numbers.
	expectedHeightSum := latestHeight*(latestHeight+1)/2 - (tests.GenesisHeight-1)*tests.GenesisHeight/2
	require.Equal(t, expectedHeightSum, actualHeightSum)
}

func TestStakingGenesis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	t.Log("Initializing data stores...")

	ctx := context.Background()

	oasisClient, err := newSourceClient(t)
	assert.Nil(t, err)

	postgresClient, err := newTargetClient(t)
	assert.Nil(t, err)

	doc, err := oasisClient.GenesisDocument(ctx)
	assert.Nil(t, err)
	chainID := strcase.ToSnake(doc.ChainID)

	t.Log("Creating checkpoint...")

	height, err := checkpointStakingBackend(t, oasisClient, postgresClient)
	assert.Nil(t, err)

	t.Log("Fetching genesis state...")

	stakingGenesis, err := oasisClient.StakingGenesis(ctx, height)
	assert.Nil(t, err)

	t.Log("Validating...")

	rows, err := postgresClient.Query(ctx, fmt.Sprintf(
		`SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
				FROM %s.accounts_checkpoint`, chainID),
	)
	require.Nil(t, err)
	for rows.Next() {
		var ta TestAccount
		err = rows.Scan(
			&ta.Address,
			&ta.Nonce,
			&ta.Available,
			&ta.Escrow,
			&ta.Debonding,
		)
		assert.Nil(t, err)

		var address staking.Address
		err = address.UnmarshalText([]byte(ta.Address))
		assert.Nil(t, err)

		a, ok := stakingGenesis.Ledger[address]
		if ok {
			assert.Equal(t, a.General.Nonce, ta.Nonce)
			assert.Equal(t, a.General.Balance.ToBigInt().Uint64(), ta.Available)
		} else {
			t.Logf("Address %s not found in staking ledger.", address.String())
		}

	}

	t.Log("Done!")
}

func TestSchedulerGenesis(t *testing.T) {
	// TODO
}

func TestGovernanceGenesis(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	t.Log("Initializing data stores...")

	ctx := context.Background()

	oasisClient, err := newSourceClient(t)
	assert.Nil(t, err)

	postgresClient, err := newTargetClient(t)
	assert.Nil(t, err)

	doc, err := oasisClient.GenesisDocument(ctx)
	assert.Nil(t, err)
	chainID := strcase.ToSnake(doc.ChainID)

	t.Log("Creating checkpoint...")

	height, err := checkpointGovernanceBackend(t, oasisClient, postgresClient)
	assert.Nil(t, err)

	t.Log("Fetching genesis state...")

	governanceGenesis, err := oasisClient.GovernanceGenesis(ctx, height)
	assert.Nil(t, err)

	t.Log("Validating...")

	expectedProposals := make([]TestProposal, 0)
	for _, p := range governanceGenesis.Proposals {
		if p == nil {
			continue
		}
		var ep TestProposal
		ep.ID = p.ID
		ep.Submitter = p.Submitter.String()
		ep.State = p.State.String()
		ep.Deposit = p.Deposit.ToBigInt().Uint64()
		if p.Content.Upgrade != nil {
			handler := string(p.Content.Upgrade.Handler)
			cpTargetVersion := p.Content.Upgrade.Target.ConsensusProtocol.String()
			rhpTargetVersion := p.Content.Upgrade.Target.RuntimeHostProtocol.String()
			rcpTargetVersion := p.Content.Upgrade.Target.RuntimeCommitteeProtocol.String()
			upgradeEpoch := uint64(p.Content.Upgrade.Epoch)

			ep.Handler = &handler
			ep.CpTargetVersion = &cpTargetVersion
			ep.RhpTargetVersion = &rhpTargetVersion
			ep.RcpTargetVersion = &rcpTargetVersion
			ep.UpgradeEpoch = &upgradeEpoch
		} else if p.Content.CancelUpgrade != nil {
			cancels := p.Content.CancelUpgrade.ProposalID
			ep.Cancels = &cancels
		} else {
			t.Logf("Malformed proposal %d", p.ID)
		}
		ep.CreatedAt = uint64(p.CreatedAt)
		ep.ClosesAt = uint64(p.ClosesAt)
		ep.InvalidVotes = p.InvalidVotes

		expectedProposals = append(expectedProposals, ep)
	}
	sort.Slice(expectedProposals, func(i, j int) bool {
		return expectedProposals[i].ID > expectedProposals[j].ID
	})

	proposalRows, err := postgresClient.Query(ctx, fmt.Sprintf(
		`SELECT id, submitter, state, executed, deposit,
						handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, cancels,
						created_at, closes_at, invalid_votes
				FROM %s.proposals_checkpoint`, chainID),
	)
	require.Nil(t, err)

	actualProposals := make([]TestProposal, 0, len(expectedProposals))
	for proposalRows.Next() {
		var p TestProposal
		err = proposalRows.Scan(
			&p.ID,
			&p.Submitter,
			&p.State,
			&p.Executed,
			&p.Deposit,
			&p.Handler,
			&p.CpTargetVersion,
			&p.RhpTargetVersion,
			&p.RcpTargetVersion,
			&p.UpgradeEpoch,
			&p.Cancels,
			&p.CreatedAt,
			&p.ClosesAt,
			&p.InvalidVotes,
		)
		assert.Nil(t, err)
		actualProposals = append(actualProposals, p)
	}
	sort.Slice(actualProposals, func(i, j int) bool {
		return actualProposals[i].ID > actualProposals[j].ID
	})

	assert.Equal(t, len(expectedProposals), len(actualProposals))
	for i := 0; i < len(expectedProposals); i++ {
		assert.Equal(t, expectedProposals[i], actualProposals[i])
	}

	expectedVotes := make([]TestVote, 0)
	for p, ves := range governanceGenesis.VoteEntries {
		for _, ve := range ves {
			v := TestVote{
				Proposal: p,
				Voter:    ve.Voter.String(),
				Vote:     ve.Vote.String(),
			}
			expectedVotes = append(expectedVotes, v)
		}
	}
	sort.Slice(expectedVotes, func(i, j int) bool {
		if expectedVotes[i].Voter < expectedVotes[j].Voter {
			return true
		}
		if expectedVotes[i].Voter > expectedVotes[j].Voter {
			return true
		}
		return expectedVotes[i].Proposal > expectedVotes[j].Proposal
	})

	voteRows, err := postgresClient.Query(ctx, fmt.Sprintf(
		`SELECT proposal, voter, vote
				FROM %s.votes_checkpoint`, chainID),
	)
	require.Nil(t, err)

	actualVotes := make([]TestVote, 0, len(expectedVotes))
	for voteRows.Next() {
		var v TestVote
		err = voteRows.Scan(
			&v.Proposal,
			&v.Voter,
			&v.Vote,
		)
		assert.Nil(t, err)
		actualVotes = append(actualVotes, v)
	}
	sort.Slice(actualVotes, func(i, j int) bool {
		if actualVotes[i].Voter < actualVotes[j].Voter {
			return true
		}
		if actualVotes[i].Voter > actualVotes[j].Voter {
			return true
		}
		return actualVotes[i].Proposal > actualVotes[j].Proposal
	})

	assert.Equal(t, len(expectedVotes), len(actualVotes))
	for i := 0; i < len(expectedVotes); i++ {
		assert.Equal(t, expectedVotes[i], actualVotes[i])
	}

	t.Log("Done!")
}
