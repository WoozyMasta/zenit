// Package fake provides utilities for generating random telemetry data for testing and development purposes.
package fake

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/woozymasta/zenit/internal/models"
	"github.com/woozymasta/zenit/internal/storage"
)

// GenerateData populates the storage with a specified number of randomized node records.
// It simulates various game maps, versions, countries, and player counts.
func GenerateData(store *storage.Repository, count int) {
	apps := []string{"MetricZ", "DayZMod", "AdminTool"}
	maps := []string{"chernarusplus", "livonia", "namalsk", "takistan", "enoch", "sakhal", "deerisle"}
	osTypes := []string{"Windows", "Linux"}
	versions := []string{"1.0.0", "1.0.1", "1.1.0", "1.2.0-beta"}
	gameVers := []string{"1.23.150000", "1.24.160000", "1.25.170000"}

	// Countries list
	countriesHigh := []string{"US", "DE", "RU", "CN", "BR", "FR", "GB", "PL", "CZ", "KZ", "UA"}
	countriesMid := []string{"CA", "AU", "IT", "ES", "NL", "SE", "JP", "KR", "TR", "BE", "RO"}
	countriesLow := []string{"ZA", "AR", "MX", "IN", "ID", "VN", "CH", "NO", "FI", "DK", "PT"}

	// Cache for ip reuse
	type cachedIP struct {
		Address string
		Country string
	}
	var ipHistory []cachedIP

	for i := 0; i < count; i++ {
		// Random date-time in 30 days range
		daysAgo := rand.Intn(30)
		seenTime := time.Now().Add(-time.Duration(daysAgo) * 24 * time.Hour).
			Add(-time.Duration(rand.Intn(1440)) * time.Minute)

		var ip string
		var country string

		// 20% chance for reuse IP address
		if len(ipHistory) > 0 && rand.Float32() < 0.2 {
			cached := ipHistory[rand.Intn(len(ipHistory))]
			ip = cached.Address
			country = cached.Country
		} else {
			// Generate new IP
			ip = fmt.Sprintf("%d.%d.%d.%d", rand.Intn(220)+1, rand.Intn(255), rand.Intn(255), rand.Intn(255))

			// Select country
			roll := rand.Float32()
			switch {
			case roll < 0.70:
				country = countriesHigh[rand.Intn(len(countriesHigh))]
			case roll < 0.90:
				country = countriesMid[rand.Intn(len(countriesMid))]
			default:
				country = countriesLow[rand.Intn(len(countriesLow))]
			}

			ipHistory = append(ipHistory, cachedIP{Address: ip, Country: country})
		}

		node := models.Node{
			Application: apps[rand.Intn(len(apps))],
			IP:          ip,
			Port:        2302 + rand.Intn(100),
			Version:     versions[rand.Intn(len(versions))],
			CountryCode: country,
			ServerName:  fmt.Sprintf("DayZ Server #%d [PvP]", rand.Intn(1000)),
			MapName:     maps[rand.Intn(len(maps))],
			Players:     byte(rand.Intn(60)),
			MaxPlayers:  60,
			GameVersion: gameVers[rand.Intn(len(gameVers))],
			GameName:    "dayz",
			ServerOS:    osTypes[rand.Intn(len(osTypes))],
			FirstSeen:   seenTime.Add(-time.Hour * 24 * 7),
			LastSeen:    seenTime,
		}

		err := store.UpsertNode(node)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to generate fake node")
		}

		if rand.Float32() < 0.3 { // 30% chance reconnect
			_ = store.UpsertNode(node)
			_ = store.UpsertNode(node)
		}

		if rand.Float32() < 0.1 { // 10% chance reconnect
			_ = store.UpsertNode(node)
			_ = store.UpsertNode(node)
		}
	}
}
