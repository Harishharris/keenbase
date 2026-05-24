package keenbase

import (
	"database/sql"
	"fmt"
	"log"
)

// SimpleBase is the top-level application instance.
// It owns the database connection and (soon) the HTTP server.
type SimpleBase struct {
	config      Config
	db          *sql.DB
	collections *CollectionStore
	records     *RecordStore
}

// Config holds the startup configuration for a SimpleBase instance.
type Config struct {
	// Port is the TCP port the HTTP server will listen on. Defaults to 8090.
	Port int
	// DataDir is the directory where the SQLite database and uploaded files
	// are stored. Defaults to "./pb_data".
	DataDir string
}

// New creates a SimpleBase instance with sensible defaults.
func New() *SimpleBase {
	return WithConfig(Config{})
}

// WithConfig creates a SimpleBase instance using the provided Config,
// filling in any zero values with their defaults.
func WithConfig(config Config) *SimpleBase {
	if config.Port == 0 {
		config.Port = 8090
	}
	if config.DataDir == "" {
		config.DataDir = "./pb_data"
	}
	return &SimpleBase{config: config}
}

// Start initialises the database and launches the HTTP server.
// It blocks until the server shuts down or an error occurs.
func (sb *SimpleBase) Start() error {
	// 1. Open the SQLite database (creates the file if it doesn't exist).
	db, err := openDB(sb.config.DataDir)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	sb.db = db

	// 2. Create system tables and seed built-in data on first run.
	if err := bootstrap(db); err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}

	sb.collections = newCollectionStore(db)
	sb.records = newRecordStore(db)

	log.Printf("SimpleBase started — data dir: %s, port: %d", sb.config.DataDir, sb.config.Port)

	return sb.serve()
}

// DB returns the underlying database connection.
// Useful for custom queries; prefer the higher-level APIs where possible.
func (sb *SimpleBase) DB() *sql.DB {
	return sb.db
}
