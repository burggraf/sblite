package pgtranslate

import (
	"regexp"
	"strings"
)

// Translator converts PostgreSQL SQL syntax to SQLite-compatible syntax.
type Translator struct {
	rules []Rule
}

// Rule represents a translation rule.
type Rule interface {
	Apply(query string) string
}

// RegexRule uses regex replacement.
type RegexRule struct {
	pattern     *regexp.Regexp
	replacement string
}

func (r *RegexRule) Apply(query string) string {
	return r.pattern.ReplaceAllString(query, r.replacement)
}

// FunctionRule translates PostgreSQL functions to SQLite equivalents.
type FunctionRule struct {
	pgFunc     string
	sqliteFunc string
}

func (r *FunctionRule) Apply(query string) string {
	// Case-insensitive function name replacement
	re := regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(r.pgFunc) + `\b`)
	return re.ReplaceAllString(query, r.sqliteFunc)
}

// NewTranslator creates a new translator with default rules.
func NewTranslator() *Translator {
	return &Translator{
		rules: defaultRules(),
	}
}

// Translate converts a PostgreSQL query to SQLite syntax.
// Returns the translated query. If translation fails for non-critical
// syntax, returns the original query (best effort).
func (t *Translator) Translate(query string) string {
	result := query
	for _, rule := range t.rules {
		result = rule.Apply(result)
	}
	return result
}

func defaultRules() []Rule {
	return []Rule{
		// Date/Time Functions
		&FunctionRule{"NOW()", "datetime('now')"},
		&FunctionRule{"CURRENT_TIMESTAMP", "datetime('now')"},
		&FunctionRule{"CURRENT_DATE", "date('now')"},
		&FunctionRule{"CURRENT_TIME", "time('now')"},

		// String Functions
		&FunctionRule{"LENGTH(", "length("},
		&FunctionRule{"LOWER(", "lower("},
		&FunctionRule{"UPPER(", "upper("},
		&FunctionRule{"TRIM(", "trim("},
		&FunctionRule{"LTRIM(", "ltrim("},
		&FunctionRule{"RTRIM(", "rtrim("},

		// PostgreSQL-specific string functions to SQLite equivalents
		// LEFT(str, n) -> SUBSTR(str, 1, n)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)LEFT\s*\(\s*([^,]+)\s*,\s*(\d+)\s*\)`),
			replacement: "SUBSTR($1, 1, $2)",
		},

		// RIGHT(str, n) -> SUBSTR(str, -n)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)RIGHT\s*\(\s*([^,]+)\s*,\s*(\d+)\s*\)`),
			replacement: "SUBSTR($1, -$2)",
		},

		// POSITION(substring IN string) -> INSTR(string, substring)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)POSITION\s*\(\s*([^)]+)\s+IN\s+([^)]+)\s*\)`),
			replacement: "INSTR($2, $1)",
		},

		// Type Casts - Remove PostgreSQL-specific casts
		// ::uuid -> (no-op, SQLite treats as text)
		&RegexRule{
			pattern:     regexp.MustCompile(`::uuid\b`),
			replacement: "",
		},

		// ::timestamptz -> (no-op, SQLite treats as text/datetime)
		&RegexRule{
			pattern:     regexp.MustCompile(`::timestamptz\b`),
			replacement: "",
		},

		// ::timestamp -> (no-op)
		&RegexRule{
			pattern:     regexp.MustCompile(`::timestamp\b`),
			replacement: "",
		},

		// ::integer -> (use CAST in SQLite style if needed, or remove)
		&RegexRule{
			pattern:     regexp.MustCompile(`::integer\b`),
			replacement: "",
		},

		// ::text -> (no-op)
		&RegexRule{
			pattern:     regexp.MustCompile(`::text\b`),
			replacement: "",
		},

		// Boolean literals
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bTRUE\b`),
			replacement: "1",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bFALSE\b`),
			replacement: "0",
		},

		// BOOLEAN type in CREATE TABLE -> INTEGER
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bBOOLEAN\b`),
			replacement: "INTEGER",
		},

		// UUID type -> TEXT
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bUUID\b`),
			replacement: "TEXT",
		},

		// TIMESTAMPTZ type -> TEXT (SQLite stores as ISO 8601 string)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bTIMESTAMPTZ\b`),
			replacement: "TEXT",
		},

		// JSONB type -> TEXT (SQLite JSON1 extension works with TEXT)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bJSONB\b`),
			replacement: "TEXT",
		},

		// SERIAL -> INTEGER (SQLite uses INTEGER PRIMARY KEY for autoincrement)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bSERIAL\b`),
			replacement: "INTEGER",
		},

		// BIGSERIAL -> INTEGER
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bBIGSERIAL\b`),
			replacement: "INTEGER",
		},

		// PostgreSQL functions that don't exist in SQLite
		// gen_random_uuid() -> Generate RFC 4122 compliant UUID v4
		// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
		// where 4 is the version and y is one of 8, 9, a, or b (variant bits)
		&RegexRule{
			pattern: regexp.MustCompile(`(?i)gen_random_uuid\s*\(\s*\)`),
			replacement: `(SELECT lower(
    substr(h, 1, 8) || '-' ||
    substr(h, 9, 4) || '-' ||
    '4' || substr(h, 14, 3) || '-' ||
    substr('89ab', (abs(random()) % 4) + 1, 1) || substr(h, 18, 3) || '-' ||
    substr(h, 21, 12)
  ) FROM (SELECT hex(randomblob(16)) as h))`,
		},

		// INTERVAL (approximate translation)
		// INTERVAL '1 day' -> '+1 day' (SQLite datetime modifier)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+day'`),
			replacement: "'+$1 day'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+hour'`),
			replacement: "'+$1 hour'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+minute'`),
			replacement: "'+$1 minute'",
		},

		// CONCAT function -> || operator (SQLite's concat)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)CONCAT\s*\((.*?)\)`),
			replacement: "($1)", // Then replace commas with || in a subsequent pass
		},

		// JSON operators
		// -> operator (JSON field access by text key) -> json_extract
		// Example: data->'field' -> json_extract(data, '$.field')
		&RegexRule{
			pattern:     regexp.MustCompile(`(\w+)\s*->\s*'([^']+)'`),
			replacement: "json_extract($1, '$.$2')",
		},

		// ->> operator (JSON field access returning text) -> json_extract
		&RegexRule{
			pattern:     regexp.MustCompile(`(\w+)\s*->>\s*'([^']+)'`),
			replacement: "json_extract($1, '$.$2')",
		},

		// ILIKE -> LIKE (case insensitive by default in SQLite, but be explicit)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bILIKE\b`),
			replacement: "LIKE",
		},

		// More date/time interval formats
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+days?'`),
			replacement: "'+$1 day'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+hours?'`),
			replacement: "'+$1 hour'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+minutes?'`),
			replacement: "'+$1 minute'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+seconds?'`),
			replacement: "'+$1 second'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+months?'`),
			replacement: "'+$1 month'",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)INTERVAL\s+'(\d+)\s+years?'`),
			replacement: "'+$1 year'",
		},

		// EXTRACT function -> strftime
		// EXTRACT(YEAR FROM date) -> CAST(strftime('%Y', date) AS INTEGER)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)EXTRACT\s*\(\s*YEAR\s+FROM\s+([^)]+)\)`),
			replacement: "CAST(strftime('%Y', $1) AS INTEGER)",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)EXTRACT\s*\(\s*MONTH\s+FROM\s+([^)]+)\)`),
			replacement: "CAST(strftime('%m', $1) AS INTEGER)",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)EXTRACT\s*\(\s*DAY\s+FROM\s+([^)]+)\)`),
			replacement: "CAST(strftime('%d', $1) AS INTEGER)",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)EXTRACT\s*\(\s*HOUR\s+FROM\s+([^)]+)\)`),
			replacement: "CAST(strftime('%H', $1) AS INTEGER)",
		},

		// AGE function approximation (PostgreSQL returns interval, SQLite uses julianday)
		// AGE(timestamp) -> (julianday('now') - julianday(timestamp)) || ' days'
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)AGE\s*\(\s*([^)]+)\s*\)`),
			replacement: "(julianday('now') - julianday($1))",
		},

		// COALESCE, NULLIF - these work in SQLite, keep as-is but normalize case
		// (Already supported natively, no translation needed)

		// GREATEST -> MAX, LEAST -> MIN (when used with multiple values)
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bGREATEST\s*\(`),
			replacement: "MAX(",
		},
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)\bLEAST\s*\(`),
			replacement: "MIN(",
		},

		// STRING_AGG -> GROUP_CONCAT
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)STRING_AGG\s*\(\s*([^,]+)\s*,\s*([^)]+)\s*\)`),
			replacement: "GROUP_CONCAT($1, $2)",
		},

		// RETURNING clause is supported in SQLite 3.35+ (most deployments)
		// No translation needed, but good to note

		// ON CONFLICT DO NOTHING -> OR IGNORE
		&RegexRule{
			pattern:     regexp.MustCompile(`(?i)ON\s+CONFLICT\s+DO\s+NOTHING`),
			replacement: "OR IGNORE",
		},

		// ON CONFLICT DO UPDATE -> needs more complex handling, leave for now
		// (Requires rewriting the entire INSERT statement)
	}
}

// IsTranslatable checks if a query can be safely translated.
// Returns false for queries with unsupported features.
func (t *Translator) IsTranslatable(query string) bool {
	// List of PostgreSQL features that can't be reliably translated
	unsupported := []string{
		"WINDOW",
		"OVER\\s*\\(",
		"PARTITION\\s+BY",
		"ARRAY\\[", // Array literals
		"ARRAY_AGG",
		"UNNEST",
		"LATERAL",
		"FOR\\s+UPDATE",
		"FOR\\s+SHARE",
	}

	queryUpper := strings.ToUpper(query)
	for _, feature := range unsupported {
		matched, _ := regexp.MatchString(feature, queryUpper)
		if matched {
			return false
		}
	}

	return true
}

// TranslateWithFallback translates a query, returning original if translation
// would produce unsafe results.
func (t *Translator) TranslateWithFallback(query string) (translated string, wasTranslated bool) {
	if !t.IsTranslatable(query) {
		return query, false
	}
	return t.Translate(query), true
}

// Package-level default translator instance for convenience
var defaultTranslator = NewTranslator()

// Translate is a convenience function that uses the default translator.
// Use this for simple translation needs throughout the codebase.
func Translate(query string) string {
	return defaultTranslator.Translate(query)
}

// TranslateWithFallback is a convenience function that uses the default translator.
// Returns the translated query and a boolean indicating if translation occurred.
func TranslateWithFallback(query string) (translated string, wasTranslated bool) {
	return defaultTranslator.TranslateWithFallback(query)
}

// IsTranslatable is a convenience function that uses the default translator.
func IsTranslatable(query string) bool {
	return defaultTranslator.IsTranslatable(query)
}
