package consensus

import (
	"fmt"
	"testing"
	"time"

	v1 "github.com/oasislabs/oasis-indexer/api/v1"
	"github.com/oasislabs/oasis-indexer/tests"
	"github.com/stretchr/testify/require"
)

type TestBlock struct {
	Height    int64
	Hash      string
	Timestamp time.Time
}

func makeTestBlocks(t *testing.T) []TestBlock {
	hashes := []string{
		"0a29ac21fa69bb9e43e5cb25d10826ff3946f1ce977e82f99a2614206a50765c",
		"21d4e61a4318d66a2b5cb0e6869604fd5e5d1fcf3d0778d9b229046f2f1ff490",
		"339e93ba66b7e9268cb1e447a970df77fd37d05bf856c60ae853aa8dbf840483",
		"a65b5685e02375f7f155f7835d81024b8ce90430b7c98d114081bb1104b4c9ed",
		"703fdac6dc2508c28415e593a932492ee59befbe26ce648235f8d1b3325c5756",
		"0e570ffb7db0ab62a21367a709047a942959f00fe5e0c5d00a13e10058c8ee36",
	}
	timestamps := []string{
		"2022-04-11T09:30:00Z",
		"2022-04-11T11:00:08Z",
		"2022-04-11T11:00:27Z",
		"2022-04-11T11:00:33Z",
		"2022-04-11T11:00:39Z",
		"2022-04-11T11:00:45Z",
	}
	require.Equal(t, len(hashes), len(timestamps))

	blocks := make([]TestBlock, 0, len(hashes))
	for i := 0; i < len(hashes); i++ {
		timestamp, err := time.Parse(time.RFC3339, timestamps[i])
		require.Nil(t, err)
		blocks = append(blocks, TestBlock{
			Height:    tests.GenesisHeight + int64(i),
			Hash:      hashes[i],
			Timestamp: timestamp,
		})
	}
	return blocks
}

func TestListBlocks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testBlocks := makeTestBlocks(t)
	endHeight := tests.GenesisHeight + int64(len(testBlocks)-1)
	<-tests.After(endHeight)

	var list v1.BlockList
	tests.GetFrom(fmt.Sprintf("/consensus/blocks?from=%d&to=%d", tests.GenesisHeight, endHeight), &list)
	require.Equal(t, len(testBlocks), len(list.Blocks))

	for i, block := range list.Blocks {
		require.Equal(t, tests.GenesisHeight+int64(i), block.Height)
		require.Equal(t, testBlocks[i].Hash, block.Hash)
		require.Equal(t, testBlocks[i].Timestamp, block.Timestamp)
	}
}

func TestGetBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping testing in short mode")
	}

	tests.Init()

	testBlocks := makeTestBlocks(t)
	endHeight := tests.GenesisHeight + int64(len(testBlocks)-1)
	<-tests.After(endHeight)

	var block v1.Block
	tests.GetFrom(fmt.Sprintf("/consensus/blocks/%d", endHeight), &block)
	require.Equal(t, endHeight, block.Height)
	require.Equal(t, testBlocks[len(testBlocks)-1].Hash, block.Hash)
	require.Equal(t, testBlocks[len(testBlocks)-1].Timestamp, block.Timestamp)
}
