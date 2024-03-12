package statecheck

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNetworkFromChainContext(t *testing.T) {
	assert.Equal(t, "mainnet", networkFromChainContext("bb3d748def55bdfb797a2ac53ee6ee141e54cd2ab2dc2375f4a0703a178e6e55")) // eden
	assert.Equal(t, "testnet", networkFromChainContext("50304f98ddb656620ea817cc1446c401752a05a249b36c9b90dba4616829977a")) // 2022-03-03
	assert.Panics(t, func() {
		networkFromChainContext("deadbeefdeadbeefdeadbeef1446c401752a05a249b36c9b90dba4616829977a")
	}, "unknown chain context should not map to any network")
}
