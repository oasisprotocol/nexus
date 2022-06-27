package tests

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAfter(t *testing.T) {
	if _, ok := os.LookupEnv("OASIS_INDEXER_E2E"); !ok {
		t.Skip("skipping test since e2e tests are not enabled")
	}

	Init()

	target := int64(8049360)
	out := <-After(target)
	require.LessOrEqual(t, target, out)
}
