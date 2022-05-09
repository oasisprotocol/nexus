// Tooling for response pagination.
package common

import (
	"net/http"
	"strconv"
)

const (
	LimitKey  = "limit"
	OffsetKey = "offset"

	// By default, just order by the first returned column so
	// we always have a deterministic ordering.
	DefaultOrder  = "1"
	DefaultLimit  = uint64(100)
	DefaultOffset = uint64(0)
)

// Pagination is used to define parameters for pagination.
type Pagination struct {
	Limit  uint64
	Offset uint64
	Order  string
}

// NewPagination extracts pagination parameters from an http request.
func NewPagination(r *http.Request) (p Pagination, err error) {
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
		Order:  DefaultOrder,
	}
	return
}
