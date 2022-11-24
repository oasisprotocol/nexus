package genesis

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"testing"

	"github.com/iancoleman/strcase"
	"github.com/oasisprotocol/oasis-core/go/common/entity"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	genesisAPI "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governanceAPI "github.com/oasisprotocol/oasis-core/go/governance/api"
	registryAPI "github.com/oasisprotocol/oasis-core/go/registry/api"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
	oasisConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/oasis"
	"github.com/oasisprotocol/oasis-indexer/storage/postgres"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

type TestEntity struct {
	ID    string
	Nodes []string
}

type TestNode struct {
	ID              string
	EntityID        string
	Expiration      uint64
	TLSPubkey       string
	TLSNextPubkey   string
	P2pPubkey       string
	ConsensusPubkey string
	VrfPubkey       string
	Roles           string
	SoftwareVersion string
}

type TestRuntime struct {
	ID          string
	Suspended   bool
	Kind        string
	TeeHardware string
	KeyManager  string
}

type TestAccount struct {
	Address   string
	Nonce     uint64
	Available uint64
	Escrow    uint64
	Debonding uint64

	Allowances map[string]uint64
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
	connString := os.Getenv("HEALTHCHECK_TEST_CONN_STRING")
	logger, err := log.NewLogger("db-test", io.Discard, log.FmtJSON, log.LevelInfo)
	assert.Nil(t, err)

	return postgres.NewClient(connString, logger)
}

func newSourceClientFactory() (*oasis.ClientFactory, error) {
	network := &oasisConfig.Network{
		ChainContext: os.Getenv("HEALTHCHECK_TEST_CHAIN_CONTEXT"),
		RPC:          os.Getenv("HEALTHCHECK_TEST_NODE_RPC"),
	}
	return oasis.NewClientFactory(context.Background(), network)
}

var chainId = "" // Memoization for getChainId(). Assumes all tests access the same chain.
func getChainID(ctx context.Context, t *testing.T, source *oasis.ConsensusClient) string {
	if chainId == "" {
		doc, err := source.GenesisDocument(ctx)
		assert.Nil(t, err)
		chainId = strcase.ToSnake(doc.ChainID)
	}
	return chainId
}

func checkpointBackends(t *testing.T, source *oasis.ConsensusClient, target *postgres.Client) (int64, error) {
	ctx := context.Background()

	chainID := getChainID(ctx, t, source)

	// Create checkpoint tables.
	batch := &storage.QueryBatch{}
	for _, t := range []string{
		// Registry backend.
		"entities",
		"claimed_nodes",
		"nodes",
		"runtimes",
		// Staking backend.
		"accounts",
		"allowances",
		"delegations",
		"debonding_delegations",
		// Governance backend.
		"proposals",
		"votes",
	} {
		batch.Queue(fmt.Sprintf(`
			DROP TABLE IF EXISTS %s.%s_checkpoint CASCADE;
		`, chainID, t))
		batch.Queue(fmt.Sprintf(`
			CREATE TABLE %s.%s_checkpoint AS TABLE %s.%s;
		`, chainID, t, chainID, t))
	}
	batch.Queue(fmt.Sprintf(`
			INSERT INTO %s.checkpointed_heights (height)
				SELECT height FROM %s.processed_blocks ORDER BY height DESC, processed_time DESC LIMIT 1
				ON CONFLICT DO NOTHING;
		`, chainID, chainID))

	if err := target.SendBatch(ctx, batch); err != nil {
		return 0, err
	}

	var checkpointHeight int64
	if err := target.QueryRow(ctx, fmt.Sprintf(`
		SELECT height FROM %s.checkpointed_heights
			ORDER BY height DESC LIMIT 1;
	`, chainID)).Scan(&checkpointHeight); err != nil {
		return 0, err
	}

	return checkpointHeight, nil
}

func TestBlocksSanityCheck(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	ctx := context.Background()

	factory, err := newSourceClientFactory()
	require.Nil(t, err)

	oasisClient, err := factory.Consensus()
	require.Nil(t, err)

	postgresClient, err := newTargetClient(t)
	require.Nil(t, err)

	chainID := getChainID(ctx, t, oasisClient)

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

func TestGenesisFull(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	t.Log("Initializing data stores...")

	ctx := context.Background()

	factory, err := newSourceClientFactory()
	require.Nil(t, err)

	oasisClient, err := factory.Consensus()
	require.Nil(t, err)

	postgresClient, err := newTargetClient(t)
	assert.Nil(t, err)

	t.Log("Creating checkpoint...")
	height, err := checkpointBackends(t, oasisClient, postgresClient)
	assert.Nil(t, err)

	t.Logf("Fetching genesis at height %d...", height)
	var registryGenesis *registryAPI.Genesis
	var stakingGenesis *stakingAPI.Genesis
	var governanceGenesis *governanceAPI.Genesis
	genesisPath := os.Getenv("OASIS_GENESIS_DUMP")
	if genesisPath != "" {
		t.Log("Reading genesis from dump at", genesisPath)
		genesisJson, err := os.ReadFile(genesisPath)
		if err != nil {
			require.Nil(t, err)
		}
		var genesis genesisAPI.Document
		err = json.Unmarshal([]byte(genesisJson), &genesis)
		if err != nil {
			require.Nil(t, err)
		}
		if genesis.Height != height {
			require.Nil(t, fmt.Errorf("height mismatch: %d (in genesis dump) != %d (in DB)", genesis.Height, height))
		}
		registryGenesis = &genesis.Registry
		stakingGenesis = &genesis.Staking
		governanceGenesis = &genesis.Governance
	} else {
		t.Log("Fetching genesis from node", genesisPath)
		registryGenesis, err = oasisClient.RegistryGenesis(ctx, height)
		require.Nil(t, err)
		stakingGenesis, err = oasisClient.StakingGenesis(ctx, height)
		require.Nil(t, err)
		governanceGenesis, err = oasisClient.GovernanceGenesis(ctx, height)
		require.Nil(t, err)
	}

	t.Logf("Validating at height %d...", height)
	validateRegistryBackend(t, registryGenesis, oasisClient, postgresClient)
	validateStakingBackend(t, stakingGenesis, oasisClient, postgresClient)
	validateGovernanceBackend(t, governanceGenesis, oasisClient, postgresClient)
}

func validateRegistryBackend(t *testing.T, genesis *registryAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("=== Validating registry backend ===")

	validateEntities(t, genesis, source, target)
	validateNodes(t, genesis, source, target)
	validateRuntimes(t, genesis, source, target)

	t.Log("=== Done validating registry backend ===")
}

func validateEntities(t *testing.T, genesis *registryAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("Validating entities...")

	ctx := context.Background()
	chainID := getChainID(ctx, t, source)

	expectedEntities := make(map[string]TestEntity)
	for _, se := range genesis.Entities {
		if se == nil {
			continue
		}
		var e entity.Entity
		err := se.Open(registryAPI.RegisterEntitySignatureContext, &e)
		assert.Nil(t, err)

		te := TestEntity{
			ID:    e.ID.String(),
			Nodes: make([]string, len(e.Nodes)),
		}
		for i, n := range e.Nodes {
			te.Nodes[i] = n.String()
		}
		sort.Slice(te.Nodes, func(i, j int) bool {
			return te.Nodes[i] < te.Nodes[j]
		})

		expectedEntities[te.ID] = te
	}

	entityRows, err := target.Query(ctx, fmt.Sprintf(
		`SELECT id FROM %s.entities_checkpoint`, chainID),
	)
	require.Nil(t, err)

	actualEntities := make(map[string]TestEntity)
	for entityRows.Next() {
		var e TestEntity
		err = entityRows.Scan(
			&e.ID,
		)
		assert.Nil(t, err)

		nodeMap := make(map[string]bool)

		// Entities can register nodes.
		// Nodes can also assert that they belong to an entity.
		//
		// Registry backend `StateToGenesis` returns the union of these nodes.
		nodeRowsFromEntity, err := target.Query(ctx, fmt.Sprintf(
			`SELECT node_id FROM %s.claimed_nodes_checkpoint WHERE entity_id = $1`, chainID),
			e.ID)
		assert.Nil(t, err)
		for nodeRowsFromEntity.Next() {
			var nid string
			err = nodeRowsFromEntity.Scan(
				&nid,
			)
			assert.Nil(t, err)
			nodeMap[nid] = true
		}

		nodeRowsFromNode, err := target.Query(ctx, fmt.Sprintf(
			`SELECT id FROM %s.nodes_checkpoint WHERE entity_id = $1`, chainID),
			e.ID)
		assert.Nil(t, err)
		for nodeRowsFromNode.Next() {
			var nid string
			err = nodeRowsFromNode.Scan(
				&nid,
			)
			assert.Nil(t, err)
			nodeMap[nid] = true
		}

		e.Nodes = make([]string, len(nodeMap))

		i := 0
		for n := range nodeMap {
			e.Nodes[i] = n
			i++
		}

		sort.Slice(e.Nodes, func(i, j int) bool {
			return e.Nodes[i] < e.Nodes[j]
		})

		actualEntities[e.ID] = e
	}

	assert.Equal(t, len(expectedEntities), len(actualEntities))
	for ke, ve := range expectedEntities {
		va, ok := actualEntities[ke]
		if !ok {
			t.Logf("entity %s expected, but not found", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
	for ka, va := range actualEntities {
		ve, ok := expectedEntities[ka]
		if !ok {
			t.Logf("entity %s found, but not expected", ka)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateNodes(t *testing.T, genesis *registryAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("Validating nodes...")

	ctx := context.Background()
	chainID := getChainID(ctx, t, source)

	expectedNodes := make(map[string]TestNode)
	for _, sn := range genesis.Nodes {
		if sn == nil {
			continue
		}
		var n node.Node
		err := sn.Open(registryAPI.RegisterNodeSignatureContext, &n)
		assert.Nil(t, err)

		vrfPubkey := ""
		if n.VRF != nil {
			vrfPubkey = n.VRF.ID.String()
		}
		tn := TestNode{
			ID:              n.ID.String(),
			EntityID:        n.EntityID.String(),
			Expiration:      n.Expiration,
			TLSPubkey:       n.TLS.PubKey.String(),
			TLSNextPubkey:   n.TLS.NextPubKey.String(),
			P2pPubkey:       n.P2P.ID.String(),
			VrfPubkey:       vrfPubkey,
			Roles:           n.Roles.String(),
			SoftwareVersion: n.SoftwareVersion,
		}

		expectedNodes[tn.ID] = tn
	}

	rows, err := target.Query(ctx, fmt.Sprintf(
		`SELECT
			id, entity_id, expiration,
			tls_pubkey, tls_next_pubkey, p2p_pubkey,
			vrf_pubkey, roles, software_version
		FROM %s.nodes_checkpoint
		WHERE roles LIKE '%s'`, chainID, "%validator%"),
	)
	require.Nil(t, err)

	actualNodes := make(map[string]TestNode)
	for rows.Next() {
		var n TestNode
		err = rows.Scan(
			&n.ID,
			&n.EntityID,
			&n.Expiration,
			&n.TLSPubkey,
			&n.TLSNextPubkey,
			&n.P2pPubkey,
			&n.VrfPubkey,
			&n.Roles,
			&n.SoftwareVersion,
		)
		assert.Nil(t, err)

		actualNodes[n.ID] = n
	}

	assert.Equal(t, len(expectedNodes), len(actualNodes))
	for ke, ve := range expectedNodes {
		va, ok := actualNodes[ke]
		if !ok {
			t.Logf("node %s expected, but not found", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
	for ka, va := range actualNodes {
		ve, ok := expectedNodes[ka]
		if !ok {
			t.Logf("node %s found, but not expected", ka)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateRuntimes(t *testing.T, genesis *registryAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("Validating runtimes...")

	ctx := context.Background()
	chainID := getChainID(ctx, t, source)

	expectedRuntimes := make(map[string]TestRuntime)
	for _, r := range genesis.Runtimes {
		if r == nil {
			continue
		}

		keyManager := "none"
		if r.KeyManager != nil {
			keyManager = r.KeyManager.String()
		}
		tr := TestRuntime{
			ID:          r.ID.String(),
			Suspended:   false,
			Kind:        r.Kind.String(),
			TeeHardware: r.TEEHardware.String(),
			KeyManager:  keyManager,
		}

		expectedRuntimes[tr.ID] = tr
	}
	for _, r := range genesis.SuspendedRuntimes {
		if r == nil {
			continue
		}

		keyManager := "none"
		if r.KeyManager != nil {
			keyManager = r.KeyManager.String()
		}
		tr := TestRuntime{
			ID:          r.ID.String(),
			Suspended:   true,
			Kind:        r.Kind.String(),
			TeeHardware: r.TEEHardware.String(),
			KeyManager:  keyManager,
		}

		expectedRuntimes[tr.ID] = tr
	}

	runtimeRows, err := target.Query(ctx, fmt.Sprintf(
		`SELECT id, suspended, kind, tee_hardware, key_manager FROM %s.runtimes_checkpoint`, chainID),
	)
	require.Nil(t, err)

	actualRuntimes := make(map[string]TestRuntime)
	for runtimeRows.Next() {
		var tr TestRuntime
		err = runtimeRows.Scan(
			&tr.ID,
			&tr.Suspended,
			&tr.Kind,
			&tr.TeeHardware,
			&tr.KeyManager,
		)
		assert.Nil(t, err)

		actualRuntimes[tr.ID] = tr
	}

	assert.Equal(t, len(expectedRuntimes), len(actualRuntimes))
	for ke, ve := range expectedRuntimes {
		va, ok := actualRuntimes[ke]
		if !ok {
			t.Logf("runtime %s expected, but not found", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
	for ka, va := range expectedRuntimes {
		ve, ok := actualRuntimes[ka]
		if !ok {
			t.Logf("runtime %s expected, but not found", ka)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateStakingBackend(t *testing.T, genesis *stakingAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("=== Validating staking backend ===")

	validateAccounts(t, genesis, source, target)

	t.Log("=== Done validating staking backend! ===")
}

func validateAccounts(t *testing.T, genesis *stakingAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("Validating accounts...")

	ctx := context.Background()
	chainID := getChainID(ctx, t, source)

	acctRows, err := target.Query(ctx, fmt.Sprintf(
		`SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
				FROM %s.accounts_checkpoint`, chainID),
	)
	require.Nil(t, err)
	for acctRows.Next() {
		var a TestAccount
		err = acctRows.Scan(
			&a.Address,
			&a.Nonce,
			&a.Available,
			&a.Escrow,
			&a.Debonding,
		)
		assert.Nil(t, err)

		actualAllowances := make(map[string]uint64)
		allowanceRows, err := target.Query(ctx, fmt.Sprintf(`
			SELECT beneficiary, allowance
				FROM %s.allowances_checkpoint
				WHERE owner = $1
			`, chainID),
			a.Address,
		)
		assert.Nil(t, err)
		for allowanceRows.Next() {
			var beneficiary string
			var amount uint64
			err = allowanceRows.Scan(
				&beneficiary,
				&amount,
			)
			assert.Nil(t, err)
			actualAllowances[beneficiary] = amount
		}
		a.Allowances = actualAllowances

		var address stakingAPI.Address
		err = address.UnmarshalText([]byte(a.Address))
		assert.Nil(t, err)

		acct, ok := genesis.Ledger[address]
		if !ok {
			t.Logf("address %s expected, but not found", address.String())
			continue
		}

		expectedAllowances := make(map[string]uint64)
		for beneficiary, amount := range acct.General.Allowances {
			expectedAllowances[beneficiary.String()] = amount.ToBigInt().Uint64()
		}

		e := TestAccount{
			Address:    address.String(),
			Nonce:      acct.General.Nonce,
			Available:  acct.General.Balance.ToBigInt().Uint64(),
			Escrow:     acct.Escrow.Active.Balance.ToBigInt().Uint64(),
			Debonding:  acct.Escrow.Debonding.Balance.ToBigInt().Uint64(),
			Allowances: expectedAllowances,
		}
		assert.Equal(t, e, a)
	}
}

func validateGovernanceBackend(t *testing.T, genesis *governanceAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("=== Validating governance backend ===")

	validateProposals(t, genesis, source, target)
	validateVotes(t, genesis, source, target)

	t.Log("=== Done validating governance backend! ===")
}

func validateProposals(t *testing.T, genesis *governanceAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("Validating proposals...")

	ctx := context.Background()
	chainID := getChainID(ctx, t, source)

	expectedProposals := make(map[uint64]TestProposal)
	for _, p := range genesis.Proposals {
		if p == nil {
			continue
		}
		var ep TestProposal
		ep.ID = p.ID
		ep.Submitter = p.Submitter.String()
		ep.State = p.State.String()
		ep.Deposit = p.Deposit.ToBigInt().Uint64()

		switch {
		case p.Content.Upgrade != nil:
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
		case p.Content.CancelUpgrade != nil:
			cancels := p.Content.CancelUpgrade.ProposalID
			ep.Cancels = &cancels
		default:
			t.Logf("Malformed proposal %d", p.ID)
			return
		}
		ep.CreatedAt = uint64(p.CreatedAt)
		ep.ClosesAt = uint64(p.ClosesAt)
		ep.InvalidVotes = p.InvalidVotes

		expectedProposals[ep.ID] = ep
	}

	proposalRows, err := target.Query(ctx, fmt.Sprintf(
		`SELECT id, submitter, state, executed, deposit,
						handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, cancels,
						created_at, closes_at, invalid_votes
				FROM %s.proposals_checkpoint`, chainID),
	)
	require.Nil(t, err)

	actualProposals := make(map[uint64]TestProposal)
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
		actualProposals[p.ID] = p
	}

	assert.Equal(t, len(expectedProposals), len(actualProposals))
	for ke, ve := range expectedProposals {
		va, ok := actualProposals[ke]
		if !ok {
			t.Logf("proposal %d expected, but not found", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateVotes(t *testing.T, genesis *governanceAPI.Genesis, source *oasis.ConsensusClient, target *postgres.Client) {
	t.Log("Validating votes...")

	ctx := context.Background()
	chainID := getChainID(ctx, t, source)

	makeProposalKey := func(v TestVote) string {
		return fmt.Sprintf("%d.%s.%s", v.Proposal, v.Voter, v.Vote)
	}

	expectedVotes := make(map[string]TestVote)
	for p, ves := range genesis.VoteEntries {
		for _, ve := range ves {
			v := TestVote{
				Proposal: p,
				Voter:    ve.Voter.String(),
				Vote:     ve.Vote.String(),
			}
			expectedVotes[makeProposalKey(v)] = v
		}
	}

	voteRows, err := target.Query(ctx, fmt.Sprintf(
		`SELECT proposal, voter, vote
				FROM %s.votes_checkpoint`, chainID),
	)
	require.Nil(t, err)

	actualVotes := make(map[string]TestVote)
	for voteRows.Next() {
		var v TestVote
		err = voteRows.Scan(
			&v.Proposal,
			&v.Voter,
			&v.Vote,
		)
		assert.Nil(t, err)
		actualVotes[makeProposalKey(v)] = v
	}

	assert.Equal(t, len(expectedVotes), len(actualVotes))
	for ke, ve := range expectedVotes {
		va, ok := actualVotes[ke]
		if !ok {
			t.Logf("vote %s expected, but not found", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
}
