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
			relDef, err := rqe.findRelationByNameOrColumn(q.Table, col.Relation.Name)
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
	// Process inner joins which may filter results
	for _, col := range parsed.Columns {
		if col.Relation != nil {
			// Get relation-specific modifiers if any
			var mods *RelationModifiers
			if q.RelationModifiers != nil {
				if m, ok := q.RelationModifiers[col.Relation.Name]; ok {
					mods = &m
				}
			}
			if err := rqe.embedRelation(&results, q.Table, col.Relation, mods); err != nil {
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
// The results pointer allows inner joins to filter out rows without matching relations.
// The mods parameter contains optional ORDER BY, LIMIT, OFFSET modifiers for the relation.
func (rqe *RelationQueryExecutor) embedRelation(results *[]map[string]any, table string, rel *SelectRelation, mods *RelationModifiers) error {
	if len(*results) == 0 {
		return nil
	}

	// Look up the relationship definition (supports both table name and FK column name)
	relDef, err := rqe.findRelationByNameOrColumn(table, rel.Name)
	if err != nil {
		return fmt.Errorf("failed to find relationship: %w", err)
	}

	if relDef == nil {
		// No relationship found - set all results to null for this relation
		embedName := rel.Alias
		if embedName == "" {
			embedName = rel.Name
		}
		for _, row := range *results {
			row[embedName] = nil
		}
		return nil
	}

	switch relDef.Type {
	case "many-to-one":
		return rqe.embedManyToOne(results, relDef, rel, mods)
	case "one-to-many":
		return rqe.embedOneToMany(results, relDef, rel, mods)
	default:
		return fmt.Errorf("unknown relationship type: %s", relDef.Type)
	}
}

// embedManyToOne handles many-to-one relationships (e.g., city -> country).
// Each result row gets a single related object (or null).
// If rel.Inner is true, rows without matching relations are filtered out.
func (rqe *RelationQueryExecutor) embedManyToOne(results *[]map[string]any, relDef *Relationship, rel *SelectRelation, mods *RelationModifiers) error {
	// Collect all foreign key values from the results
	fkValues := make([]any, 0)
	fkSet := make(map[any]bool)

	for _, row := range *results {
		if fk, ok := row[relDef.LocalColumn]; ok && fk != nil {
			// Deduplicate FK values
			if !fkSet[fk] {
				fkValues = append(fkValues, fk)
				fkSet[fk] = true
			}
		}
	}

	embedName := rel.Alias
	if embedName == "" {
		embedName = rel.Name
	}

	if len(fkValues) == 0 {
		// No foreign keys to look up - set all to null
		for _, row := range *results {
			row[embedName] = nil
		}
		// If inner join, filter out all rows (none have matching relations)
		if rel.Inner {
			*results = (*results)[:0]
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
			nestedRelDef, err := rqe.findRelationByNameOrColumn(relDef.ForeignTable, relCol.Relation.Name)
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
			if err := rqe.embedRelation(&relResults, relDef.ForeignTable, relCol.Relation, nil); err != nil {
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
	for _, row := range *results {
		fk := row[relDef.LocalColumn]
		if relData, ok := relIndex[fk]; ok {
			row[embedName] = relData
		} else {
			row[embedName] = nil
		}
	}

	// If inner join, filter out rows without matching relation
	if rel.Inner {
		filtered := make([]map[string]any, 0, len(*results))
		for _, row := range *results {
			if row[embedName] != nil {
				filtered = append(filtered, row)
			}
		}
		*results = filtered
	}

	return nil
}

// embedOneToMany handles one-to-many relationships (e.g., country -> cities).
// Each result row gets an array of related objects (possibly empty).
// If rel.Inner is true, rows without matching relations are filtered out.
func (rqe *RelationQueryExecutor) embedOneToMany(results *[]map[string]any, relDef *Relationship, rel *SelectRelation, mods *RelationModifiers) error {
	// For one-to-many: LocalColumn is the column in our table being referenced (e.g., "id")
	// ForeignColumn is the FK column in the child table (e.g., "country_id")

	// Collect all primary key values from the results
	pkValues := make([]any, 0)
	pkSet := make(map[any]bool)

	for _, row := range *results {
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

	for _, row := range *results {
		row[embedName] = []map[string]any{}
	}

	if len(pkValues) == 0 {
		// If inner join, filter out all rows (none have matching relations)
		if rel.Inner {
			*results = (*results)[:0]
		}
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
			nestedRelDef, err := rqe.findRelationByNameOrColumn(relDef.ForeignTable, relCol.Relation.Name)
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

	// Collect all query arguments (starts with pkValues)
	args := make([]any, len(pkValues))
	copy(args, pkValues)

	// Apply relation-specific filters
	if mods != nil {
		// Apply regular filters
		for _, f := range mods.Filters {
			condition, filterArgs := f.ToSQL()
			sqlStr += " AND " + condition
			args = append(args, filterArgs...)
		}

		// Apply logical filters (OR/AND)
		for _, lf := range mods.LogicalFilters {
			condition, lfArgs := lf.ToSQL()
			sqlStr += " AND (" + condition + ")"
			args = append(args, lfArgs...)
		}

		// Apply ORDER BY
		if len(mods.Order) > 0 {
			orderParts := make([]string, len(mods.Order))
			for i, o := range mods.Order {
				dir := "ASC"
				if o.Desc {
					dir = "DESC"
				}
				orderParts[i] = fmt.Sprintf("\"%s\" %s", o.Column, dir)
			}
			sqlStr += " ORDER BY " + strings.Join(orderParts, ", ")
		}
		// Note: LIMIT on relation applies per-parent-row, handled in grouping below
	}

	rows, err := rqe.db.Query(sqlStr, args...)
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
			if err := rqe.embedRelation(&relResults, relDef.ForeignTable, relCol.Relation, nil); err != nil {
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

	// Apply per-parent limit if specified
	limitPerParent := 0
	if mods != nil && mods.Limit > 0 {
		limitPerParent = mods.Limit
	}

	// Embed into results
	for _, row := range *results {
		pk := row[relDef.LocalColumn]
		if relData, ok := relIndex[pk]; ok {
			// Apply limit per parent if specified
			if limitPerParent > 0 && len(relData) > limitPerParent {
				relData = relData[:limitPerParent]
			}
			row[embedName] = relData
		}
		// If no matches, the empty array was already set
	}

	// If inner join, filter out rows without matching relations (empty arrays)
	if rel.Inner {
		filtered := make([]map[string]any, 0, len(*results))
		for _, row := range *results {
			if arr, ok := row[embedName].([]map[string]any); ok && len(arr) > 0 {
				filtered = append(filtered, row)
			}
		}
		*results = filtered
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

// findRelationByNameOrColumn looks up a relationship by either table name or FK column name.
// This supports multi-reference scenarios like "from:sender_id(name), to:receiver_id(name)"
// where sender_id and receiver_id are FK column names pointing to the same table.
//
// Lookup priority:
// 1. Match by FK column name (LocalColumn) - for multi-reference support
// 2. Match by relation name (table name) - traditional lookup
func (rqe *RelationQueryExecutor) findRelationByNameOrColumn(table, relationName string) (*Relationship, error) {
	rels, err := rqe.relCache.GetRelationships(table)
	if err != nil {
		return nil, err
	}

	// First try: match by FK column name (for multi-reference support)
	for _, r := range rels {
		if r.LocalColumn == relationName {
			return &r, nil
		}
	}

	// Second try: match by table/relation name (traditional lookup)
	for _, r := range rels {
		if r.Name == relationName || r.ForeignTable == relationName {
			return &r, nil
		}
	}

	return nil, nil
}
