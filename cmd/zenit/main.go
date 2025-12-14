// main is the entry point of the Zenit application.
// It initializes the configuration, logger, database, GeoIP provider, and starts the HTTP server.
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/woozymasta/zenit/internal/config"
	"github.com/woozymasta/zenit/internal/fake"
	"github.com/woozymasta/zenit/internal/geoip"
	"github.com/woozymasta/zenit/internal/logger"
	"github.com/woozymasta/zenit/internal/maintenance"
	"github.com/woozymasta/zenit/internal/server"
	"github.com/woozymasta/zenit/internal/storage"
)

func main() {
	cfg := config.Parse()

	logger.Setup(cfg.Logger)
	log.Info().Msg("Starting zenit service...")

	// GeoIP Update
	log.Info().Msg("Checking GeoIP database...")
	if err := geoip.EnsureDB(cfg.GeoIP.Path, cfg.GeoIP.URL, cfg.GeoIP.Interval); err != nil {
		log.Error().Err(err).Msg("Failed to download GeoIP database")
	}

	geoProvider, err := geoip.Open(cfg.GeoIP.Path)
	if err != nil {
		log.Error().Err(err).Msg("Failed to open GeoIP database, country detection disabled")
		geoProvider = nil
	} else {
		defer func() {
			if err := geoProvider.Close(); err != nil {
				log.Error().Err(err).Msg("Error closing GeoIP provider")
			}
		}()
	}

	// Database
	store, err := storage.New(cfg.Storage.Path)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
	defer func() {
		if err := store.Close(); err != nil {
			log.Error().Err(err).Msg("Error closing database")
		}
	}()

	// data generation or database maintenance
	if cfg.Storage.GenerateCount > 0 {
		fake.GenerateData(store, cfg.Storage.GenerateCount)
		return
	} else if maintenance.Run(cfg, store) {
		return
	}

	// Init server
	srvHandler := server.New(store, geoProvider, cfg)

	// Background queue
	srvHandler.StartWorkers()

	httpServer := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      srvHandler.Run(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info().Str("address", cfg.Server.Address).Msg("Server listening")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("Server failed")
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")

	// Shut down HTTP
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error().Err(err).Msg("Server forced to shutdown")
	}

	// Stop workers (wait queue done)
	srvHandler.StopWorkers()

	// Close DB
	if err := store.Close(); err != nil {
		log.Error().Err(err).Msg("Error closing database")
	}

	log.Info().Msg("Server exited")
}
