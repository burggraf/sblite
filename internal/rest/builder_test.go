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
