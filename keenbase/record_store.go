package keenbase

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// ListParams holds the parsed query parameters for a list/search request.
type ListParams struct {
	Page      int
	PerPage   int
	Sort      string
	Filter    string
	SkipTotal bool
}

// ListResult is the paginated response envelope returned by List.
type ListResult struct {
	Page       int       `json:"page"`
	PerPage    int       `json:"perPage"`
	TotalItems int       `json:"totalItems"`
	TotalPages int       `json:"totalPages"`
	Items      []*Record `json:"items"`
}

// RecordStore handles persistence for collection records.
type RecordStore struct {
	db *sql.DB
}

func newRecordStore(db *sql.DB) *RecordStore {
	return &RecordStore{db: db}
}

// ---------------------------------------------------------------- List

// List returns a paginated, optionally filtered and sorted list of records.
func (rs *RecordStore) List(col *Collection, params ListParams) (*ListResult, error) {
	if params.Page < 1 {
		params.Page = 1
	}
	if params.PerPage < 1 {
		params.PerPage = 30
	}
	if params.PerPage > 500 {
		params.PerPage = 500
	}

	whereSQL, whereArgs, err := filterToSQL(params.Filter)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}

	orderSQL, err := sortToSQL(params.Sort, col)
	if err != nil {
		return nil, fmt.Errorf("sort: %w", err)
	}

	offset := (params.Page - 1) * params.PerPage

	result := &ListResult{
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalItems: -1,
		TotalPages: -1,
		Items:      []*Record{},
	}

	// Count query (skipped when SkipTotal is set for performance).
	if !params.SkipTotal {
		countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %q WHERE %s`, col.Name, whereSQL)
		if err := rs.db.QueryRow(countSQL, whereArgs...).Scan(&result.TotalItems); err != nil {
			return nil, fmt.Errorf("count query: %w", err)
		}
		if result.TotalItems == 0 {
			result.TotalPages = 0
			return result, nil
		}
		result.TotalPages = (result.TotalItems + params.PerPage - 1) / params.PerPage
	}

	// Data query.
	dataSQL := fmt.Sprintf(
		`SELECT * FROM %q WHERE %s ORDER BY %s LIMIT %d OFFSET %d`,
		col.Name, whereSQL, orderSQL, params.PerPage, offset,
	)
	rows, err := rs.db.Query(dataSQL, whereArgs...)
	if err != nil {
		return nil, fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		rec, err := scanRecord(col, rows)
		if err != nil {
			return nil, err
		}
		result.Items = append(result.Items, rec)
	}
	return result, rows.Err()
}

// ---------------------------------------------------------------- GetByID

// GetByID fetches a single record by ID, or returns nil if not found.
func (rs *RecordStore) GetByID(col *Collection, id string) (*Record, error) {
	query := fmt.Sprintf(`SELECT * FROM %q WHERE "id" = ?`, col.Name)
	rows, err := rs.db.Query(query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil
	}
	return scanRecord(col, rows)
}

// ---------------------------------------------------------------- Create

// Create inserts a new record into the collection's table.
// The id, created, and updated fields are set automatically.
func (rs *RecordStore) Create(col *Collection, data map[string]any) (*Record, error) {
	if err := validateRecordData(col, data, true); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	id := newID()

	// Build the full row: system fields + user data.
	row := make(map[string]any, len(data)+3)
	for k, v := range data {
		row[k] = coerceFieldValue(col, k, v)
	}
	row["id"] = id
	row["created"] = now.Format(timeLayout)
	row["updated"] = now.Format(timeLayout)

	cols, placeholders, args := buildInsertParts(row)
	stmt := fmt.Sprintf(
		`INSERT INTO %q (%s) VALUES (%s)`,
		col.Name, cols, placeholders,
	)

	if _, err := rs.db.Exec(stmt, args...); err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}

	return rs.GetByID(col, id)
}

// ---------------------------------------------------------------- Update

// Update applies a partial patch to an existing record.
// Only the keys present in data are changed; other fields are left as-is.
func (rs *RecordStore) Update(col *Collection, id string, data map[string]any) (*Record, error) {
	existing, err := rs.GetByID(col, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Remove system fields from the patch — they can't be set by callers.
	delete(data, "id")
	delete(data, "created")
	delete(data, "updated")

	if err := validateRecordData(col, data, false); err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return existing, nil
	}

	now := time.Now().UTC()
	data["updated"] = now.Format(timeLayout)

	sets, args := buildUpdateParts(col, data)
	args = append(args, id)
	stmt := fmt.Sprintf(`UPDATE %q SET %s WHERE "id" = ?`, col.Name, sets)

	if _, err := rs.db.Exec(stmt, args...); err != nil {
		return nil, fmt.Errorf("update: %w", err)
	}

	return rs.GetByID(col, id)
}

// ---------------------------------------------------------------- Delete

// Delete removes a record by ID. Returns nil if the record didn't exist.
func (rs *RecordStore) Delete(col *Collection, id string) error {
	stmt := fmt.Sprintf(`DELETE FROM %q WHERE "id" = ?`, col.Name)
	_, err := rs.db.Exec(stmt, id)
	return err
}

// ---------------------------------------------------------------- validation

func validateRecordData(col *Collection, data map[string]any, isCreate bool) error {
	for _, f := range col.Fields {
		val, provided := data[f.Name]

		if f.Required && isCreate && (!provided || isZeroValue(val)) {
			return fmt.Errorf("field %q is required", f.Name)
		}

		if !provided {
			continue
		}

		if err := validateFieldValue(f, val); err != nil {
			return err
		}
	}
	return nil
}

func isZeroValue(v any) bool {
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return val == ""
	case float64:
		return val == 0
	case bool:
		return !val
	}
	return false
}

func validateFieldValue(f Field, v any) error {
	if v == nil {
		return nil
	}
	switch f.Type {
	case FieldTypeNumber:
		switch v.(type) {
		case float64, float32, int, int64, int32:
			// ok
		default:
			return fmt.Errorf("field %q expects a number", f.Name)
		}
	case FieldTypeBool:
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("field %q expects a boolean", f.Name)
		}
	case FieldTypeSelect:
		if len(f.Options.Values) > 0 {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("field %q expects a string", f.Name)
			}
			for _, allowed := range f.Options.Values {
				if s == allowed {
					return nil
				}
			}
			return fmt.Errorf("field %q: %q is not an allowed value", f.Name, s)
		}
	}
	return nil
}

// coerceFieldValue converts a value to the storage representation
// appropriate for the field type.
func coerceFieldValue(col *Collection, fieldName string, v any) any {
	f := col.FieldByName(fieldName)
	if f == nil {
		return v
	}
	switch f.Type {
	case FieldTypeBool:
		if b, ok := v.(bool); ok {
			if b {
				return 1
			}
			return 0
		}
	}
	return v
}

// ---------------------------------------------------------------- SQL builders

func buildInsertParts(row map[string]any) (cols, placeholders string, args []any) {
	colParts := make([]string, 0, len(row))
	phParts := make([]string, 0, len(row))
	args = make([]any, 0, len(row))

	for k, v := range row {
		colParts = append(colParts, fmt.Sprintf("%q", k))
		phParts = append(phParts, "?")
		args = append(args, v)
	}
	return strings.Join(colParts, ", "), strings.Join(phParts, ", "), args
}

func buildUpdateParts(col *Collection, data map[string]any) (sets string, args []any) {
	setParts := make([]string, 0, len(data))
	args = make([]any, 0, len(data))

	for k, v := range data {
		setParts = append(setParts, fmt.Sprintf("%q = ?", k))
		args = append(args, coerceFieldValue(col, k, v))
	}
	return strings.Join(setParts, ", "), args
}

// ---------------------------------------------------------------- scan helpers

// scanRecord reads the current row from rows into a Record, using the
// collection schema to type-convert values correctly.
func scanRecord(col *Collection, rows *sql.Rows) (*Record, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Scan into a slice of any.
	raw := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}

	// Build a field-type map for type conversion.
	fieldTypes := make(map[string]FieldType, len(col.Fields))
	for _, f := range col.Fields {
		fieldTypes[f.Name] = f.Type
	}

	rec := &Record{
		collectionID:   col.ID,
		collectionName: col.Name,
		data:           make(map[string]any, len(columns)),
	}

	for i, colName := range columns {
		v := raw[i]
		switch colName {
		case "id":
			if s, ok := v.(string); ok {
				rec.id = s
			}
		case "created":
			if s, ok := v.(string); ok {
				rec.created, _ = time.Parse(timeLayout, s)
			}
		case "updated":
			if s, ok := v.(string); ok {
				rec.updated, _ = time.Parse(timeLayout, s)
			}
		default:
			rec.data[colName] = convertDBValue(v, fieldTypes[colName])
		}
	}

	return rec, nil
}

// convertDBValue converts a raw SQLite value to the idiomatic Go type for
// the given field type.
func convertDBValue(v any, ft FieldType) any {
	if v == nil {
		return nil
	}
	switch ft {
	case FieldTypeBool:
		// SQLite stores bools as INTEGER 0/1.
		switch n := v.(type) {
		case int64:
			return n != 0
		case float64:
			return n != 0
		}
	case FieldTypeNumber:
		switch n := v.(type) {
		case int64:
			return float64(n)
		}
	}
	return v
}

// ---------------------------------------------------------------- sort helper

// sortToSQL converts a PocketBase sort string to a SQL ORDER BY fragment.
//
// Format: comma-separated field names, optionally prefixed with - (DESC) or + (ASC).
// Example: "-created,title" → `"created" DESC, "title" ASC`
func sortToSQL(sort string, col *Collection) (string, error) {
	if strings.TrimSpace(sort) == "" {
		return `"created" DESC`, nil
	}

	// Build a set of allowed field names (schema fields + system fields).
	allowed := map[string]bool{
		"id": true, "created": true, "updated": true,
	}
	for _, f := range col.Fields {
		allowed[f.Name] = true
	}

	parts := strings.Split(sort, ",")
	sqlParts := make([]string, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		dir := "ASC"
		if strings.HasPrefix(p, "-") {
			dir = "DESC"
			p = p[1:]
		} else if strings.HasPrefix(p, "+") {
			p = p[1:]
		}

		if p == "@random" {
			sqlParts = append(sqlParts, "RANDOM()")
			continue
		}

		if !allowed[p] {
			return "", fmt.Errorf("unknown sort field %q", p)
		}
		sqlParts = append(sqlParts, fmt.Sprintf("%q %s", p, dir))
	}

	if len(sqlParts) == 0 {
		return `"created" DESC`, nil
	}
	return strings.Join(sqlParts, ", "), nil
}
