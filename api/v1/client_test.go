package v1

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasislabs/oasis-indexer/api/common"
	"github.com/oasislabs/oasis-indexer/storage"
)

const (
	queryBase = "SELECT * FROM table"
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

func (m *MockStorage) Shutdown() {}

func (m *MockStorage) Name() string {
	return m.name
}

// TestQueryBuilderBasic simply creates a new QueryBuilder
// and sees if it returns the initial base query when built.
func TestQueryBuilderBasic(t *testing.T) {
	qb := NewQueryBuilder(queryBase, NewMockStorage())
	require.Equal(t, queryBase, qb.String())
}

// TestQueryBuilderPagination tests adding pagination
// to a query.
func TestQueryBuilderPagination(t *testing.T) {
	ctx := context.Background()

	qb := NewQueryBuilder(queryBase, NewMockStorage())

	r, err := http.NewRequestWithContext(ctx, "GET", "https://fake-api.com/get-resource", nil)
	require.Nil(t, err)

	p, err := common.NewPagination(r)
	require.Nil(t, err)

	err = qb.AddPagination(ctx, p)
	require.Nil(t, err)
	require.Equal(t, fmt.Sprintf("%s\n\tORDER BY 1\n\tLIMIT 100\n\tOFFSET 0", queryBase), qb.String())
}

// TestQueryBuilderFilters tests adding filters
// to a query.
func TestQueryBuilderFilters(t *testing.T) {
	ctx := context.Background()

	qb := NewQueryBuilder(queryBase, NewMockStorage())

	filters := []string{
		"n > 10",
		"s = 'hello'",
		"NOT b",
	}
	err := qb.AddFilters(ctx, filters)
	require.Nil(t, err)
	require.Equal(t, fmt.Sprintf("%s\n\tWHERE %s AND %s AND %s", queryBase, filters[0], filters[1], filters[2]), qb.String())
}
