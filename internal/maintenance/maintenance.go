// Package maintenance provide tools for clean and update database
package maintenance

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/woozymasta/zenit/internal/config"
	"github.com/woozymasta/zenit/internal/game"
	"github.com/woozymasta/zenit/internal/models"
	"github.com/woozymasta/zenit/internal/storage"
)

// Run checks if any maintenance flags are set and executes the corresponding tasks.
// Returns true if a maintenance task was executed (indicating the program should exit).
func Run(cfg *config.Config, store *storage.Repository) bool {
	// Prune Empty
	if cfg.Storage.PruneEmpty != "" {
		appName := parseAppName(cfg.Storage.PruneEmpty)
		log.Info().Str("app_filter", appName).Msg("Pruning empty nodes...")

		count, err := store.DeleteEmptyNodes(appName)
		if err != nil {
			log.Error().Err(err).Msg("Failed to prune nodes")
		} else {
			log.Info().Int64("deleted", count).Msg("Prune finished")
		}

		return true
	}

	// Check Inactive OR Check All
	// Since these are mutually exclusive or run sequentially, we prioritize them logic.
	// If both are present, we usually pick one or run sequentially. Here we pick one.
	var nodes []models.Node
	var err error
	var taskName string

	if cfg.Storage.CheckInactive != "" {
		taskName = "Check Inactive"
		appName := parseAppName(cfg.Storage.CheckInactive)
		log.Info().Str("app_filter", appName).Msg("Fetching inactive nodes for check...")
		nodes, err = store.GetNodesSubset(appName, true)
	} else if cfg.Storage.CheckAll != "" {
		taskName = "Check All"
		appName := parseAppName(cfg.Storage.CheckAll)
		log.Info().Str("app_filter", appName).Msg("Fetching all nodes for re-check...")
		nodes, err = store.GetNodesSubset(appName, false)
	} else {
		// No flags set
		return false
	}

	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch nodes")
		return true
	}

	if len(nodes) == 0 {
		log.Info().Msg("No nodes found for maintenance")
		return true
	}

	log.Info().Int("count", len(nodes)).Msgf("Starting '%s' task with 10 workers...", taskName)
	runWorkerPool(nodes, store, cfg.A2S)
	log.Info().Msg("Maintenance task completed")

	return true
}

// parseAppName handles the optional value logic.
// If the flag is provided without a value (or with default), it might come as "AnyApp".
// We convert "AnyApp" to empty string for the storage layer (which means "no filter").
func parseAppName(input string) string {
	if input == config.AnyApplications {
		return ""
	}

	return input
}

func runWorkerPool(nodes []models.Node, store *storage.Repository, a2sOpts config.A2S) {
	const workers = 10
	jobs := make(chan models.Node, len(nodes))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for node := range jobs {
				processNode(node, store, a2sOpts)
			}
		}()
	}

	// Send jobs
	for _, n := range nodes {
		jobs <- n
	}
	close(jobs)

	wg.Wait()
}

func processNode(node models.Node, store *storage.Repository, a2sOpts config.A2S) {
	logCtx := log.With().
		Str("app", node.Application).
		Str("ip", node.IP).
		Int("port", node.Port).
		Logger()

	// 1. Port validation (1000 - 65536)
	// We use 65535 as standard max port, but requirement mentioned 65536, adjusted to standard range.
	if node.Port <= 1000 || node.Port > 65535 {
		logCtx.Debug().Msg("Invalid port, deleting node")
		if err := store.DeleteNode(node.Application, node.IP, node.Port); err != nil {
			logCtx.Error().Err(err).Msg("Failed to delete invalid node")
		}
		return
	}

	// 2. A2S Query
	info, err := game.QueryServer(node.IP, node.Port, a2sOpts)
	if err != nil {
		// Check failed -> Delete
		logCtx.Debug().Err(err).Msg("Server unreachable, deleting node")
		if err := store.DeleteNode(node.Application, node.IP, node.Port); err != nil {
			logCtx.Error().Err(err).Msg("Failed to delete unreachable node")
		}
		return
	}

	// 3. Success -> Update
	// We update the node struct with new info
	node.ServerName = info.Name
	node.MapName = info.Map
	node.Players = info.Players
	node.MaxPlayers = info.MaxPlayers
	node.GameVersion = info.Version
	node.GameName = info.Game
	node.ServerOS = info.Environment.String()
	node.LastSeen = time.Now()

	// UpsertNode handles the update logic
	if err := store.UpsertNode(node); err != nil {
		logCtx.Error().Err(err).Msg("Failed to update node")
	} else {
		logCtx.Trace().Msg("Node updated successfully")
	}
}
