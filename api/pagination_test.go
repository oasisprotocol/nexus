package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPaginationWithNoParams tests if the default pagination
// values are set correctly in the default case.
func TestPaginationWithNoParams(t *testing.T) {
	r, err := http.NewRequest("GET", "https://fake-api.com/get-resource", nil)
	assert.Nil(t, err)

	p, err := NewPagination(r)
	assert.Nil(t, err)

	assert.Equal(t, p.Order, DefaultOrder)
	assert.Equal(t, p.Limit, DefaultLimit)
	assert.Equal(t, p.Offset, DefaultOffset)
}

// TestPaginationWithValidParams tests if the pagination values
// are set correctly when providing query params.
func TestPaginationWithValidParams(t *testing.T) {
	limit := uint64(10)
	offset := uint64(20)

	r, err := http.NewRequest("GET", fmt.Sprintf("https://fake-api.com/get-resource?limit=%d&offset=%d", limit, offset), nil)
	assert.Nil(t, err)

	p, err := NewPagination(r)
	assert.Nil(t, err)

	assert.Equal(t, p.Order, DefaultOrder)
	assert.Equal(t, p.Limit, limit)
	assert.Equal(t, p.Offset, offset)
}

// TestPaginationWithInvalidParams tests if the pagination values
// are set correctly when providing invalid query params.
func TestPaginationWithInvalidParams(t *testing.T) {
	limit := "nonsense"

	r, err := http.NewRequest("GET", fmt.Sprintf("https://fake-api.com/get-resource?limit=%s", limit), nil)
	assert.Nil(t, err)

	_, err = NewPagination(r)
	assert.NotNil(t, err)

	offset := -1

	r, err = http.NewRequest("GET", fmt.Sprintf("https://fake-api.com/get-resource?offset=%d", offset), nil)
	assert.Nil(t, err)

	_, err = NewPagination(r)
	assert.NotNil(t, err)
}
