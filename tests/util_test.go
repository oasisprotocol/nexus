package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAfter(t *testing.T) {
	SkipUnlessE2E(t)

	Init()

	target := int64(8049360)
	out := <-After(target)
	require.LessOrEqual(t, target, out)
}
