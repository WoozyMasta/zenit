package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/rs/zerolog/log"
	"github.com/woozymasta/zenit/internal/game"
	"github.com/woozymasta/zenit/internal/models"
)

// handleTelemetry processes incoming telemetry reports.
// It validates the application name, checks rate limits (soft), verifies the user agent,
// and queues the request for asynchronous processing to avoid blocking the client.
func (s *Server) handleTelemetry(w http.ResponseWriter, r *http.Request) {
	// Check headers
	if r.Method != http.MethodPost {
		log.Debug().
			Str("ua", r.UserAgent()).
			Str("method", r.Method).
			Msg("Invalid method")

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Real IP
	ip := GetRealIP(r, s.trustProxy)

	// Content-Type Validation
	ct := r.Header.Get("Content-Type")
	if s.expectedCT != "" && !strings.HasPrefix(ct, s.expectedCT) {
		log.Debug().
			Str("content_type", ct).
			Str("expected", s.expectedCT).
			Msg("Invalid Content-Type")

		respondOK(w, "not accounted")
		return
	}

	// Check user agent - DayZ use blank UA
	if !s.ignoreUA {
		if r.UserAgent() != s.expectedUA {
			log.Debug().
				Str("ip", ip).
				Str("ua", r.UserAgent()).
				Str("method", r.Method).
				Msg("Invalid UserAgent")

			respondOK(w, "not accounted")
			return
		}
	}

	// Max body limit size
	r.Body = http.MaxBytesReader(w, r.Body, s.maxBody)

	// Decode body payload
	var req models.TelemetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Debug().
			Err(err).
			Str("ip", ip).
			Str("ua", r.UserAgent()).
			Msg("Invalid JSON")

		respondOK(w, "not accounted")
		return
	}

	// Port check
	if req.Port < 0 || req.Port > 65535 {
		log.Debug().
			Str("ip", ip).
			Str("application", req.Application).
			Str("version", req.Version).
			Int("port", req.Port).
			Msg("Invalid port")

		respondOK(w, "not accounted")
		return
	}
	if req.Port == 0 {
		req.Port = 27016
	}

	// Chech application name whitelist
	if len(s.allowedApps) > 0 {
		hash := xxhash.Sum64String(req.Application)
		if _, allowed := s.allowedApps[hash]; !allowed {
			log.Debug().
				Str("ip", ip).
				Str("application", req.Application).
				Str("version", req.Version).
				Int("port", req.Port).
				Msg("Invalid app")

			respondOK(w, "not accounted")
			return
		}
	}

	// Soft Limit
	softKey := fmt.Sprintf("%s:%d", ip, req.Port)
	if val, ok := s.seenCache.Load(softKey); ok {
		if lastSeen, ok := val.(time.Time); ok {
			if time.Since(lastSeen) < s.softLimitDur {
				log.Trace().
					Str("ip", ip).
					Str("application", req.Application).
					Str("version", req.Version).
					Int("port", req.Port).
					Msg("Dropped by soft limit hit")

				respondOK(w, "ok")
				return
			}
		}
	}
	s.seenCache.Store(softKey, time.Now())

	// Send to queue
	select {
	case s.queue <- telemetryJob{Req: req, IP: ip}:
		log.Trace().
			Str("ip", ip).
			Str("application", req.Application).
			Str("version", req.Version).
			Int("port", req.Port).
			Msg("Success added")

		respondOK(w, "successfully accounted")
	default:
		log.Warn().
			Str("ip", ip).
			Str("application", req.Application).
			Str("version", req.Version).
			Int("port", req.Port).
			Msg("Queue full, telemetry dropped")

		respondOK(w, "not accounted")
	}
}

// worker is a background goroutine that processes jobs from the telemetry queue.
func (s *Server) worker() {
	defer s.wg.Done()

	for job := range s.queue {
		s.processJob(job)
	}
}

// respondOK writes a standard text/plain response required by the DayZ mod interface.
func respondOK(w http.ResponseWriter, status string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, status)
}

// processJob executes the logic for a single telemetry request.
// It queries the game server (A2S), resolves the country (GeoIP), and upserts the data to the storage.
func (s *Server) processJob(job telemetryJob) {
	nodeType := job.Req.Type
	if nodeType == "" {
		nodeType = "generic"
	}

	// A2S Query
	var (
		serverName   string
		mapName      string
		gameVer      string
		gameName     string
		serverOS     string
		players      byte
		maxPlayers   byte
		a2sSucceeded bool
	)

	queryIP := job.IP
	if queryIP == "::1" {
		queryIP = "127.0.0.1"
	}

	if nodeType == "steam" || nodeType == "a2s" {
		parsedIP := net.ParseIP(queryIP)
		if parsedIP != nil && parsedIP.To4() != nil {
			info, err := game.QueryServer(queryIP, job.Req.Port, s.a2sOptions)
			if err != nil {
				log.Debug().
					Err(err).
					Str("ip", queryIP).
					Int("port", job.Req.Port).
					Msg("A2S query failed")
				a2sSucceeded = false
			} else {
				serverName = info.Name
				mapName = info.Map
				gameVer = info.Version
				gameName = info.Game
				serverOS = info.Environment.String()
				players = info.Players
				maxPlayers = info.MaxPlayers
				a2sSucceeded = true
			}
		} else {
			log.Trace().
				Str("ip", queryIP).
				Msg("Skipping A2S query for IPv6 address")
		}
	} else {
		log.Trace().
			Str("ip", queryIP).
			Str("type", nodeType).
			Msg("Skipping not A2S type application")
	}

	// GeoIP
	var country string
	if s.geoip != nil {
		country = s.geoip.GetCountryCode(queryIP)
	}

	// Model prepare
	node := models.Node{
		Application: job.Req.Application,
		IP:          queryIP,
		Port:        job.Req.Port,
		Version:     job.Req.Version,
		Type:        nodeType,
		CountryCode: country,

		// A2S data (can be emty)
		ServerName:  serverName,
		MapName:     mapName,
		Players:     players,
		MaxPlayers:  maxPlayers,
		GameVersion: gameVer,
		GameName:    gameName,
		ServerOS:    serverOS,

		FirstSeen: time.Now(),
		LastSeen:  time.Now(),
	}

	// Write to DB
	if err := s.storage.UpsertNode(node); err != nil {
		log.Error().Err(err).Msg("Failed to save node to DB")
		return
	}

	log.Debug().
		Str("ip", node.IP).
		Bool("a2s", a2sSucceeded).
		Msg("Telemetry saved")
}
