package server

import (
	"sync"
	"time"

	"github.com/woozymasta/zenit/internal/config"
	"github.com/woozymasta/zenit/internal/geoip"
	"github.com/woozymasta/zenit/internal/models"
	"github.com/woozymasta/zenit/internal/storage"
)

// Server holds the dependencies, configuration, and runtime state required
// to handle HTTP requests and background telemetry processing.
type Server struct {
	// storage provides access to the persistent database layer for reading and writing node data.
	storage *storage.Repository

	// geoip provides functionality for resolving IP addresses to country codes.
	// It can be nil if the GeoIP database is not initialized.
	geoip *geoip.Provider

	// allowedApps is a set of hashed application names (using xxhash) that are authorized
	// to submit telemetry data. Used for fast whitelist verification.
	allowedApps map[uint64]struct{}

	// queue is a buffered channel used to pass telemetry jobs from HTTP handlers
	// to background workers for asynchronous processing.
	queue chan telemetryJob

	// shutdown is a signal channel used to broadcast a stop signal to all background workers
	// during a graceful shutdown.
	shutdown chan struct{}

	// seenCache is a thread-safe map used to track recently updated servers.
	// It supports the "soft rate limit" logic to reduce unnecessary database writes.
	seenCache sync.Map

	// authToken is the secret token required to access administrative API endpoints
	// (e.g., /api/stats, /dashboard).
	authToken string

	// expectedUA expected User-Agent string
	expectedUA string

	// expectedCT expected Content-Type header
	expectedCT string

	// a2sOptions holds configuration settings for querying game servers (e.g., timeouts, retries).
	a2sOptions config.A2S

	// wg is used to wait for all background workers to finish processing
	// before the server shuts down completely.
	wg sync.WaitGroup

	// maxBody specifies the maximum allowed size (in bytes) for incoming HTTP request bodies
	// to prevent denial-of-service attacks.
	maxBody int64

	// hardLimitCount is the maximum number of requests allowed per IP address
	// within the hardLimitWin duration.
	hardLimitCount int

	// hardLimitWin is the time window duration for the hard rate limiter.
	hardLimitWin time.Duration

	// softLimitDur is the duration for which a server update is ignored (skipped)
	// if it was recently seen. This helps reduce load from frequent updates.
	softLimitDur time.Duration

	// trustProxy indicates whether the server should trust headers like X-Forwarded-For
	// or CF-Connecting-IP when determining the client's real IP address.
	trustProxy bool

	// ignoreUA disable User-Agent validation entirely
	ignoreUA bool
}

// telemetryJob represents a unit of work to be processed by background workers.
// It bundles the raw client request data with the resolved source IP address.
type telemetryJob struct {
	// IP is the resolved IPv4 or IPv6 address of the reporting client.
	// This address is used for both GeoIP location resolution and A2S server queries.
	IP string

	// Req contains the deserialized payload from the incoming HTTP request,
	// including the application name, server port, and version information.
	Req models.TelemetryRequest
}
