package postgres_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/log"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/postgres"
	"github.com/oasisprotocol/nexus/storage/postgres/testutil"
	"github.com/oasisprotocol/nexus/tests"
)

func TestConnect(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	client.Close()
}

func TestInvalidConnect(t *testing.T) {
	tests.SkipIfShort(t)

	connString := "an invalid connstring"
	logger, err := log.NewLogger("postgres-test", io.Discard, log.FmtJSON, log.LevelInfo)
	require.NoError(t, err)

	_, err = postgres.NewClient(connString, logger)
	require.Error(t, err)
}

func TestQuery(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	defer client.Close()

	rows, err := client.Query(context.Background(), `
		SELECT * FROM ( VALUES (0),(1),(2) ) AS q;
	`)
	require.NoError(t, err)
	defer rows.Close()

	i := 0
	for rows.Next() {
		var result int
		err = rows.Scan(&result)
		require.NoError(t, err)
		require.Equal(t, i, result)

		i++
	}
	require.Equal(t, 3, i)
}

func TestInvalidQuery(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	defer client.Close()

	_, err := client.Query(context.Background(), `
		an invalid query
	`)
	require.Error(t, err)
}

func TestQueryRow(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	defer client.Close()

	var result int
	err := client.QueryRow(context.Background(), `
		SELECT 1+1;
	`).Scan(&result)
	require.NoError(t, err)
	require.Equal(t, 2, result)
}

func TestInvalidQueryRow(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	defer client.Close()

	var result int
	err := client.QueryRow(context.Background(), `
		an invalid query
	`).Scan(&result)
	require.Error(t, err)
}

func TestSendBatch(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	defer client.Close()

	defer func() {
		destroy := &storage.QueryBatch{}
		destroy.Queue(`
			DROP TABLE films;
		`)
		err := client.SendBatch(context.Background(), destroy)
		require.NoError(t, err)
	}()

	create := &storage.QueryBatch{}
	create.Queue(`
		CREATE TABLE films (
			fid  INTEGER PRIMARY KEY,
			name TEXT
		);
	`)
	err := client.SendBatch(context.Background(), create)
	require.NoError(t, err)

	insert := &storage.QueryBatch{}
	queueFilms := func(b *storage.QueryBatch, f []string, idOffset int) {
		rows := make([]string, 0, len(f))
		for i, film := range f {
			rows = append(rows, fmt.Sprintf("(%d, '%s')", i+idOffset, film))
		}
		b.Queue(fmt.Sprintf(`
			INSERT INTO films (fid, name)
			VALUES %s;
		`, strings.Join(rows, ", ")))
	}

	films1 := []string{
		"Gone with the Wind",
		"Avatar",
		"Titanic",
	}
	films2 := []string{
		"Star Wars",
		"Avengers: Endgame",
	}
	queueFilms(insert, films1, 0)
	queueFilms(insert, films2, len(films1))
	err = client.SendBatch(context.Background(), insert)
	require.NoError(t, err)

	var wg sync.WaitGroup
	for i, film := range append(films1, films2...) {
		wg.Add(1)
		go func(i int, film string) {
			defer wg.Done()

			var result string
			err := client.QueryRow(context.Background(), `
				SELECT name FROM films WHERE fid = $1;
			`, i).Scan(&result)
			require.NoError(t, err)
			require.Equal(t, film, result)
		}(i, film)
	}

	wg.Wait()
}

func TestInvalidSendBatch(t *testing.T) {
	tests.SkipIfShort(t)

	client := testutil.NewTestClient(t)
	defer client.Close()

	invalid := &storage.QueryBatch{}
	invalid.Queue(`
		an invalid query
	`)
	err := client.SendBatch(context.Background(), invalid)
	require.Error(t, err)
}

func TestNumeric(t *testing.T) {
	tests.SkipIfShort(t)
	client := testutil.NewTestClient(t)
	defer client.Close()

	ctx := context.Background()

	// Ensure database is empty before running the test.
	require.NoError(t, client.Wipe(ctx), "failed to wipe database")

	// Create custom type, derived from NUMERIC.
	row, err := client.Query(ctx, `CREATE DOMAIN mynumeric NUMERIC(1000,0) CHECK(VALUE >= 0)`)
	require.NoError(t, err, "failed to create custom type")
	row.Close()

	// Test that we can scan both null and non-null values into a *common.BigInt.
	var mynull *common.BigInt
	var my2 *common.BigInt
	err = client.QueryRow(ctx, `
		SELECT null::mynumeric, 2::mynumeric;
	`).Scan(&mynull, &my2)
	require.NoError(t, err, "failed to scan null and non-null values")
	require.Nil(t, mynull)
	require.Equal(t, int64(2), my2.Int64())
}
