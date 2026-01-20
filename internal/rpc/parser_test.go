// internal/rpc/parser_test.go
package rpc

import (
	"testing"
)

func TestParseCreateFunction_Simple(t *testing.T) {
	sql := `CREATE FUNCTION get_one() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Name != "get_one" {
		t.Errorf("Name = %q, want %q", fn.Name, "get_one")
	}
	if fn.ReturnType != "integer" {
		t.Errorf("ReturnType = %q, want %q", fn.ReturnType, "integer")
	}
	if fn.Body != "SELECT 1" {
		t.Errorf("Body = %q, want %q", fn.Body, "SELECT 1")
	}
	if fn.OrReplace {
		t.Error("OrReplace = true, want false")
	}
}

func TestParseCreateFunction_OrReplace(t *testing.T) {
	sql := `CREATE OR REPLACE FUNCTION my_func() RETURNS void LANGUAGE sql AS $$ SELECT 1 $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !fn.OrReplace {
		t.Error("OrReplace = false, want true")
	}
}

func TestParseCreateFunction_WithArgs(t *testing.T) {
	sql := `CREATE FUNCTION get_user(user_id uuid, include_deleted boolean DEFAULT false)
	        RETURNS TABLE(id uuid, email text)
	        LANGUAGE sql AS $$
	          SELECT id, email FROM users WHERE id = user_id
	        $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Name != "get_user" {
		t.Errorf("Name = %q, want %q", fn.Name, "get_user")
	}
	if len(fn.Args) != 2 {
		t.Fatalf("Args len = %d, want 2", len(fn.Args))
	}
	if fn.Args[0].Name != "user_id" {
		t.Errorf("Args[0].Name = %q, want %q", fn.Args[0].Name, "user_id")
	}
	if fn.Args[0].Type != "uuid" {
		t.Errorf("Args[0].Type = %q, want %q", fn.Args[0].Type, "uuid")
	}
	if fn.Args[1].DefaultValue == nil || *fn.Args[1].DefaultValue != "false" {
		t.Errorf("Args[1].DefaultValue unexpected")
	}
	if !fn.ReturnsSet {
		t.Error("ReturnsSet = false, want true (for TABLE)")
	}
}

func TestParseCreateFunction_ReturnsSetof(t *testing.T) {
	sql := `CREATE FUNCTION get_all_users() RETURNS SETOF users LANGUAGE sql AS $$ SELECT * FROM users $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if !fn.ReturnsSet {
		t.Error("ReturnsSet = false, want true")
	}
	if fn.ReturnType != "users" {
		t.Errorf("ReturnType = %q, want %q", fn.ReturnType, "users")
	}
}

func TestParseCreateFunction_Volatility(t *testing.T) {
	sql := `CREATE FUNCTION cached_count() RETURNS integer LANGUAGE sql STABLE AS $$ SELECT count(*) FROM users $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Volatility != "STABLE" {
		t.Errorf("Volatility = %q, want %q", fn.Volatility, "STABLE")
	}
}

func TestParseCreateFunction_SecurityDefiner(t *testing.T) {
	sql := `CREATE FUNCTION admin_only() RETURNS void LANGUAGE sql SECURITY DEFINER AS $$ SELECT 1 $$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Security != "DEFINER" {
		t.Errorf("Security = %q, want %q", fn.Security, "DEFINER")
	}
}

func TestParseCreateFunction_DollarTag(t *testing.T) {
	sql := `CREATE FUNCTION with_tag() RETURNS text LANGUAGE sql AS $body$ SELECT 'hello' $body$;`

	fn, err := ParseCreateFunction(sql)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if fn.Body != "SELECT 'hello'" {
		t.Errorf("Body = %q, want %q", fn.Body, "SELECT 'hello'")
	}
}

func TestParseCreateFunction_RejectPlpgsql(t *testing.T) {
	sql := `CREATE FUNCTION plpg() RETURNS void LANGUAGE plpgsql AS $$ BEGIN END; $$;`

	_, err := ParseCreateFunction(sql)
	if err == nil {
		t.Error("expected error for plpgsql, got nil")
	}
}

func TestIsCreateFunction(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"CREATE FUNCTION foo() RETURNS int LANGUAGE sql AS $$ SELECT 1 $$", true},
		{"CREATE OR REPLACE FUNCTION bar() RETURNS void LANGUAGE sql AS $$$$", true},
		{"  create function baz() returns text language sql as $$x$$", true},
		{"SELECT * FROM functions", false},
		{"CREATE TABLE functions (id int)", false},
		{"DROP FUNCTION foo", false},
	}

	for _, tt := range tests {
		got := IsCreateFunction(tt.sql)
		if got != tt.want {
			t.Errorf("IsCreateFunction(%q) = %v, want %v", tt.sql, got, tt.want)
		}
	}
}

func TestIsDropFunction(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"DROP FUNCTION foo", true},
		{"DROP FUNCTION IF EXISTS foo", true},
		{"drop function bar()", true},
		{"SELECT * FROM foo", false},
	}

	for _, tt := range tests {
		got := IsDropFunction(tt.sql)
		if got != tt.want {
			t.Errorf("IsDropFunction(%q) = %v, want %v", tt.sql, got, tt.want)
		}
	}
}

func TestParseDropFunction(t *testing.T) {
	tests := []struct {
		sql      string
		name     string
		ifExists bool
	}{
		{"DROP FUNCTION foo", "foo", false},
		{"DROP FUNCTION IF EXISTS bar", "bar", true},
		{"DROP FUNCTION baz()", "baz", false},
		{"DROP FUNCTION IF EXISTS qux(uuid, text)", "qux", true},
	}

	for _, tt := range tests {
		name, ifExists, err := ParseDropFunction(tt.sql)
		if err != nil {
			t.Errorf("ParseDropFunction(%q) error: %v", tt.sql, err)
			continue
		}
		if name != tt.name {
			t.Errorf("ParseDropFunction(%q) name = %q, want %q", tt.sql, name, tt.name)
		}
		if ifExists != tt.ifExists {
			t.Errorf("ParseDropFunction(%q) ifExists = %v, want %v", tt.sql, ifExists, tt.ifExists)
		}
	}
}
