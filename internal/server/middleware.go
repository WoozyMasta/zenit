package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
)

// GetRealIP attempts to determine the client's real IP address, trusting
// headers like CF-Connecting-IP or X-Forwarded-For if configured to do so.
func GetRealIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		if cf := r.Header.Get("CF-Connecting-IP"); cf != "" {
			return cf
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			return strings.TrimSpace(parts[0])
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

// RateLimitMiddleware applies a hard rate limit based on the client's IP address.
// It rejects requests with "429 Too Many Requests" if the limit is exceeded.
func (s *Server) RateLimitMiddleware(next http.Handler) http.Handler {
	type client struct {
		limiter  *rate.Limiter
		lastSeen time.Time
	}

	var (
		mu      sync.Mutex
		clients = make(map[string]*client)
	)

	// Drop old clients every 5 min
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			mu.Lock()
			now := time.Now()
			for ip, c := range clients {
				if now.Sub(c.lastSeen) > 10*time.Minute {
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetRealIP(r, s.trustProxy)

		mu.Lock()
		cli, found := clients[ip]
		if !found {
			limit := rate.Limit(float64(s.hardLimitCount) / s.hardLimitWin.Seconds())
			cli = &client{limiter: rate.NewLimiter(limit, s.hardLimitCount)}
			clients[ip] = cli
		}
		cli.lastSeen = time.Now()
		limiter := cli.limiter
		mu.Unlock()

		if !limiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// LoggingMiddleware logs the details of each HTTP request, including method, path, IP, and duration.
func (s *Server) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		realIP := GetRealIP(r, s.trustProxy)
		log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Str("ip", realIP).
			Dur("duration", time.Since(start)).
			Msg("Request handled")
	})
}

// AdminAuthMiddleware protects endpoints by requiring a valid Bearer token in the Authorization header.
func AdminAuthMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// BasicAuthMiddleware protects endpoints using HTTP Basic Authentication (username: "admin").
func BasicAuthMiddleware(authToken string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != authToken {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
