package keenbase

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const SuperusersCollectionID = "_pbs_superusers_"

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

func createSystemTables(db *sql.DB) error {

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

func createSuperusersTable(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _superusers (
			id               TEXT PRIMARY KEY,
			email            TEXT UNIQUE NOT NULL,
			emailVisibility  INTEGER NOT NULL DEFAULT 0,
			verified         INTEGER NOT NULL DEFAULT 1,
			passwordHash     TEXT NOT NULL DEFAULT '',
			tokenKey         TEXT NOT NULL DEFAULT '',
			created          TEXT NOT NULL DEFAULT '',
			updated          TEXT NOT NULL DEFAULT ''
		)
	`)
	if err != nil {
		return fmt.Errorf("create _superusers: %w", err)
	}

	return nil
}

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

	_, err = db.Exec(`
		INSERT INTO _collections
			(id, name, type, fields, auth_options, created, updated)
		VALUES
			(?, '_superusers', 'auth', '[]', ?, ?, ?)
	`, SuperusersCollectionID, string(authOptsJSON), now, now)

	return err
}
