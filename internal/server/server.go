// Package server implements the HTTP server, middleware, and request handlers for the application.
package server

import (
	"net/http"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/woozymasta/zenit/assets"
	"github.com/woozymasta/zenit/internal/config"
	"github.com/woozymasta/zenit/internal/geoip"
	"github.com/woozymasta/zenit/internal/storage"
)

// New creates a new Server instance with the provided storage, GeoIP provider, and configuration.
func New(store *storage.Repository, geo *geoip.Provider, cfg *config.Config) *Server {
	appMap := make(map[uint64]struct{})
	for _, app := range cfg.Server.AllowedApps {
		hash := xxhash.Sum64String(app)
		appMap[hash] = struct{}{}
	}

	return &Server{
		storage:        store,
		geoip:          geo,
		a2sOptions:     cfg.A2S,
		authToken:      cfg.Server.AuthToken,
		allowedApps:    appMap,
		maxBody:        cfg.Server.MaxBodySize,
		trustProxy:     cfg.Server.TrustProxy,
		hardLimitCount: cfg.RateLimit.HardLimitCount,
		hardLimitWin:   cfg.RateLimit.HardLimitWin,
		softLimitDur:   cfg.RateLimit.SoftLimitDur,
		expectedUA:     cfg.Server.ExpectedUA,
		ignoreUA:       cfg.Server.IgnoreUA,
		expectedCT:     cfg.Server.ContentType,

		queue:    make(chan telemetryJob, 1000),
		shutdown: make(chan struct{}),
	}
}

// StartWorkers initializes the background worker pool for processing telemetry jobs
// and the cache cleanup routine.
func (s *Server) StartWorkers() {
	workers := 10
	for i := 0; i < workers; i++ {
		s.wg.Add(1)
		go s.worker()
	}

	// Clean soft-limit cache
	go s.gcSoftLimitCache()
}

// StopWorkers gracefully stops the background workers and closes the job queue.
func (s *Server) StopWorkers() {
	close(s.shutdown)
	close(s.queue)
	s.wg.Wait()
}

// Run configures the HTTP routes and returns the main handler.
func (s *Server) Run() http.Handler {
	mux := http.NewServeMux()

	mux.Handle("POST /api/telemetry", s.RateLimitMiddleware(http.HandlerFunc(s.handleTelemetry)))
	mux.Handle("GET /api/stats", AdminAuthMiddleware(s.authToken, http.HandlerFunc(s.handleStats)))
	mux.Handle("GET /api/a2s", AdminAuthMiddleware(s.authToken, http.HandlerFunc(s.handleServerQuery)))
	mux.Handle("GET /api/node", AdminAuthMiddleware(s.authToken, http.HandlerFunc(s.handleGetNode)))
	mux.Handle("DELETE /api/node", AdminAuthMiddleware(s.authToken, http.HandlerFunc(s.handleDeleteNode)))

	fileServer := http.FileServer(assets.GetFileSystem())
	mux.Handle("GET /js/", fileServer)
	mux.Handle("GET /css/", fileServer)
	mux.Handle("GET /data/", fileServer)
	mux.Handle("GET /img/", fileServer)
	mux.Handle("GET /favicon.ico", fileServer)

	mux.Handle("GET /", http.HandlerFunc(s.handleIndex))
	mux.Handle("GET /dashboard", BasicAuthMiddleware(s.authToken, http.HandlerFunc(s.handleDashboard)))

	return s.LoggingMiddleware(mux)
}

// gcSoftLimitCache periodically cleans up expired entries from the soft rate-limit cache.
func (s *Server) gcSoftLimitCache() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.shutdown:
			return
		case <-ticker.C:
			now := time.Now()
			s.seenCache.Range(func(key, value interface{}) bool {
				if t, ok := value.(time.Time); ok {
					if now.Sub(t) > s.softLimitDur {
						s.seenCache.Delete(key)
					}
				} else {
					s.seenCache.Delete(key)
				}
				return true
			})
		}
	}
}
