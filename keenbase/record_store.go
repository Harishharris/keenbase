package keenbase

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type ListParams struct {
	Page      int
	PerPage   int
	Sort      string
	Filter    string
	SkipTotal bool
}

type ListResult struct {
	Page       int       `json:"page"`
	PerPage    int       `json:"perPage"`
	TotalItems int       `json:"totalItems"`
	TotalPages int       `json:"totalPages"`
	Items      []*Record `json:"items"`
}

type RecordStore struct {
	db *sql.DB
}

func newRecordStore(db *sql.DB) *RecordStore {
	return &RecordStore{db: db}
}

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

	source := tableSource(col)

	result := &ListResult{
		Page:       params.Page,
		PerPage:    params.PerPage,
		TotalItems: -1,
		TotalPages: -1,
		Items:      []*Record{},
	}

	if !params.SkipTotal {
		countSQL := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, source, whereSQL)
		if err := rs.db.QueryRow(countSQL, whereArgs...).Scan(&result.TotalItems); err != nil {
			return nil, fmt.Errorf("count query: %w", err)
		}
		if result.TotalItems == 0 {
			result.TotalPages = 0
			return result, nil
		}
		result.TotalPages = (result.TotalItems + params.PerPage - 1) / params.PerPage
	}

	dataSQL := fmt.Sprintf(
		`SELECT * FROM %s WHERE %s ORDER BY %s LIMIT %d OFFSET %d`,
		source, whereSQL, orderSQL, params.PerPage, offset,
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

func (rs *RecordStore) GetByID(col *Collection, id string) (*Record, error) {
	source := tableSource(col)
	query := fmt.Sprintf(`SELECT * FROM %s WHERE "id" = ? LIMIT 1`, source)
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

func (rs *RecordStore) Create(col *Collection, data map[string]any) (*Record, error) {
	if err := validateRecordData(col, data, true); err != nil {
		return nil, err
	}

	now := time.Now().UTC()

	id := newID()
	if supplied, ok := data["id"].(string); ok && supplied != "" {
		id = supplied
	}

	row := make(map[string]any, len(data)+3)
	for k, v := range data {
		if k == "id" {
			continue // handled above
		}
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

func (rs *RecordStore) Update(col *Collection, id string, data map[string]any) (*Record, error) {
	existing, err := rs.GetByID(col, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

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

func (rs *RecordStore) Delete(col *Collection, id string) error {
	stmt := fmt.Sprintf(`DELETE FROM %q WHERE "id" = ?`, col.Name)
	_, err := rs.db.Exec(stmt, id)
	return err
}

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

func scanRecord(col *Collection, rows *sql.Rows) (*Record, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	raw := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range raw {
		ptrs[i] = &raw[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, err
	}

	fieldTypes := make(map[string]FieldType, len(col.Fields)+4)
	for _, f := range col.Fields {
		fieldTypes[f.Name] = f.Type
	}
	if col.IsAuth() {
		fieldTypes["emailVisibility"] = FieldTypeBool
		fieldTypes["verified"] = FieldTypeBool
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

func tableSource(col *Collection) string {
	if col.Type == CollectionTypeView {
		return fmt.Sprintf("(%s) AS _view", col.ViewQuery)
	}
	return fmt.Sprintf("%q", col.Name)
}

func sortToSQL(sort string, col *Collection) (string, error) {
	if strings.TrimSpace(sort) == "" {
		return `"created" DESC`, nil
	}

	isView := col.Type == CollectionTypeView
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

		if !isView && !allowed[p] {
			return "", fmt.Errorf("unknown sort field %q", p)
		}
		sqlParts = append(sqlParts, fmt.Sprintf("%q %s", p, dir))
	}

	if len(sqlParts) == 0 {
		return `"created" DESC`, nil
	}
	return strings.Join(sqlParts, ", "), nil
}
