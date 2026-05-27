package keenbase

import (
	"fmt"
	"net/http"
	"strings"
)

type RuleOp string

const (
	RuleList   RuleOp = "list"
	RuleView   RuleOp = "view"
	RuleCreate RuleOp = "create"
	RuleUpdate RuleOp = "update"
	RuleDelete RuleOp = "delete"
)

func ruleFor(col *Collection, op RuleOp) *string {
	switch op {
	case RuleList:
		return col.Rules.ListRule
	case RuleView:
		return col.Rules.ViewRule
	case RuleCreate:
		return col.Rules.CreateRule
	case RuleUpdate:
		return col.Rules.UpdateRule
	case RuleDelete:
		return col.Rules.DeleteRule
	}
	return nil
}

func (sb *SimpleBase) checkRule(
	w http.ResponseWriter,
	r *http.Request,
	col *Collection,
	op RuleOp,
	record *Record, // the record being accessed (nil for list/create)
) error {
	auth := authFromContext(r.Context())

	if auth.IsSuperuser {
		return nil
	}

	rule := ruleFor(col, op)

	if rule == nil {
		writeError(w, http.StatusForbidden, "Only superusers can perform this action.")
		return fmt.Errorf("locked rule")
	}

	expr := strings.TrimSpace(*rule)
	if expr == "" {
		return nil
	}

	allowed, err := sb.evalRule(expr, auth, r, record)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("Invalid rule expression: %v", err))
		return err
	}
	if !allowed {
		// Return 403 for list (empty result would be confusing), and 404 for
		// view/update/delete (don't reveal the record exists).
		switch op {
		case RuleList, RuleCreate:
			writeError(w, http.StatusForbidden, "You are not allowed to perform this action.")
		default:
			writeError(w, http.StatusNotFound, "The requested resource wasn't found.")
		}
		return fmt.Errorf("rule denied")
	}

	return nil
}

func (sb *SimpleBase) evalRule(
	expr string,
	auth *RequestAuth,
	r *http.Request,
	record *Record,
) (bool, error) {

	data := map[string]any{}

	if record != nil {
		for k, v := range record.data {
			data[k] = v
		}
		data["id"] = record.id
	}

	expr = sb.resolveAuthPlaceholders(expr, auth, r)

	if strings.Contains(expr, "@request") {
		return false, fmt.Errorf("unsupported @request placeholder in rule: %q", expr)
	}

	return sb.evalRuleSQL(expr, data)
}

func (sb *SimpleBase) resolveAuthPlaceholders(expr string, auth *RequestAuth, r *http.Request) string {

	authID := ""
	if !auth.IsGuest() {
		authID = auth.RecordID
	}
	expr = strings.ReplaceAll(expr, "@request.auth.id", fmt.Sprintf("%q", authID))

	if strings.Contains(expr, "@request.auth.") && !auth.IsGuest() {
		authRecord := sb.loadAuthRecord(auth)
		if authRecord != nil {
			// Replace each @request.auth.<field> with its value.
			for field, val := range authRecord.data {
				placeholder := "@request.auth." + field
				if strings.Contains(expr, placeholder) {
					expr = strings.ReplaceAll(expr, placeholder, fmt.Sprintf("%q", fmt.Sprintf("%v", val)))
				}
			}
		}
	}

	for strings.Contains(expr, "@request.auth.") {
		start := strings.Index(expr, "@request.auth.")
		end := start + len("@request.auth.")
		for end < len(expr) && (expr[end] == '_' || (expr[end] >= 'a' && expr[end] <= 'z') || (expr[end] >= 'A' && expr[end] <= 'Z') || (expr[end] >= '0' && expr[end] <= '9')) {
			end++
		}
		expr = expr[:start] + `""` + expr[end:]
	}

	return expr
}

func (sb *SimpleBase) loadAuthRecord(auth *RequestAuth) *Record {
	if auth.record != nil {
		return auth.record
	}
	col, err := sb.collections.GetByID(auth.CollectionID)
	if err != nil || col == nil {
		return nil
	}
	rec, err := sb.records.GetByID(col, auth.RecordID)
	if err != nil {
		return nil
	}
	auth.record = rec
	return rec
}

func (sb *SimpleBase) evalRuleSQL(expr string, data map[string]any) (bool, error) {
	whereSQL, whereArgs, err := filterToSQL(expr)
	if err != nil {
		return false, err
	}

	if len(data) == 0 {
		// No record context — just evaluate the WHERE clause directly.
		query := fmt.Sprintf("SELECT 1 WHERE %s", whereSQL)
		var n int
		err := sb.db.QueryRow(query, whereArgs...).Scan(&n)
		if err != nil {
			return false, nil // no rows = condition false
		}
		return true, nil
	}

	cols := make([]string, 0, len(data))
	vals := make([]any, 0, len(data))
	placeholders := make([]string, 0, len(data))
	for k, v := range data {
		cols = append(cols, fmt.Sprintf("%q", k))
		vals = append(vals, v)
		placeholders = append(placeholders, "?")
	}

	inner := fmt.Sprintf("SELECT %s", joinAliased(cols, placeholders))
	query := fmt.Sprintf("SELECT 1 FROM (%s) WHERE %s", inner, whereSQL)

	allArgs := append(vals, whereArgs...)
	var n int
	err = sb.db.QueryRow(query, allArgs...).Scan(&n)
	if err != nil {
		return false, nil // no rows = condition false
	}
	return true, nil
}

func joinAliased(cols, placeholders []string) string {
	parts := make([]string, len(cols))
	for i := range cols {
		parts[i] = placeholders[i] + " AS " + cols[i]
	}
	return strings.Join(parts, ", ")
}
