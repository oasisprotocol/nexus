// Package postgres implements the target storage interface
// backed by PostgreSQL.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

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

func (l *pgxLogger) logFuncForLevel(level pgx.LogLevel) func(string, ...interface{}) {
	switch level {
	case pgx.LogLevelTrace, pgx.LogLevelDebug:
		return l.logger.Debug
	case pgx.LogLevelInfo:
		return l.logger.Info
	case pgx.LogLevelWarn:
		return l.logger.Warn
	case pgx.LogLevelError, pgx.LogLevelNone:
		return l.logger.Error
	default:
		l.logger.Warn("Unknown log level", "unknown_level", level)
		return l.logger.Info
	}
}

// Implements pgx.Logger interface.
func (l *pgxLogger) Log(ctx context.Context, level pgx.LogLevel, msg string, data map[string]interface{}) {
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
	config.ConnConfig.LogLevel = pgx.LogLevelWarn
	config.ConnConfig.Logger = &pgxLogger{
		logger: l.WithModule(moduleName).With("db", config.ConnConfig.Database),
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), config)
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
	pgxBatch := batch.AsPgxBatch()
	if err := c.pool.BeginTxFunc(ctx, pgx.TxOptions{}, func(tx pgx.Tx) error {
		batchResults := tx.SendBatch(ctx, &pgxBatch)
		defer common.CloseOrLog(batchResults, c.logger)
		for i := 0; i < pgxBatch.Len(); i++ {
			if _, err := batchResults.Exec(); err != nil {
				return fmt.Errorf("query %d %v: %w", i, batch.Queries()[i], err)
			}
		}

		return nil
	}); err != nil {
		c.logger.Error("failed to execute db batch",
			"error", err,
			"batch", batch.Queries(),
		)
		return err
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
