package keenbase

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/ganigeorgiev/fexpr"
)

// filterToSQL parses a PocketBase-style filter expression and returns a
// SQL WHERE clause fragment plus the ordered bind arguments.
//
// Examples:
//
//	"title = 'hello'"           →  `"title" = ?`          args: ["hello"]
//	"count > 5 && active = true" →  `"count" > ? AND "active" = ?`  args: [5, true]
//	"title ~ 'go'"              →  `"title" LIKE ?`        args: ["%go%"]
func filterToSQL(expr string) (string, []any, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return "1=1", nil, nil
	}

	items, err := fexpr.Parse(expr)
	if err != nil {
		return "", nil, fmt.Errorf("invalid filter: %w", err)
	}

	fb := &filterBuilder{}
	sql, err := fb.buildGroup(items)
	if err != nil {
		return "", nil, err
	}
	return sql, fb.args, nil
}

// filterBuilder walks an fexpr AST and accumulates SQL fragments + bind args.
type filterBuilder struct {
	args []any
}

func (fb *filterBuilder) buildGroup(items []fexpr.ExprGroup) (string, error) {
	var parts []string

	for _, item := range items {
		var fragment string
		var err error

		switch v := item.Item.(type) {
		case fexpr.Expr:
			fragment, err = fb.buildExpr(v)
		case []fexpr.ExprGroup:
			inner, err := fb.buildGroup(v)
			if err != nil {
				return "", err
			}
			fragment = "(" + inner + ")"
		default:
			return "", fmt.Errorf("unexpected filter item type %T", item.Item)
		}

		if err != nil {
			return "", err
		}

		if len(parts) > 0 {
			if item.Join == fexpr.JoinOr {
				parts = append(parts, "OR")
			} else {
				parts = append(parts, "AND")
			}
		}
		parts = append(parts, fragment)
	}

	return strings.Join(parts, " "), nil
}

func (fb *filterBuilder) buildExpr(expr fexpr.Expr) (string, error) {
	// Left side is always a column identifier (quoted for safety).
	if expr.Left.Type != fexpr.TokenIdentifier {
		return "", fmt.Errorf("left operand must be a field name, got %q", expr.Left.Literal)
	}
	col := fmt.Sprintf("%q", expr.Left.Literal)

	// Right side is a literal value turned into a bind argument.
	val := tokenValue(expr.Right)
	switch expr.Op {
	case fexpr.SignEq:
		if val == nil {
			return fmt.Sprintf("%s IS NULL", col), nil
		}
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s = ?", col), nil

	case fexpr.SignNeq:
		if val == nil {
			return fmt.Sprintf("%s IS NOT NULL", col), nil
		}
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s != ?", col), nil

	case fexpr.SignGt:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s > ?", col), nil

	case fexpr.SignGte:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s >= ?", col), nil

	case fexpr.SignLt:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s < ?", col), nil

	case fexpr.SignLte:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s <= ?", col), nil

	case fexpr.SignLike:
		// Auto-wrap the value in % wildcards if it's a plain string
		// (matches PocketBase behaviour).
		s := fmt.Sprintf("%v", val)
		if !strings.Contains(s, "%") {
			s = "%" + s + "%"
		}
		fb.args = append(fb.args, s)
		return fmt.Sprintf("%s LIKE ?", col), nil

	case fexpr.SignNlike:
		s := fmt.Sprintf("%v", val)
		if !strings.Contains(s, "%") {
			s = "%" + s + "%"
		}
		fb.args = append(fb.args, s)
		return fmt.Sprintf("%s NOT LIKE ?", col), nil

	// "Any/at-least-one-of" variants — for simple scalar columns they
	// behave identically to the non-? form.
	case fexpr.SignAnyEq:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s = ?", col), nil

	case fexpr.SignAnyNeq:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s != ?", col), nil

	case fexpr.SignAnyGt:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s > ?", col), nil

	case fexpr.SignAnyGte:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s >= ?", col), nil

	case fexpr.SignAnyLt:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s < ?", col), nil

	case fexpr.SignAnyLte:
		fb.args = append(fb.args, val)
		return fmt.Sprintf("%s <= ?", col), nil

	case fexpr.SignAnyLike:
		s := fmt.Sprintf("%v", val)
		if !strings.Contains(s, "%") {
			s = "%" + s + "%"
		}
		fb.args = append(fb.args, s)
		return fmt.Sprintf("%s LIKE ?", col), nil

	case fexpr.SignAnyNlike:
		s := fmt.Sprintf("%v", val)
		if !strings.Contains(s, "%") {
			s = "%" + s + "%"
		}
		fb.args = append(fb.args, s)
		return fmt.Sprintf("%s NOT LIKE ?", col), nil

	default:
		return "", fmt.Errorf("unsupported filter operator %q", expr.Op)
	}
}

// tokenValue converts an fexpr Token to a typed Go value suitable for use
// as a SQL bind argument.
//
// fexpr represents true/false/null as TokenIdentifier with those literal
// strings — there are no separate Bool or Null token types.
func tokenValue(t fexpr.Token) any {
	switch t.Type {
	case fexpr.TokenText:
		return t.Literal
	case fexpr.TokenNumber:
		if f, err := strconv.ParseFloat(t.Literal, 64); err == nil {
			return f
		}
		return t.Literal
	case fexpr.TokenIdentifier:
		switch t.Literal {
		case "true":
			return true
		case "false":
			return false
		case "null":
			return nil
		default:
			// Bare identifier on the right side — treat as a string value.
			return t.Literal
		}
	default:
		return t.Literal
	}
}
