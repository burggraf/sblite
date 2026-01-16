// internal/rest/csv.go
package rest

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
)

// writeCSV writes the results as CSV format to the response writer.
// Column headers are sorted alphabetically for consistent ordering.
// nil values are converted to empty strings, and nested objects are JSON-encoded.
func (h *Handler) writeCSV(w http.ResponseWriter, results []map[string]any) {
	w.Header().Set("Content-Type", "text/csv")

	if len(results) == 0 {
		// Empty results - write nothing (no headers, no rows)
		return
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// Get column headers from first row and sort for consistent ordering
	var headers []string
	for key := range results[0] {
		headers = append(headers, key)
	}
	sort.Strings(headers)

	// Write header row
	if err := writer.Write(headers); err != nil {
		return
	}

	// Write data rows
	for _, row := range results {
		values := make([]string, len(headers))
		for i, header := range headers {
			values[i] = formatCSVValue(row[header])
		}
		if err := writer.Write(values); err != nil {
			return
		}
	}
}

// formatCSVValue converts a value to its CSV string representation.
// - nil values become empty strings
// - strings are returned as-is
// - maps and slices are JSON-encoded
// - all other types use fmt.Sprintf
func formatCSVValue(v any) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case map[string]any, []any:
		// JSON-encode nested objects and arrays
		jsonBytes, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(jsonBytes)
	default:
		return fmt.Sprintf("%v", val)
	}
}
