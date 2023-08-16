// The nodeapi package provides a low-level interface to the Oasis node API.
// The types that it exposes are simplified versions of the types exposed by
// oasis-core Damask: The top-level type structs are defined in api.go, and
// the types of their fields are almost universally directly the types exposed
// by oasis-core Damask. The reason is that as oasis-core evolves, Damask types
// are mostly able to represent all the information from Cobalt, plus some.
//
// This file contains the exceptions to the above rule: It provides substitute
// types for those Damask types that cannot express Cobalt information.

package nodeapi

import "fmt"

// Copy-pasted from Cobalt; Damask does not support the "storage" kind.
type CommitteeKind uint8

const (
	KindInvalid         CommitteeKind = 0
	KindComputeExecutor CommitteeKind = 1
	KindStorage         CommitteeKind = 2

	// MaxCommitteeKind is a dummy value used for iterating all committee kinds.
	MaxCommitteeKind = 3

	KindInvalidName         = "invalid"
	KindComputeExecutorName = "executor"
	KindStorageName         = "storage"
)

// MarshalText encodes a CommitteeKind into text form.
func (k CommitteeKind) MarshalText() ([]byte, error) {
	switch k {
	case KindInvalid:
		return []byte(KindInvalidName), nil
	case KindComputeExecutor:
		return []byte(KindComputeExecutorName), nil
	case KindStorage:
		return []byte(KindStorageName), nil
	default:
		return nil, fmt.Errorf("invalid role: %d", k)
	}
}

// UnmarshalText decodes a text slice into a CommitteeKind.
func (k *CommitteeKind) UnmarshalText(text []byte) error {
	switch string(text) {
	case KindComputeExecutorName:
		*k = KindComputeExecutor
	case KindStorageName:
		*k = KindStorage
	default:
		return fmt.Errorf("invalid role: %s", string(text))
	}
	return nil
}
