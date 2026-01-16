// internal/rest/query_test.go
package rest

import (
	"reflect"
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
