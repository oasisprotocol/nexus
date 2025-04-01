//go:build tools
// +build tools

// This "fake" package is never built. It exists solely to document our build tools:
// to force `go mod` to download them, to prevent GitHub dependabot from removing them from our
// dependencies list, etc.
//
// This is the officially recommended way to depend on tools in Go modules. See
//   https://github.com/golang/go/issues/25922#issuecomment-412992431

package tools

import (
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
)
