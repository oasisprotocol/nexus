package v1

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	storage "github.com/oasisprotocol/nexus/storage/client"
	"github.com/oasisprotocol/nexus/tests"
)

func makeTestEntities() []storage.Entity {
	return []storage.Entity{
		{
			ID:      "gb8SHLeDc69Elk7OTfqhtVgE2sqxrBCDQI84xKR+Bjg=",
			Address: "oasis1qpgl52u29wy4hjla89f46ntkn2qsa6zpdvhv6s6n",
			Nodes: []string{
				"5RIMVgnsN1D/HdvNxXCpE+lWH5U/SGYUrYsvhsTMbyA=",
			},
		},
	}
}

func makeTestNodes() []storage.Node {
	return []storage.Node{
		{
			ID:              "5RIMVgnsN1D/HdvNxXCpE+lWH5U/SGYUrYsvhsTMbyA=",
			EntityID:        "gb8SHLeDc69Elk7OTfqhtVgE2sqxrBCDQI84xKR+Bjg=",
			TLSPubkey:       "ULLgoIQNFizH5l6O3YpK6+pD6JYiJ/0bEj0OsXP4x7k=",
			TLSNextPubkey:   "drjBggoGlujuzfoCV/aqhIKrwCaQdclE+Pc/XLx7a2I=",
			P2PPubkey:       "ZjerSSlcR3yrd4dff3ZjoPI6TvMivoiwyvNWSXsYzWA=",
			ConsensusPubkey: "s7peyC7dJcqo58KIMJXNQTwIivRPX7OQX0h/eOUs/cU=",
			Roles:           "validator",
		},
	}
}

func escape(s string) string {
	return url.PathEscape(s)
}

func TestListEntities(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testEntities := makeTestEntities()
	endHeight := tests.GenesisHeight + int64(len(testEntities)-1)
	<-tests.After(endHeight)

	var list storage.EntityList
	err := tests.GetFrom("/consensus/entities", &list)
	require.NoError(t, err)

	check := func(e storage.Entity) bool {
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
	tests.SkipUnlessE2E(t)

	tests.Init()

	testEntities := makeTestEntities()
	endHeight := tests.GenesisHeight + int64(len(testEntities)-1)
	<-tests.After(endHeight)

	var entity storage.Entity
	err := tests.GetFrom(fmt.Sprintf("/consensus/entities/%s", escape(testEntities[0].ID)), &entity)
	require.NoError(t, err)

	require.Equal(t, testEntities[0].ID, entity.ID)
	require.Equal(t, testEntities[0].Address, entity.Address)
	require.Equal(t, testEntities[0].Nodes, entity.Nodes)
}

func TestListEntityNodes(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testNodes := makeTestNodes()
	endHeight := tests.GenesisHeight + int64(len(testNodes)-1)
	<-tests.After(endHeight)

	var list storage.NodeList
	err := tests.GetFrom(fmt.Sprintf("/consensus/entities/%s/nodes", escape(testNodes[0].EntityID)), &list)
	require.NoError(t, err)
	require.Equal(t, len(testNodes), len(list.Nodes))

	for i, node := range list.Nodes {
		// The expiration is dynamic, until we have oasis-net-runner with a halt epoch.
		testNodes[i].Expiration = node.Expiration
		require.Equal(t, testNodes[i], node)
	}
}

func TestGetEntityNode(t *testing.T) {
	tests.SkipUnlessE2E(t)

	tests.Init()

	testNodes := makeTestNodes()
	endHeight := tests.GenesisHeight + int64(len(testNodes)-1)
	<-tests.After(endHeight)

	var node storage.Node
	err := tests.GetFrom(fmt.Sprintf("/consensus/entities/%s/nodes/%s", escape(testNodes[0].EntityID), escape(testNodes[0].ID)), &node)
	require.NoError(t, err)
	// The expiration is dynamic, until we have oasis-net-runner with a halt epoch.
	testNodes[0].Expiration = node.Expiration
	require.Equal(t, testNodes[0], node)
}
