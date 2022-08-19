package emerald

import "fmt"

// QueryFactory is a convenience type for creating API queries.
type QueryFactory struct {
	chainID string
}

func NewQueryFactory(chainID string) QueryFactory {
	return QueryFactory{chainID}
}

func (qf QueryFactory) LatestRoundQuery() string {
	return fmt.Sprintf(`
		SELECT height FROM %s.processed_blocks
			WHERE analyzer = $1
		ORDER BY height DESC
		LIMIT 1`, qf.chainID)
}

func (qf QueryFactory) IndexingProgressQuery() string {
	return fmt.Sprintf(`
		INSERT INTO %s.processed_blocks (height, analyzer, processed_time)
			VALUES
				($1, $2, CURRENT_TIMESTAMP)`, qf.chainID)
}
