// Package models defines the data structures used for API requests and database persistence.
package models

import "time"

// TelemetryRequest represents the payload sent by the game client/mod.
type TelemetryRequest struct {
	Application string `json:"application"`
	Type        string `json:"type,omitempty"`
	Version     string `json:"version,omitempty"`
	Port        int    `json:"port"`
}

// Node represents a registered game server stored in the database.
type Node struct {
	FirstSeen   time.Time `json:"first_seen"`
	LastSeen    time.Time `json:"last_seen"`
	Application string    `json:"application"`
	Type        string    `json:"type"`
	IP          string    `json:"ip"`
	CountryCode string    `json:"country_code"`
	Version     string    `json:"version"`
	ServerName  string    `json:"server_name"`
	MapName     string    `json:"map_name"`
	GameVersion string    `json:"game_version"`
	GameName    string    `json:"game_name"`
	ServerOS    string    `json:"server_os"`
	Port        int       `json:"port"`
	Count       int64     `json:"count"`
	Players     byte      `json:"players"`
	MaxPlayers  byte      `json:"max_players"`
}
