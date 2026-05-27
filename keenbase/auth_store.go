package keenbase

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

type AuthStore struct {
	db      *sql.DB
	records *RecordStore
}

func newAuthStore(db *sql.DB, records *RecordStore) *AuthStore {
	return &AuthStore{db: db, records: records}
}

func (as *AuthStore) CreateAuthRecord(col *Collection, email, password string, extraData map[string]any) (*Record, error) {
	if !col.IsAuth() {
		return nil, errors.New("collection is not an auth collection")
	}
	if strings.TrimSpace(email) == "" {
		return nil, errors.New("email is required")
	}
	if len(password) < 8 {
		return nil, errors.New("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	tokenKey := newID() + newID() // 30-char random secret

	data := make(map[string]any, len(extraData)+4)
	for k, v := range extraData {
		data[k] = v
	}
	data["email"] = email
	data["passwordHash"] = string(hash)
	data["tokenKey"] = tokenKey
	data["verified"] = false
	data["emailVisibility"] = false

	return as.records.Create(col, data)
}

func (as *AuthStore) FindByIdentityField(col *Collection, field, value string) (*Record, error) {
	query := fmt.Sprintf(`SELECT * FROM %q WHERE %q = ? LIMIT 1`, col.Name, field)
	rows, err := as.db.Query(query, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	return scanRecord(col, rows)
}

func (as *AuthStore) VerifyPassword(rec *Record, password string) bool {
	hash := rec.GetString("passwordHash")
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (as *AuthStore) ChangePassword(col *Collection, recordID, newPassword string) error {
	if len(newPassword) < 8 {
		return errors.New("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	newTokenKey := newID() + newID()
	now := time.Now().UTC().Format(timeLayout)

	stmt := fmt.Sprintf(
		`UPDATE %q SET "passwordHash" = ?, "tokenKey" = ?, "updated" = ? WHERE "id" = ?`,
		col.Name,
	)
	_, err = as.db.Exec(stmt, string(hash), newTokenKey, now, recordID)
	return err
}

func (as *AuthStore) RotateTokenKey(col *Collection, recordID string) (string, error) {
	newKey := newID() + newID()
	now := time.Now().UTC().Format(timeLayout)

	stmt := fmt.Sprintf(
		`UPDATE %q SET "tokenKey" = ?, "updated" = ? WHERE "id" = ?`,
		col.Name,
	)
	if _, err := as.db.Exec(stmt, newKey, now, recordID); err != nil {
		return "", err
	}
	return newKey, nil
}

func (as *AuthStore) IssueToken(col *Collection, rec *Record) (string, error) {
	tokenKey := rec.GetString("tokenKey")
	if tokenKey == "" {
		var err error
		tokenKey, err = as.RotateTokenKey(col, rec.id)
		if err != nil {
			return "", err
		}
	}
	return generateAuthToken(rec.id, col.ID, tokenKey)
}

func (as *AuthStore) MarkVerified(col *Collection, recordID string) error {
	now := time.Now().UTC().Format(timeLayout)
	stmt := fmt.Sprintf(
		`UPDATE %q SET "verified" = 1, "updated" = ? WHERE "id" = ?`,
		col.Name,
	)
	_, err := as.db.Exec(stmt, now, recordID)
	return err
}

func identityFields(col *Collection) []string {
	if col.AuthOptions != nil &&
		len(col.AuthOptions.PasswordAuth.IdentityFields) > 0 {
		return col.AuthOptions.PasswordAuth.IdentityFields
	}
	return []string{"email"}
}

func authRecordToPublic(rec *Record) *Record {
	pub := &Record{
		id:             rec.id,
		collectionID:   rec.collectionID,
		collectionName: rec.collectionName,
		created:        rec.created,
		updated:        rec.updated,
		data:           make(map[string]any, len(rec.data)),
	}
	skip := map[string]bool{"passwordHash": true, "tokenKey": true}
	for k, v := range rec.data {
		if !skip[k] {
			pub.data[k] = v
		}
	}
	return pub
}

type authResponse struct {
	Token  string  `json:"token"`
	Record *Record `json:"record"`
}

func newAuthResponse(token string, rec *Record) authResponse {
	return authResponse{
		Token:  token,
		Record: authRecordToPublic(rec),
	}
}

var ErrPasswordAuthDisabled = errors.New("password authentication is not enabled for this collection")

func (as *AuthStore) AuthWithPassword(col *Collection, identity, password string) (authResponse, error) {
	empty := authResponse{}

	if col.AuthOptions == nil || !col.AuthOptions.PasswordAuth.Enabled {
		return empty, ErrPasswordAuthDisabled
	}

	// Try each configured identity field until we find a match.
	var rec *Record
	for _, field := range identityFields(col) {
		found, err := as.FindByIdentityField(col, field, identity)
		if err != nil {
			return empty, fmt.Errorf("lookup: %w", err)
		}
		if found != nil {
			rec = found
			break
		}
	}

	if rec == nil || !as.VerifyPassword(rec, password) {
		return empty, errors.New("invalid identity or password")
	}

	token, err := as.IssueToken(col, rec)
	if err != nil {
		return empty, fmt.Errorf("issue token: %w", err)
	}

	return newAuthResponse(token, rec), nil
}

func (as *AuthStore) RefreshToken(col *Collection, recordID string) (authResponse, error) {
	empty := authResponse{}

	rec, err := as.records.GetByID(col, recordID)
	if err != nil || rec == nil {
		return empty, errors.New("auth record not found")
	}

	token, err := as.IssueToken(col, rec)
	if err != nil {
		return empty, fmt.Errorf("issue token: %w", err)
	}

	return newAuthResponse(token, rec), nil
}

func SuperuserExists(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM _superusers`).Scan(&count)
	return count > 0, err
}
