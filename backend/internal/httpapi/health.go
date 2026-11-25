package httpapi

import (
	"context"
	"net/http"
	"time"
)

// GET /health — used by Docker, the hosting platform and the frontend's
// "waking up the server" check.
func (a *API) health(r *http.Request) (int, any, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	status, dbStatus, code := "ok", "ok", http.StatusOK
	if err := a.store.Ping(ctx); err != nil {
		status, dbStatus, code = "degraded", "unreachable", http.StatusServiceUnavailable
		a.log.Warn("health check: database unreachable", "error", err)
	}
	return code, map[string]any{
		"status":            status,
		"database":          dbStatus,
		"websocket_clients": a.hub.ClientCount(),
	}, nil
}
