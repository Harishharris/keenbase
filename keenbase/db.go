package keenbase

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

// openDB opens (or creates) the SQLite database file inside dataDir,
// applies recommended pragmas, and returns the connection pool.
func openDB(dataDir string) (*sql.DB, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}

	dbPath := filepath.Join(dataDir, "data.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite at %q: %w", dbPath, err)
	}

	// SQLite works best with a single writer connection.
	// WAL mode allows concurrent readers alongside that writer.
	db.SetMaxOpenConns(1)

	pragmas := []string{
		// Write-Ahead Logging — readers don't block the writer and vice versa.
		`PRAGMA journal_mode=WAL`,
		// NORMAL is safe with WAL and faster than FULL.
		`PRAGMA synchronous=NORMAL`,
		// Wait up to 5 s before returning SQLITE_BUSY.
		`PRAGMA busy_timeout=5000`,
		// Enforce foreign-key constraints (SQLite ignores them by default).
		`PRAGMA foreign_keys=ON`,
		// Keep 64 MB of pages in memory to speed up repeated queries.
		`PRAGMA cache_size=-65536`,
	}

	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("apply %q: %w", p, err)
		}
	}

	return db, nil
}
