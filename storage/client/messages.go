package client

import (
	"fmt"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	apiTypes "github.com/oasisprotocol/nexus/api/v1/types"
	governance "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	registry "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	staking "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
)

func extractMessageResult(resultRaw cbor.RawMessage, messageType apiTypes.RoothashMessageType) (interface{}, error) {
	switch messageType {
	case "staking.transfer":
		var result staking.TransferResult
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	case "staking.withdraw":
		var result staking.WithdrawResult
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	case "staking.add_escrow":
		var result staking.AddEscrowResult
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	case "staking.reclaim_escrow":
		var result staking.ReclaimEscrowResult
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	case "registry.update_runtime":
		var result registry.Runtime
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	case "governance.cast_vote":
		// result is always nil, but still unmarshal so we can detect changes
		var result *struct{}
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	case "governance.submit_proposal":
		var result governance.Proposal
		if err := cbor.Unmarshal(resultRaw, &result); err != nil {
			return nil, fmt.Errorf("CBOR unmarshal: %w", err)
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unhandled message type %s", messageType)
	}
}
