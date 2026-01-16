// internal/rest/builder_test.go
package rest

import (
	"testing"
)

func TestBuildSelectQuery(t *testing.T) {
	query := Query{
		Table:   "todos",
		Select:  []string{"id", "title", "completed"},
		Filters: []Filter{{Column: "completed", Operator: "eq", Value: "false"}},
		Order:   []OrderBy{{Column: "created_at", Desc: true}},
		Limit:   10,
		Offset:  0,
	}

	sql, args := BuildSelectQuery(query)

	expectedSQL := `SELECT "id", "title", "completed" FROM "todos" WHERE "completed" = ? ORDER BY "created_at" DESC LIMIT 10 OFFSET 0`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "false" {
		t.Errorf("expected args [false], got %v", args)
	}

	// Test without limit - should not have OFFSET
	queryNoLimit := Query{
		Table:  "todos",
		Select: []string{"*"},
	}
	sqlNoLimit, _ := BuildSelectQuery(queryNoLimit)
	expectedNoLimit := `SELECT * FROM "todos"`
	if sqlNoLimit != expectedNoLimit {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedNoLimit, sqlNoLimit)
	}
}

func TestBuildSelectQueryWithoutLimit(t *testing.T) {
	query := Query{
		Table:   "todos",
		Select:  []string{"id", "title", "completed"},
		Filters: []Filter{{Column: "completed", Operator: "eq", Value: "false"}},
		Order:   []OrderBy{{Column: "created_at", Desc: true}},
	}

	sql, args := BuildSelectQuery(query)

	expectedSQL := `SELECT "id", "title", "completed" FROM "todos" WHERE "completed" = ? ORDER BY "created_at" DESC`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "false" {
		t.Errorf("expected args [false], got %v", args)
	}
}

func TestBuildInsertQuery(t *testing.T) {
	data := map[string]any{
		"title":     "Test Todo",
		"completed": false,
	}

	sql, args := BuildInsertQuery("todos", data)

	// Note: map iteration order is not guaranteed, so we check both possibilities
	if sql != `INSERT INTO "todos" ("completed", "title") VALUES (?, ?)` &&
		sql != `INSERT INTO "todos" ("title", "completed") VALUES (?, ?)` {
		t.Errorf("unexpected SQL: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildUpdateQuery(t *testing.T) {
	data := map[string]any{
		"completed": true,
	}
	query := Query{
		Table:   "todos",
		Filters: []Filter{{Column: "id", Operator: "eq", Value: "1"}},
	}

	sql, args := BuildUpdateQuery(query, data)

	expectedSQL := `UPDATE "todos" SET "completed" = ? WHERE "id" = ?`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildUpdateQueryWithRLS(t *testing.T) {
	data := map[string]any{
		"completed": true,
	}
	query := Query{
		Table:        "todos",
		Filters:      []Filter{{Column: "id", Operator: "eq", Value: "1"}},
		RLSCondition: `"user_id" = 'user-123'`,
	}

	sql, args := BuildUpdateQuery(query, data)

	expectedSQL := `UPDATE "todos" SET "completed" = ? WHERE "id" = ? AND "user_id" = 'user-123'`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildUpdateQueryWithRLSNoFilters(t *testing.T) {
	data := map[string]any{
		"completed": true,
	}
	query := Query{
		Table:        "todos",
		RLSCondition: `"user_id" = 'user-123'`,
	}

	sql, args := BuildUpdateQuery(query, data)

	expectedSQL := `UPDATE "todos" SET "completed" = ? WHERE "user_id" = 'user-123'`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 args, got %d", len(args))
	}
}

func TestBuildDeleteQuery(t *testing.T) {
	query := Query{
		Table:   "todos",
		Filters: []Filter{{Column: "id", Operator: "eq", Value: "1"}},
	}

	sql, args := BuildDeleteQuery(query)

	expectedSQL := `DELETE FROM "todos" WHERE "id" = ?`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "1" {
		t.Errorf("expected args [1], got %v", args)
	}
}

func TestBuildDeleteQueryWithRLS(t *testing.T) {
	query := Query{
		Table:        "todos",
		Filters:      []Filter{{Column: "id", Operator: "eq", Value: "1"}},
		RLSCondition: `"user_id" = 'user-123'`,
	}

	sql, args := BuildDeleteQuery(query)

	expectedSQL := `DELETE FROM "todos" WHERE "id" = ? AND "user_id" = 'user-123'`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "1" {
		t.Errorf("expected args [1], got %v", args)
	}
}

func TestBuildDeleteQueryWithRLSNoFilters(t *testing.T) {
	query := Query{
		Table:        "todos",
		RLSCondition: `"user_id" = 'user-123'`,
	}

	sql, args := BuildDeleteQuery(query)

	expectedSQL := `DELETE FROM "todos" WHERE "user_id" = 'user-123'`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestBuildCountQuery(t *testing.T) {
	query := Query{
		Table: "todos",
	}

	sql, args := BuildCountQuery(query)

	expectedSQL := `SELECT COUNT(*) FROM "todos"`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestBuildCountQueryWithFilter(t *testing.T) {
	query := Query{
		Table:   "todos",
		Filters: []Filter{{Column: "completed", Operator: "eq", Value: "true"}},
	}

	sql, args := BuildCountQuery(query)

	expectedSQL := `SELECT COUNT(*) FROM "todos" WHERE "completed" = ?`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "true" {
		t.Errorf("expected args [true], got %v", args)
	}
}

func TestBuildCountQueryWithRLS(t *testing.T) {
	query := Query{
		Table:        "todos",
		RLSCondition: `"user_id" = 'user-123'`,
	}

	sql, args := BuildCountQuery(query)

	expectedSQL := `SELECT COUNT(*) FROM "todos" WHERE "user_id" = 'user-123'`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestBuildCountQueryWithFilterAndRLS(t *testing.T) {
	query := Query{
		Table:        "todos",
		Filters:      []Filter{{Column: "completed", Operator: "eq", Value: "true"}},
		RLSCondition: `"user_id" = 'user-123'`,
	}

	sql, args := BuildCountQuery(query)

	expectedSQL := `SELECT COUNT(*) FROM "todos" WHERE "completed" = ? AND "user_id" = 'user-123'`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 1 || args[0] != "true" {
		t.Errorf("expected args [true], got %v", args)
	}
}

func TestBuildCountQueryWithLogicalFilter(t *testing.T) {
	query := Query{
		Table: "todos",
		LogicalFilters: []LogicalFilter{
			{
				Operator: "or",
				Filters: []Filter{
					{Column: "status", Operator: "eq", Value: "active"},
					{Column: "status", Operator: "eq", Value: "pending"},
				},
			},
		},
	}

	sql, args := BuildCountQuery(query)

	expectedSQL := `SELECT COUNT(*) FROM "todos" WHERE ("status" = ? OR "status" = ?)`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildUpsertQueryDefaultConflict(t *testing.T) {
	data := map[string]any{
		"id":    1,
		"name":  "Test",
		"email": "test@example.com",
	}

	sql, args := BuildUpsertQuery("users", data, nil, false)

	// Default conflict column is "id"
	expectedSQL := `INSERT INTO "users" ("email", "id", "name") VALUES (?, ?, ?) ON CONFLICT ("id") DO UPDATE SET "email" = excluded."email", "name" = excluded."name"`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuildUpsertQueryCustomConflictColumn(t *testing.T) {
	data := map[string]any{
		"email": "test@example.com",
		"name":  "Test",
	}

	sql, args := BuildUpsertQuery("users", data, []string{"email"}, false)

	// Custom conflict column is "email"
	expectedSQL := `INSERT INTO "users" ("email", "name") VALUES (?, ?) ON CONFLICT ("email") DO UPDATE SET "name" = excluded."name"`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildUpsertQueryMultipleConflictColumns(t *testing.T) {
	data := map[string]any{
		"user_id": 1,
		"date":    "2024-01-15",
		"value":   100,
	}

	sql, args := BuildUpsertQuery("metrics", data, []string{"user_id", "date"}, false)

	// Multiple conflict columns
	expectedSQL := `INSERT INTO "metrics" ("date", "user_id", "value") VALUES (?, ?, ?) ON CONFLICT ("user_id", "date") DO UPDATE SET "value" = excluded."value"`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuildUpsertQueryIgnoreDuplicates(t *testing.T) {
	data := map[string]any{
		"id":    1,
		"name":  "Test",
		"email": "test@example.com",
	}

	sql, args := BuildUpsertQuery("users", data, nil, true)

	// Should use DO NOTHING instead of DO UPDATE
	expectedSQL := `INSERT INTO "users" ("email", "id", "name") VALUES (?, ?, ?) ON CONFLICT ("id") DO NOTHING`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestBuildUpsertQueryIgnoreDuplicatesWithCustomConflict(t *testing.T) {
	data := map[string]any{
		"email": "test@example.com",
		"name":  "Test",
	}

	sql, args := BuildUpsertQuery("users", data, []string{"email"}, true)

	// Should use DO NOTHING with custom conflict column
	expectedSQL := `INSERT INTO "users" ("email", "name") VALUES (?, ?) ON CONFLICT ("email") DO NOTHING`
	if sql != expectedSQL {
		t.Errorf("expected SQL:\n%s\ngot:\n%s", expectedSQL, sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}
