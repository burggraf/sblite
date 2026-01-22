package vector

import (
	"fmt"
)

// BuiltinFunctions lists the names of built-in vector functions.
var BuiltinFunctions = []string{"vector_search"}

// IsBuiltinFunction checks if a function name is a built-in vector function.
func IsBuiltinFunction(name string) bool {
	for _, fn := range BuiltinFunctions {
		if name == fn {
			return true
		}
	}
	return false
}

// ParseSearchParams extracts SearchParams from RPC arguments.
func ParseSearchParams(args map[string]any) (SearchParams, error) {
	params := SearchParams{}

	// Required: table_name
	if tableName, ok := args["table_name"].(string); ok && tableName != "" {
		params.TableName = tableName
	} else {
		return params, fmt.Errorf("missing required argument: table_name")
	}

	// Required: embedding_column
	if embCol, ok := args["embedding_column"].(string); ok && embCol != "" {
		params.EmbeddingColumn = embCol
	} else {
		return params, fmt.Errorf("missing required argument: embedding_column")
	}

	// Required: query_embedding
	if queryEmb, ok := args["query_embedding"]; ok {
		vec, err := ValidateVectorValue(queryEmb, 0) // 0 = no dimension check
		if err != nil {
			return params, fmt.Errorf("invalid query_embedding: %w", err)
		}
		params.QueryEmbedding = vec
	} else {
		return params, fmt.Errorf("missing required argument: query_embedding")
	}

	// Optional: match_count (default: 10)
	if matchCount, ok := args["match_count"]; ok {
		switch v := matchCount.(type) {
		case float64:
			params.MatchCount = int(v)
		case int:
			params.MatchCount = v
		case int64:
			params.MatchCount = int(v)
		}
	}

	// Optional: match_threshold (default: 0)
	if threshold, ok := args["match_threshold"]; ok {
		switch v := threshold.(type) {
		case float64:
			params.MatchThreshold = v
		case int:
			params.MatchThreshold = float64(v)
		case int64:
			params.MatchThreshold = float64(v)
		}
	}

	// Optional: select_columns
	if selectCols, ok := args["select_columns"]; ok {
		switch v := selectCols.(type) {
		case []string:
			params.SelectColumns = v
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					params.SelectColumns = append(params.SelectColumns, s)
				}
			}
		}
	}

	// Optional: metric (default: "cosine")
	if metric, ok := args["metric"].(string); ok && metric != "" {
		params.Metric = metric
	}

	// Optional: filter
	if filter, ok := args["filter"].(map[string]any); ok {
		params.Filter = filter
	}

	return params, nil
}

// FormatSearchResults converts SearchResults to the format expected by RPC response.
// Returns a slice of maps with all row fields plus a "similarity" field.
func FormatSearchResults(results []SearchResult) []map[string]any {
	formatted := make([]map[string]any, len(results))
	for i, r := range results {
		row := make(map[string]any)
		for k, v := range r.Row {
			row[k] = v
		}
		row["similarity"] = r.Similarity
		formatted[i] = row
	}
	return formatted
}
