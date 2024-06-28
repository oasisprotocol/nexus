package client

import (
	"fmt"
	"reflect"

	"github.com/oasisprotocol/oasis-core/go/common/cbor"

	governanceV22 "github.com/oasisprotocol/nexus/coreapi/v22.2.11/governance/api"
	registryV22 "github.com/oasisprotocol/nexus/coreapi/v22.2.11/registry/api"
	roothashV22 "github.com/oasisprotocol/nexus/coreapi/v22.2.11/roothash/api"
	schedulerV22 "github.com/oasisprotocol/nexus/coreapi/v22.2.11/scheduler/api"
	stakingV22 "github.com/oasisprotocol/nexus/coreapi/v22.2.11/staking/api"
	governanceEden "github.com/oasisprotocol/nexus/coreapi/v24.0/governance/api"
	keymanagerEden "github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/api"
	keymanagerSecretsEden "github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/secrets"
	registryEden "github.com/oasisprotocol/nexus/coreapi/v24.0/registry/api"
	roothashEden "github.com/oasisprotocol/nexus/coreapi/v24.0/roothash/api"
	schedulerEden "github.com/oasisprotocol/nexus/coreapi/v24.0/scheduler/api"
	stakingEden "github.com/oasisprotocol/nexus/coreapi/v24.0/staking/api"
	vaultEden "github.com/oasisprotocol/nexus/coreapi/v24.0/vault/api"
)

func extractProposalParametersChange(raw cbor.RawMessage, module string) (interface{}, error) {
	switch module {
	case governanceEden.ModuleName:
		for _, changesType := range []interface{}{
			governanceEden.ConsensusParameterChanges{},
			governanceV22.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown governance consensus parameter changes")
	case keymanagerEden.ModuleName:
		for _, changesType := range []interface{}{
			// No keymanager consensus parameter changes in V22.
			keymanagerSecretsEden.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown keymanager consensus parameter changes")
	case registryEden.ModuleName:
		for _, changesType := range []interface{}{
			registryEden.ConsensusParameterChanges{},
			registryV22.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown registry consensus parameter changes")
	case roothashEden.ModuleName:
		for _, changesType := range []interface{}{
			roothashEden.ConsensusParameterChanges{},
			roothashV22.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown roothash consensus parameter changes")
	case schedulerEden.ModuleName:
		for _, changesType := range []interface{}{
			schedulerEden.ConsensusParameterChanges{},
			schedulerV22.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown scheduler consensus parameter changes")
	case stakingEden.ModuleName:
		for _, changesType := range []interface{}{
			stakingEden.ConsensusParameterChanges{},
			stakingV22.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown staking consensus parameter changes")
	case vaultEden.ModuleName:
		for _, changesType := range []interface{}{
			// No vault in v22.
			vaultEden.ConsensusParameterChanges{},
		} {
			v := reflect.New(reflect.TypeOf(changesType)).Interface()
			if err := cbor.Unmarshal(raw, v); err != nil {
				continue
			}
			return v, nil
		}
		return nil, fmt.Errorf("CBOR unmarshal: unknown staking consensus parameter changes")
	default:
		return nil, fmt.Errorf("unhandled module: %s", module)
	}
}
