package keenbase

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var reCollectionName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// systemCollections lists the built-in collection IDs that cannot be deleted
// or have their type changed by users.
var systemCollections = map[string]bool{
	SuperusersCollectionID: true,
}

// CollectionStore handles all persistence for collection definitions.
// It manages both the metadata rows in _collections and the actual SQLite
// tables that back each collection's records.
type CollectionStore struct {
	db *sql.DB
}

func newCollectionStore(db *sql.DB) *CollectionStore {
	return &CollectionStore{db: db}
}

// ---------------------------------------------------------------- validation

func validateCollection(c *Collection) error {
	if strings.TrimSpace(c.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if strings.HasPrefix(c.Name, "_") {
		return fmt.Errorf("name cannot start with '_' (reserved for system collections)")
	}
	if !reCollectionName.MatchString(c.Name) {
		return fmt.Errorf("name must start with a letter and contain only letters, digits, and underscores")
	}
	switch c.Type {
	case CollectionTypeBase, CollectionTypeAuth, CollectionTypeView:
	default:
		return fmt.Errorf("type must be one of: base, auth, view")
	}
	if c.Type == CollectionTypeView && strings.TrimSpace(c.ViewQuery) == "" {
		return fmt.Errorf("viewQuery is required for view collections")
	}

	// Validate fields.
	reserved := map[string]bool{"id": true, "created": true, "updated": true}
	if c.Type == CollectionTypeAuth {
		for _, n := range []string{"email", "emailVisibility", "verified", "password", "tokenKey"} {
			reserved[n] = true
		}
	}
	seen := map[string]bool{}
	for i, f := range c.Fields {
		if strings.TrimSpace(f.Name) == "" {
			return fmt.Errorf("field[%d]: name is required", i)
		}
		if reserved[f.Name] {
			return fmt.Errorf("field %q: name is reserved", f.Name)
		}
		if seen[f.Name] {
			return fmt.Errorf("field %q: duplicate name", f.Name)
		}
		if !isValidFieldType(f.Type) {
			return fmt.Errorf("field %q: unknown type %q", f.Name, f.Type)
		}
		seen[f.Name] = true
	}
	return nil
}

func isValidFieldType(ft FieldType) bool {
	switch ft {
	case FieldTypeText, FieldTypeNumber, FieldTypeBool, FieldTypeEmail,
		FieldTypeURL, FieldTypeEditor, FieldTypeDate, FieldTypeAutodate,
		FieldTypeSelect, FieldTypeFile, FieldTypeRelation, FieldTypeJSON,
		FieldTypeGeoPoint:
		return true
	}
	return false
}

// ---------------------------------------------------------------- DDL helpers

// fieldSQLType maps a FieldType to the appropriate SQLite column affinity.
func fieldSQLType(ft FieldType) string {
	switch ft {
	case FieldTypeNumber:
		return "REAL"
	case FieldTypeBool:
		return "INTEGER"
	default:
		return "TEXT"
	}
}

// fieldSQLDefault returns the DEFAULT expression for a field column.
func fieldSQLDefault(ft FieldType) string {
	switch ft {
	case FieldTypeNumber:
		return "0"
	case FieldTypeBool:
		return "0"
	case FieldTypeJSON:
		return "NULL" // JSON is the one nullable field type
	default:
		return "''"
	}
}

// createTable issues a CREATE TABLE statement for the collection's data table.
func (cs *CollectionStore) createTable(tx *sql.Tx, c *Collection) error {
	var b strings.Builder
	fmt.Fprintf(&b, "CREATE TABLE %q (\n", c.Name)
	b.WriteString(`  "id" TEXT PRIMARY KEY`)

	if c.Type == CollectionTypeAuth {
		b.WriteString(",\n  \"email\"            TEXT UNIQUE NOT NULL DEFAULT ''")
		b.WriteString(",\n  \"emailVisibility\"  INTEGER NOT NULL DEFAULT 0")
		b.WriteString(",\n  \"verified\"         INTEGER NOT NULL DEFAULT 0")
		b.WriteString(",\n  \"passwordHash\"     TEXT NOT NULL DEFAULT ''")
		b.WriteString(",\n  \"tokenKey\"         TEXT NOT NULL DEFAULT ''")
	}

	for _, f := range c.Fields {
		sqlType := fieldSQLType(f.Type)
		def := fieldSQLDefault(f.Type)
		if f.Type == FieldTypeJSON {
			fmt.Fprintf(&b, ",\n  %q %s DEFAULT %s", f.Name, sqlType, def)
		} else {
			fmt.Fprintf(&b, ",\n  %q %s NOT NULL DEFAULT %s", f.Name, sqlType, def)
		}
	}

	b.WriteString(",\n  \"created\" TEXT NOT NULL DEFAULT ''")
	b.WriteString(",\n  \"updated\" TEXT NOT NULL DEFAULT ''")
	b.WriteString("\n)")

	_, err := tx.Exec(b.String())
	return err
}

// addColumn appends a new column to an existing collection table.
func (cs *CollectionStore) addColumn(tx *sql.Tx, tableName string, f Field) error {
	sqlType := fieldSQLType(f.Type)
	def := fieldSQLDefault(f.Type)
	var stmt string
	if f.Type == FieldTypeJSON {
		stmt = fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q %s DEFAULT %s", tableName, f.Name, sqlType, def)
	} else {
		stmt = fmt.Sprintf("ALTER TABLE %q ADD COLUMN %q %s NOT NULL DEFAULT %s", tableName, f.Name, sqlType, def)
	}
	_, err := tx.Exec(stmt)
	return err
}

// dropTable drops a collection's data table.
func (cs *CollectionStore) dropTable(tx *sql.Tx, tableName string) error {
	_, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %q", tableName))
	return err
}

// ---------------------------------------------------------------- CRUD

// List returns all collections ordered by name.
func (cs *CollectionStore) List() ([]*Collection, error) {
	rows, err := cs.db.Query(`
		SELECT id, name, type, fields,
		       list_rule, view_rule, create_rule, update_rule, delete_rule, manage_rule,
		       view_query, auth_options, created, updated
		FROM _collections
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []*Collection
	for rows.Next() {
		c, err := scanCollection(rows.Scan)
		if err != nil {
			return nil, err
		}
		cols = append(cols, c)
	}
	if cols == nil {
		cols = []*Collection{}
	}
	return cols, rows.Err()
}

// GetByName returns the collection with the given name, or nil if not found.
func (cs *CollectionStore) GetByName(name string) (*Collection, error) {
	row := cs.db.QueryRow(`
		SELECT id, name, type, fields,
		       list_rule, view_rule, create_rule, update_rule, delete_rule, manage_rule,
		       view_query, auth_options, created, updated
		FROM _collections WHERE name = ?
	`, name)
	c, err := scanCollection(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// GetByID returns the collection with the given ID, or nil if not found.
func (cs *CollectionStore) GetByID(id string) (*Collection, error) {
	row := cs.db.QueryRow(`
		SELECT id, name, type, fields,
		       list_rule, view_rule, create_rule, update_rule, delete_rule, manage_rule,
		       view_query, auth_options, created, updated
		FROM _collections WHERE id = ?
	`, id)
	c, err := scanCollection(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// Create validates and persists a new collection, then creates its data table.
func (cs *CollectionStore) Create(c *Collection) error {
	if err := validateCollection(c); err != nil {
		return err
	}

	existing, err := cs.GetByName(c.Name)
	if err != nil {
		return err
	}
	if existing != nil {
		return fmt.Errorf("collection with name %q already exists", c.Name)
	}

	if c.ID == "" {
		c.ID = newID()
	}
	for i := range c.Fields {
		if c.Fields[i].ID == "" {
			c.Fields[i].ID = newID()
		}
	}

	now := time.Now().UTC()
	c.Created = now
	c.Updated = now

	fieldsJSON, err := json.Marshal(c.Fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	authOptsJSON, err := marshalAuthOptions(c.AuthOptions)
	if err != nil {
		return err
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		INSERT INTO _collections
			(id, name, type, fields,
			 list_rule, view_rule, create_rule, update_rule, delete_rule, manage_rule,
			 view_query, auth_options, created, updated)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		c.ID, c.Name, string(c.Type), string(fieldsJSON),
		ptrToNull(c.Rules.ListRule),
		ptrToNull(c.Rules.ViewRule),
		ptrToNull(c.Rules.CreateRule),
		ptrToNull(c.Rules.UpdateRule),
		ptrToNull(c.Rules.DeleteRule),
		ptrToNull(c.Rules.ManageRule),
		c.ViewQuery, authOptsJSON,
		now.Format(timeLayout), now.Format(timeLayout),
	)
	if err != nil {
		return fmt.Errorf("insert _collections: %w", err)
	}

	// View collections use a SQL query, not a dedicated table.
	if c.Type != CollectionTypeView {
		if err := cs.createTable(tx, c); err != nil {
			return fmt.Errorf("create table %q: %w", c.Name, err)
		}
	}

	return tx.Commit()
}

// Update applies changes to an existing collection.
//
// Supported changes:
//   - Rename the collection (also renames the underlying table)
//   - Add new fields (issues ALTER TABLE ADD COLUMN for each)
//   - Update field metadata / rules / auth options
//
// Not supported in this version: removing or renaming individual fields.
// The collection type cannot be changed after creation.
func (cs *CollectionStore) Update(c *Collection) error {
	old, err := cs.GetByID(c.ID)
	if err != nil {
		return err
	}
	if old == nil {
		return fmt.Errorf("collection %q not found", c.ID)
	}
	if c.Type != old.Type {
		return fmt.Errorf("cannot change collection type after creation")
	}

	if err := validateCollection(c); err != nil {
		return err
	}

	// Check name uniqueness when renaming.
	if c.Name != old.Name {
		existing, err := cs.GetByName(c.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			return fmt.Errorf("collection with name %q already exists", c.Name)
		}
	}

	// Assign IDs to any newly added fields.
	for i := range c.Fields {
		if c.Fields[i].ID == "" {
			c.Fields[i].ID = newID()
		}
	}

	c.Updated = time.Now().UTC()

	fieldsJSON, err := json.Marshal(c.Fields)
	if err != nil {
		return fmt.Errorf("marshal fields: %w", err)
	}
	authOptsJSON, err := marshalAuthOptions(c.AuthOptions)
	if err != nil {
		return err
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if c.Type != CollectionTypeView {
		// Add columns for any fields that didn't exist before.
		oldNames := make(map[string]bool, len(old.Fields))
		for _, f := range old.Fields {
			oldNames[f.Name] = true
		}
		for _, f := range c.Fields {
			if !oldNames[f.Name] {
				if err := cs.addColumn(tx, old.Name, f); err != nil {
					return fmt.Errorf("add column %q to %q: %w", f.Name, old.Name, err)
				}
			}
		}

		// Rename the table when the collection name changes.
		if c.Name != old.Name {
			stmt := fmt.Sprintf("ALTER TABLE %q RENAME TO %q", old.Name, c.Name)
			if _, err := tx.Exec(stmt); err != nil {
				return fmt.Errorf("rename table %q → %q: %w", old.Name, c.Name, err)
			}
		}
	}

	_, err = tx.Exec(`
		UPDATE _collections SET
			name = ?, fields = ?,
			list_rule = ?, view_rule = ?, create_rule = ?,
			update_rule = ?, delete_rule = ?, manage_rule = ?,
			view_query = ?, auth_options = ?, updated = ?
		WHERE id = ?
	`,
		c.Name, string(fieldsJSON),
		ptrToNull(c.Rules.ListRule),
		ptrToNull(c.Rules.ViewRule),
		ptrToNull(c.Rules.CreateRule),
		ptrToNull(c.Rules.UpdateRule),
		ptrToNull(c.Rules.DeleteRule),
		ptrToNull(c.Rules.ManageRule),
		c.ViewQuery, authOptsJSON,
		c.Updated.Format(timeLayout),
		c.ID,
	)
	if err != nil {
		return fmt.Errorf("update _collections: %w", err)
	}

	return tx.Commit()
}

// Delete removes a collection's metadata row and drops its data table.
// System collections (e.g. _superusers) cannot be deleted.
func (cs *CollectionStore) Delete(c *Collection) error {
	if systemCollections[c.ID] {
		return fmt.Errorf("cannot delete system collection %q", c.Name)
	}

	tx, err := cs.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if c.Type != CollectionTypeView {
		if err := cs.dropTable(tx, c.Name); err != nil {
			return fmt.Errorf("drop table %q: %w", c.Name, err)
		}
	}

	if _, err := tx.Exec("DELETE FROM _collections WHERE id = ?", c.ID); err != nil {
		return fmt.Errorf("delete from _collections: %w", err)
	}

	return tx.Commit()
}

// ---------------------------------------------------------------- scan helpers

// scanCollection reads a collection row using the provided Scan function.
// It works with both *sql.Row.Scan and *sql.Rows.Scan because both accept
// the same variadic-pointer signature.
func scanCollection(scan func(...any) error) (*Collection, error) {
	var (
		c            Collection
		colType      string
		fieldsJSON   string
		authOptsJSON string
		listRule     sql.NullString
		viewRule     sql.NullString
		createRule   sql.NullString
		updateRule   sql.NullString
		deleteRule   sql.NullString
		manageRule   sql.NullString
		createdStr   string
		updatedStr   string
	)

	err := scan(
		&c.ID, &c.Name, &colType, &fieldsJSON,
		&listRule, &viewRule, &createRule, &updateRule, &deleteRule, &manageRule,
		&c.ViewQuery, &authOptsJSON,
		&createdStr, &updatedStr,
	)
	if err != nil {
		return nil, err
	}

	c.Type = CollectionType(colType)

	if err := json.Unmarshal([]byte(fieldsJSON), &c.Fields); err != nil {
		return nil, fmt.Errorf("unmarshal fields: %w", err)
	}
	if c.Fields == nil {
		c.Fields = []Field{}
	}

	if authOptsJSON != "" && authOptsJSON != "{}" {
		var ao AuthOptions
		if err := json.Unmarshal([]byte(authOptsJSON), &ao); err != nil {
			return nil, fmt.Errorf("unmarshal auth_options: %w", err)
		}
		c.AuthOptions = &ao
	}

	nullToPtr := func(ns sql.NullString) *string {
		if !ns.Valid {
			return nil
		}
		s := ns.String
		return &s
	}
	c.Rules = APIRules{
		ListRule:   nullToPtr(listRule),
		ViewRule:   nullToPtr(viewRule),
		CreateRule: nullToPtr(createRule),
		UpdateRule: nullToPtr(updateRule),
		DeleteRule: nullToPtr(deleteRule),
		ManageRule: nullToPtr(manageRule),
	}

	c.Created, _ = time.Parse(timeLayout, createdStr)
	c.Updated, _ = time.Parse(timeLayout, updatedStr)

	return &c, nil
}

// ---------------------------------------------------------------- small utils

// ptrToNull converts a *string rule into a sql.NullString for storage.
// nil → NULL (locked), &"" → valid empty string (public), &"expr" → valid expression.
func ptrToNull(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: *s, Valid: true}
}

func marshalAuthOptions(ao *AuthOptions) (string, error) {
	if ao == nil {
		return "{}", nil
	}
	b, err := json.Marshal(ao)
	if err != nil {
		return "", fmt.Errorf("marshal auth_options: %w", err)
	}
	return string(b), nil
}
