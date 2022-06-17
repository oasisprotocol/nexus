package v1

import (
	"fmt"
	"testing"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/oasislabs/oasis-indexer/tests"
	"github.com/stretchr/testify/require"
)

func makeTestEntities(t *testing.T) []v1.Entity {
	return []v1.Entity{
		{
			ID:      "J3ou5m1abrwcbYwTksSj1Hj21QoDjyrSkjB40rEKV4E=",
			Address: "oasis1qqnmppt4j5d2yl584euhn6g2cw9gewdswga9frg4",
			Nodes: []string{
				"dMaOtmpPbdB7ukYfXdo+ssTTCjX2Qspmi1YJIg5Hcr0=",
			},
		},
	}
}

func makeTestNodes(t *testing.T) []v1.Node {
	return []v1.Node{
		{
			ID:              "dMaOtmpPbdB7ukYfXdo+ssTTCjX2Qspmi1YJIg5Hcr0=",
			EntityID:        "J3ou5m1abrwcbYwTksSj1Hj21QoDjyrSkjB40rEKV4E=",
			Expiration:      13405,
			TLSPubkey:       "SpIv9/Io9kooRVO5DWF6hoxYn3YA0PZMgFkldevAd/o=",
			TLSNextPubkey:   "v+0vnXvaZRvJqlSJw5nog9jgHx25aMo89IMVwt+EeN0=",
			P2PPubkey:       "n6zGACCsR+cn5LclGFp/DrHgcKtLrvbG5cgVb7EVSXo=",
			ConsensusPubkey: "U2x+rr2iQJOCsw3Gj/eJspBiF/jVxSek9OD47Gvqw5I=",
			Roles:           "validator",
		},
	}
}

func TestListEntities(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testEntities := makeTestEntities(t)
	endHeight := tests.GenesisHeight + int64(len(testEntities)-1)
	<-tests.After(endHeight)

	var list v1.EntityList
	tests.GetFrom("/consensus/entities", &list)

	check := func(e v1.Entity) bool {
		for _, entity := range list.Entities {
			if e.ID == entity.ID {
				require.Equal(t, e.Address, entity.Address)
				return true
			}
		}
		return false
	}

	for _, entity := range testEntities {
		require.True(t, check(entity))
	}
}

func TestGetEntity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testEntities := makeTestEntities(t)
	endHeight := tests.GenesisHeight + int64(len(testEntities)-1)
	<-tests.After(endHeight)

	var entity v1.Entity
	tests.GetFrom(fmt.Sprintf("/consensus/entities/%s", testEntities[0].ID), &entity)

	require.Equal(t, testEntities[0].ID, entity.ID)
	require.Equal(t, testEntities[0].Address, entity.Address)
	require.Equal(t, testEntities[0].Nodes, entity.Nodes)
}

func TestListEntityNodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testNodes := makeTestNodes(t)
	endHeight := tests.GenesisHeight + int64(len(testNodes)-1)
	<-tests.After(endHeight)

	var list v1.NodeList
	tests.GetFrom(fmt.Sprintf("/consensus/entities/%s/nodes", testNodes[0].EntityID), &list)
	require.Equal(t, len(testNodes), len(list.Nodes))

	for i, node := range list.Nodes {
		require.Equal(t, testNodes[i], node)
	}
}

func TestGetEntityNode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testNodes := makeTestNodes(t)
	endHeight := tests.GenesisHeight + int64(len(testNodes)-1)
	<-tests.After(endHeight)

	var node v1.Node
	tests.GetFrom(fmt.Sprintf("/consensus/entities/%s/nodes/%s", testNodes[0].EntityID, testNodes[0].ID), &node)
	require.Equal(t, testNodes[0], node)
}
