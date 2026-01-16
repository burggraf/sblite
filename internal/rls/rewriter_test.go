// internal/rls/rewriter_test.go
package rls

import (
	"testing"
)

func TestSubstituteAuthFunctions(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "authenticated",
		Claims: map[string]any{
			"custom_claim": "custom_value",
		},
	}

	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "auth.uid()",
			expr:     "user_id = auth.uid()",
			expected: "user_id = 'user-123'",
		},
		{
			name:     "auth.role()",
			expr:     "role = auth.role()",
			expected: "role = 'authenticated'",
		},
		{
			name:     "auth.email()",
			expr:     "email = auth.email()",
			expected: "email = 'test@example.com'",
		},
		{
			name:     "auth.jwt()->>'key'",
			expr:     "custom = auth.jwt()->>'custom_claim'",
			expected: "custom = 'custom_value'",
		},
		{
			name:     "multiple substitutions",
			expr:     "user_id = auth.uid() AND role = auth.role()",
			expected: "user_id = 'user-123' AND role = 'authenticated'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SubstituteAuthFunctions(tt.expr, ctx)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSubstituteAuthFunctionsWithSQLInjection(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user'; DROP TABLE users; --",
		Email:  "test@example.com",
		Role:   "authenticated",
	}

	result := SubstituteAuthFunctions("user_id = auth.uid()", ctx)
	expected := "user_id = 'user''; DROP TABLE users; --'"

	if result != expected {
		t.Errorf("SQL injection not properly escaped: got %q, expected %q", result, expected)
	}
}

func TestSubstituteAuthFunctionsNilContext(t *testing.T) {
	result := SubstituteAuthFunctions("user_id = auth.uid()", nil)
	if result != "user_id = auth.uid()" {
		t.Errorf("expected unchanged expression with nil context, got %q", result)
	}
}

func TestSubstituteAuthFunctionsMissingClaim(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "authenticated",
		Claims: map[string]any{},
	}

	result := SubstituteAuthFunctions("custom = auth.jwt()->>'nonexistent'", ctx)
	expected := "custom = NULL"

	if result != expected {
		t.Errorf("expected %q for missing claim, got %q", expected, result)
	}
}

func TestSubstituteAuthFunctionsNumericClaim(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "authenticated",
		Claims: map[string]any{
			"age": float64(25),
		},
	}

	result := SubstituteAuthFunctions("age = auth.jwt()->>'age'", ctx)
	expected := "age = '25'"

	if result != expected {
		t.Errorf("expected %q for numeric claim, got %q", expected, result)
	}
}

func TestSubstituteAuthFunctionsBooleanClaim(t *testing.T) {
	ctx := &AuthContext{
		UserID: "user-123",
		Email:  "test@example.com",
		Role:   "authenticated",
		Claims: map[string]any{
			"admin": true,
		},
	}

	result := SubstituteAuthFunctions("is_admin = auth.jwt()->>'admin'", ctx)
	expected := "is_admin = 'true'"

	if result != expected {
		t.Errorf("expected %q for boolean claim, got %q", expected, result)
	}
}

func TestEscapeSQLString(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"it's", "it''s"},
		{"'quoted'", "''quoted''"},
		{"O'Brien's", "O''Brien''s"},
	}

	for _, tt := range tests {
		result := escapeSQLString(tt.input)
		if result != tt.expected {
			t.Errorf("escapeSQLString(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestToString(t *testing.T) {
	tests := []struct {
		input    any
		expected string
	}{
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{true, "true"},
		{false, "false"},
		{123, "123"},
	}

	for _, tt := range tests {
		result := toString(tt.input)
		if result != tt.expected {
			t.Errorf("toString(%v) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}
