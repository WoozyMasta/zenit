package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"text/template"

	"github.com/rs/zerolog/log"
	"github.com/woozymasta/zenit/assets"
	"github.com/woozymasta/zenit/internal/game"
	"github.com/woozymasta/zenit/internal/models"
)

// handleIndex serves the main landing page (landing.min.html).
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content, _ := assets.ReadFile("landing.min.html")
	_, _ = w.Write(content)
}

// handleDashboard serves the administrative dashboard template, injecting the authentication token.
func (s *Server) handleDashboard(w http.ResponseWriter, _ *http.Request) {
	content, _ := assets.ReadFile("dashboard.min.html")
	tmpl, _ := template.New("dashboard").Parse(string(content))
	_ = tmpl.Execute(w, map[string]string{"AuthToken": s.authToken})
}

// handleStats returns a JSON list of all collected server nodes.
// This endpoint is protected by AdminAuthMiddleware.
func (s *Server) handleStats(w http.ResponseWriter, _ *http.Request) {
	nodes, err := s.storage.GetNodes()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch nodes")
		http.Error(w, "Database Error", http.StatusInternalServerError)
		return
	}

	if nodes == nil {
		nodes = []models.Node{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

// handleServerQuery performs a live A2S query to a specific game server IP and port.
// It acts as a proxy to retrieve real-time server status.
// Query params: ?ip=1.2.3.4&port=2302
func (s *Server) handleServerQuery(w http.ResponseWriter, r *http.Request) {
	ip := r.URL.Query().Get("ip")
	portStr := r.URL.Query().Get("port")

	if ip == "" || portStr == "" {
		http.Error(w, "Missing ip or port", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	// Execute A2S_INFO request
	info, err := game.QueryServer(ip, port, s.a2sOptions)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusGatewayTimeout)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(info)
}

// handleGetNode returns details for a specific node.
// Query params: ?app=MetricZ&ip=1.2.3.4&port=2302
func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {
	app := r.URL.Query().Get("app")
	ip := r.URL.Query().Get("ip")
	portStr := r.URL.Query().Get("port")

	if app == "" || ip == "" || portStr == "" {
		http.Error(w, "Missing required params (app, ip, port)", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	node, err := s.storage.GetNode(app, ip, port)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch node")
		http.Error(w, "Database Error", http.StatusInternalServerError)
		return
	}

	if node == nil {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(node)
}

// handleDeleteNode removes a specific node from the database.
// Query params: ?app=MetricZ&ip=1.2.3.4&port=2302
func (s *Server) handleDeleteNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	app := r.URL.Query().Get("app")
	ip := r.URL.Query().Get("ip")
	portStr := r.URL.Query().Get("port")

	if app == "" || ip == "" || portStr == "" {
		http.Error(w, "Missing required params (app, ip, port)", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		http.Error(w, "Invalid port", http.StatusBadRequest)
		return
	}

	if err := s.storage.DeleteNode(app, ip, port); err != nil {
		log.Error().Err(err).
			Str("app", app).
			Str("ip", ip).
			Int("port", port).
			Msg("Failed to delete node")

		http.Error(w, "Database Error", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("app", app).
		Str("ip", ip).
		Int("port", port).
		Msg("Node deleted manually")

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Node deleted"})
}
