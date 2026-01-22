// internal/realtime/filter.go
package realtime

import (
	"fmt"
	"strconv"
	"strings"
)

// matchesFilter evaluates a PostgREST-style filter against row data
// filter format: "column=operator.value" (e.g., "user_id=eq.123")
func matchesFilter(filter string, newRow, oldRow map[string]any) bool {
	// Parse filter: column=operator.value
	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return false
	}

	column := parts[0]
	opValue := parts[1]

	// Parse operator.value
	dotIdx := strings.Index(opValue, ".")
	if dotIdx == -1 {
		return false
	}

	operator := opValue[:dotIdx]
	value := opValue[dotIdx+1:]

	// Get row value (prefer new, fall back to old for DELETE)
	row := newRow
	if row == nil {
		row = oldRow
	}
	if row == nil {
		return false
	}

	rowValue, exists := row[column]
	if !exists {
		return false
	}

	return evaluateOperator(operator, rowValue, value)
}

// evaluateOperator evaluates a single operator comparison
func evaluateOperator(operator string, rowValue any, filterValue string) bool {
	switch operator {
	case "eq":
		return compareEqual(rowValue, filterValue)
	case "neq":
		return !compareEqual(rowValue, filterValue)
	case "gt":
		return compareNumeric(rowValue, filterValue) > 0
	case "gte":
		return compareNumeric(rowValue, filterValue) >= 0
	case "lt":
		return compareNumeric(rowValue, filterValue) < 0
	case "lte":
		return compareNumeric(rowValue, filterValue) <= 0
	case "in":
		return compareIn(rowValue, filterValue)
	default:
		return false
	}
}

// compareEqual checks if row value equals filter value
func compareEqual(rowValue any, filterValue string) bool {
	switch v := rowValue.(type) {
	case string:
		return v == filterValue
	case float64:
		fv, err := strconv.ParseFloat(filterValue, 64)
		if err != nil {
			return false
		}
		return v == fv
	case int64:
		iv, err := strconv.ParseInt(filterValue, 10, 64)
		if err != nil {
			return false
		}
		return v == iv
	case int:
		iv, err := strconv.Atoi(filterValue)
		if err != nil {
			return false
		}
		return v == iv
	case bool:
		return fmt.Sprintf("%v", v) == filterValue
	case nil:
		return filterValue == "null"
	default:
		return fmt.Sprintf("%v", v) == filterValue
	}
}

// compareNumeric compares row value to filter value numerically
// Returns: -1 if row < filter, 0 if equal, 1 if row > filter
func compareNumeric(rowValue any, filterValue string) int {
	var rowNum float64

	switch v := rowValue.(type) {
	case float64:
		rowNum = v
	case int64:
		rowNum = float64(v)
	case int:
		rowNum = float64(v)
	case string:
		var err error
		rowNum, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return 0 // Can't compare, treat as equal
		}
	default:
		return 0
	}

	filterNum, err := strconv.ParseFloat(filterValue, 64)
	if err != nil {
		return 0
	}

	if rowNum < filterNum {
		return -1
	} else if rowNum > filterNum {
		return 1
	}
	return 0
}

// compareIn checks if row value is in the filter value list
// filterValue format: "(val1,val2,val3)"
func compareIn(rowValue any, filterValue string) bool {
	// Remove parentheses
	filterValue = strings.TrimPrefix(filterValue, "(")
	filterValue = strings.TrimSuffix(filterValue, ")")

	values := strings.Split(filterValue, ",")
	for _, v := range values {
		v = strings.TrimSpace(v)
		if compareEqual(rowValue, v) {
			return true
		}
	}
	return false
}
