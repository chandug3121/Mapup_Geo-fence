// Package httpapi exposes the REST API and the WebSocket endpoint.
package httpapi

import (
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/config"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/realtime"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/store"
	"github.com/chandug3121/mapup_geo-fence/backend/internal/tracking"
)

// API holds the dependencies shared by all handlers.
type API struct {
	cfg      config.Config
	store    *store.Store
	tracking *tracking.Service
	hub      *realtime.Hub
	log      *slog.Logger
}

// NewRouter builds the HTTP handler with every route and middleware.
func NewRouter(cfg config.Config, s *store.Store, t *tracking.Service, hub *realtime.Hub, log *slog.Logger) http.Handler {
	a := &API{cfg: cfg, store: s, tracking: t, hub: hub, log: log}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger(log))
	r.Use(recoverer(log))
	r.Use(cors.Handler(cors.Options{
		AllowOriginFunc:  func(_ *http.Request, origin string) bool { return a.originAllowed(origin) },
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "X-API-Key", "X-Request-Id"},
		ExposedHeaders:   []string{"X-Request-Id"},
		AllowCredentials: false,
		MaxAge:           300,
	}))
	r.Use(limitBody)
	if cfg.APIKey != "" {
		r.Use(requireAPIKey(cfg.APIKey))
	}

	r.NotFound(a.handle(func(r *http.Request) (int, any, error) {
		return http.StatusNotFound, newError("not_found", "no route for "+r.Method+" "+r.URL.Path), nil
	}))
	r.MethodNotAllowed(a.handle(func(r *http.Request) (int, any, error) {
		return http.StatusMethodNotAllowed, newError("method_not_allowed", r.Method+" is not supported on "+r.URL.Path), nil
	}))

	r.Get("/health", a.handle(a.health))
	r.Get("/ws/alerts", realtime.Handler(hub, a.originAllowed, log))

	r.Post("/geofences", a.handle(a.createGeofence))
	r.Get("/geofences", a.handle(a.listGeofences))

	r.Post("/vehicles", a.handle(a.createVehicle))
	r.Get("/vehicles", a.handle(a.listVehicles))
	r.With(a.locationRateLimit()).Post("/vehicles/location", a.handle(a.updateLocation))
	r.Get("/vehicles/location/{vehicle_id}", a.handle(a.getVehicleLocation))

	r.Post("/alerts/configure", a.handle(a.configureAlert))
	r.Get("/alerts", a.handle(a.listAlerts))
	r.Patch("/alerts/{alert_id}", a.handle(a.updateAlertStatus))
	r.Delete("/alerts/{alert_id}", a.handle(a.deleteAlert))

	r.Get("/violations/history", a.handle(a.violationHistory))

	return r
}

// originAllowed applies CORS_ORIGINS to both REST and WebSocket requests.
func (a *API) originAllowed(origin string) bool {
	return a.cfg.AllowsAnyOrigin() || slices.Contains(a.cfg.CORSOrigins, origin)
}

// locationRateLimit caps POST /vehicles/location per client IP, answering
// with a JSON 429 (including time_ns) when the limit is hit.
func (a *API) locationRateLimit() func(http.Handler) http.Handler {
	return httprate.Limit(
		a.cfg.LocationRateLimit,
		time.Minute,
		httprate.WithKeyFuncs(httprate.KeyByIP),
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusTooManyRequests,
				newError("rate_limited", "too many location updates; please slow down and retry shortly"), 0)
		}),
	)
}
