// Package storage handles database connections, schema migrations, and data operations using SQLite.
package storage

import (
	"database/sql"
	"time"

	"github.com/woozymasta/zenit/internal/models"
	_ "modernc.org/sqlite" // Driver sqlite
)

// Repository manages the SQLite database connection.
type Repository struct {
	db *sql.DB
}

// New initializes a new SQLite connection, sets connection pool parameters, and runs migrations.
func New(dbPath string) (*Repository, error) {
	dsn := dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(1 * time.Hour)

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Repository{db: db}, nil
}

// Close closes the underlying database connection.
func (r *Repository) Close() error {
	return r.db.Close()
}

// UpsertNode inserts a new node or updates an existing one based on the Application, IP, and Port constraint.
// It handles logic for updating fields only when they are non-empty or changed.
func (r *Repository) UpsertNode(n models.Node) error {
	query := `
	INSERT INTO nodes (
		application, ip, port, version, country_code, type,
		server_name, map_name, players, max_players, game_version, game_name, server_os,
		count, first_seen, last_seen
	)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?)
	ON CONFLICT(application, ip, port) DO UPDATE SET
		count = count + 1,
		last_seen = excluded.last_seen,
		version = excluded.version,
		type = excluded.type,

		-- Update country if updated and not blank
		country_code = CASE WHEN excluded.country_code != '' THEN excluded.country_code ELSE nodes.country_code END,

		-- Update A2S fields only if updated
		server_name  = CASE WHEN excluded.server_name != '' THEN excluded.server_name ELSE nodes.server_name END,
		map_name     = CASE WHEN excluded.server_name != '' THEN excluded.map_name ELSE nodes.map_name END,
		players      = CASE WHEN excluded.server_name != '' THEN excluded.players ELSE nodes.players END,
		max_players  = CASE WHEN excluded.server_name != '' THEN excluded.max_players ELSE nodes.max_players END,
		game_version = CASE WHEN excluded.server_name != '' THEN excluded.game_version ELSE nodes.game_version END,
		game_name    = CASE WHEN excluded.server_name != '' THEN excluded.game_name ELSE nodes.game_name END,
		server_os    = CASE WHEN excluded.server_name != '' THEN excluded.server_os ELSE nodes.server_os END;
	`

	// Use LastSeen and for FirstSeen when insert new record
	_, err := r.db.Exec(query,
		n.Application, n.IP, n.Port, n.Version, n.CountryCode, n.Type,
		n.ServerName, n.MapName, n.Players, n.MaxPlayers, n.GameVersion, n.GameName, n.ServerOS,
		n.FirstSeen, n.LastSeen,
	)

	return err
}

// GetNodes retrieves all nodes from the database, sorted by the last seen timestamp in descending order.
func (r *Repository) GetNodes() ([]models.Node, error) {
	rows, err := r.db.Query(`
		SELECT application, ip, port, version, country_code, type,
		       server_name, map_name, players, max_players, game_version, game_name, server_os,
		       count, first_seen, last_seen
		FROM nodes
		ORDER BY last_seen DESC
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var nodes []models.Node
	for rows.Next() {
		var n models.Node
		if err := rows.Scan(
			&n.Application, &n.IP, &n.Port, &n.Version, &n.CountryCode, &n.Type,
			&n.ServerName, &n.MapName, &n.Players, &n.MaxPlayers, &n.GameVersion, &n.GameName, &n.ServerOS,
			&n.Count, &n.FirstSeen, &n.LastSeen,
		); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// GetNode retrieves a specific node by its unique identifier (Application, IP, Port).
func (r *Repository) GetNode(app, ip string, port int) (*models.Node, error) {
	query := `
		SELECT application, ip, port, version, country_code, type,
		       server_name, map_name, players, max_players, game_version, game_name, server_os,
		       count, first_seen, last_seen
		FROM nodes
		WHERE application = ? AND ip = ? AND port = ?
	`
	row := r.db.QueryRow(query, app, ip, port)

	var n models.Node
	err := row.Scan(
		&n.Application, &n.IP, &n.Port, &n.Version, &n.CountryCode, &n.Type,
		&n.ServerName, &n.MapName, &n.Players, &n.MaxPlayers, &n.GameVersion, &n.GameName, &n.ServerOS,
		&n.Count, &n.FirstSeen, &n.LastSeen,
	)

	if err == sql.ErrNoRows {
		return nil, nil // Not found
	}
	if err != nil {
		return nil, err
	}

	return &n, nil
}

// DeleteEmptyNodes removes records that have empty A2S data (server_name is empty).
// If appName is provided (not empty), it restricts deletion to that application.
func (r *Repository) DeleteEmptyNodes(appName string) (int64, error) {
	query := `DELETE FROM nodes WHERE (server_name IS NULL OR server_name = '')`
	var args []interface{}

	if appName != "" {
		query += ` AND application = ?`
		args = append(args, appName)
	}

	res, err := r.db.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteNode removes a specific node identified by app, ip, and port.
func (r *Repository) DeleteNode(app, ip string, port int) error {
	query := `DELETE FROM nodes WHERE application = ? AND ip = ? AND port = ?`
	_, err := r.db.Exec(query, app, ip, port)
	return err
}

// GetNodesSubset retrieves nodes for maintenance.
// if onlyEmptyA2S is true, it returns only nodes where server_name is empty.
// if appName is provided, it filters by application.
func (r *Repository) GetNodesSubset(appName string, onlyEmptyA2S bool) ([]models.Node, error) {
	query := `
		SELECT application, ip, port, version, country_code, type,
		       server_name, map_name, players, max_players, game_version, game_name, server_os,
		       count, first_seen, last_seen
		FROM nodes
		WHERE 1=1
	`
	var args []interface{}

	if appName != "" {
		query += " AND application = ?"
		args = append(args, appName)
	}

	if onlyEmptyA2S {
		query += " AND (server_name IS NULL OR server_name = '')"
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var nodes []models.Node
	for rows.Next() {
		var n models.Node
		// Scanning matches the GetNodes method
		if err := rows.Scan(
			&n.Application, &n.IP, &n.Port, &n.Version, &n.CountryCode, &n.Type,
			&n.ServerName, &n.MapName, &n.Players, &n.MaxPlayers, &n.GameVersion, &n.GameName, &n.ServerOS,
			&n.Count, &n.FirstSeen, &n.LastSeen,
		); err != nil {
			continue
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return nodes, nil
}
