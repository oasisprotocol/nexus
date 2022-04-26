// Reusable pagination middleware.
package api

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	LimitKey  = "limit"
	OffsetKey = "offset"

	DefaultLimit  = uint64(100)
	DefaultOffset = uint64(0)
)

// Pagination is used to define parameters for pagination.
type Pagination struct {
	Limit  uint64
	Offset uint64
}

func unpackPagination(r *http.Request) (p Pagination, err error) {
	values := r.URL.Query()

	limit := DefaultLimit
	if v := values.Get(LimitKey); v != "" {
		limit, err = strconv.ParseUint(v, 10, 64)
	}

	offset := DefaultOffset
	if v := values.Get(OffsetKey); v != "" {
		offset, err = strconv.ParseUint(v, 10, 64)
	}

	p = Pagination{
		Limit:  limit,
		Offset: offset,
	}
	return
}

func withPagination(sql string, p Pagination) string {
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", sql, p.Limit, p.Offset)
}
