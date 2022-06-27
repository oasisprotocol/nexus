package common

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPaginationWithNoParams tests if the default pagination
// values are set correctly in the default case.
func TestPaginationWithNoParams(t *testing.T) {
	ctx := context.Background()

	r, err := http.NewRequestWithContext(ctx, "GET", "https://fake-api.com/get-resource", nil)
	require.Nil(t, err)

	p, err := NewPagination(r)
	require.Nil(t, err)

	require.Equal(t, p.Order, DefaultOrder)
	require.Equal(t, p.Limit, DefaultLimit)
	require.Equal(t, p.Offset, DefaultOffset)
}

// TestPaginationWithValidParams tests if the pagination values
// are set correctly when providing query params.
func TestPaginationWithValidParams(t *testing.T) {
	ctx := context.Background()

	limit := uint64(10)
	offset := uint64(20)

	r, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://fake-api.com/get-resource?limit=%d&offset=%d", limit, offset), nil)
	require.Nil(t, err)

	p, err := NewPagination(r)
	require.Nil(t, err)

	require.Equal(t, p.Order, DefaultOrder)
	require.Equal(t, p.Limit, limit)
	require.Equal(t, p.Offset, offset)
}

// TestPaginationWithInvalidParams tests if the pagination values
// are set correctly when providing invalid query params.
func TestPaginationWithInvalidParams(t *testing.T) {
	ctx := context.Background()

	limit := "nonsense"

	r, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://fake-api.com/get-resource?limit=%s", limit), nil)
	require.Nil(t, err)

	_, err = NewPagination(r)
	require.NotNil(t, err)

	offset := -1

	r, err = http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://fake-api.com/get-resource?offset=%d", offset), nil)
	require.Nil(t, err)

	_, err = NewPagination(r)
	require.NotNil(t, err)
}

// TestPaginationWithTooHighLimit tests if the pagination values
// are set correctly when providing query params.
func TestPaginationWithTooHighLimit(t *testing.T) {
	ctx := context.Background()

	limit := uint64(100000000000)
	offset := uint64(20)

	r, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://fake-api.com/get-resource?limit=%d&offset=%d", limit, offset), nil)
	require.Nil(t, err)

	p, err := NewPagination(r)
	require.Nil(t, err)

	require.Equal(t, p.Order, DefaultOrder)
	require.Equal(t, p.Limit, MaximumLimit)
	require.Equal(t, p.Offset, offset)
}
