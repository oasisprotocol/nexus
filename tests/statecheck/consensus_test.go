package statecheck

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/oasisprotocol/oasis-core/go/common/entity"
	"github.com/oasisprotocol/oasis-core/go/common/node"
	consensusAPI "github.com/oasisprotocol/oasis-core/go/consensus/api"
	genesisAPI "github.com/oasisprotocol/oasis-core/go/genesis/api"
	governanceAPI "github.com/oasisprotocol/oasis-core/go/governance/api"
	registryAPI "github.com/oasisprotocol/oasis-core/go/registry/api"
	stakingAPI "github.com/oasisprotocol/oasis-core/go/staking/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage/postgres"
	"github.com/oasisprotocol/nexus/tests"
)

const (
	ConsensusName = "consensus"
)

var ConsensusTables = []string{
	// Registry backend.
	"entities",
	"claimed_nodes",
	"nodes",
	"runtimes",
	// Staking backend.
	"accounts",
	"allowances",

	"debonding_delegations",
	// Governance backend.
	"proposals",
	"votes",
}

type BigInt = common.BigInt

type TestEntity struct {
	ID    string
	Nodes []string
}

type TestNode struct {
	ID              string
	EntityID        string
	Expiration      uint64
	TLSPubkey       string
	P2pPubkey       string
	ConsensusPubkey string
	VrfPubkey       string
	Roles           string
	SoftwareVersion node.SoftwareVersion
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
	Available BigInt
	Escrow    BigInt
	Debonding BigInt

	Allowances map[string]BigInt
}

type TestProposal struct {
	ID               uint64
	Submitter        string
	State            string
	Executed         bool
	Deposit          BigInt
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

func TestBlocksSanityCheck(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_HEALTHCHECK"); !ok {
		t.Skip("skipping test since healthcheck tests are not enabled")
	}

	ctx := context.Background()

	postgresClient, err := newTargetClient(t)
	require.NoError(t, err)

	var latestHeight int64
	err = postgresClient.QueryRow(ctx,
		`SELECT height FROM chain.blocks ORDER BY height DESC LIMIT 1;`,
	).Scan(&latestHeight)
	require.NoError(t, err)

	var actualHeightSum int64
	err = postgresClient.QueryRow(ctx,
		`SELECT SUM(height) FROM chain.blocks WHERE height <= $1;`,
		latestHeight).Scan(&actualHeightSum)
	require.NoError(t, err)

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

	conn, err := newSdkConnection(ctx)
	require.NoError(t, err)
	oasisClient := conn.Consensus()

	postgresClient, err := newTargetClient(t)
	require.NoError(t, err)

	t.Log("Creating snapshot...")
	height, err := snapshotBackends(postgresClient, ConsensusName, ConsensusTables)
	require.NoError(t, err)

	t.Logf("Fetching genesis at height %d...", height)
	genesis := &genesisAPI.Document{}
	if genesisPath := os.Getenv("OASIS_GENESIS_DUMP"); genesisPath != "" {
		t.Log("Reading genesis from dump at", genesisPath)
		gensisJSON, err := os.ReadFile(genesisPath)
		require.NoError(t, err)
		err = json.Unmarshal(gensisJSON, genesis)
		require.NoError(t, err)
		if genesis.Height != height {
			require.NoError(t, fmt.Errorf("height mismatch: %d (in genesis dump) != %d (in DB)", genesis.Height, height))
		}
	} else {
		t.Log("Fetching state dump at height", height, "from node")
		genesis, err = oasisClient.StateToGenesis(ctx, height)
		require.NoError(t, err)
	}
	registryGenesis := &genesis.Registry
	stakingGenesis := &genesis.Staking
	governanceGenesis := &genesis.Governance

	t.Logf("Validating at height %d...", height)
	validateRegistryBackend(t, registryGenesis, oasisClient, postgresClient, height)
	validateStakingBackend(t, stakingGenesis, postgresClient)
	validateGovernanceBackend(t, governanceGenesis, postgresClient)
}

func validateRegistryBackend(t *testing.T, genesis *registryAPI.Genesis, source consensusAPI.ClientBackend, target *postgres.Client, height int64) {
	t.Log("=== Validating registry backend ===")

	validateEntities(t, genesis, target)
	validateNodes(t, genesis, source, target, height)
	validateRuntimes(t, genesis, target)

	t.Log("=== Done validating registry backend ===")
}

func validateEntities(t *testing.T, genesis *registryAPI.Genesis, target *postgres.Client) {
	t.Log("Validating entities...")
	ctx := context.Background()

	expectedEntities := make(map[string]TestEntity)
	for _, se := range genesis.Entities {
		if se == nil {
			continue
		}
		var e entity.Entity
		err := se.Open(registryAPI.RegisterEntitySignatureContext, &e)
		assert.NoError(t, err)

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

	entityRows, err := target.Query(ctx,
		`SELECT id FROM snapshot_consensus.entities`,
	)
	require.NoError(t, err)

	actualEntities := make(map[string]TestEntity)
	for entityRows.Next() {
		var e TestEntity
		err = entityRows.Scan(
			&e.ID,
		)
		assert.NoError(t, err)

		nodeMap := make(map[string]bool)

		// Entities can register nodes.
		// Nodes can also assert that they belong to an entity.
		//
		// Registry backend `StateToGenesis` returns the union of these nodes.
		nodeRowsFromEntity, err := target.Query(ctx,
			`SELECT node_id FROM snapshot_consensus.claimed_nodes WHERE entity_id = $1`,
			e.ID)
		assert.NoError(t, err)
		for nodeRowsFromEntity.Next() {
			var nid string
			err = nodeRowsFromEntity.Scan(
				&nid,
			)
			assert.NoError(t, err)
			nodeMap[nid] = true
		}

		nodeRowsFromNode, err := target.Query(ctx,
			`SELECT id FROM snapshot_consensus.nodes WHERE entity_id = $1`,
			e.ID)
		assert.NoError(t, err)
		for nodeRowsFromNode.Next() {
			var nid string
			err = nodeRowsFromNode.Scan(
				&nid,
			)
			assert.NoError(t, err)
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
			t.Logf("entity %s is missing in nexus", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
	for ka, va := range actualEntities {
		ve, ok := expectedEntities[ka]
		if !ok {
			t.Logf("entity %s found in nexus but not on the chain", ka)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateNodes(t *testing.T, genesis *registryAPI.Genesis, source consensusAPI.ClientBackend, target *postgres.Client, height int64) {
	t.Log("Validating nodes...")
	ctx := context.Background()

	epoch, err := source.Beacon().GetEpoch(ctx, height)
	assert.NoError(t, err)

	expectedNodes := make(map[string]TestNode)
	for _, sn := range genesis.Nodes {
		if sn == nil {
			continue
		}
		var n node.Node
		err := sn.Open(registryAPI.RegisterNodeSignatureContext, &n)
		assert.NoError(t, err)

		if n.IsExpired(uint64(epoch)) {
			// Nexus prunes expired nodes immediately. oasis-client doesn't,
			// so we prune its output here to prevent false mismatches.
			continue
		}

		tn := TestNode{
			ID:              n.ID.String(),
			EntityID:        n.EntityID.String(),
			Expiration:      n.Expiration,
			TLSPubkey:       n.TLS.PubKey.String(),
			P2pPubkey:       n.P2P.ID.String(),
			VrfPubkey:       n.VRF.ID.String(),
			Roles:           n.Roles.String(),
			SoftwareVersion: n.SoftwareVersion,
		}

		expectedNodes[tn.ID] = tn
	}

	rows, err := target.Query(ctx, `
		SELECT
			id, entity_id, expiration,
			tls_pubkey, p2p_pubkey,
			vrf_pubkey, roles, software_version
		FROM
			snapshot_consensus.nodes
		WHERE
			roles LIKE '%validator%'
	`)
	require.NoError(t, err)

	actualNodes := make(map[string]TestNode)
	defer rows.Close()
	for rows.Next() {
		var n TestNode
		err = rows.Scan(
			&n.ID,
			&n.EntityID,
			&n.Expiration,
			&n.TLSPubkey,
			&n.P2pPubkey,
			&n.VrfPubkey,
			&n.Roles,
			&n.SoftwareVersion,
		)
		assert.NoError(t, err)

		if (&node.Node{Expiration: n.Expiration}).IsExpired(uint64(epoch)) {
			// Nexus DB stores some nodes that are expired because
			// an expiration event was never produced for them. Ignore them.
			continue
		}

		actualNodes[n.ID] = n
	}

	assert.Equal(t, len(expectedNodes), len(actualNodes), "wrong number of nodes")
	for ke, ve := range expectedNodes {
		va, ok := actualNodes[ke]
		if !ok {
			t.Logf("node %s is missing in nexus", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
	for ka, va := range actualNodes {
		ve, ok := expectedNodes[ka]
		if !ok {
			t.Logf("node %s found in nexus but not on the chain", ka)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateRuntimes(t *testing.T, genesis *registryAPI.Genesis, target *postgres.Client) {
	t.Log("Validating runtimes...")
	ctx := context.Background()

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

	runtimeRows, err := target.Query(ctx,
		`SELECT id, suspended, kind, tee_hardware, COALESCE(key_manager, 'none') FROM snapshot_consensus.runtimes`,
	)
	require.NoError(t, err)

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
		if err != nil {
			// We want to display err.Error(), or else the message is incomprehensible when it fails.
			require.NoError(t, err, "error scanning runtime row", "errMsg", err.Error())
		}

		actualRuntimes[tr.ID] = tr
	}

	assert.Equal(t, len(expectedRuntimes), len(actualRuntimes))
	for ke, ve := range expectedRuntimes {
		va, ok := actualRuntimes[ke]
		if !ok {
			t.Logf("runtime %s is missing in nexus", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
	for ka, va := range expectedRuntimes {
		ve, ok := actualRuntimes[ka]
		if !ok {
			t.Logf("runtime %s is missing in nexus", ka)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateStakingBackend(t *testing.T, genesis *stakingAPI.Genesis, target *postgres.Client) {
	t.Log("=== Validating staking backend ===")

	validateAccounts(t, genesis, target)

	t.Log("=== Done validating staking backend! ===")
}

func validateAccounts(t *testing.T, genesis *stakingAPI.Genesis, target *postgres.Client) {
	t.Log("Validating accounts...")
	ctx := context.Background()

	acctRows, err := target.Query(ctx,
		`SELECT address, nonce, general_balance, escrow_balance_active, escrow_balance_debonding
				FROM snapshot_consensus.accounts`,
	)
	require.NoError(t, err)
	actualAccts := make(map[string]bool)
	for acctRows.Next() {
		var actualAcct TestAccount
		err = acctRows.Scan(
			&actualAcct.Address,
			&actualAcct.Nonce,
			&actualAcct.Available,
			&actualAcct.Escrow,
			&actualAcct.Debonding,
		)
		assert.NoError(t, err)
		actualAccts[actualAcct.Address] = true

		isReservedAddress := actualAcct.Address == stakingAPI.CommonPoolAddress.String() ||
			actualAcct.Address == stakingAPI.FeeAccumulatorAddress.String() ||
			actualAcct.Address == stakingAPI.GovernanceDepositsAddress.String() ||
			actualAcct.Address == "oasis1qzq8u7xs328puu2jy524w3fygzs63rv3u5967970" // == stakingAPI.BurnAddress.String(); not yet exposed in the released stakingAPI
		if isReservedAddress {
			// Reserved addresses are explicitly not included in the ledger (and thus in the genesis dump).
			continue
		}

		actualAllowances := make(map[string]BigInt)
		allowanceRows, err := target.Query(ctx, `
			SELECT beneficiary, allowance
				FROM snapshot_consensus.allowances
				WHERE owner = $1
			`,
			actualAcct.Address,
		)
		assert.NoError(t, err)
		for allowanceRows.Next() {
			var beneficiary string
			var amount BigInt
			err = allowanceRows.Scan(
				&beneficiary,
				&amount,
			)
			assert.NoError(t, err)
			actualAllowances[beneficiary] = amount
		}
		actualAcct.Allowances = actualAllowances

		var address stakingAPI.Address
		err = address.UnmarshalText([]byte(actualAcct.Address))
		assert.NoError(t, err)

		genesisAcct, ok := genesis.Ledger[address]
		if !ok {
			t.Logf("address %s found in nexus but not on the chain", address.String())
			t.Fail()
			continue
		}

		expectedAllowances := make(map[string]BigInt)
		for beneficiary, amount := range genesisAcct.General.Allowances {
			expectedAllowances[beneficiary.String()] = common.BigIntFromQuantity(amount)
		}

		expectedAcct := TestAccount{
			Address:    address.String(),
			Nonce:      genesisAcct.General.Nonce,
			Available:  common.BigIntFromQuantity(genesisAcct.General.Balance),
			Escrow:     common.BigIntFromQuantity(genesisAcct.Escrow.Active.Balance),
			Debonding:  common.BigIntFromQuantity(genesisAcct.Escrow.Debonding.Balance),
			Allowances: expectedAllowances,
		}
		assert.Equal(t, expectedAcct, actualAcct)
	}
	for addr, genesisAcct := range genesis.Ledger {
		hasBalance := !genesisAcct.General.Balance.IsZero() ||
			!genesisAcct.Escrow.Active.Balance.IsZero() ||
			!genesisAcct.Escrow.Debonding.Balance.IsZero()
		if !hasBalance && genesisAcct.General.Nonce == 0 { // nexus doesn't have to know about this acct
			continue
		}

		if !actualAccts[addr.String()] {
			t.Logf("address %s is missing in nexus", addr.String())
			t.Fail()
		}
	}
}

func validateGovernanceBackend(t *testing.T, genesis *governanceAPI.Genesis, target *postgres.Client) {
	t.Log("=== Validating governance backend ===")

	validateProposals(t, genesis, target)
	validateVotes(t, genesis, target)

	t.Log("=== Done validating governance backend! ===")
}

func validateProposals(t *testing.T, genesis *governanceAPI.Genesis, target *postgres.Client) {
	t.Log("Validating proposals...")
	ctx := context.Background()

	expectedProposals := make(map[uint64]TestProposal)
	for _, p := range genesis.Proposals {
		if p == nil {
			continue
		}
		var ep TestProposal
		ep.ID = p.ID
		ep.Submitter = p.Submitter.String()
		ep.State = p.State.String()
		ep.Deposit = common.BigIntFromQuantity(p.Deposit)

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

	proposalRows, err := target.Query(ctx, `
		SELECT id, submitter, state, executed, deposit,
				handler, cp_target_version, rhp_target_version, rcp_target_version, upgrade_epoch, cancels,
				created_at, closes_at, invalid_votes
		FROM snapshot_consensus.proposals`,
	)
	require.NoError(t, err)

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
		assert.NoError(t, err)
		actualProposals[p.ID] = p
	}

	assert.Equal(t, len(expectedProposals), len(actualProposals))
	for ke, ve := range expectedProposals {
		va, ok := actualProposals[ke]
		if !ok {
			t.Logf("proposal %d is missing in nexus", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
}

func validateVotes(t *testing.T, genesis *governanceAPI.Genesis, target *postgres.Client) {
	t.Log("Validating votes...")
	ctx := context.Background()

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

	voteRows, err := target.Query(ctx, `SELECT proposal, voter, vote FROM snapshot_consensus.votes`)
	require.NoError(t, err)

	actualVotes := make(map[string]TestVote)
	for voteRows.Next() {
		var v TestVote
		err = voteRows.Scan(
			&v.Proposal,
			&v.Voter,
			&v.Vote,
		)
		assert.NoError(t, err)
		actualVotes[makeProposalKey(v)] = v
	}

	assert.Equal(t, len(expectedVotes), len(actualVotes))
	for ke, ve := range expectedVotes {
		va, ok := actualVotes[ke]
		if !ok {
			t.Logf("vote %s is missing in nexus", ke)
			continue
		}
		assert.Equal(t, ve, va)
	}
}
