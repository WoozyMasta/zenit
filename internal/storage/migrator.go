package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/woozymasta/zenit/assets"
)

// runMigrations checks for new SQL files in embedded assets and applies them.
func runMigrations(db *sql.DB) error {
	// Create table to track migrations history
	const migrationTableSchema = `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at DATETIME
	);`

	if _, err := db.Exec(migrationTableSchema); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	// Get list of migration files
	entries, err := assets.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	// Filter and sort .sql files
	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)

	// Apply migrations
	for _, file := range files {
		var exists int
		err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE version = ?", file).Scan(&exists)
		if err == nil {
			continue // applied
		} else if err != sql.ErrNoRows {
			return fmt.Errorf("failed to check migration status: %w", err)
		}

		log.Info().Str("file", file).Msg("Applying database migration...")

		content, err := assets.ReadFile(filepath.ToSlash(filepath.Join("migrations", file)))
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", file, err)
		}

		// Execute migration in transaction
		tx, err := db.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(string(content)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to exec migration %s: %w", file, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", file, time.Now()); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("failed to record migration %s: %w", file, err)
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
