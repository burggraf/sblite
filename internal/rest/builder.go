// internal/rest/builder.go
package rest

import (
	"fmt"
	"sort"
	"strings"
)

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
			quotedCols[i] = fmt.Sprintf("\"%s\"", strings.TrimSpace(col))
		}
		sb.WriteString(strings.Join(quotedCols, ", "))
	}

	// FROM clause
	sb.WriteString(fmt.Sprintf(" FROM \"%s\"", q.Table))

	// WHERE clause
	if len(q.Filters) > 0 {
		sb.WriteString(" WHERE ")
		var conditions []string
		for _, f := range q.Filters {
			sql, filterArgs := f.ToSQL()
			conditions = append(conditions, sql)
			args = append(args, filterArgs...)
		}
		sb.WriteString(strings.Join(conditions, " AND "))
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
			orders = append(orders, fmt.Sprintf("\"%s\" %s", o.Column, dir))
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
		quotedCols[i] = fmt.Sprintf("\"%s\"", k)
		placeholders[i] = "?"
		args[i] = data[k]
	}

	sql := fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s)`,
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)

	return sql, args
}

func BuildUpdateQuery(table string, data map[string]any, filters []Filter) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var args []any
	setClauses := make([]string, len(keys))
	for i, k := range keys {
		setClauses[i] = fmt.Sprintf("\"%s\" = ?", k)
		args = append(args, data[k])
	}

	sql := fmt.Sprintf(`UPDATE "%s" SET %s`, table, strings.Join(setClauses, ", "))

	if len(filters) > 0 {
		var conditions []string
		for _, f := range filters {
			condSQL, filterArgs := f.ToSQL()
			conditions = append(conditions, condSQL)
			args = append(args, filterArgs...)
		}
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	return sql, args
}

func BuildDeleteQuery(table string, filters []Filter) (string, []any) {
	var args []any
	sql := fmt.Sprintf(`DELETE FROM "%s"`, table)

	if len(filters) > 0 {
		var conditions []string
		for _, f := range filters {
			condSQL, filterArgs := f.ToSQL()
			conditions = append(conditions, condSQL)
			args = append(args, filterArgs...)
		}
		sql += " WHERE " + strings.Join(conditions, " AND ")
	}

	return sql, args
}

// BuildUpsertQuery builds an INSERT ... ON CONFLICT DO UPDATE query
func BuildUpsertQuery(table string, data map[string]any) (string, []any) {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	quotedCols := make([]string, len(keys))
	placeholders := make([]string, len(keys))
	updateClauses := make([]string, 0, len(keys))
	args := make([]any, len(keys))

	for i, k := range keys {
		quotedCols[i] = fmt.Sprintf("\"%s\"", k)
		placeholders[i] = "?"
		args[i] = data[k]
		// Don't update the id column
		if k != "id" {
			updateClauses = append(updateClauses, fmt.Sprintf("\"%s\" = excluded.\"%s\"", k, k))
		}
	}

	// Default to id as conflict column
	sql := fmt.Sprintf(
		`INSERT INTO "%s" (%s) VALUES (%s) ON CONFLICT ("id") DO UPDATE SET %s`,
		table,
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
		strings.Join(updateClauses, ", "),
	)

	return sql, args
}
