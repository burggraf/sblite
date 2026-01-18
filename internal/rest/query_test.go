// internal/rest/query_test.go
package rest

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Filter
		wantErr  bool
	}{
		{
			name:     "eq operator",
			input:    "status=eq.active",
			expected: Filter{Column: "status", Operator: "eq", Value: "active"},
		},
		{
			name:     "neq operator",
			input:    "status=neq.deleted",
			expected: Filter{Column: "status", Operator: "neq", Value: "deleted"},
		},
		{
			name:     "gt operator",
			input:    "age=gt.21",
			expected: Filter{Column: "age", Operator: "gt", Value: "21"},
		},
		{
			name:     "gte operator",
			input:    "age=gte.21",
			expected: Filter{Column: "age", Operator: "gte", Value: "21"},
		},
		{
			name:     "lt operator",
			input:    "age=lt.65",
			expected: Filter{Column: "age", Operator: "lt", Value: "65"},
		},
		{
			name:     "lte operator",
			input:    "age=lte.65",
			expected: Filter{Column: "age", Operator: "lte", Value: "65"},
		},
		{
			name:     "is null",
			input:    "deleted=is.null",
			expected: Filter{Column: "deleted", Operator: "is", Value: "null"},
		},
		{
			name:    "invalid format",
			input:   "status",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(filter, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, filter)
			}
		})
	}
}

func TestFilterToSQL(t *testing.T) {
	tests := []struct {
		name     string
		filter   Filter
		expected string
		args     []any
	}{
		{
			name:     "eq operator",
			filter:   Filter{Column: "status", Operator: "eq", Value: "active"},
			expected: "\"status\" = ?",
			args:     []any{"active"},
		},
		{
			name:     "neq operator",
			filter:   Filter{Column: "status", Operator: "neq", Value: "deleted"},
			expected: "\"status\" != ?",
			args:     []any{"deleted"},
		},
		{
			name:     "is null",
			filter:   Filter{Column: "deleted", Operator: "is", Value: "null"},
			expected: "\"deleted\" IS NULL",
			args:     nil,
		},
		{
			name:     "is not null",
			filter:   Filter{Column: "deleted", Operator: "is", Value: "not.null"},
			expected: "\"deleted\" IS NOT NULL",
			args:     nil,
		},
		// NOT operator tests
		{
			name:     "not eq",
			filter:   Filter{Column: "status", Operator: "not", Value: "eq.deleted"},
			expected: "\"status\" != ?",
			args:     []any{"deleted"},
		},
		{
			name:     "not neq",
			filter:   Filter{Column: "status", Operator: "not", Value: "neq.active"},
			expected: "\"status\" = ?",
			args:     []any{"active"},
		},
		{
			name:     "not gt",
			filter:   Filter{Column: "age", Operator: "not", Value: "gt.21"},
			expected: "\"age\" <= ?",
			args:     []any{"21"},
		},
		{
			name:     "not gte",
			filter:   Filter{Column: "age", Operator: "not", Value: "gte.21"},
			expected: "\"age\" < ?",
			args:     []any{"21"},
		},
		{
			name:     "not lt",
			filter:   Filter{Column: "age", Operator: "not", Value: "lt.65"},
			expected: "\"age\" >= ?",
			args:     []any{"65"},
		},
		{
			name:     "not lte",
			filter:   Filter{Column: "age", Operator: "not", Value: "lte.65"},
			expected: "\"age\" > ?",
			args:     []any{"65"},
		},
		{
			name:     "not is null",
			filter:   Filter{Column: "deleted", Operator: "not", Value: "is.null"},
			expected: "\"deleted\" IS NOT NULL",
			args:     nil,
		},
		{
			name:     "not in",
			filter:   Filter{Column: "status", Operator: "not", Value: "in.(active,pending)"},
			expected: "\"status\" NOT IN (?, ?)",
			args:     []any{"active", "pending"},
		},
		{
			name:     "not like",
			filter:   Filter{Column: "name", Operator: "not", Value: "like.*john*"},
			expected: "\"name\" NOT LIKE ?",
			args:     []any{"%john%"},
		},
		{
			name:     "not ilike",
			filter:   Filter{Column: "name", Operator: "not", Value: "ilike.*john*"},
			expected: "LOWER(\"name\") NOT LIKE LOWER(?)",
			args:     []any{"%john%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.filter.ToSQL()
			if sql != tt.expected {
				t.Errorf("expected SQL %q, got %q", tt.expected, sql)
			}
			if !reflect.DeepEqual(args, tt.args) {
				t.Errorf("expected args %v, got %v", tt.args, args)
			}
		})
	}
}

func TestParseNotFilter(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Filter
		wantErr  bool
	}{
		{
			name:     "not eq",
			input:    "status=not.eq.deleted",
			expected: Filter{Column: "status", Operator: "not", Value: "eq.deleted"},
		},
		{
			name:     "not in",
			input:    "id=not.in.(1,2,3)",
			expected: Filter{Column: "id", Operator: "not", Value: "in.(1,2,3)"},
		},
		{
			name:     "not like",
			input:    "name=not.like.*test*",
			expected: Filter{Column: "name", Operator: "not", Value: "like.*test*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(filter, tt.expected) {
				t.Errorf("expected %+v, got %+v", tt.expected, filter)
			}
		})
	}
}

func TestParseLogicalFilter(t *testing.T) {
	tests := []struct {
		name        string
		operator    string
		value       string
		wantFilters int
		wantOp      string
		wantErr     bool
	}{
		{
			name:        "or with two filters",
			operator:    "or",
			value:       "(status.eq.active,status.eq.pending)",
			wantFilters: 2,
			wantOp:      "or",
		},
		{
			name:        "and with two filters",
			operator:    "and",
			value:       "(age.gt.18,age.lt.65)",
			wantFilters: 2,
			wantOp:      "and",
		},
		{
			name:        "or with three filters",
			operator:    "or",
			value:       "(status.eq.active,status.eq.pending,status.eq.draft)",
			wantFilters: 3,
			wantOp:      "or",
		},
		{
			name:        "empty value",
			operator:    "or",
			value:       "()",
			wantFilters: 0,
			wantOp:      "or",
		},
		{
			name:        "without parentheses",
			operator:    "or",
			value:       "status.eq.active,status.eq.pending",
			wantFilters: 2,
			wantOp:      "or",
		},
		{
			name:     "invalid operator",
			operator: "xor",
			value:    "(status.eq.active)",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lf, err := ParseLogicalFilter(tt.operator, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lf.Operator != tt.wantOp {
				t.Errorf("expected operator %q, got %q", tt.wantOp, lf.Operator)
			}
			if len(lf.Filters) != tt.wantFilters {
				t.Errorf("expected %d filters, got %d", tt.wantFilters, len(lf.Filters))
			}
		})
	}
}

func TestParseLogicalFilterContent(t *testing.T) {
	// Test specific filter content
	lf, err := ParseLogicalFilter("or", "(status.eq.active,status.eq.pending)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(lf.Filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(lf.Filters))
	}

	// First filter
	if lf.Filters[0].Column != "status" {
		t.Errorf("expected column 'status', got %q", lf.Filters[0].Column)
	}
	if lf.Filters[0].Operator != "eq" {
		t.Errorf("expected operator 'eq', got %q", lf.Filters[0].Operator)
	}
	if lf.Filters[0].Value != "active" {
		t.Errorf("expected value 'active', got %q", lf.Filters[0].Value)
	}

	// Second filter
	if lf.Filters[1].Column != "status" {
		t.Errorf("expected column 'status', got %q", lf.Filters[1].Column)
	}
	if lf.Filters[1].Operator != "eq" {
		t.Errorf("expected operator 'eq', got %q", lf.Filters[1].Operator)
	}
	if lf.Filters[1].Value != "pending" {
		t.Errorf("expected value 'pending', got %q", lf.Filters[1].Value)
	}
}

func TestLogicalFilterToSQL(t *testing.T) {
	tests := []struct {
		name         string
		lf           LogicalFilter
		expectedSQL  string
		expectedArgs []any
	}{
		{
			name: "or filter",
			lf: LogicalFilter{
				Operator: "or",
				Filters: []Filter{
					{Column: "status", Operator: "eq", Value: "active"},
					{Column: "status", Operator: "eq", Value: "pending"},
				},
			},
			expectedSQL:  "(\"status\" = ? OR \"status\" = ?)",
			expectedArgs: []any{"active", "pending"},
		},
		{
			name: "and filter",
			lf: LogicalFilter{
				Operator: "and",
				Filters: []Filter{
					{Column: "age", Operator: "gt", Value: "18"},
					{Column: "age", Operator: "lt", Value: "65"},
				},
			},
			expectedSQL:  "(\"age\" > ? AND \"age\" < ?)",
			expectedArgs: []any{"18", "65"},
		},
		{
			name: "empty filter",
			lf: LogicalFilter{
				Operator: "or",
				Filters:  []Filter{},
			},
			expectedSQL:  "",
			expectedArgs: nil,
		},
		{
			name: "single filter",
			lf: LogicalFilter{
				Operator: "or",
				Filters: []Filter{
					{Column: "status", Operator: "eq", Value: "active"},
				},
			},
			expectedSQL:  "(\"status\" = ?)",
			expectedArgs: []any{"active"},
		},
		{
			name: "three filters with or",
			lf: LogicalFilter{
				Operator: "or",
				Filters: []Filter{
					{Column: "status", Operator: "eq", Value: "active"},
					{Column: "status", Operator: "eq", Value: "pending"},
					{Column: "status", Operator: "eq", Value: "draft"},
				},
			},
			expectedSQL:  "(\"status\" = ? OR \"status\" = ? OR \"status\" = ?)",
			expectedArgs: []any{"active", "pending", "draft"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.lf.ToSQL()
			if sql != tt.expectedSQL {
				t.Errorf("expected SQL %q, got %q", tt.expectedSQL, sql)
			}
			if !reflect.DeepEqual(args, tt.expectedArgs) {
				t.Errorf("expected args %v, got %v", tt.expectedArgs, args)
			}
		})
	}
}

func TestSplitLogicalParts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple comma separation",
			input:    "a.eq.1,b.eq.2",
			expected: []string{"a.eq.1", "b.eq.2"},
		},
		{
			name:     "three parts",
			input:    "a.eq.1,b.eq.2,c.eq.3",
			expected: []string{"a.eq.1", "b.eq.2", "c.eq.3"},
		},
		{
			name:     "with in clause containing commas",
			input:    "status.in.(a,b,c),name.eq.test",
			expected: []string{"status.in.(a,b,c)", "name.eq.test"},
		},
		{
			name:     "single part",
			input:    "a.eq.1",
			expected: []string{"a.eq.1"},
		},
		{
			name:     "empty",
			input:    "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitLogicalParts(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestBuildSelectQueryWithLogicalFilters(t *testing.T) {
	// Test that logical filters are properly included in SELECT queries
	q := Query{
		Table:  "posts",
		Select: []string{"*"},
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

	sql, args := BuildSelectQuery(q)

	if !strings.Contains(sql, "WHERE") {
		t.Error("expected WHERE clause in SQL")
	}
	if !strings.Contains(sql, "OR") {
		t.Errorf("expected OR in SQL, got: %s", sql)
	}
	if len(args) != 2 {
		t.Errorf("expected 2 args, got %d", len(args))
	}
}

func TestBuildSelectQueryWithMixedFilters(t *testing.T) {
	// Test combining regular filters and logical filters
	q := Query{
		Table:  "posts",
		Select: []string{"*"},
		Filters: []Filter{
			{Column: "published", Operator: "eq", Value: "true"},
		},
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

	sql, args := BuildSelectQuery(q)

	// Should have both regular filter AND logical filter
	if !strings.Contains(sql, "AND") {
		t.Errorf("expected AND in SQL to join filters, got: %s", sql)
	}
	if !strings.Contains(sql, "OR") {
		t.Errorf("expected OR in SQL for logical filter, got: %s", sql)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestParseMatchFilter(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantFilters int
		wantErr     bool
	}{
		{
			name:        "two fields",
			input:       `{"status":"active","priority":"high"}`,
			wantFilters: 2,
		},
		{
			name:        "single field",
			input:       `{"status":"active"}`,
			wantFilters: 1,
		},
		{
			name:        "empty object",
			input:       `{}`,
			wantFilters: 0,
		},
		{
			name:        "numeric value",
			input:       `{"age":25}`,
			wantFilters: 1,
		},
		{
			name:        "boolean value",
			input:       `{"active":true}`,
			wantFilters: 1,
		},
		{
			name:        "multiple fields mixed types",
			input:       `{"status":"active","count":10,"enabled":true}`,
			wantFilters: 3,
		},
		{
			name:    "invalid json",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "not an object",
			input:   `["array"]`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filters, err := ParseMatchFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(filters) != tt.wantFilters {
				t.Errorf("expected %d filters, got %d", tt.wantFilters, len(filters))
			}

			// Verify all filters use eq operator
			for _, f := range filters {
				if f.Operator != "eq" {
					t.Errorf("expected operator 'eq', got %q", f.Operator)
				}
			}
		})
	}
}

func TestParseMatchFilterContent(t *testing.T) {
	// Test specific filter content
	filters, err := ParseMatchFilter(`{"status":"active","priority":"high"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(filters) != 2 {
		t.Fatalf("expected 2 filters, got %d", len(filters))
	}

	// Build a map for easier testing (order is not guaranteed)
	filterMap := make(map[string]Filter)
	for _, f := range filters {
		filterMap[f.Column] = f
	}

	// Check status filter
	if statusFilter, ok := filterMap["status"]; ok {
		if statusFilter.Operator != "eq" {
			t.Errorf("expected operator 'eq' for status, got %q", statusFilter.Operator)
		}
		if statusFilter.Value != "active" {
			t.Errorf("expected value 'active' for status, got %q", statusFilter.Value)
		}
	} else {
		t.Error("expected 'status' filter not found")
	}

	// Check priority filter
	if priorityFilter, ok := filterMap["priority"]; ok {
		if priorityFilter.Operator != "eq" {
			t.Errorf("expected operator 'eq' for priority, got %q", priorityFilter.Operator)
		}
		if priorityFilter.Value != "high" {
			t.Errorf("expected value 'high' for priority, got %q", priorityFilter.Value)
		}
	} else {
		t.Error("expected 'priority' filter not found")
	}
}

func TestParseMatchFilterNumericValue(t *testing.T) {
	filters, err := ParseMatchFilter(`{"count":42}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}

	// Numeric values should be converted to string
	if filters[0].Value != "42" {
		t.Errorf("expected value '42', got %q", filters[0].Value)
	}
}

func TestParseMatchFilterBooleanValue(t *testing.T) {
	filters, err := ParseMatchFilter(`{"active":true}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(filters) != 1 {
		t.Fatalf("expected 1 filter, got %d", len(filters))
	}

	// Boolean values should be converted to string
	if filters[0].Value != "true" {
		t.Errorf("expected value 'true', got %q", filters[0].Value)
	}
}

func TestParseFilterRelatedTable(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTable string
		expectedCol   string
		expectedOp    string
		expectedVal   string
		wantErr       bool
	}{
		{
			name:          "related table eq filter",
			input:         "country.name=eq.Canada",
			expectedTable: "country",
			expectedCol:   "name",
			expectedOp:    "eq",
			expectedVal:   "Canada",
		},
		{
			name:          "related table neq filter",
			input:         "author.status=neq.banned",
			expectedTable: "author",
			expectedCol:   "status",
			expectedOp:    "neq",
			expectedVal:   "banned",
		},
		{
			name:          "related table gt filter",
			input:         "category.priority=gt.5",
			expectedTable: "category",
			expectedCol:   "priority",
			expectedOp:    "gt",
			expectedVal:   "5",
		},
		{
			name:          "related table like filter",
			input:         "company.name=like.*Tech*",
			expectedTable: "company",
			expectedCol:   "name",
			expectedOp:    "like",
			expectedVal:   "*Tech*",
		},
		{
			name:          "related table in filter",
			input:         "user.role=in.(admin,editor)",
			expectedTable: "user",
			expectedCol:   "role",
			expectedOp:    "in",
			expectedVal:   "(admin,editor)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if filter.RelatedTable != tt.expectedTable {
				t.Errorf("expected RelatedTable %q, got %q", tt.expectedTable, filter.RelatedTable)
			}
			if filter.RelatedColumn != tt.expectedCol {
				t.Errorf("expected RelatedColumn %q, got %q", tt.expectedCol, filter.RelatedColumn)
			}
			if filter.Operator != tt.expectedOp {
				t.Errorf("expected Operator %q, got %q", tt.expectedOp, filter.Operator)
			}
			if filter.Value != tt.expectedVal {
				t.Errorf("expected Value %q, got %q", tt.expectedVal, filter.Value)
			}
		})
	}
}

func TestParseFilterRelatedTableIsRelated(t *testing.T) {
	// Test IsRelatedFilter method
	relatedFilter, _ := ParseFilter("country.name=eq.Canada")
	if !relatedFilter.IsRelatedFilter() {
		t.Error("expected IsRelatedFilter() to return true for related filter")
	}

	regularFilter, _ := ParseFilter("name=eq.Canada")
	if regularFilter.IsRelatedFilter() {
		t.Error("expected IsRelatedFilter() to return false for regular filter")
	}
}

func TestFilterToRelatedSQL(t *testing.T) {
	tests := []struct {
		name        string
		filter      Filter
		expectedSQL string
		expectedLen int // expected number of args
	}{
		{
			name: "eq operator",
			filter: Filter{
				Column:        "country.name",
				Operator:      "eq",
				Value:         "Canada",
				RelatedTable:  "country",
				RelatedColumn: "name",
			},
			expectedSQL: `"name" = ?`,
			expectedLen: 1,
		},
		{
			name: "is null",
			filter: Filter{
				Column:        "user.deleted_at",
				Operator:      "is",
				Value:         "null",
				RelatedTable:  "user",
				RelatedColumn: "deleted_at",
			},
			expectedSQL: `"deleted_at" IS NULL`,
			expectedLen: 0,
		},
		{
			name: "like operator",
			filter: Filter{
				Column:        "company.name",
				Operator:      "like",
				Value:         "*Tech*",
				RelatedTable:  "company",
				RelatedColumn: "name",
			},
			expectedSQL: `"name" LIKE ?`,
			expectedLen: 1,
		},
		{
			name: "in operator",
			filter: Filter{
				Column:        "role.name",
				Operator:      "in",
				Value:         "(admin,editor)",
				RelatedTable:  "role",
				RelatedColumn: "name",
			},
			expectedSQL: `"name" IN (?, ?)`,
			expectedLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql, args := tt.filter.ToRelatedSQL()
			if sql != tt.expectedSQL {
				t.Errorf("expected SQL %q, got %q", tt.expectedSQL, sql)
			}
			if len(args) != tt.expectedLen {
				t.Errorf("expected %d args, got %d", tt.expectedLen, len(args))
			}
		})
	}
}

func TestParseOrderRelatedTable(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedTable string
		expectedCol   string
		expectedDesc  bool
	}{
		{
			name:          "simple related order",
			input:         "country(name)",
			expectedTable: "country",
			expectedCol:   "name",
			expectedDesc:  false,
		},
		{
			name:          "related order with desc",
			input:         "country(name).desc",
			expectedTable: "country",
			expectedCol:   "name",
			expectedDesc:  true,
		},
		{
			name:          "related order with asc",
			input:         "author(last_name).asc",
			expectedTable: "author",
			expectedCol:   "last_name",
			expectedDesc:  false,
		},
		{
			name:          "regular column (no relation)",
			input:         "created_at.desc",
			expectedTable: "",
			expectedCol:   "created_at",
			expectedDesc:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := ParseOrder(tt.input)
			if len(orders) != 1 {
				t.Fatalf("expected 1 order, got %d", len(orders))
			}
			order := orders[0]
			if order.RelatedTable != tt.expectedTable {
				t.Errorf("expected RelatedTable %q, got %q", tt.expectedTable, order.RelatedTable)
			}
			if order.Column != tt.expectedCol {
				t.Errorf("expected Column %q, got %q", tt.expectedCol, order.Column)
			}
			if order.Desc != tt.expectedDesc {
				t.Errorf("expected Desc %v, got %v", tt.expectedDesc, order.Desc)
			}
		})
	}
}

func TestParseOrderMultipleWithRelated(t *testing.T) {
	// Test parsing multiple order clauses including related
	orders := ParseOrder("country(name).asc,created_at.desc")
	if len(orders) != 2 {
		t.Fatalf("expected 2 orders, got %d", len(orders))
	}

	// First: country(name).asc
	if orders[0].RelatedTable != "country" {
		t.Errorf("expected first RelatedTable 'country', got %q", orders[0].RelatedTable)
	}
	if orders[0].Column != "name" {
		t.Errorf("expected first Column 'name', got %q", orders[0].Column)
	}
	if orders[0].Desc {
		t.Error("expected first Desc to be false")
	}

	// Second: created_at.desc
	if orders[1].RelatedTable != "" {
		t.Errorf("expected second RelatedTable to be empty, got %q", orders[1].RelatedTable)
	}
	if orders[1].Column != "created_at" {
		t.Errorf("expected second Column 'created_at', got %q", orders[1].Column)
	}
	if !orders[1].Desc {
		t.Error("expected second Desc to be true")
	}
}

func TestJSONPathFilter(t *testing.T) {
	f, err := ParseFilter("address->postcode=eq.90210")
	if err != nil {
		t.Fatalf("ParseFilter error: %v", err)
	}
	if f.Column != "address" {
		t.Errorf("expected Column 'address', got %q", f.Column)
	}
	if f.JSONPath != "$.postcode" {
		t.Errorf("expected JSONPath '$.postcode', got %q", f.JSONPath)
	}
	if f.Operator != "eq" {
		t.Errorf("expected Operator 'eq', got %q", f.Operator)
	}
	if f.Value != "90210" {
		t.Errorf("expected Value '90210', got %q", f.Value)
	}
}

func TestParseFTSOperator(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedBaseOp string
		expectedConfig string
	}{
		{
			name:           "simple fts",
			input:          "fts",
			expectedBaseOp: "fts",
			expectedConfig: "",
		},
		{
			name:           "fts with english config",
			input:          "fts(english)",
			expectedBaseOp: "fts",
			expectedConfig: "english",
		},
		{
			name:           "plfts with german config",
			input:          "plfts(german)",
			expectedBaseOp: "plfts",
			expectedConfig: "german",
		},
		{
			name:           "phfts with spanish config",
			input:          "phfts(spanish)",
			expectedBaseOp: "phfts",
			expectedConfig: "spanish",
		},
		{
			name:           "wfts with simple config",
			input:          "wfts(simple)",
			expectedBaseOp: "wfts",
			expectedConfig: "simple",
		},
		{
			name:           "simple plfts",
			input:          "plfts",
			expectedBaseOp: "plfts",
			expectedConfig: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseOp, config := parseFTSOperator(tt.input)
			if baseOp != tt.expectedBaseOp {
				t.Errorf("expected baseOp %q, got %q", tt.expectedBaseOp, baseOp)
			}
			if config != tt.expectedConfig {
				t.Errorf("expected config %q, got %q", tt.expectedConfig, config)
			}
		})
	}
}

func TestIsFTSOperator(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"fts", "fts", true},
		{"plfts", "plfts", true},
		{"phfts", "phfts", true},
		{"wfts", "wfts", true},
		{"fts with config", "fts(english)", true},
		{"plfts with config", "plfts(german)", true},
		{"phfts with config", "phfts(spanish)", true},
		{"wfts with config", "wfts(simple)", true},
		{"unknown operator", "unknown", false},
		{"eq operator", "eq", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsFTSOperator(tt.input)
			if result != tt.expected {
				t.Errorf("IsFTSOperator(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFTSFilter(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedColumn string
		expectedOp     string
		expectedQuery  string
		expectedConfig string
		isNil          bool
		wantErr        bool
	}{
		{
			name:           "simple fts",
			input:          "content=fts.programming",
			expectedColumn: "content",
			expectedOp:     "fts",
			expectedQuery:  "programming",
			expectedConfig: "",
		},
		{
			name:           "fts with config",
			input:          "content=fts(english).programming",
			expectedColumn: "content",
			expectedOp:     "fts",
			expectedQuery:  "programming",
			expectedConfig: "english",
		},
		{
			name:           "plfts with config",
			input:          "body=plfts(german).'fat cat'",
			expectedColumn: "body",
			expectedOp:     "plfts",
			expectedQuery:  "'fat cat'",
			expectedConfig: "german",
		},
		{
			name:           "phfts with config",
			input:          "title=phfts(spanish).'hello world'",
			expectedColumn: "title",
			expectedOp:     "phfts",
			expectedQuery:  "'hello world'",
			expectedConfig: "spanish",
		},
		{
			name:           "wfts with config",
			input:          "text=wfts(simple).fat or cat",
			expectedColumn: "text",
			expectedOp:     "wfts",
			expectedQuery:  "fat or cat",
			expectedConfig: "simple",
		},
		{
			name:  "not an FTS filter",
			input: "status=eq.active",
			isNil: true,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseFTSFilter(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.isNil {
				if filter != nil {
					t.Errorf("expected nil filter, got %+v", filter)
				}
				return
			}
			if filter == nil {
				t.Fatal("expected filter, got nil")
			}
			if filter.Column != tt.expectedColumn {
				t.Errorf("expected Column %q, got %q", tt.expectedColumn, filter.Column)
			}
			if filter.Operator != tt.expectedOp {
				t.Errorf("expected Operator %q, got %q", tt.expectedOp, filter.Operator)
			}
			if filter.Query != tt.expectedQuery {
				t.Errorf("expected Query %q, got %q", tt.expectedQuery, filter.Query)
			}
			if filter.Config != tt.expectedConfig {
				t.Errorf("expected Config %q, got %q", tt.expectedConfig, filter.Config)
			}
		})
	}
}
