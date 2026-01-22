package vector

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/markb/sblite/internal/rls"
	"github.com/markb/sblite/internal/schema"
)

// SearchParams defines the parameters for a vector search.
type SearchParams struct {
	TableName       string         `json:"table_name"`
	EmbeddingColumn string         `json:"embedding_column"`
	QueryEmbedding  Vector         `json:"query_embedding"`
	MatchCount      int            `json:"match_count"`
	MatchThreshold  float64        `json:"match_threshold"`
	SelectColumns   []string       `json:"select_columns"`
	Metric          string         `json:"metric"`
	Filter          map[string]any `json:"filter"`
}

// SearchResult represents a single search result with its similarity score.
type SearchResult struct {
	Row        map[string]any `json:"row"`
	Similarity float64        `json:"similarity"`
}

// Searcher performs vector similarity search with RLS support.
type Searcher struct {
	db          *sql.DB
	rlsEnforcer *rls.Enforcer
	schema      *schema.Schema
}

// NewSearcher creates a new Searcher.
func NewSearcher(db *sql.DB, rlsEnforcer *rls.Enforcer, schema *schema.Schema) *Searcher {
	return &Searcher{
		db:          db,
		rlsEnforcer: rlsEnforcer,
		schema:      schema,
	}
}

// Search performs a vector similarity search with RLS enforcement.
func (s *Searcher) Search(params SearchParams, authCtx *rls.AuthContext) ([]SearchResult, error) {
	// Set defaults
	if params.MatchCount <= 0 {
		params.MatchCount = 10
	}
	if params.Metric == "" {
		params.Metric = "cosine"
	}

	// Validate metric
	validMetric := false
	for _, m := range SupportedMetrics {
		if params.Metric == m {
			validMetric = true
			break
		}
	}
	if !validMetric {
		return nil, fmt.Errorf("invalid metric: %s (supported: %v)", params.Metric, SupportedMetrics)
	}

	// Validate required parameters
	if params.TableName == "" {
		return nil, fmt.Errorf("table_name is required")
	}
	if params.EmbeddingColumn == "" {
		return nil, fmt.Errorf("embedding_column is required")
	}
	if len(params.QueryEmbedding) == 0 {
		return nil, fmt.Errorf("query_embedding is required")
	}

	// Validate table exists and get column info
	columns, err := s.schema.GetColumns(params.TableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get table schema: %w", err)
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("table %q not found", params.TableName)
	}

	// Validate embedding column exists and is a vector type
	embCol, ok := columns[params.EmbeddingColumn]
	if !ok {
		return nil, fmt.Errorf("embedding column %q not found in table %q", params.EmbeddingColumn, params.TableName)
	}
	if !IsVectorType(embCol.PgType) {
		return nil, fmt.Errorf("column %q is not a vector type (type: %s)", params.EmbeddingColumn, embCol.PgType)
	}

	// Get expected dimension from column type
	expectedDim, _ := ParseVectorType(embCol.PgType)
	if expectedDim > 0 && len(params.QueryEmbedding) != expectedDim {
		return nil, fmt.Errorf("query_embedding dimension mismatch: expected %d, got %d", expectedDim, len(params.QueryEmbedding))
	}

	// Build column list for SELECT
	selectCols := params.SelectColumns
	if len(selectCols) == 0 {
		// Select all columns
		for colName := range columns {
			selectCols = append(selectCols, colName)
		}
	}

	// Ensure embedding column is selected for similarity calculation
	hasEmbedding := false
	for _, col := range selectCols {
		if col == params.EmbeddingColumn {
			hasEmbedding = true
			break
		}
	}
	if !hasEmbedding {
		selectCols = append(selectCols, params.EmbeddingColumn)
	}

	// Build query
	quotedCols := make([]string, len(selectCols))
	for i, col := range selectCols {
		quotedCols[i] = fmt.Sprintf(`"%s"`, col)
	}

	query := fmt.Sprintf(`SELECT %s FROM "%s"`, strings.Join(quotedCols, ", "), params.TableName)

	// Build WHERE conditions
	var conditions []string
	var args []any

	// Apply RLS conditions
	if s.rlsEnforcer != nil {
		rlsCondition, err := s.rlsEnforcer.GetSelectConditions(params.TableName, authCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to get RLS conditions: %w", err)
		}
		if rlsCondition != "" {
			conditions = append(conditions, rlsCondition)
		}
	}

	// Apply filter conditions
	if len(params.Filter) > 0 {
		for col, val := range params.Filter {
			// Verify column exists
			if _, ok := columns[col]; !ok {
				return nil, fmt.Errorf("filter column %q not found", col)
			}
			conditions = append(conditions, fmt.Sprintf(`"%s" = ?`, col))
			args = append(args, val)
		}
	}

	// Only include rows with non-null embeddings
	conditions = append(conditions, fmt.Sprintf(`"%s" IS NOT NULL`, params.EmbeddingColumn))

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	// Execute query
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	colNames, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Find embedding column index
	embeddingIdx := -1
	for i, name := range colNames {
		if name == params.EmbeddingColumn {
			embeddingIdx = i
			break
		}
	}
	if embeddingIdx < 0 {
		return nil, fmt.Errorf("embedding column not in result set")
	}

	// Collect all rows and compute similarities
	var results []SearchResult
	for rows.Next() {
		values := make([]any, len(colNames))
		valuePtrs := make([]any, len(colNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}

		// Parse embedding from stored JSON
		embeddingStr, ok := values[embeddingIdx].(string)
		if !ok {
			continue // Skip rows with invalid embeddings
		}
		rowEmbedding, err := ParseVector(embeddingStr)
		if err != nil {
			continue // Skip rows with invalid embeddings
		}

		// Compute similarity
		similarity, err := ComputeSimilarity(params.QueryEmbedding, rowEmbedding, params.Metric)
		if err != nil {
			continue // Skip on error
		}

		// Apply threshold (for cosine, higher is better; for l2, similarity is negative distance)
		if params.MatchThreshold != 0 {
			if params.Metric == "cosine" || params.Metric == "dot" {
				if similarity < params.MatchThreshold {
					continue
				}
			} else if params.Metric == "l2" {
				// For L2, threshold is maximum distance (we return negative)
				// So if threshold is 0.5, we want similarity >= -0.5
				if similarity < -params.MatchThreshold {
					continue
				}
			}
		}

		// Build row map (excluding embedding if not in original select)
		row := make(map[string]any)
		for i, name := range colNames {
			// Skip embedding column if it wasn't requested
			if name == params.EmbeddingColumn && !hasEmbedding {
				continue
			}
			row[name] = values[i]
		}

		results = append(results, SearchResult{
			Row:        row,
			Similarity: similarity,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	// Sort by similarity (descending for cosine/dot, ascending for l2 since we negate)
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Limit results
	if len(results) > params.MatchCount {
		results = results[:params.MatchCount]
	}

	return results, nil
}
