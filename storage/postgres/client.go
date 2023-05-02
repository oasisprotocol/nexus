// Package postgres implements the target storage interface
// backed by PostgreSQL.
package postgres

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"

	common "github.com/oasisprotocol/oasis-indexer/analyzer/uncategorized"
	"github.com/oasisprotocol/oasis-indexer/log"
	"github.com/oasisprotocol/oasis-indexer/storage"
)

const (
	moduleName = "postgres"
)

// Client is a client for connecting to PostgreSQL.
type Client struct {
	pool   *pgxpool.Pool
	logger *log.Logger
}

// pgxLogger is a pgx-compatible logger interface that uses indexer's standard
// logger as the backend.
type pgxLogger struct {
	logger *log.Logger
}

// logFuncForLevel maps a pgx log severity level to a corresponding indexer logger function.
func (l *pgxLogger) logFuncForLevel(level tracelog.LogLevel) func(string, ...interface{}) {
	switch level {
	case tracelog.LogLevelTrace, tracelog.LogLevelDebug:
		return l.logger.Debug
	case tracelog.LogLevelInfo:
		return l.logger.Info
	case tracelog.LogLevelWarn:
		return l.logger.Warn
	case tracelog.LogLevelError, tracelog.LogLevelNone:
		return l.logger.Error
	default:
		l.logger.Warn("Unknown log level", "unknown_level", level)
		return l.logger.Info
	}
}

// Implements pgx.Logger interface. Logs to indexer logger.
func (l *pgxLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	args := []interface{}{}
	for k, v := range data {
		args = append(args, k, v)
	}

	logFunc := l.logFuncForLevel(level)
	logFunc(msg, args...)
}

// NewClient creates a new PostgreSQL client.
func NewClient(connString string, l *log.Logger) (*Client, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, err
	}

	// Set up pgx logging. For a log line to be produced, it needs to be >= the level
	// specified here, and >= the level of the underlying indexer logger. "Info" level
	// logs every SQL statement executed.
	config.ConnConfig.Tracer = &tracelog.TraceLog{
		LogLevel: tracelog.LogLevelWarn,
		Logger: &pgxLogger{
			logger: l.WithModule(moduleName).With("db", config.ConnConfig.Database),
		},
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, err
	}
	return &Client{
		pool:   pool,
		logger: l.WithModule(moduleName),
	}, nil
}

// SendBatch submits a new batch of queries as an atomic transaction to PostgreSQL.
//
// For now, updated row counts are discarded as this is not intended to be used
// by any indexer. We only care about atomic success or failure of the batch of queries
// corresponding to a new block.
func (c *Client) SendBatch(ctx context.Context, batch *storage.QueryBatch) error {
	return c.SendBatchWithOptions(ctx, batch, pgx.TxOptions{})
}

func (c *Client) SendBatchWithOptions(ctx context.Context, batch *storage.QueryBatch, opts pgx.TxOptions) error {
	// NOTE: Sending txs with tx.SendBatch(batch.AsPgxBatch()) is more efficient as it happens
	// in a single roundtrip to the server.
	// However, it reports errors poorly: If _any_ query is syntactically
	// malformed, called with the wrong number of args, or has a type conversion problem,
	// pgx will report the _first_ query as failing.
	//
	// TODO: Remove the first branch if we verify that the performance gain is negligible.
	if true {
		// skip db writes
	} else if os.Getenv("PGX_FAST_BATCH") == "1" { //nolint:nestif
		pgxBatch := batch.AsPgxBatch()
		var batchResults pgx.BatchResults
		var emptyTxOptions pgx.TxOptions
		var tx pgx.Tx
		var err error

		// Begin a transaction.
		useExplicitTx := opts != emptyTxOptions
		if useExplicitTx {
			// set up our own tx with the specified options
			tx, err = c.pool.BeginTx(ctx, opts)
			if err != nil {
				return fmt.Errorf("failed to begin tx: %w", err)
			}
			batchResults = c.pool.SendBatch(ctx, &pgxBatch)
		} else {
			// use implicit tx provided by SendBatch; see https://github.com/jackc/pgx/issues/879
			batchResults = c.pool.SendBatch(ctx, &pgxBatch)
		}
		defer common.CloseOrLog(batchResults, c.logger)

		// Exec indiviual queries in the batch.
		for i := 0; i < pgxBatch.Len(); i++ {
			if _, err := batchResults.Exec(); err != nil {
				rollbackErr := ""
				if useExplicitTx {
					err2 := tx.Rollback(ctx)
					if err2 != nil {
						rollbackErr = fmt.Sprintf("; also failed to rollback tx: %s", err2.Error())
					}
				}
				return fmt.Errorf("query %d %v: %w%s", i, batch.Queries()[i], err, rollbackErr)
			}
		}

		// Commit the tx.
		if useExplicitTx {
			err := tx.Commit(ctx)
			if err != nil {
				return fmt.Errorf("failed to commit tx: %w", err)
			}
		}
	} else {
		// Begin a transaction.
		tx, err := c.pool.BeginTx(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to begin tx: %w", err)
		}

		// Exec indiviual queries in the batch.
		for i, q := range batch.Queries() {
			if _, err2 := tx.Exec(ctx, q.Cmd, q.Args...); err2 != nil {
				rollbackErr := ""
				err3 := tx.Rollback(ctx)
				if err3 != nil {
					rollbackErr = fmt.Sprintf("; also failed to rollback tx: %s", err3.Error())
				}
				return fmt.Errorf("query %d %v: %w%s", i, q, err2, rollbackErr)
			}
		}

		// Commit the transaction.
		err = tx.Commit(ctx)
		if err != nil {
			c.logger.Error("failed to submit tx",
				"error", err,
				"batch", batch.Queries(),
			)
			return err
		}
	}
	return nil
}

// Query submits a new read query to PostgreSQL.
func (c *Client) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	rows, err := c.pool.Query(ctx, sql, args...)
	if err != nil {
		c.logger.Error("failed to query db",
			"error", err,
			"query_cmd", sql,
			"query_args", args,
		)
		return nil, err
	}
	return rows, nil
}

// QueryRow submits a new read query for a single row to PostgreSQL.
func (c *Client) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	return c.pool.QueryRow(ctx, sql, args...)
}

// Shutdown implements the storage.TargetStorage interface for Client.
func (c *Client) Shutdown() {
	c.pool.Close()
}

// Name implements the storage.TargetStorage interface for Client.
func (c *Client) Name() string {
	return moduleName
}

// Wipe removes all contents of the database.
func (c *Client) Wipe(ctx context.Context) error {
	// List, then drop all tables.
	rows, err := c.Query(ctx, `
		SELECT schemaname, tablename
		FROM pg_tables
		WHERE schemaname != 'information_schema' AND schemaname NOT LIKE 'pg_%'
	`)
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}
	for rows.Next() {
		var schema, table string
		if err = rows.Scan(&schema, &table); err != nil {
			return err
		}
		c.logger.Info("dropping table", "schema", schema, "table", table)
		if _, err = c.pool.Exec(ctx, fmt.Sprintf("DROP TABLE %s.%s CASCADE;", schema, table)); err != nil {
			return err
		}
	}

	// List, then drop all custom types.
	// Query from https://stackoverflow.com/questions/3660787/how-to-list-custom-types-using-postgres-information-schema
	rows, err = c.Query(ctx, `
		SELECT      n.nspname as schema, t.typname as type 
		FROM        pg_type t 
		LEFT JOIN   pg_catalog.pg_namespace n ON n.oid = t.typnamespace 
		WHERE       (t.typrelid = 0 OR (SELECT c.relkind = 'c' FROM pg_catalog.pg_class c WHERE c.oid = t.typrelid)) 
		AND     NOT EXISTS(SELECT 1 FROM pg_catalog.pg_type el WHERE el.oid = t.typelem AND el.typarray = t.oid)
		AND     n.nspname != 'information_schema' AND n.nspname NOT LIKE 'pg_%';
	`)
	if err != nil {
		return fmt.Errorf("failed to list types: %w", err)
	}
	for rows.Next() {
		var schema, typ string
		if err = rows.Scan(&schema, &typ); err != nil {
			return err
		}
		c.logger.Info("dropping type", "schema", schema, "type", typ)
		if _, err = c.pool.Exec(ctx, fmt.Sprintf("DROP TYPE %s.%s CASCADE;", schema, typ)); err != nil {
			return err
		}
	}

	// List, then drop all custom functions.
	rows, err = c.Query(ctx, `
		SELECT n.nspname as schema, p.proname as function
		FROM pg_proc p
		LEFT JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema');
	`)
	if err != nil {
		return fmt.Errorf("failed to list functions: %w", err)
	}
	for rows.Next() {
		var schema, fn string
		if err = rows.Scan(&schema, &fn); err != nil {
			return err
		}
		c.logger.Info("dropping function", "schema", schema, "function", fn)
		if _, err = c.pool.Exec(ctx, fmt.Sprintf("DROP FUNCTION %s.%s CASCADE;", schema, fn)); err != nil {
			return err
		}
	}

	// List, then drop all materialized views.
	rows, err = c.Query(ctx, `
		SELECT schemaname, matviewname
		FROM pg_matviews
		WHERE schemaname != 'information_schema' AND schemaname NOT LIKE 'pg_%'
	`)
	if err != nil {
		return fmt.Errorf("failed to list materialized views: %w", err)
	}
	for rows.Next() {
		var schema, view string
		if err = rows.Scan(&schema, &view); err != nil {
			return err
		}
		c.logger.Info("dropping materialized view", "schema", schema, "view", view)
		if _, err = c.pool.Exec(ctx, fmt.Sprintf("DROP MATERIALIZED VIEW %s.%s CASCADE;", schema, view)); err != nil {
			return err
		}
	}

	return nil
}
