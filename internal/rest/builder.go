// internal/rest/builder.go
package rest

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// SQL reserved words that require quoting when used as identifiers
var sqlReservedWords = map[string]bool{
	"select": true, "from": true, "where": true, "and": true, "or": true,
	"not": true, "null": true, "true": true, "false": true, "is": true,
	"in": true, "like": true, "between": true, "exists": true, "case": true,
	"when": true, "then": true, "else": true, "end": true, "as": true,
	"join": true, "left": true, "right": true, "inner": true, "outer": true,
	"on": true, "using": true, "group": true, "by": true, "having": true,
	"order": true, "asc": true, "desc": true, "limit": true, "offset": true,
	"union": true, "intersect": true, "except": true, "all": true, "distinct": true,
	"insert": true, "into": true, "values": true, "update": true, "set": true,
	"delete": true, "create": true, "table": true, "index": true, "drop": true,
	"alter": true, "add": true, "column": true, "primary": true, "key": true,
	"foreign": true, "references": true, "default": true, "check": true,
	"unique": true, "constraint": true, "cascade": true, "restrict": true,
}

// needsQuoting returns true if the identifier contains characters that require quoting.
func needsQuoting(name string) bool {
	if len(name) == 0 {
		return true
	}
	// Check if it's a reserved word
	if sqlReservedWords[strings.ToLower(name)] {
		return true
	}
	for i, r := range name {
		// First character can't be a digit
		if i == 0 && unicode.IsDigit(r) {
			return true
		}
		// Only allow letters, digits, and underscores without quoting
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return true
		}
	}
	return false
}

// quoteIdentifier safely quotes a SQL identifier (table or column name).
// It escapes any embedded double quotes by doubling them.
func quoteIdentifier(name string) string {
	// Escape any existing double quotes by doubling them
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}

// quoteIdentifierIfNeeded only quotes the identifier if it contains special characters.
// This produces cleaner SQL for simple identifiers.
func quoteIdentifierIfNeeded(name string) string {
	if needsQuoting(name) {
		return quoteIdentifier(name)
	}
	return name
}

// buildSelectColumn converts a column specification to SQL, handling JSON paths.
// Supports:
// - Simple columns: "name" -> "name"
// - Aliased columns: "myAlias:name" -> "name" AS "myAlias"
// - JSON paths: "data->key" -> json_extract("data", '$.key') AS "key"
// - JSON text paths: "data->>key" -> json_extract("data", '$.key') AS "key"
// - Aliased JSON: "myAlias:data->key" -> json_extract("data", '$.key') AS "myAlias"
func buildSelectColumn(col string) string {
	// Check for alias first: "alias:column" or "alias:column->path"
	var alias string
	remaining := col
	if colonIdx := strings.Index(col, ":"); colonIdx != -1 {
		// Ensure colon comes before any JSON operator
		arrowIdx := strings.Index(col, "->")
		if arrowIdx == -1 || colonIdx < arrowIdx {
			alias = col[:colonIdx]
			remaining = col[colonIdx+1:]
		}
	}

	// Check for JSON path: "column->key" or "column->>key"
	jsonArrowIdx := strings.Index(remaining, "->")
	if jsonArrowIdx != -1 {
		baseCol := remaining[:jsonArrowIdx]
		pathPart := remaining[jsonArrowIdx:]

		// Parse the path
		var pathParts []string
		for len(pathPart) > 0 {
			if strings.HasPrefix(pathPart, "->>") {
				pathPart = pathPart[3:]
			} else if strings.HasPrefix(pathPart, "->") {
				pathPart = pathPart[2:]
			} else {
				// Find next arrow or end
				nextArrow := strings.Index(pathPart, "->")
				if nextArrow == -1 {
					pathParts = append(pathParts, pathPart)
					pathPart = ""
				} else {
					pathParts = append(pathParts, pathPart[:nextArrow])
					pathPart = pathPart[nextArrow:]
				}
			}
		}

		if len(pathParts) > 0 {
			jsonPath := "$." + strings.Join(pathParts, ".")
			// Default alias to the last key
			if alias == "" {
				alias = pathParts[len(pathParts)-1]
			}
			return fmt.Sprintf("json_extract(%s, '%s') AS %s", quoteIdentifier(baseCol), jsonPath, quoteIdentifier(alias))
		}
	}

	// Simple column or aliased column
	if alias != "" {
		return fmt.Sprintf("%s AS %s", quoteIdentifier(remaining), quoteIdentifier(alias))
	}
	return quoteIdentifier(col)
}

// ToSQL converts a LogicalFilter to SQL with grouped conditions
func (lf LogicalFilter) ToSQL() (string, []any) {
	if len(lf.Filters) == 0 {
		return "", nil
	}

	var conditions []string
	var args []any

	for _, f := range lf.Filters {
		sql, filterArgs := f.ToSQL()
		conditions = append(conditions, sql)
		args = append(args, filterArgs...)
	}

	joiner := " AND "
	if lf.Operator == "or" {
		joiner = " OR "
	}

	return "(" + strings.Join(conditions, joiner) + ")", args
}

func BuildSelectQuery(q Query) (string, []any) {
	var args []any
	var sb strings.Builder

	// SELECT clause
	sb.WriteString("SELECT ")
	if len(q.Select) == 0 || (len(q.Select) == 1 && q.Select[0] == "*") {
		sb.WriteString("*")
	} else {
		quotedCols := make([]string, len(q.Select))
		for i, col := range q.Select {
			quotedCols[i] = buildSelectColumn(strings.TrimSpace(col))
		}
		sb.WriteString(strings.Join(quotedCols, ", "))
	}

	// FROM clause
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdentifier(q.Table))

	// WHERE clause (regular filters and logical filters)
	// Note: Related filters (f.ToSQL() returns empty string) are skipped here
	var conditions []string
	for _, f := range q.Filters {
		sql, filterArgs := f.ToSQL()
		if sql != "" {
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}
	}

	// Logical filters (or/and groups)
	for _, lf := range q.LogicalFilters {
		sql, filterArgs := lf.ToSQL()
		if sql != "" {
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}
	}

	hasConditions := len(conditions) > 0
	if hasConditions {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// RLS condition (added after filters)
	if q.RLSCondition != "" {
		if hasConditions {
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		sb.WriteString(q.RLSCondition)
	}

	// ORDER BY clause
	if len(q.Order) > 0 {
		sb.WriteString(" ORDER BY ")
		var orders []string
		for _, o := range q.Order {
			dir := "ASC"
			if o.Desc {
				dir = "DESC"
			}
			orders = append(orders, fmt.Sprintf("%s %s", quoteIdentifier(o.Column), dir))
		}
		sb.WriteString(strings.Join(orders, ", "))
	}

	// LIMIT and OFFSET
	if q.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))
		sb.WriteString(fmt.Sprintf(" OFFSET %d", q.Offset))
	}

	return sb.String(), args
}

// BuildSelectQueryWithRelations builds a SELECT query that can include
// related table filters (via EXISTS subquery) and ordering (via LEFT JOIN).
// The relCache is used to look up foreign key relationships between tables.
func BuildSelectQueryWithRelations(q Query, relCache *RelationshipCache) (string, []any, error) {
	var args []any
	var sb strings.Builder

	// Collect related order columns that need JOINs
	var joinedTables []struct {
		alias        string
		table        string
		localCol     string
		foreignCol   string
		orderColumn  string
		orderDesc    bool
	}

	// Check if any ORDER BY needs a related table
	joinIdx := 0
	for _, o := range q.Order {
		if o.RelatedTable != "" {
			rel, err := relCache.FindRelationship(q.Table, o.RelatedTable)
			if err != nil {
				return "", nil, fmt.Errorf("failed to find relationship for %s: %w", o.RelatedTable, err)
			}
			if rel == nil {
				// Relationship not found - skip this order
				continue
			}
			alias := fmt.Sprintf("_rel%d", joinIdx)
			joinIdx++
			joinedTables = append(joinedTables, struct {
				alias        string
				table        string
				localCol     string
				foreignCol   string
				orderColumn  string
				orderDesc    bool
			}{
				alias:        alias,
				table:        rel.ForeignTable,
				localCol:     rel.LocalColumn,
				foreignCol:   rel.ForeignColumn,
				orderColumn:  o.Column,
				orderDesc:    o.Desc,
			})
		}
	}

	// SELECT clause - qualify columns with main table name if we have JOINs
	mainTable := q.Table
	sb.WriteString("SELECT ")
	if len(joinedTables) > 0 {
		// When joining, select main table columns explicitly
		if len(q.Select) == 0 || (len(q.Select) == 1 && q.Select[0] == "*") {
			sb.WriteString(quoteIdentifier(mainTable))
			sb.WriteString(".*")
		} else {
			quotedCols := make([]string, len(q.Select))
			for i, col := range q.Select {
				quotedCols[i] = quoteIdentifier(mainTable) + "." + quoteIdentifier(strings.TrimSpace(col))
			}
			sb.WriteString(strings.Join(quotedCols, ", "))
		}
	} else {
		if len(q.Select) == 0 || (len(q.Select) == 1 && q.Select[0] == "*") {
			sb.WriteString("*")
		} else {
			quotedCols := make([]string, len(q.Select))
			for i, col := range q.Select {
				quotedCols[i] = quoteIdentifier(strings.TrimSpace(col))
			}
			sb.WriteString(strings.Join(quotedCols, ", "))
		}
	}

	// FROM clause
	sb.WriteString(" FROM ")
	sb.WriteString(quoteIdentifier(mainTable))

	// Add LEFT JOINs for related table ordering
	for _, jt := range joinedTables {
		sb.WriteString(fmt.Sprintf(" LEFT JOIN %s AS %s ON %s.%s = %s.%s",
			quoteIdentifier(jt.table), quoteIdentifier(jt.alias),
			quoteIdentifier(mainTable), quoteIdentifier(jt.localCol),
			quoteIdentifier(jt.alias), quoteIdentifier(jt.foreignCol)))
	}

	// Build WHERE clause
	var conditions []string

	// Regular filters (non-related)
	for _, f := range q.Filters {
		if f.IsRelatedFilter() {
			continue // Handle related filters separately
		}
		sql, filterArgs := f.ToSQL()
		if sql != "" {
			// Qualify column with main table if we have JOINs
			if len(joinedTables) > 0 {
				// Prefix the column with table name: "column" -> "table"."column"
				sql = qualifyColumnWithTable(sql, mainTable)
			}
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}
	}

	// Related filters via EXISTS subquery
	for _, f := range q.Filters {
		if !f.IsRelatedFilter() {
			continue
		}
		rel, err := relCache.FindRelationship(q.Table, f.RelatedTable)
		if err != nil {
			return "", nil, fmt.Errorf("failed to find relationship for %s: %w", f.RelatedTable, err)
		}
		if rel == nil {
			// Relationship not found - return error
			return "", nil, fmt.Errorf("no relationship found between %s and %s", q.Table, f.RelatedTable)
		}

		// Build EXISTS subquery based on relationship type
		existsSQL, existsArgs := buildRelatedFilterExists(mainTable, f, rel)
		if existsSQL != "" {
			conditions = append(conditions, existsSQL)
			args = append(args, existsArgs...)
		}
	}

	// Logical filters
	for _, lf := range q.LogicalFilters {
		sql, filterArgs := lf.ToSQL()
		if sql != "" {
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}
	}

	if len(conditions) > 0 {
		sb.WriteString(" WHERE ")
		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// RLS condition
	if q.RLSCondition != "" {
		if len(conditions) > 0 {
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		sb.WriteString(q.RLSCondition)
	}

	// ORDER BY clause
	if len(q.Order) > 0 {
		sb.WriteString(" ORDER BY ")
		var orders []string
		joinedTableIdx := 0
		for _, o := range q.Order {
			dir := "ASC"
			if o.Desc {
				dir = "DESC"
			}
			if o.RelatedTable != "" {
				// Use the joined table alias
				if joinedTableIdx < len(joinedTables) {
					jt := joinedTables[joinedTableIdx]
					orders = append(orders, fmt.Sprintf("%s.%s %s", quoteIdentifier(jt.alias), quoteIdentifier(o.Column), dir))
					joinedTableIdx++
				}
			} else {
				if len(joinedTables) > 0 {
					orders = append(orders, fmt.Sprintf("%s.%s %s", quoteIdentifier(mainTable), quoteIdentifier(o.Column), dir))
				} else {
					orders = append(orders, fmt.Sprintf("%s %s", quoteIdentifier(o.Column), dir))
				}
			}
		}
		sb.WriteString(strings.Join(orders, ", "))
	}

	// LIMIT and OFFSET
	if q.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", q.Limit))
		sb.WriteString(fmt.Sprintf(" OFFSET %d", q.Offset))
	}

	return sb.String(), args, nil
}

// buildRelatedFilterExists creates an EXISTS subquery for filtering on a related table
func buildRelatedFilterExists(mainTable string, f Filter, rel *Relationship) (string, []any) {
	filterSQL, filterArgs := f.ToRelatedSQL()
	if filterSQL == "" {
		return "", nil
	}

	var existsSQL string
	if rel.Type == "many-to-one" {
		// Main table has FK to related table
		// EXISTS (SELECT 1 FROM related WHERE related.id = main.fk_col AND related.column = ?)
		existsSQL = fmt.Sprintf(
			`EXISTS (SELECT 1 FROM %s WHERE %s.%s = %s.%s AND %s)`,
			quoteIdentifier(rel.ForeignTable),
			quoteIdentifier(rel.ForeignTable), quoteIdentifier(rel.ForeignColumn),
			quoteIdentifier(mainTable), quoteIdentifier(rel.LocalColumn),
			filterSQL,
		)
	} else {
		// one-to-many: Related table has FK to main table
		// EXISTS (SELECT 1 FROM related WHERE related.fk_col = main.id AND related.column = ?)
		existsSQL = fmt.Sprintf(
			`EXISTS (SELECT 1 FROM %s WHERE %s.%s = %s.%s AND %s)`,
			quoteIdentifier(rel.ForeignTable),
			quoteIdentifier(rel.ForeignTable), quoteIdentifier(rel.ForeignColumn),
			quoteIdentifier(mainTable), quoteIdentifier(rel.LocalColumn),
			filterSQL,
		)
	}

	return existsSQL, filterArgs
}

// HasRelatedFilters checks if a query has any filters on related tables
func (q Query) HasRelatedFilters() bool {
	for _, f := range q.Filters {
		if f.IsRelatedFilter() {
			return true
		}
	}
	return false
}

// HasRelatedOrdering checks if a query has any ordering on related tables
func (q Query) HasRelatedOrdering() bool {
	for _, o := range q.Order {
		if o.RelatedTable != "" {
			return true
		}
	}
	return false
}

// qualifyColumnWithTable prefixes the first quoted column in SQL with a table name.
// Transforms: "column" > ? -> "table"."column" > ?
// Also handles LOWER("column") and other function wrappings
func qualifyColumnWithTable(sql, table string) string {
	// Find the first quoted identifier and prefix it with table
	// Pattern: "column" at the start or LOWER("column") etc.
	if strings.HasPrefix(sql, "\"") {
		// Simple case: starts with quoted column
		return quoteIdentifier(table) + "." + sql
	}
	// Case: LOWER("column") or similar function
	if idx := strings.Index(sql, "(\""); idx != -1 {
		// Find the closing quote
		closeIdx := strings.Index(sql[idx+2:], "\"")
		if closeIdx != -1 {
			// Extract parts and rebuild with table prefix
			prefix := sql[:idx+1]                   // "LOWER("
			column := sql[idx+1 : idx+2+closeIdx+1] // "\"column\""
			suffix := sql[idx+2+closeIdx+1:]        // ") LIKE LOWER(?)"
			return prefix + quoteIdentifier(table) + "." + column + suffix
		}
	}
	// Fallback: return as-is
	return sql
}

func BuildInsertQuery(table string, data map[string]any) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	quotedCols := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))

	for i, k := range keys {
		quotedCols[i] = quoteIdentifier(k)
		placeholders[i] = "?"
		args[i] = data[k]
	}

	sql := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s)`,
		quoteIdentifier(table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)

	return sql, args
}

func BuildUpdateQuery(q Query, data map[string]any) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var args []any
	setClauses := make([]string, len(keys))
	for i, k := range keys {
		setClauses[i] = fmt.Sprintf("%s = ?", quoteIdentifier(k))
		args = append(args, data[k])
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`UPDATE %s SET %s`, quoteIdentifier(q.Table), strings.Join(setClauses, ", ")))

	// WHERE clause from filters (regular and logical)
	hasConditions := len(q.Filters) > 0 || len(q.LogicalFilters) > 0
	if hasConditions {
		sb.WriteString(" WHERE ")
		var conditions []string

		// Regular filters
		for _, f := range q.Filters {
			condSQL, filterArgs := f.ToSQL()
			conditions = append(conditions, condSQL)
			args = append(args, filterArgs...)
		}

		// Logical filters (or/and groups)
		for _, lf := range q.LogicalFilters {
			sql, filterArgs := lf.ToSQL()
			if sql != "" {
				conditions = append(conditions, sql)
				args = append(args, filterArgs...)
			}
		}

		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// RLS condition (added after filters)
	if q.RLSCondition != "" {
		if hasConditions {
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		sb.WriteString(q.RLSCondition)
	}

	return sb.String(), args
}

func BuildDeleteQuery(q Query) (string, []any) {
	var args []any
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`DELETE FROM %s`, quoteIdentifier(q.Table)))

	// WHERE clause from filters (regular and logical)
	hasConditions := len(q.Filters) > 0 || len(q.LogicalFilters) > 0
	if hasConditions {
		sb.WriteString(" WHERE ")
		var conditions []string

		// Regular filters
		for _, f := range q.Filters {
			condSQL, filterArgs := f.ToSQL()
			conditions = append(conditions, condSQL)
			args = append(args, filterArgs...)
		}

		// Logical filters (or/and groups)
		for _, lf := range q.LogicalFilters {
			sql, filterArgs := lf.ToSQL()
			if sql != "" {
				conditions = append(conditions, sql)
				args = append(args, filterArgs...)
			}
		}

		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// RLS condition (added after filters)
	if q.RLSCondition != "" {
		if hasConditions {
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		sb.WriteString(q.RLSCondition)
	}

	return sb.String(), args
}

// BuildCountQuery builds a COUNT(*) query with filters and RLS conditions
func BuildCountQuery(q Query) (string, []any) {
	var args []any
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`SELECT COUNT(*) FROM %s`, quoteIdentifier(q.Table)))

	// WHERE clause (regular filters and logical filters)
	hasConditions := len(q.Filters) > 0 || len(q.LogicalFilters) > 0
	if hasConditions {
		sb.WriteString(" WHERE ")
		var conditions []string

		// Regular filters
		for _, f := range q.Filters {
			sql, filterArgs := f.ToSQL()
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}

		// Logical filters (or/and groups)
		for _, lf := range q.LogicalFilters {
			sql, filterArgs := lf.ToSQL()
			if sql != "" {
				conditions = append(conditions, sql)
				args = append(args, filterArgs...)
			}
		}

		sb.WriteString(strings.Join(conditions, " AND "))
	}

	// RLS condition (added after filters)
	if q.RLSCondition != "" {
		if hasConditions {
			sb.WriteString(" AND ")
		} else {
			sb.WriteString(" WHERE ")
		}
		sb.WriteString(q.RLSCondition)
	}

	return sb.String(), args
}

// BuildUpsertQuery builds an INSERT ... ON CONFLICT query
// onConflict specifies which columns to use for conflict detection (defaults to ["id"])
// ignoreDuplicates: if true, uses DO NOTHING; if false, uses DO UPDATE SET
func BuildUpsertQuery(table string, data map[string]any, onConflict []string, ignoreDuplicates bool) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	quotedCols := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	args := make([]any, len(keys))

	for i, k := range keys {
		quotedCols[i] = quoteIdentifier(k)
		placeholders[i] = "?"
		args[i] = data[k]
	}

	// Default conflict columns to "id" if not specified
	conflictCols := onConflict
	if len(conflictCols) == 0 {
		conflictCols = []string{"id"}
	}

	// Quote conflict columns
	quotedConflictCols := make([]string, len(conflictCols))
	for i, col := range conflictCols {
		quotedConflictCols[i] = quoteIdentifier(col)
	}

	// Build conflict column set for excluding from update
	conflictColSet := make(map[string]bool)
	for _, col := range conflictCols {
		conflictColSet[col] = true
	}

	if ignoreDuplicates {
		// DO NOTHING - skip rows that conflict
		sql := fmt.Sprintf(
			`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO NOTHING`,
			quoteIdentifier(table),
			strings.Join(quotedCols, ", "),
			strings.Join(placeholders, ", "),
			strings.Join(quotedConflictCols, ", "),
		)
		return sql, args
	}

	// DO UPDATE SET - build update clauses excluding conflict columns
	updateClauses := make([]string, 0, len(keys))
	for _, k := range keys {
		// Don't update conflict columns (they're used for matching)
		if !conflictColSet[k] {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = excluded.%s", quoteIdentifier(k), quoteIdentifier(k)))
		}
	}

	// If all columns are conflict columns, we need at least one update clause
	// In this case, we'll do a no-op update (set first non-conflict column to itself)
	if len(updateClauses) == 0 {
		// Use a dummy update that doesn't change anything
		// This handles the edge case where all columns are conflict columns
		if len(keys) > 0 {
			updateClauses = append(updateClauses, fmt.Sprintf("%s = excluded.%s", quoteIdentifier(keys[0]), quoteIdentifier(keys[0])))
		}
	}

	sql := fmt.Sprintf(
		`INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s`,
		quoteIdentifier(table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(quotedConflictCols, ", "),
		strings.Join(updateClauses, ", "),
	)

	return sql, args
}
