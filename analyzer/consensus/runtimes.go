package consensus

import (
	"bytes"
	"encoding/hex"

	coreCommon "github.com/oasisprotocol/oasis-core/go/common"
	sdkConfig "github.com/oasisprotocol/oasis-sdk/client-sdk/go/config"

	"github.com/oasisprotocol/nexus/common"
)

func RuntimeFromID(rtid coreCommon.Namespace, network sdkConfig.Network) *common.Runtime {
	for name, pt := range network.ParaTimes.All {
		id, err := hex.DecodeString(pt.ID)
		if err != nil {
			return nil
		}
		if bytes.Equal(id, rtid[:]) {
			return common.Ptr(common.Runtime(name))
		}
	}
	return nil
}
