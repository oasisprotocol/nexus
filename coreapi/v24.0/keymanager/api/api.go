// Package api implements the key manager management API and common data types.
package api

import (
	"github.com/oasisprotocol/nexus/coreapi/v24.0/keymanager/secrets"
)

const (
	// ModuleName is a unique module name for the keymanager module.
	ModuleName = "keymanager"
)

// removed var block

// Backend is a key manager management implementation.
// removed interface

// Genesis is the key manager management genesis state.
type Genesis = secrets.Genesis

// removed func
