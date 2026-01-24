package pgtranslate

import (
	"strings"
	"testing"
)

// TestLexer tests the lexer/tokenizer
func TestLexer(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		tokens []TokenType
	}{
		{
			name:   "Simple SELECT",
			input:  "SELECT * FROM users",
			tokens: []TokenType{TokenSelect, TokenStar, TokenFrom, TokenIdent, TokenEOF},
		},
		{
			name:   "String literal",
			input:  "'hello world'",
			tokens: []TokenType{TokenString, TokenEOF},
		},
		{
			name:   "Numeric literals",
			input:  "42 3.14 -10",
			tokens: []TokenType{TokenNumber, TokenNumber, TokenMinus, TokenNumber, TokenEOF},
		},
		{
			name:   "Type cast",
			input:  "col::TEXT",
			tokens: []TokenType{TokenIdent, TokenCast, TokenIdent, TokenEOF},
		},
		{
			name:   "JSON operators",
			input:  "data->'key'->>'value'",
			tokens: []TokenType{TokenIdent, TokenJsonArrow, TokenString, TokenJsonArrow2, TokenString, TokenEOF},
		},
		{
			name:   "Dollar-quoted string",
			input:  "$$SELECT * FROM users$$",
			tokens: []TokenType{TokenDollarStr, TokenEOF},
		},
		{
			name:   "Operators",
			input:  "a + b - c * d / e",
			tokens: []TokenType{TokenIdent, TokenPlus, TokenIdent, TokenMinus, TokenIdent, TokenStar, TokenIdent, TokenSlash, TokenIdent, TokenEOF},
		},
		{
			name:   "Comparison operators",
			input:  "a = b != c <> d < e > f <= g >= h",
			tokens: []TokenType{TokenIdent, TokenEq, TokenIdent, TokenNe, TokenIdent, TokenNe, TokenIdent, TokenLt, TokenIdent, TokenGt, TokenIdent, TokenLe, TokenIdent, TokenGe, TokenIdent, TokenEOF},
		},
		{
			name:   "Keywords",
			input:  "CREATE TABLE IF NOT EXISTS",
			tokens: []TokenType{TokenCreate, TokenTable, TokenIf, TokenNot, TokenExists, TokenEOF},
		},
		{
			name:   "Quoted identifier",
			input:  `"column name"`,
			tokens: []TokenType{TokenQIdent, TokenEOF},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lexer := NewLexer(tt.input)
			var got []TokenType

			for {
				tok := lexer.NextToken()
				got = append(got, tok.Type)
				if tok.Type == TokenEOF {
					break
				}
			}

			if len(got) != len(tt.tokens) {
				t.Errorf("token count mismatch: got %d, want %d", len(got), len(tt.tokens))
				return
			}

			for i, want := range tt.tokens {
				if got[i] != want {
					t.Errorf("token %d: got %v, want %v", i, got[i], want)
				}
			}
		})
	}
}

// TestParseExpr tests expression parsing
func TestParseExpr(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Simple identifier", "foo", false},
		{"Qualified reference", "t.col", false},
		{"Number literal", "42", false},
		{"String literal", "'hello'", false},
		{"Binary operation", "a + b", false},
		{"Complex expression", "(a + b) * c", false},
		{"Function call", "NOW()", false},
		{"Function with args", "SUBSTR(name, 1, 5)", false},
		{"Type cast", "id::TEXT", false},
		{"JSON access", "data->>'key'", false},
		{"Comparison", "a = b", false},
		{"AND/OR", "a = 1 AND b = 2 OR c = 3", false},
		{"NOT", "NOT active", false},
		{"IS NULL", "name IS NULL", false},
		{"IS NOT NULL", "name IS NOT NULL", false},
		{"IN list", "status IN ('active', 'pending')", false},
		{"BETWEEN", "age BETWEEN 18 AND 65", false},
		{"LIKE", "name LIKE '%test%'", false},
		{"CASE expression", "CASE WHEN a > 0 THEN 'pos' ELSE 'neg' END", false},
		{"Simple CASE", "CASE status WHEN 1 THEN 'one' WHEN 2 THEN 'two' END", false},
		{"Aggregate with DISTINCT", "COUNT(DISTINCT user_id)", false},
		{"Star expression", "*", false},
		{"Qualified star", "t.*", false},
		{"Negative number", "-10", false},
		{"Boolean literal TRUE", "TRUE", false},
		{"Boolean literal FALSE", "FALSE", false},
		{"NULL literal", "NULL", false},
		{"Nested parentheses", "((a + b) * (c + d))", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			_, err := parser.ParseExpr()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseExpr() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseSelect tests SELECT statement parsing
func TestParseSelect(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Simple select", "SELECT * FROM users", false},
		{"Select columns", "SELECT id, name FROM users", false},
		{"Select with alias", "SELECT id AS user_id FROM users", false},
		{"Select with WHERE", "SELECT * FROM users WHERE active = true", false},
		{"Select with JOIN", "SELECT u.* FROM users u JOIN orders o ON u.id = o.user_id", false},
		{"Select with LEFT JOIN", "SELECT * FROM users u LEFT JOIN orders o ON u.id = o.user_id", false},
		{"Select with GROUP BY", "SELECT status, COUNT(*) FROM users GROUP BY status", false},
		{"Select with HAVING", "SELECT status, COUNT(*) AS cnt FROM users GROUP BY status HAVING COUNT(*) > 5", false},
		{"Select with ORDER BY", "SELECT * FROM users ORDER BY name ASC, created_at DESC", false},
		{"Select with LIMIT", "SELECT * FROM users LIMIT 10", false},
		{"Select with OFFSET", "SELECT * FROM users LIMIT 10 OFFSET 20", false},
		{"Select DISTINCT", "SELECT DISTINCT category FROM products", false},
		{"Select with CTE", "WITH active_users AS (SELECT * FROM users WHERE active = true) SELECT * FROM active_users", false},
		{"Select with subquery", "SELECT * FROM (SELECT id, name FROM users) AS sub", false},
		{"UNION", "SELECT id FROM users UNION SELECT id FROM admins", false},
		{"UNION ALL", "SELECT id FROM users UNION ALL SELECT id FROM admins", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			_, err := parser.ParseSelect()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSelect() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseInsert tests INSERT statement parsing
func TestParseInsert(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Simple insert", "INSERT INTO users (name) VALUES ('John')", false},
		{"Multiple columns", "INSERT INTO users (name, email) VALUES ('John', 'john@example.com')", false},
		{"Multiple rows", "INSERT INTO users (name) VALUES ('John'), ('Jane')", false},
		{"INSERT with RETURNING", "INSERT INTO users (name) VALUES ('John') RETURNING id", false},
		{"INSERT ON CONFLICT DO NOTHING", "INSERT INTO users (email) VALUES ('test@test.com') ON CONFLICT DO NOTHING", false},
		{"INSERT ON CONFLICT DO UPDATE", "INSERT INTO users (email, name) VALUES ('test@test.com', 'Test') ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			_, err := parser.ParseInsert()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseInsert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseUpdate tests UPDATE statement parsing
func TestParseUpdate(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Simple update", "UPDATE users SET name = 'John'", false},
		{"Multiple columns", "UPDATE users SET name = 'John', email = 'john@example.com'", false},
		{"With WHERE", "UPDATE users SET active = false WHERE last_login < '2024-01-01'", false},
		{"With RETURNING", "UPDATE users SET name = 'John' WHERE id = 1 RETURNING *", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			_, err := parser.ParseUpdate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseUpdate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseDelete tests DELETE statement parsing
func TestParseDelete(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Simple delete", "DELETE FROM users", false},
		{"With WHERE", "DELETE FROM users WHERE active = false", false},
		{"With RETURNING", "DELETE FROM users WHERE id = 1 RETURNING *", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			_, err := parser.ParseDelete()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDelete() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseCreateTable tests CREATE TABLE parsing
func TestParseCreateTable(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"Simple table", "CREATE TABLE users (id INTEGER PRIMARY KEY)", false},
		{"Multiple columns", "CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL)", false},
		{"With DEFAULT", "CREATE TABLE users (id INTEGER PRIMARY KEY, created_at TIMESTAMPTZ DEFAULT NOW())", false},
		{"IF NOT EXISTS", "CREATE TABLE IF NOT EXISTS users (id INTEGER)", false},
		{"UUID primary key", "CREATE TABLE users (id UUID PRIMARY KEY DEFAULT gen_random_uuid())", false},
		{"Foreign key", "CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER REFERENCES users(id))", false},
		{"Unique constraint", "CREATE TABLE users (id INTEGER PRIMARY KEY, email TEXT UNIQUE NOT NULL)", false},
		{"Check constraint", "CREATE TABLE users (id INTEGER PRIMARY KEY, age INTEGER CHECK (age >= 0))", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(tt.input)
			_, err := parser.ParseCreateTable()
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCreateTable() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestParseCreateFunction tests CREATE FUNCTION parsing
func TestParseCreateFunction(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			"Simple function",
			`CREATE FUNCTION get_user(user_id INTEGER) RETURNS TEXT LANGUAGE sql AS $$ SELECT name FROM users WHERE id = user_id $$`,
			false,
		},
		{
			"OR REPLACE",
			`CREATE OR REPLACE FUNCTION get_user(user_id INTEGER) RETURNS TEXT LANGUAGE sql AS $$ SELECT name FROM users WHERE id = user_id $$`,
			false,
		},
		{
			"RETURNS TABLE",
			`CREATE FUNCTION get_users() RETURNS TABLE(id INTEGER, name TEXT) LANGUAGE sql AS $$ SELECT id, name FROM users $$`,
			false,
		},
		{
			"RETURNS SETOF",
			`CREATE FUNCTION get_all_users() RETURNS SETOF users LANGUAGE sql AS $$ SELECT * FROM users $$`,
			false,
		},
		{
			"With IMMUTABLE",
			`CREATE FUNCTION add_numbers(a INTEGER, b INTEGER) RETURNS INTEGER LANGUAGE sql IMMUTABLE AS $$ SELECT a + b $$`,
			false,
		},
		{
			"With SECURITY DEFINER",
			`CREATE FUNCTION secure_func() RETURNS TEXT LANGUAGE sql SECURITY DEFINER AS $$ SELECT 'secret' $$`,
			false,
		},
		{
			"With DEFAULT argument",
			`CREATE FUNCTION greet(name TEXT DEFAULT 'World') RETURNS TEXT LANGUAGE sql AS $$ SELECT 'Hello, ' || name $$`,
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stmt, err := ParseCreateFunctionSQL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCreateFunctionSQL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && stmt == nil {
				t.Error("ParseCreateFunctionSQL() returned nil statement")
			}
		})
	}
}

// TestGenerator tests SQL generation from AST
func TestGenerator(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dialect Dialect
		want    string
	}{
		{
			name:    "NOW() to strftime('%Y-%m-%d %H:%M:%f+00', 'now') for SQLite",
			input:   "SELECT NOW()",
			dialect: DialectSQLite,
			want:    "SELECT strftime('%Y-%m-%d %H:%M:%f+00', 'now')",
		},
		{
			name:    "gen_random_uuid() to SQLite subquery",
			input:   "SELECT gen_random_uuid()",
			dialect: DialectSQLite,
			want:    "", // Just verify it doesn't error
		},
		{
			name:    "Boolean TRUE to 1 for SQLite",
			input:   "SELECT TRUE",
			dialect: DialectSQLite,
			want:    "SELECT 1",
		},
		{
			name:    "Boolean FALSE to 0 for SQLite",
			input:   "SELECT FALSE",
			dialect: DialectSQLite,
			want:    "SELECT 0",
		},
		{
			name:    "Type cast preserved as CAST for SQLite",
			input:   "SELECT id::TEXT FROM users",
			dialect: DialectSQLite,
			want:    "CAST", // Type casts become CAST(expr AS type) in SQLite
		},
		{
			name:    "JSON arrow to json_extract for SQLite",
			input:   "SELECT data->>'key' FROM users",
			dialect: DialectSQLite,
			want:    "SELECT json_extract(data, '$.key') FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseAndTranslate(tt.input, tt.dialect)
			if err != nil {
				t.Errorf("ParseAndTranslate() error = %v", err)
				return
			}

			if tt.want != "" && !strings.Contains(result, tt.want) && result != tt.want {
				t.Errorf("ParseAndTranslate() = %v, want to contain %v", result, tt.want)
			}
		})
	}
}

// TestReverseTranslator tests SQLite to PostgreSQL translation
func TestReverseTranslator(t *testing.T) {
	tests := []struct {
		name    string
		sqlite  string
		want    string
	}{
		{
			name:   "strftime('%Y-%m-%d %H:%M:%f+00', 'now') to NOW()",
			sqlite: "strftime('%Y-%m-%d %H:%M:%f+00', 'now')",
			want:   "NOW()",
		},
		{
			name:   "date('now') to CURRENT_DATE",
			sqlite: "date('now')",
			want:   "CURRENT_DATE",
		},
		{
			name:   "time('now') to CURRENT_TIME",
			sqlite: "time('now')",
			want:   "CURRENT_TIME",
		},
		{
			name:   "INSTR to POSITION",
			sqlite: "INSTR(str, 'sub')",
			want:   "POSITION('sub' IN str)",
		},
		{
			name:   "GROUP_CONCAT to STRING_AGG",
			sqlite: "GROUP_CONCAT(name, ', ')",
			want:   "STRING_AGG(name, ', ')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReverseTranslate(tt.sqlite)
			if !strings.Contains(result, tt.want) {
				t.Errorf("ReverseTranslate(%q) = %q, want to contain %q", tt.sqlite, result, tt.want)
			}
		})
	}
}

// TestReverseTranslateDefault tests default value translation
func TestReverseTranslateDefault(t *testing.T) {
	tests := []struct {
		name         string
		sqliteDefault string
		pgType        string
		want          string
	}{
		{
			name:         "strftime('%Y-%m-%d %H:%M:%f+00', 'now') to NOW()",
			sqliteDefault: "strftime('%Y-%m-%d %H:%M:%f+00', 'now')",
			pgType:        "TIMESTAMPTZ",
			want:          "NOW()",
		},
		{
			name:         "date('now') to CURRENT_DATE",
			sqliteDefault: "date('now')",
			pgType:        "DATE",
			want:          "CURRENT_DATE",
		},
		{
			name:         "gen_uuid() to gen_random_uuid()",
			sqliteDefault: "gen_uuid()",
			pgType:        "UUID",
			want:          "gen_random_uuid()",
		},
		{
			name:         "1 to TRUE for boolean",
			sqliteDefault: "1",
			pgType:        "BOOLEAN",
			want:          "TRUE",
		},
		{
			name:         "0 to FALSE for boolean",
			sqliteDefault: "0",
			pgType:        "BOOLEAN",
			want:          "FALSE",
		},
		{
			name:         "Keep numeric unchanged",
			sqliteDefault: "42",
			pgType:        "INTEGER",
			want:          "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ReverseTranslateDefault(tt.sqliteDefault, tt.pgType)
			if result != tt.want {
				t.Errorf("ReverseTranslateDefault(%q, %q) = %q, want %q", tt.sqliteDefault, tt.pgType, result, tt.want)
			}
		})
	}
}

// TestFunctionMapper tests bidirectional function mappings
func TestFunctionMapper(t *testing.T) {
	mapper := NewFunctionMapper()

	t.Run("PostgreSQL to SQLite", func(t *testing.T) {
		tests := []struct {
			name string
			fn   string
			args []Expr
			want string
		}{
			{"NOW", "NOW", nil, "strftime('%Y-%m-%d %H:%M:%f+00', 'now')"},
			{"CURRENT_DATE", "CURRENT_DATE", nil, "date('now')"},
			{"CURRENT_TIME", "CURRENT_TIME", nil, "time('now')"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				call := &FunctionCall{Name: tt.fn, Args: tt.args}
				result, ok := mapper.MapToSQLite(call)
				if !ok {
					t.Errorf("MapToSQLite(%q) returned ok=false", tt.fn)
					return
				}
				if result != tt.want {
					t.Errorf("MapToSQLite(%q) = %q, want %q", tt.fn, result, tt.want)
				}
			})
		}
	})

	t.Run("SQLite to PostgreSQL", func(t *testing.T) {
		tests := []struct {
			name string
			fn   string
			args []Expr
			want string
		}{
			{"DATETIME now", "DATETIME", []Expr{&Literal{Type: LitString, Value: "now"}}, "NOW()"},
			{"DATE now", "DATE", []Expr{&Literal{Type: LitString, Value: "now"}}, "CURRENT_DATE"},
			{"TIME now", "TIME", []Expr{&Literal{Type: LitString, Value: "now"}}, "CURRENT_TIME"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				call := &FunctionCall{Name: tt.fn, Args: tt.args}
				result, ok := mapper.MapToPostgreSQL(call)
				if !ok {
					t.Errorf("MapToPostgreSQL(%q) returned ok=false", tt.fn)
					return
				}
				if result != tt.want {
					t.Errorf("MapToPostgreSQL(%q) = %q, want %q", tt.fn, result, tt.want)
				}
			})
		}
	})
}

// TestTypeMapper tests type mappings
func TestTypeMapper(t *testing.T) {
	mapper := NewTypeMapper()

	t.Run("PostgreSQL to SQLite", func(t *testing.T) {
		tests := []struct {
			pgType     string
			sqliteType string
		}{
			{"UUID", "TEXT"},
			{"BOOLEAN", "INTEGER"},
			{"BOOL", "INTEGER"},
			{"TIMESTAMPTZ", "TEXT"},
			{"TIMESTAMP", "TEXT"},
			{"JSONB", "TEXT"},
			{"JSON", "TEXT"},
			{"SERIAL", "INTEGER"},
			{"BIGSERIAL", "INTEGER"},
			{"VARCHAR(255)", "TEXT"},
			{"CHAR(10)", "TEXT"},
			{"BYTEA", "BLOB"},
			{"vector(1536)", "TEXT"},
		}

		for _, tt := range tests {
			t.Run(tt.pgType, func(t *testing.T) {
				result := mapper.MapToSQLite(tt.pgType)
				if result != tt.sqliteType {
					t.Errorf("MapToSQLite(%q) = %q, want %q", tt.pgType, result, tt.sqliteType)
				}
			})
		}
	})

	t.Run("SQLite to PostgreSQL", func(t *testing.T) {
		tests := []struct {
			sqliteType string
			pgType     string
		}{
			{"TEXT", "TEXT"},
			{"INTEGER", "INTEGER"},
			{"REAL", "DOUBLE PRECISION"},
			{"BLOB", "BYTEA"},
			{"NUMERIC", "NUMERIC"},
		}

		for _, tt := range tests {
			t.Run(tt.sqliteType, func(t *testing.T) {
				result := mapper.MapToPostgreSQL(tt.sqliteType)
				if result != tt.pgType {
					t.Errorf("MapToPostgreSQL(%q) = %q, want %q", tt.sqliteType, result, tt.pgType)
				}
			})
		}
	})
}

// TestErrors tests error types
func TestErrors(t *testing.T) {
	t.Run("ParseError", func(t *testing.T) {
		err := NewParseError("unexpected token", Position{Line: 1, Column: 5})
		if !strings.Contains(err.Error(), "line 1") {
			t.Errorf("ParseError.Error() should contain line number")
		}
		if !strings.Contains(err.Error(), "column 5") {
			t.Errorf("ParseError.Error() should contain column number")
		}
	})

	t.Run("ParseErrorVerbose", func(t *testing.T) {
		err := NewParseErrorWithSource("unexpected token", Position{Line: 1, Column: 5}, "SELECT * FROM")
		verbose := err.Verbose()
		if !strings.Contains(verbose, "SELECT * FROM") {
			t.Errorf("ParseError.Verbose() should contain source")
		}
	})

	t.Run("TranslationError", func(t *testing.T) {
		err := NewTranslationError("unsupported operation", &Identifier{Name: "test"})
		if !strings.Contains(err.Error(), "unsupported operation") {
			t.Errorf("TranslationError.Error() should contain message")
		}
	})

	t.Run("UnsupportedFeatureError", func(t *testing.T) {
		err := NewUnsupportedFeatureError("WINDOW functions", DialectSQLite, Position{})
		if !strings.Contains(err.Error(), "WINDOW functions") {
			t.Errorf("UnsupportedFeatureError.Error() should contain feature name")
		}
		if !strings.Contains(err.Error(), "SQLite") {
			t.Errorf("UnsupportedFeatureError.Error() should contain dialect")
		}
	})
}

// TestEndToEnd tests full translation workflows
func TestEndToEnd(t *testing.T) {
	t.Run("PG to SQLite", func(t *testing.T) {
		tests := []struct {
			name  string
			pg    string
			check func(string) bool
		}{
			{
				name:  "Simple SELECT with NOW()",
				pg:    "SELECT NOW(), id FROM users",
				check: func(s string) bool { return strings.Contains(s, "strftime('%Y-%m-%d %H:%M:%f+00', 'now')") },
			},
			{
				name:  "SELECT with boolean",
				pg:    "SELECT * FROM users WHERE active = TRUE",
				check: func(s string) bool { return strings.Contains(s, "= 1") },
			},
			{
				name:  "SELECT with type cast",
				pg:    "SELECT id::TEXT FROM users",
				check: func(s string) bool { return !strings.Contains(s, "::TEXT") },
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := TranslateToSQLite(tt.pg)
				if !tt.check(result) {
					t.Errorf("TranslateToSQLite(%q) = %q, check failed", tt.pg, result)
				}
			})
		}
	})
}

// TestIsCreateFunctionSQL tests the helper function
func TestIsCreateFunctionSQL(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"CREATE FUNCTION foo() RETURNS TEXT AS $$SELECT 1$$", true},
		{"CREATE OR REPLACE FUNCTION foo() RETURNS TEXT AS $$SELECT 1$$", true},
		{"create function foo() returns text as $$SELECT 1$$", true},
		{"SELECT * FROM users", false},
		{"DROP FUNCTION foo", false},
		{"CREATE TABLE foo (id INT)", false},
	}

	for _, tt := range tests {
		t.Run(tt.sql[:min(30, len(tt.sql))], func(t *testing.T) {
			got := IsCreateFunctionSQL(tt.sql)
			if got != tt.want {
				t.Errorf("IsCreateFunctionSQL(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

// TestIsDropFunctionSQL tests the helper function
func TestIsDropFunctionSQL(t *testing.T) {
	tests := []struct {
		sql  string
		want bool
	}{
		{"DROP FUNCTION foo", true},
		{"DROP FUNCTION IF EXISTS foo", true},
		{"drop function foo", true},
		{"SELECT * FROM users", false},
		{"CREATE FUNCTION foo() RETURNS TEXT AS $$SELECT 1$$", false},
	}

	for _, tt := range tests {
		t.Run(tt.sql[:min(30, len(tt.sql))], func(t *testing.T) {
			got := IsDropFunctionSQL(tt.sql)
			if got != tt.want {
				t.Errorf("IsDropFunctionSQL(%q) = %v, want %v", tt.sql, got, tt.want)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
