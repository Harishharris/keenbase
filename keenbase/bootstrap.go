package keenbase

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SuperusersCollectionID is the fixed, well-known ID for the built-in
// _superusers auth collection so it can be referenced without a lookup.
const SuperusersCollectionID = "_pbs_superusers_"

// bootstrap ensures the database is fully initialised:
//  1. Creates the system metadata tables if they don't exist yet.
//  2. Creates the _superusers data table if it doesn't exist yet.
//  3. Seeds the _superusers collection definition on first run.
func bootstrap(db *sql.DB) error {
	steps := []struct {
		name string
		fn   func(*sql.DB) error
	}{
		{"create system tables", createSystemTables},
		{"create superusers table", createSuperusersTable},
		{"seed superusers collection", seedSuperusersCollection},
	}

	for _, step := range steps {
		if err := step.fn(db); err != nil {
			return fmt.Errorf("bootstrap %s: %w", step.name, err)
		}
	}

	return nil
}

// createSystemTables creates the metadata tables that simple-base uses
// internally to track collection definitions and their fields.
func createSystemTables(db *sql.DB) error {
	// _collections holds one row per user-defined (or built-in) collection.
	//
	// Rules are stored as nullable TEXT columns because the three states
	// (NULL = locked, "" = public, "expr" = conditional) map naturally to
	// SQL NULL vs empty string — a JSON blob would lose that distinction.
	//
	// Fields, view_query, and auth_options are stored as JSON because they
	// are structured objects that don't need to be queried column-by-column.
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _collections (
			id           TEXT PRIMARY KEY,
			name         TEXT UNIQUE NOT NULL,
			type         TEXT NOT NULL DEFAULT 'base',
			fields       TEXT NOT NULL DEFAULT '[]',
			list_rule    TEXT,
			view_rule    TEXT,
			create_rule  TEXT,
			update_rule  TEXT,
			delete_rule  TEXT,
			manage_rule  TEXT,
			view_query   TEXT NOT NULL DEFAULT '',
			auth_options TEXT NOT NULL DEFAULT '{}',
			created      TEXT NOT NULL,
			updated      TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("create _collections: %w", err)
	}

	return nil
}

// createSuperusersTable creates the actual data table that holds superuser
// accounts. Auth collections always have a dedicated table in addition to
// their metadata row in _collections.
//
// Column notes:
//   - email_visibility: stored as INTEGER (0/1) because SQLite has no BOOLEAN.
//   - password_hash:    bcrypt hash of the password; empty until first login setup.
//   - token_key:        random value rotated on password change to invalidate
//     all previously issued JWTs for this account.
func createSuperusersTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _superusers (
			id               TEXT PRIMARY KEY,
			email            TEXT UNIQUE NOT NULL,
			email_visibility INTEGER NOT NULL DEFAULT 0,
			verified         INTEGER NOT NULL DEFAULT 1,
			password_hash    TEXT NOT NULL DEFAULT '',
			token_key        TEXT NOT NULL DEFAULT '',
			created          TEXT NOT NULL DEFAULT '',
			updated          TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("create _superusers: %w", err)
	}

	return nil
}

// seedSuperusersCollection inserts the _superusers collection definition into
// _collections on first run. On subsequent startups the row already exists and
// this function is a no-op.
func seedSuperusersCollection(db *sql.DB) error {
	var exists int
	err := db.QueryRow(
		`SELECT COUNT(*) FROM _collections WHERE id = ?`,
		SuperusersCollectionID,
	).Scan(&exists)
	if err != nil {
		return err
	}
	if exists > 0 {
		return nil // already seeded — nothing to do
	}

	authOpts := AuthOptions{
		PasswordAuth: PasswordAuthOptions{
			Enabled:        true,
			IdentityFields: []string{"email"},
		},
		OTP: OTPOptions{
			Enabled:  false,
			Duration: 300,
			Length:   8,
		},
		MFA: MFAOptions{
			Enabled:  false,
			Duration: 1800,
		},
	}

	authOptsJSON, err := json.Marshal(authOpts)
	if err != nil {
		return fmt.Errorf("marshal auth options: %w", err)
	}

	now := time.Now().UTC().Format(timeLayout)

	// Superusers collection is fully locked down by default —
	// all rule columns stay NULL (only superusers can manage superusers).
	_, err = db.Exec(`
		INSERT INTO _collections
			(id, name, type, fields, auth_options, created, updated)
		VALUES
			(?, '_superusers', 'auth', '[]', ?, ?, ?)
	`, SuperusersCollectionID, string(authOptsJSON), now, now)

	return err
}
