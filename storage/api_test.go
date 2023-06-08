package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/oasis-indexer/storage"
	"github.com/oasisprotocol/oasis-indexer/storage/postgres/testutil"
	"github.com/oasisprotocol/oasis-indexer/tests"
)

func TestInvalidText(t *testing.T) {
	tests.SkipIfShort(t)
	client := testutil.NewTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Ensure database is empty before running the test.
	require.NoError(t, client.Wipe(ctx), "failed to wipe database")

	// Test that invalid text is rejected.
	row, err := client.Query(ctx, `
		CREATE TABLE test (
			test TEXT
		);
	`)
	require.NoError(t, err, "failed to create test table")
	row.Close()

	// Test strings.
	for _, tc := range []struct {
		text  string
		valid bool
	}{
		{"", true},
		{"a", true},
		{"a\000\000b", false},
		{"a\000", false},
		{"\xc5z", false},
	} {
		switch tc.valid {
		case true:
			row, err = client.Query(ctx, `
				INSERT INTO test (test) VALUES ($1);
			`, tc.text)
			require.NoError(t, err, "failed to execute query", "text", tc.text)
			row.Close()

			// Ensure that the text is inserted and that the IsValidText() function allows the string.
			require.NoError(t, row.Err(), "failed to insert valid text", "text", tc.text)
			require.True(t, storage.IsValidText(tc.text), "IsValidText() returned false for valid text", "text", tc.text)
		case false:
			row, err = client.Query(ctx, `
				INSERT INTO test (test) VALUES ($1);
			`, tc.text)
			require.NoError(t, err, "failed to execute query", "text", tc.text)
			row.Close()

			// Ensure that the text was not inserted and that the IsValidText() function rejects the string.
			require.Error(t, row.Err(), "didn't fail to insert invalid text", "text", tc.text)
			require.False(t, storage.IsValidText(tc.text), "IsValidText() returned true for invalid text", "text", tc.text)
		}
	}
}
