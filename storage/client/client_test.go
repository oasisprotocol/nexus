package client

import (
	"context"

	"github.com/oasisprotocol/oasis-indexer/storage"
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

func (m *MockStorage) QueryRow(ctx context.Context, sql string, args ...interface{}) storage.QueryResult {
	return nil
}

func (m *MockStorage) Shutdown() {}

func (m *MockStorage) Name() string {
	return m.name
}
