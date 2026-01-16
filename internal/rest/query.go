// internal/rest/query.go
package rest

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Filter struct {
	Column   string
	Operator string
	Value    string
}

// LogicalFilter groups multiple filters with OR or AND logic
type LogicalFilter struct {
	Operator string   // "or", "and"
	Filters  []Filter
}

type Query struct {
	Table          string
	Select         []string
	Filters        []Filter
	LogicalFilters []LogicalFilter
	Order          []OrderBy
	Limit          int
	Offset         int
	RLSCondition   string // RLS WHERE condition to apply
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
	"not":   "NOT", // not is a modifier, actual SQL depends on inner operator
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

	case "not":
		// not.operator.value -> negated SQL
		// Parse the inner operator and value
		return parseNotFilter(quotedColumn, f.Value)

	default:
		sqlOp := validOperators[f.Operator]
		return fmt.Sprintf("%s %s ?", quotedColumn, sqlOp), []any{f.Value}
	}
}

// parseNotFilter handles the not operator by negating the inner operator
// Value format: "operator.value" e.g., "eq.deleted" or "in.(1,2,3)"
func parseNotFilter(quotedColumn, value string) (string, []any) {
	// Split on first dot to get inner operator
	dotIdx := strings.Index(value, ".")
	if dotIdx == -1 {
		// Malformed, return a safe default
		return fmt.Sprintf("%s != ?", quotedColumn), []any{value}
	}

	innerOp := value[:dotIdx]
	innerVal := value[dotIdx+1:]

	switch innerOp {
	case "eq":
		return fmt.Sprintf("%s != ?", quotedColumn), []any{innerVal}
	case "neq":
		return fmt.Sprintf("%s = ?", quotedColumn), []any{innerVal}
	case "gt":
		return fmt.Sprintf("%s <= ?", quotedColumn), []any{innerVal}
	case "gte":
		return fmt.Sprintf("%s < ?", quotedColumn), []any{innerVal}
	case "lt":
		return fmt.Sprintf("%s >= ?", quotedColumn), []any{innerVal}
	case "lte":
		return fmt.Sprintf("%s > ?", quotedColumn), []any{innerVal}
	case "is":
		if innerVal == "null" {
			return fmt.Sprintf("%s IS NOT NULL", quotedColumn), nil
		}
		if innerVal == "not.null" {
			return fmt.Sprintf("%s IS NULL", quotedColumn), nil
		}
		return fmt.Sprintf("%s IS NOT ?", quotedColumn), []any{innerVal}
	case "in":
		values := parseInValues(innerVal)
		if len(values) == 0 {
			// Empty NOT IN is always true
			return "1 = 1", nil
		}
		placeholders := make([]string, len(values))
		args := make([]any, len(values))
		for i, v := range values {
			placeholders[i] = "?"
			args[i] = v
		}
		return fmt.Sprintf("%s NOT IN (%s)", quotedColumn, strings.Join(placeholders, ", ")), args
	case "like":
		pattern := convertWildcards(innerVal)
		return fmt.Sprintf("%s NOT LIKE ?", quotedColumn), []any{pattern}
	case "ilike":
		pattern := convertWildcards(innerVal)
		return fmt.Sprintf("LOWER(%s) NOT LIKE LOWER(?)", quotedColumn), []any{pattern}
	default:
		// Unknown inner operator, do basic negation
		return fmt.Sprintf("NOT (%s = ?)", quotedColumn), []any{innerVal}
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

// ParseLogicalFilter parses or/and query params like "(status.eq.active,status.eq.pending)"
func ParseLogicalFilter(operator, value string) (LogicalFilter, error) {
	// Validate operator
	if operator != "or" && operator != "and" {
		return LogicalFilter{}, fmt.Errorf("invalid logical operator: %s", operator)
	}

	// Remove surrounding parentheses
	value = strings.TrimPrefix(value, "(")
	value = strings.TrimSuffix(value, ")")

	if value == "" {
		return LogicalFilter{Operator: operator, Filters: nil}, nil
	}

	// Split by comma (handling nested parens for future extensibility)
	parts := splitLogicalParts(value)

	var filters []Filter
	for _, part := range parts {
		f, err := parseLogicalPart(part)
		if err != nil {
			return LogicalFilter{}, err
		}
		filters = append(filters, f)
	}

	return LogicalFilter{Operator: operator, Filters: filters}, nil
}

// splitLogicalParts splits a comma-separated list, respecting parentheses nesting
func splitLogicalParts(value string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, char := range value {
		switch char {
		case '(':
			depth++
			current.WriteRune(char)
		case ')':
			depth--
			current.WriteRune(char)
		case ',':
			if depth == 0 {
				if current.Len() > 0 {
					parts = append(parts, strings.TrimSpace(current.String()))
					current.Reset()
				}
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	// Don't forget the last part
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}

	return parts
}

// parseLogicalPart parses a single filter in logical format: "column.operator.value"
func parseLogicalPart(part string) (Filter, error) {
	// Format: column.operator.value (e.g., "status.eq.active")
	// Need to split carefully: column could have dots, operator is known set
	// Split on first dot to get column, then find operator

	firstDot := strings.Index(part, ".")
	if firstDot == -1 {
		return Filter{}, fmt.Errorf("invalid logical filter format: %s", part)
	}

	column := part[:firstDot]
	rest := part[firstDot+1:]

	// Find the operator by checking for known operators
	secondDot := strings.Index(rest, ".")
	if secondDot == -1 {
		return Filter{}, fmt.Errorf("invalid logical filter format: %s", part)
	}

	operator := rest[:secondDot]
	value := rest[secondDot+1:]

	// Validate operator
	if _, ok := validOperators[operator]; !ok {
		return Filter{}, fmt.Errorf("unknown operator: %s", operator)
	}

	return Filter{
		Column:   column,
		Operator: operator,
		Value:    value,
	}, nil
}

// ParseMatchFilter parses a match query parameter (JSON object) into multiple eq filters.
// match() is shorthand for multiple .eq() filters.
// Example: ?match={"status":"active","priority":"high"}
// becomes equivalent to ?status=eq.active&priority=eq.high
func ParseMatchFilter(jsonValue string) ([]Filter, error) {
	var matches map[string]any
	if err := json.Unmarshal([]byte(jsonValue), &matches); err != nil {
		return nil, fmt.Errorf("invalid match JSON: %w", err)
	}

	var filters []Filter
	for col, val := range matches {
		filters = append(filters, Filter{
			Column:   col,
			Operator: "eq",
			Value:    fmt.Sprintf("%v", val),
		})
	}
	return filters, nil
}
