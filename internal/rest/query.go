// internal/rest/query.go
package rest

import (
	"fmt"
	"strings"
)

type Filter struct {
	Column   string
	Operator string
	Value    string
}

type Query struct {
	Table        string
	Select       []string
	Filters      []Filter
	Order        []OrderBy
	Limit        int
	Offset       int
	RLSCondition string // RLS WHERE condition to apply
}

type OrderBy struct {
	Column string
	Desc   bool
}

var validOperators = map[string]string{
	"eq":    "=",
	"neq":   "!=",
	"gt":    ">",
	"gte":   ">=",
	"lt":    "<",
	"lte":   "<=",
	"is":    "IS",
	"in":    "IN",
	"like":  "LIKE",
	"ilike": "ILIKE",
}

func ParseFilter(input string) (Filter, error) {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return Filter{}, fmt.Errorf("invalid filter format: %s", input)
	}

	column := parts[0]
	opValue := parts[1]

	opParts := strings.SplitN(opValue, ".", 2)
	if len(opParts) != 2 {
		return Filter{}, fmt.Errorf("invalid operator format: %s", opValue)
	}

	operator := opParts[0]
	value := opParts[1]

	if _, ok := validOperators[operator]; !ok {
		return Filter{}, fmt.Errorf("unknown operator: %s", operator)
	}

	return Filter{
		Column:   column,
		Operator: operator,
		Value:    value,
	}, nil
}

func (f Filter) ToSQL() (string, []any) {
	quotedColumn := fmt.Sprintf("\"%s\"", f.Column)

	switch f.Operator {
	case "is":
		if f.Value == "null" {
			return fmt.Sprintf("%s IS NULL", quotedColumn), nil
		}
		if f.Value == "not.null" {
			return fmt.Sprintf("%s IS NOT NULL", quotedColumn), nil
		}
		return fmt.Sprintf("%s IS ?", quotedColumn), []any{f.Value}

	case "in":
		// Parse value format: (val1,val2,val3)
		values := parseInValues(f.Value)
		if len(values) == 0 {
			// Empty IN clause - return a condition that's always false
			return "1 = 0", nil
		}
		placeholders := make([]string, len(values))
		args := make([]any, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args[i] = v
		}
		return fmt.Sprintf("%s IN (%s)", quotedColumn, strings.Join(placeholders, ", ")), args

	case "like":
		// Convert PostgREST wildcards (*) to SQL wildcards (%)
		pattern := convertWildcards(f.Value)
		return fmt.Sprintf("%s LIKE ?", quotedColumn), []any{pattern}

	case "ilike":
		// Case-insensitive LIKE for SQLite using LOWER()
		pattern := convertWildcards(f.Value)
		return fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", quotedColumn), []any{pattern}

	default:
		sqlOp := validOperators[f.Operator]
		return fmt.Sprintf("%s %s ?", quotedColumn, sqlOp), []any{f.Value}
	}
}

// parseInValues parses "(val1,val2,val3)" format into slice of values
func parseInValues(value string) []string {
	// Remove surrounding parentheses
	value = strings.TrimPrefix(value, "(")
	value = strings.TrimSuffix(value, ")")

	if value == "" {
		return nil
	}

	// Split by comma, handling potential whitespace
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// convertWildcards converts PostgREST wildcards (*) to SQL wildcards (%)
func convertWildcards(pattern string) string {
	// PostgREST uses * as wildcard, SQL uses %
	// But supabase-js client sends % directly, so handle both
	return strings.ReplaceAll(pattern, "*", "%")
}

func ParseSelect(selectParam string) []string {
	if selectParam == "" {
		return []string{"*"}
	}
	return strings.Split(selectParam, ",")
}

func ParseOrder(orderParam string) []OrderBy {
	if orderParam == "" {
		return nil
	}

	var orders []OrderBy
	parts := strings.Split(orderParam, ",")
	for _, part := range parts {
		order := OrderBy{Column: part, Desc: false}
		if strings.HasSuffix(part, ".desc") {
			order.Column = strings.TrimSuffix(part, ".desc")
			order.Desc = true
		} else if strings.HasSuffix(part, ".asc") {
			order.Column = strings.TrimSuffix(part, ".asc")
		}
		orders = append(orders, order)
	}
	return orders
}
