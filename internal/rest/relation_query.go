// internal/rest/relation_query.go
package rest

import (
	"database/sql"
	"fmt"
	"strings"
)

// RelationQueryExecutor handles executing queries with embedded relations.
// It uses the RelationshipCache to determine relationship types and executes
// sub-queries to fetch and embed related data.
type RelationQueryExecutor struct {
	db       *sql.DB
	relCache *RelationshipCache
}

// NewRelationQueryExecutor creates a new RelationQueryExecutor with the given database
// connection and relationship cache.
func NewRelationQueryExecutor(db *sql.DB, relCache *RelationshipCache) *RelationQueryExecutor {
	return &RelationQueryExecutor{
		db:       db,
		relCache: relCache,
	}
}

// ExecuteWithRelations executes a query and embeds related data according to the parsed select.
// It first executes the main query for base columns, then for each relation in the select,
// it executes sub-queries and embeds the results into the main results.
func (rqe *RelationQueryExecutor) ExecuteWithRelations(q Query, parsed *ParsedSelect) ([]map[string]any, error) {
	// 1. Extract base columns (non-relation columns)
	baseColumns := parsed.ToColumnNames()

	// If no base columns, default to * for relationship matching
	if len(baseColumns) == 0 {
		baseColumns = []string{"*"}
	}

	// 2. For each relation, ensure we include the necessary FK/PK columns in the main query
	// This is needed for the relationship lookups
	neededColumns := make(map[string]bool)
	for _, col := range parsed.Columns {
		if col.Relation != nil {
			relDef, err := rqe.relCache.FindRelationship(q.Table, col.Relation.Name)
			if err != nil {
				return nil, fmt.Errorf("failed to find relationship %s: %w", col.Relation.Name, err)
			}
			if relDef != nil {
				// For many-to-one, we need the local FK column
				// For one-to-many, we need the local PK column (usually 'id')
				neededColumns[relDef.LocalColumn] = true
			}
		}
	}

	// Add needed columns to baseColumns if not already present and not using *
	if !containsColumn(baseColumns, "*") {
		for col := range neededColumns {
			baseColumns = ensureColumnIncluded(baseColumns, col)
		}
	}

	// 3. Execute main query with base columns
	mainQ := Query{
		Table:          q.Table,
		Select:         baseColumns,
		Filters:        q.Filters,
		LogicalFilters: q.LogicalFilters,
		Order:          q.Order,
		Limit:          q.Limit,
		Offset:         q.Offset,
		RLSCondition:   q.RLSCondition,
	}

	mainSQL, args := BuildSelectQuery(mainQ)
	rows, err := rqe.db.Query(mainSQL, args...)
	if err != nil {
		return nil, fmt.Errorf("main query failed: %w", err)
	}

	results, err := scanRowsGeneric(rows)
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to scan main results: %w", err)
	}

	// 4. For each relation in the parsed select, execute sub-query and embed
	for _, col := range parsed.Columns {
		if col.Relation != nil {
			if err := rqe.embedRelation(results, q.Table, col.Relation); err != nil {
				return nil, fmt.Errorf("failed to embed relation %s: %w", col.Relation.Name, err)
			}
		}
	}

	// 5. Remove added FK columns that weren't explicitly requested
	requestedCols := parsed.ToColumnNames()
	if !containsColumn(requestedCols, "*") {
		for _, row := range results {
			for col := range neededColumns {
				if !containsColumn(requestedCols, col) {
					delete(row, col)
				}
			}
		}
	}

	return results, nil
}

// embedRelation embeds related data into the results based on the relationship type.
func (rqe *RelationQueryExecutor) embedRelation(results []map[string]any, table string, rel *SelectRelation) error {
	if len(results) == 0 {
		return nil
	}

	// Look up the relationship definition
	relDef, err := rqe.relCache.FindRelationship(table, rel.Name)
	if err != nil {
		return fmt.Errorf("failed to find relationship: %w", err)
	}

	if relDef == nil {
		// No relationship found - set all results to null for this relation
		embedName := rel.Alias
		if embedName == "" {
			embedName = rel.Name
		}
		for _, row := range results {
			row[embedName] = nil
		}
		return nil
	}

	switch relDef.Type {
	case "many-to-one":
		return rqe.embedManyToOne(results, relDef, rel)
	case "one-to-many":
		return rqe.embedOneToMany(results, relDef, rel)
	default:
		return fmt.Errorf("unknown relationship type: %s", relDef.Type)
	}
}

// embedManyToOne handles many-to-one relationships (e.g., city -> country).
// Each result row gets a single related object (or null).
func (rqe *RelationQueryExecutor) embedManyToOne(results []map[string]any, relDef *Relationship, rel *SelectRelation) error {
	// Collect all foreign key values from the results
	fkValues := make([]any, 0)
	fkSet := make(map[any]bool)

	for _, row := range results {
		if fk, ok := row[relDef.LocalColumn]; ok && fk != nil {
			// Deduplicate FK values
			if !fkSet[fk] {
				fkValues = append(fkValues, fk)
				fkSet[fk] = true
			}
		}
	}

	if len(fkValues) == 0 {
		// No foreign keys to look up - set all to null
		embedName := rel.Alias
		if embedName == "" {
			embedName = rel.Name
		}
		for _, row := range results {
			row[embedName] = nil
		}
		return nil
	}

	// Determine which columns to select from the related table
	cols := extractColumnNames(rel.Columns)
	// Always include the foreign column for matching
	colsWithFK := ensureColumnIncluded(cols, relDef.ForeignColumn)

	// Also include FK columns needed for any nested relations
	nestedFKCols := make(map[string]bool)
	for _, relCol := range rel.Columns {
		if relCol.Relation != nil {
			nestedRelDef, err := rqe.relCache.FindRelationship(relDef.ForeignTable, relCol.Relation.Name)
			if err == nil && nestedRelDef != nil {
				nestedFKCols[nestedRelDef.LocalColumn] = true
				colsWithFK = ensureColumnIncluded(colsWithFK, nestedRelDef.LocalColumn)
			}
		}
	}

	// Build and execute query for related table
	placeholders := make([]string, len(fkValues))
	for i := range fkValues {
		placeholders[i] = "?"
	}

	quotedCols := make([]string, len(colsWithFK))
	for i, c := range colsWithFK {
		if c == "*" {
			quotedCols[i] = "*"
		} else {
			quotedCols[i] = fmt.Sprintf("\"%s\"", c)
		}
	}

	sqlStr := fmt.Sprintf(`SELECT %s FROM "%s" WHERE "%s" IN (%s)`,
		strings.Join(quotedCols, ", "),
		relDef.ForeignTable,
		relDef.ForeignColumn,
		strings.Join(placeholders, ", "))

	rows, err := rqe.db.Query(sqlStr, fkValues...)
	if err != nil {
		return fmt.Errorf("relation query failed: %w", err)
	}

	relResults, err := scanRowsGeneric(rows)
	rows.Close()
	if err != nil {
		return fmt.Errorf("failed to scan relation results: %w", err)
	}

	// Handle nested relations in the related results
	for _, relCol := range rel.Columns {
		if relCol.Relation != nil {
			if err := rqe.embedRelation(relResults, relDef.ForeignTable, relCol.Relation); err != nil {
				return fmt.Errorf("failed to embed nested relation %s: %w", relCol.Relation.Name, err)
			}
		}
	}

	// Index related results by foreign key
	relIndex := make(map[any]map[string]any)
	for _, relRow := range relResults {
		key := relRow[relDef.ForeignColumn]
		// Remove the FK column from the result if it wasn't explicitly requested
		if !containsColumn(cols, relDef.ForeignColumn) && !containsColumn(cols, "*") {
			delete(relRow, relDef.ForeignColumn)
		}
		// Also remove nested FK columns that weren't explicitly requested
		for nestedCol := range nestedFKCols {
			if !containsColumn(cols, nestedCol) && !containsColumn(cols, "*") {
				delete(relRow, nestedCol)
			}
		}
		relIndex[key] = relRow
	}

	// Embed into results
	embedName := rel.Alias
	if embedName == "" {
		embedName = rel.Name
	}

	for _, row := range results {
		fk := row[relDef.LocalColumn]
		if relData, ok := relIndex[fk]; ok {
			row[embedName] = relData
		} else {
			row[embedName] = nil
		}
	}

	return nil
}

// embedOneToMany handles one-to-many relationships (e.g., country -> cities).
// Each result row gets an array of related objects (possibly empty).
func (rqe *RelationQueryExecutor) embedOneToMany(results []map[string]any, relDef *Relationship, rel *SelectRelation) error {
	// For one-to-many: LocalColumn is the column in our table being referenced (e.g., "id")
	// ForeignColumn is the FK column in the child table (e.g., "country_id")

	// Collect all primary key values from the results
	pkValues := make([]any, 0)
	pkSet := make(map[any]bool)

	for _, row := range results {
		if pk, ok := row[relDef.LocalColumn]; ok && pk != nil {
			if !pkSet[pk] {
				pkValues = append(pkValues, pk)
				pkSet[pk] = true
			}
		}
	}

	// Initialize all results with empty arrays
	embedName := rel.Alias
	if embedName == "" {
		embedName = rel.Name
	}

	for _, row := range results {
		row[embedName] = []map[string]any{}
	}

	if len(pkValues) == 0 {
		return nil
	}

	// Determine which columns to select from the child table
	cols := extractColumnNames(rel.Columns)
	// Always include the FK column for grouping
	colsWithFK := ensureColumnIncluded(cols, relDef.ForeignColumn)

	// Also include FK columns needed for any nested relations
	nestedFKCols := make(map[string]bool)
	for _, relCol := range rel.Columns {
		if relCol.Relation != nil {
			nestedRelDef, err := rqe.relCache.FindRelationship(relDef.ForeignTable, relCol.Relation.Name)
			if err == nil && nestedRelDef != nil {
				nestedFKCols[nestedRelDef.LocalColumn] = true
				colsWithFK = ensureColumnIncluded(colsWithFK, nestedRelDef.LocalColumn)
			}
		}
	}

	// Build and execute query for child table
	placeholders := make([]string, len(pkValues))
	for i := range pkValues {
		placeholders[i] = "?"
	}

	quotedCols := make([]string, len(colsWithFK))
	for i, c := range colsWithFK {
		if c == "*" {
			quotedCols[i] = "*"
		} else {
			quotedCols[i] = fmt.Sprintf("\"%s\"", c)
		}
	}

	sqlStr := fmt.Sprintf(`SELECT %s FROM "%s" WHERE "%s" IN (%s)`,
		strings.Join(quotedCols, ", "),
		relDef.ForeignTable,
		relDef.ForeignColumn,
		strings.Join(placeholders, ", "))

	rows, err := rqe.db.Query(sqlStr, pkValues...)
	if err != nil {
		return fmt.Errorf("relation query failed: %w", err)
	}

	relResults, err := scanRowsGeneric(rows)
	rows.Close()
	if err != nil {
		return fmt.Errorf("failed to scan relation results: %w", err)
	}

	// Handle nested relations in the related results
	for _, relCol := range rel.Columns {
		if relCol.Relation != nil {
			if err := rqe.embedRelation(relResults, relDef.ForeignTable, relCol.Relation); err != nil {
				return fmt.Errorf("failed to embed nested relation %s: %w", relCol.Relation.Name, err)
			}
		}
	}

	// Group related results by foreign key
	relIndex := make(map[any][]map[string]any)
	for _, relRow := range relResults {
		key := relRow[relDef.ForeignColumn]
		// Remove the FK column from the result if it wasn't explicitly requested
		if !containsColumn(cols, relDef.ForeignColumn) && !containsColumn(cols, "*") {
			delete(relRow, relDef.ForeignColumn)
		}
		// Also remove nested FK columns that weren't explicitly requested
		for nestedCol := range nestedFKCols {
			if !containsColumn(cols, nestedCol) && !containsColumn(cols, "*") {
				delete(relRow, nestedCol)
			}
		}
		relIndex[key] = append(relIndex[key], relRow)
	}

	// Embed into results
	for _, row := range results {
		pk := row[relDef.LocalColumn]
		if relData, ok := relIndex[pk]; ok {
			row[embedName] = relData
		}
		// If no matches, the empty array was already set
	}

	return nil
}

// scanRowsGeneric scans database rows into a slice of maps.
func scanRowsGeneric(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, nil
}

// extractColumnNames extracts column names from SelectColumns, excluding relations.
func extractColumnNames(cols []SelectColumn) []string {
	var names []string
	for _, col := range cols {
		if col.Relation == nil {
			names = append(names, col.Name)
		}
	}
	return names
}

// ensureColumnIncluded ensures a column is in the list, adding it if not present.
func ensureColumnIncluded(cols []string, col string) []string {
	// If we have "*", it includes everything
	for _, c := range cols {
		if c == "*" {
			return cols
		}
	}

	// Check if column already present
	for _, c := range cols {
		if c == col {
			return cols
		}
	}

	// Add the column
	return append(cols, col)
}

// containsColumn checks if a column name is in the list.
func containsColumn(cols []string, col string) bool {
	for _, c := range cols {
		if c == col {
			return true
		}
	}
	return false
}
