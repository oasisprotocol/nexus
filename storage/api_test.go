package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/postgres/testutil"
	"github.com/oasisprotocol/nexus/tests"
)

func TestSantizeString(t *testing.T) {
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
		text     string
		valid    bool
		santized string
	}{
		{"", true, ""},
		{"a", true, "a"},
		{"a\000\000b", false, "a??b"},
		{"a\000", false, "a?"},
		{"\xc5z", false, "?z"},
	} {
		switch tc.valid {
		case true:
			row, err = client.Query(ctx, `
				INSERT INTO test (test) VALUES ($1);
			`, tc.text)
			require.NoError(t, err, "failed to execute query", "text", tc.text)
			row.Close()

			// Ensure that the text is inserted and that SanitizeString() correctly parses the string.
			require.NoError(t, row.Err(), "failed to insert valid text", "text", tc.text)
			require.Equal(t, tc.santized, storage.SanitizeString(tc.text), "SanitizeString() altered valid text", "text", tc.text)
		case false:
			row, err = client.Query(ctx, `
				INSERT INTO test (test) VALUES ($1);
			`, tc.text)
			require.NoError(t, err, "failed to execute query", "text", tc.text)
			row.Close()

			// Ensure that the text was not inserted and that the IsValidText() function rejects the string.
			require.Error(t, row.Err(), "didn't fail to insert invalid text", "text", tc.text)
			sanitized := storage.SanitizeString(tc.text)
			require.Equal(t, tc.santized, sanitized, "SanitizeString() did not sanitize text as expected", "text", tc.text)

			// Ensure that the database accepts the sanitized text.
			row, err = client.Query(ctx, `
				INSERT INTO test (test) VALUES ($1);
			`, sanitized)
			require.NoError(t, err, "failed to execute query", "text", tc.text)
			row.Close()
			require.NoError(t, row.Err(), "failed to insert valid text", "text", sanitized)
		}
	}
}
