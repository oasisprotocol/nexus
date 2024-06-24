package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/common/node"
	"github.com/oasisprotocol/oasis-core/go/common"
	"github.com/oasisprotocol/oasis-core/go/common/crypto/signature"
)

// SanityCheck performs a sanity check on the consensus parameters.
// removed func

// SanityCheck performs a sanity check on the consensus parameter changes.
// removed func

// SanityCheck does basic sanity checking on the genesis state.
// removed func

// SanityCheckEntities examines the entities table.
// Returns lookup of entity ID to the entity record for use in other checks.
// removed func

// SanityCheckRuntimes examines the runtimes table.
// removed func

// SanityCheckNodes examines the nodes table.
// Pass lookups of entities and runtimes from SanityCheckEntities
// and SanityCheckRuntimes for cross referencing purposes.
// removed func

// AddStakeClaims adds stake claims for entities and all their registered nodes
// and runtimes.
// removed func

// Runtimes lookup used in sanity checks.
type sanityCheckRuntimeLookup struct {
	runtimes          map[common.Namespace]*Runtime
	suspendedRuntimes map[common.Namespace]*Runtime
	allRuntimes       []*Runtime
}

// removed func

// removed func

// removed func

// removed func

// removed func

// removed func

// Node lookup used in sanity checks.
type sanityCheckNodeLookup struct {
	nodes map[signature.PublicKey]*node.Node

	nodesList []*node.Node
}

// removed func

// removed func

// removed func
