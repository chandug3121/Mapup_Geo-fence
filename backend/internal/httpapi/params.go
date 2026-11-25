package httpapi

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
)

// pageParams reads ?limit= and ?offset=. When limit is absent, defaultLimit
// is used (0 means "return everything", which keeps the list endpoints
// behaving exactly like the spec unless the client asks for paging).
func pageParams(r *http.Request, defaultLimit, maxLimit int) (store.Page, error) {
	page := store.Page{Limit: defaultLimit}
	q := r.URL.Query()

	if raw := strings.TrimSpace(q.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > maxLimit {
			return store.Page{}, badRequest("limit must be a whole number between 1 and %d", maxLimit)
		}
		page.Limit = n
	}
	if raw := strings.TrimSpace(q.Get("offset")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			return store.Page{}, badRequest("offset must be a whole number of 0 or more")
		}
		page.Offset = n
	}
	return page, nil
}

// timeParam reads an optional ISO 8601 query parameter.
func timeParam(r *http.Request, name string) (*time.Time, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return nil, nil
	}
	t, err := domain.ParseTimestamp(raw)
	if err != nil {
		return nil, badRequest("%s must be an ISO 8601 timestamp with a timezone, e.g. 2025-01-01T00:00:00Z", name)
	}
	return &t, nil
}

func queryParam(r *http.Request, name string) string {
	return strings.TrimSpace(r.URL.Query().Get(name))
}
