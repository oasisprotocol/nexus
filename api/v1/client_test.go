package v1

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/oasislabs/oasis-block-indexer/go/api/common"
	"github.com/oasislabs/oasis-block-indexer/go/storage"
)

// MockStorage is a mock object that implements the
// storage.TargetStorage interface.
type MockStorage struct {
	name string
}

func NewMockStorage() *MockStorage {
	return &MockStorage{"mock"}
}

func (m *MockStorage) SendBatch(ctx context.Context, batch *storage.QueryBatch) error {
	return nil
}

func (m *MockStorage) Query(ctx context.Context, sql string, args ...interface{}) (storage.QueryResults, error) {
	return nil, nil
}

func (m *MockStorage) QueryRow(ctx context.Context, sql string, args ...interface{}) (storage.QueryResult, error) {
	return nil, nil
}

func (m *MockStorage) Name() string {
	return m.name
}

// TestQueryBuilderBasic simply creates a new QueryBuilder
// and sees if it returns the initial base query when built.
func TestQueryBuilderBasic(t *testing.T) {
	base := "SELECT * FROM table"
	qb := NewQueryBuilder(base, NewMockStorage())
	assert.Equal(t, base, qb.String())
}

// TestQueryBuilderPagination tests adding pagination
// to a query.
func TestQueryBuilderPagination(t *testing.T) {
	base := "SELECT * FROM table"
	ctx := context.Background()

	qb := NewQueryBuilder(base, NewMockStorage())

	r, err := http.NewRequest("GET", "https://fake-api.com/get-resource", nil)
	assert.Nil(t, err)

	p, err := common.NewPagination(r)
	assert.Nil(t, err)

	err = qb.AddPagination(ctx, p)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n\tORDER BY 1\n\tLIMIT 100\n\tOFFSET 0", base), qb.String())
}

// TestQueryBuilderFilters tests adding filters
// to a query.
func TestQueryBuilderFilters(t *testing.T) {
	base := "SELECT * FROM table"
	ctx := context.Background()

	qb := NewQueryBuilder(base, NewMockStorage())

	filters := []string{
		"n > 10",
		"s = 'hello'",
		"NOT b",
	}
	err := qb.AddFilters(ctx, filters)
	assert.Nil(t, err)
	assert.Equal(t, fmt.Sprintf("%s\n\tWHERE %s AND %s AND %s", base, filters[0], filters[1], filters[2]), qb.String())
}
