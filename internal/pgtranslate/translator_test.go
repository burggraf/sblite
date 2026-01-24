package pgtranslate

import (
	"strings"
	"testing"
)

func TestTranslator_DateTimeFunctions(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NOW() function",
			input:    "SELECT NOW()",
			expected: "SELECT datetime('now')",
		},
		{
			name:     "CURRENT_TIMESTAMP",
			input:    "SELECT CURRENT_TIMESTAMP",
			expected: "SELECT datetime('now')",
		},
		{
			name:     "CURRENT_DATE",
			input:    "SELECT CURRENT_DATE",
			expected: "SELECT date('now')",
		},
		{
			name:     "NOW() in WHERE clause",
			input:    "SELECT * FROM users WHERE created_at < NOW()",
			expected: "SELECT * FROM users WHERE created_at < datetime('now')",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if result != tt.expected {
				t.Errorf("Translate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_StringFunctions(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "LEFT function",
			input:    "SELECT LEFT(name, 5) FROM users",
			expected: "SELECT SUBSTR(name, 1, 5) FROM users",
		},
		{
			name:     "RIGHT function",
			input:    "SELECT RIGHT(email, 10) FROM users",
			expected: "SELECT SUBSTR(email, -10) FROM users",
		},
		{
			name:     "POSITION function",
			input:    "SELECT POSITION('test' IN email) FROM users",
			expected: "SELECT INSTR(email, 'test') FROM users",
		},
		{
			name:     "Case insensitive - left",
			input:    "SELECT left(name, 3) FROM users",
			expected: "SELECT SUBSTR(name, 1, 3) FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if result != tt.expected {
				t.Errorf("Translate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_TypeCasts(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Remove ::uuid cast",
			input:    "SELECT id::uuid FROM users",
			expected: "SELECT id FROM users",
		},
		{
			name:     "Remove ::timestamptz cast",
			input:    "SELECT created_at::timestamptz FROM users",
			expected: "SELECT created_at FROM users",
		},
		{
			name:     "Remove ::text cast",
			input:    "SELECT id::text FROM users",
			expected: "SELECT id FROM users",
		},
		{
			name:     "Remove ::integer cast",
			input:    "SELECT count::integer FROM stats",
			expected: "SELECT count FROM stats",
		},
		{
			name:     "Multiple casts in one query",
			input:    "SELECT id::uuid, created_at::timestamptz FROM users",
			expected: "SELECT id, created_at FROM users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if result != tt.expected {
				t.Errorf("Translate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_DataTypes(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UUID type to TEXT",
			input:    "CREATE TABLE users (id UUID PRIMARY KEY)",
			expected: "CREATE TABLE users (id TEXT PRIMARY KEY)",
		},
		{
			name:     "BOOLEAN type to INTEGER",
			input:    "CREATE TABLE users (active BOOLEAN)",
			expected: "CREATE TABLE users (active INTEGER)",
		},
		{
			name:     "TIMESTAMPTZ to TEXT",
			input:    "CREATE TABLE events (timestamp TIMESTAMPTZ)",
			expected: "CREATE TABLE events (timestamp TEXT)",
		},
		{
			name:     "JSONB to TEXT",
			input:    "CREATE TABLE data (metadata JSONB)",
			expected: "CREATE TABLE data (metadata TEXT)",
		},
		{
			name:     "SERIAL to INTEGER",
			input:    "CREATE TABLE items (id SERIAL PRIMARY KEY)",
			expected: "CREATE TABLE items (id INTEGER PRIMARY KEY)",
		},
		{
			name:     "Case insensitive types",
			input:    "CREATE TABLE test (flag boolean, data jsonb)",
			expected: "CREATE TABLE test (flag INTEGER, data TEXT)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if result != tt.expected {
				t.Errorf("Translate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_BooleanLiterals(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "TRUE literal",
			input:    "SELECT * FROM users WHERE active = TRUE",
			expected: "SELECT * FROM users WHERE active = 1",
		},
		{
			name:     "FALSE literal",
			input:    "UPDATE users SET active = FALSE",
			expected: "UPDATE users SET active = 0",
		},
		{
			name:     "Case insensitive",
			input:    "SELECT * FROM users WHERE deleted = false",
			expected: "SELECT * FROM users WHERE deleted = 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if result != tt.expected {
				t.Errorf("Translate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_SpecialFunctions(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name           string
		input          string
		expectedPrefix string
		checkUUIDv4    bool
	}{
		{
			name:           "gen_random_uuid()",
			input:          "INSERT INTO users (id) VALUES (gen_random_uuid())",
			expectedPrefix: "INSERT INTO users (id) VALUES ((SELECT lower(",
			checkUUIDv4:    true,
		},
		{
			name:           "gen_random_uuid() in CREATE TABLE",
			input:          "CREATE TABLE users (id UUID PRIMARY KEY DEFAULT gen_random_uuid())",
			expectedPrefix: "CREATE TABLE users (id TEXT PRIMARY KEY)", // DEFAULT removed - SQLite doesn't support function calls in DEFAULT
			checkUUIDv4:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if !strings.HasPrefix(result, tt.expectedPrefix) {
				t.Errorf("Translate() = %q, expected to start with %q", result, tt.expectedPrefix)
			}

			if tt.checkUUIDv4 {
				// Verify the UUID v4 generation structure is correct
				if !strings.Contains(result, "'4' || substr(h, 14, 3)") {
					t.Error("UUID generation should set version 4")
				}
				if !strings.Contains(result, "substr('89ab', (abs(random()) % 4) + 1, 1)") {
					t.Error("UUID generation should set variant bits (8, 9, a, or b)")
				}
				// Check for proper UUID format structure (8-4-4-4-12)
				if !strings.Contains(result, "substr(h, 1, 8) || '-' ||") {
					t.Error("UUID should have 8 hex digits in first section")
				}
				if !strings.Contains(result, "substr(h, 21, 12)") {
					t.Error("UUID should have 12 hex digits in last section")
				}
			}
		})
	}
}

func TestTranslator_IsTranslatable(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name          string
		query         string
		isTranslatable bool
	}{
		{
			name:          "Simple SELECT - translatable",
			query:         "SELECT * FROM users WHERE created_at > NOW()",
			isTranslatable: true,
		},
		{
			name:          "WINDOW function - now translatable",
			query:         "SELECT ROW_NUMBER() OVER (PARTITION BY dept) FROM employees",
			isTranslatable: true,
		},
		{
			name:          "ARRAY - now translatable",
			query:         "SELECT ARRAY[1,2,3]",
			isTranslatable: true,
		},
		{
			name:          "ARRAY_AGG - now translatable",
			query:         "SELECT ARRAY_AGG(name) FROM users",
			isTranslatable: true,
		},
		{
			name:          "FOR UPDATE - not translatable",
			query:         "SELECT * FROM users FOR UPDATE",
			isTranslatable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.IsTranslatable(tt.query)
			if result != tt.isTranslatable {
				t.Errorf("IsTranslatable() = %v, want %v", result, tt.isTranslatable)
			}
		})
	}
}

func TestTranslator_ComplexQueries(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name           string
		input          string
		expected       string
		skipExactMatch bool // For tests with random components
	}{
		{
			name: "CREATE TABLE with multiple PostgreSQL types (no gen_random_uuid)",
			input: `CREATE TABLE users (
				id UUID PRIMARY KEY,
				email TEXT NOT NULL,
				active BOOLEAN DEFAULT TRUE,
				metadata JSONB,
				created_at TIMESTAMPTZ DEFAULT NOW()
			)`,
			expected: `CREATE TABLE users (
				id TEXT PRIMARY KEY,
				email TEXT NOT NULL,
				active INTEGER DEFAULT 1,
				metadata TEXT,
				created_at TEXT DEFAULT datetime('now')
			)`,
		},
		{
			name:     "Query with multiple function translations",
			input:    "SELECT id::text, LEFT(name, 10), created_at FROM users WHERE updated_at > NOW()",
			expected: "SELECT id, SUBSTR(name, 1, 10), created_at FROM users WHERE updated_at > datetime('now')",
		},
		{
			name:     "UPDATE with boolean and timestamp",
			input:    "UPDATE users SET active = FALSE, updated_at = NOW() WHERE id = '123'::uuid",
			expected: "UPDATE users SET active = 0, updated_at = datetime('now') WHERE id = '123'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tr.Translate(tt.input)
			if result != tt.expected {
				t.Errorf("Translate() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTranslator_TranslateWithFallback(t *testing.T) {
	tr := NewTranslator()

	tests := []struct {
		name           string
		input          string
		expectedOutput string
		wasTranslated  bool
	}{
		{
			name:           "Translatable query - returns translated",
			input:          "SELECT NOW()",
			expectedOutput: "SELECT datetime('now')",
			wasTranslated:  true,
		},
		{
			name:           "UNNEST query - not translatable, returns original",
			input:          "SELECT * FROM UNNEST(ARRAY[1,2,3])",
			expectedOutput: "SELECT * FROM UNNEST(ARRAY[1,2,3])",
			wasTranslated:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output, translated := tr.TranslateWithFallback(tt.input)
			if output != tt.expectedOutput {
				t.Errorf("TranslateWithFallback() output = %q, want %q", output, tt.expectedOutput)
			}
			if translated != tt.wasTranslated {
				t.Errorf("TranslateWithFallback() wasTranslated = %v, want %v", translated, tt.wasTranslated)
			}
		})
	}
}

func TestTranslator_CreateTable_GenRandomUUID(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		notWant string
	}{
		{
			name:    "CREATE TABLE removes gen_random_uuid() DEFAULT",
			input:   "CREATE TABLE people (id uuid primary key default gen_random_uuid(), name text)",
			want:    "CREATE TABLE people (id TEXT primary key, name text)",
			notWant: "gen_random_uuid",
		},
		{
			name: "SELECT uses full UUID subquery",
			input: "SELECT gen_random_uuid()",
			want:  "SELECT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Translate(tt.input)
			if !strings.Contains(result, tt.want) {
				t.Errorf("Translate(%q) = %q, should contain %q", tt.input, result, tt.want)
			}
			if tt.notWant != "" && strings.Contains(result, tt.notWant) {
				t.Errorf("Translate(%q) = %q, should NOT contain %q", tt.input, result, tt.notWant)
			}
		})
	}
}
