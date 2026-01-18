package fts

import (
	"fmt"
	"regexp"
	"strings"
)

// QueryType represents the type of full-text search query.
type QueryType string

const (
	// QueryTypeFTS is the default FTS query type (uses to_tsquery in PostgreSQL).
	QueryTypeFTS QueryType = "fts"
	// QueryTypePlain is plain text search (uses plainto_tsquery in PostgreSQL).
	QueryTypePlain QueryType = "plfts"
	// QueryTypePhrase is phrase search (uses phraseto_tsquery in PostgreSQL).
	QueryTypePhrase QueryType = "phfts"
	// QueryTypeWebsearch is websearch syntax (uses websearch_to_tsquery in PostgreSQL).
	QueryTypeWebsearch QueryType = "wfts"
)

// ParseQueryType extracts the query type from an operator string.
// Returns the query type and the search terms.
func ParseQueryType(op string) (QueryType, string, error) {
	// Check for prefixed query types
	switch {
	case strings.HasPrefix(op, "plfts."):
		return QueryTypePlain, strings.TrimPrefix(op, "plfts."), nil
	case strings.HasPrefix(op, "phfts."):
		return QueryTypePhrase, strings.TrimPrefix(op, "phfts."), nil
	case strings.HasPrefix(op, "wfts."):
		return QueryTypeWebsearch, strings.TrimPrefix(op, "wfts."), nil
	case strings.HasPrefix(op, "fts."):
		return QueryTypeFTS, strings.TrimPrefix(op, "fts."), nil
	default:
		return "", "", fmt.Errorf("invalid FTS operator: %s", op)
	}
}

// ToFTS5Query converts a search query to FTS5 MATCH syntax.
func ToFTS5Query(queryType QueryType, query string) string {
	switch queryType {
	case QueryTypePlain:
		return plainToFTS5(query)
	case QueryTypePhrase:
		return phraseToFTS5(query)
	case QueryTypeWebsearch:
		return websearchToFTS5(query)
	case QueryTypeFTS:
		return ftsToFTS5(query)
	default:
		return query
	}
}

// plainToFTS5 converts plain text to FTS5 query.
// All terms are ANDed together.
func plainToFTS5(query string) string {
	// Split into words and join with AND
	words := strings.Fields(query)
	if len(words) == 0 {
		return ""
	}
	// Escape special characters
	for i, w := range words {
		words[i] = escapeWord(w)
	}
	return strings.Join(words, " AND ")
}

// phraseToFTS5 converts phrase text to FTS5 query.
// The entire query is treated as a phrase (exact sequence).
func phraseToFTS5(query string) string {
	// Wrap in quotes for phrase matching
	return fmt.Sprintf(`"%s"`, strings.TrimSpace(query))
}

// websearchToFTS5 converts websearch syntax to FTS5 query.
// Supports: "quoted phrases", OR, -, and implicit AND.
func websearchToFTS5(query string) string {
	// This is a simplified websearch parser
	// Full implementation would handle more complex cases

	result := query

	// Handle quoted phrases (already FTS5 compatible)
	// No change needed for "quoted phrases"

	// Handle OR (case insensitive) - websearch uses lowercase 'or'
	result = regexp.MustCompile(`(?i)\bor\b`).ReplaceAllString(result, " OR ")

	// Handle negation: -word becomes NOT word
	result = regexp.MustCompile(`-(\w+)`).ReplaceAllString(result, "NOT $1")

	// Split remaining terms and handle them
	// Keep quoted strings intact
	parts := splitPreservingQuotes(result)
	var processed []string

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "OR" || part == "NOT" {
			processed = append(processed, part)
			continue
		}
		if strings.HasPrefix(part, `"`) && strings.HasSuffix(part, `"`) {
			// Quoted phrase - keep as is
			processed = append(processed, part)
		} else if strings.HasPrefix(part, "NOT ") {
			// Already processed negation
			processed = append(processed, part)
		} else {
			// Regular word
			processed = append(processed, escapeWord(part))
		}
	}

	// Join with spaces - FTS5 will treat adjacent terms as AND
	result = strings.Join(processed, " ")

	// Clean up multiple spaces
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")

	return strings.TrimSpace(result)
}

// ftsToFTS5 converts standard FTS query (PostgreSQL tsquery format) to FTS5.
// PostgreSQL format: 'term1' & 'term2' | 'term3'
func ftsToFTS5(query string) string {
	// Remove single quotes around terms
	result := regexp.MustCompile(`'([^']*)'`).ReplaceAllString(query, "$1")

	// Convert & to AND
	result = strings.ReplaceAll(result, "&", " AND ")

	// Convert | to OR
	result = strings.ReplaceAll(result, "|", " OR ")

	// Convert ! to NOT
	result = strings.ReplaceAll(result, "!", " NOT ")

	// Handle :* prefix matching - convert to FTS5 prefix syntax
	result = regexp.MustCompile(`:(\*)`).ReplaceAllString(result, "*")

	// Clean up spaces
	result = regexp.MustCompile(`\s+`).ReplaceAllString(result, " ")

	return strings.TrimSpace(result)
}

// escapeWord escapes special FTS5 characters in a word.
func escapeWord(word string) string {
	// FTS5 special characters that need escaping
	// For simple words, we just need to handle basic cases
	word = strings.TrimSpace(word)

	// Remove any stray quotes
	word = strings.Trim(word, `"'`)

	// If the word contains spaces, quote it
	if strings.Contains(word, " ") {
		return fmt.Sprintf(`"%s"`, word)
	}

	return word
}

// splitPreservingQuotes splits a string by spaces but keeps quoted strings together.
func splitPreservingQuotes(s string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for _, r := range s {
		switch r {
		case '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case ' ':
			if inQuotes {
				current.WriteRune(r)
			} else {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// BuildMatchCondition builds a SQL condition for FTS matching.
// Returns a condition that can be used in a WHERE clause.
func BuildMatchCondition(ftsTable, query string) string {
	return fmt.Sprintf(`rowid IN (SELECT rowid FROM %q WHERE %q MATCH ?)`, ftsTable, ftsTable)
}

// BuildMatchConditionWithRank builds a SQL condition for FTS matching with ranking.
// Returns a subquery that includes the rank for ordering.
func BuildMatchConditionWithRank(tableName, ftsTable, pkColumn, query string) string {
	return fmt.Sprintf(
		`%q IN (SELECT rowid FROM %q WHERE %q MATCH ? ORDER BY rank)`,
		pkColumn, ftsTable, ftsTable,
	)
}

// ConvertQuery converts a search query to FTS5 MATCH syntax using the given type.
// Accepts simplified type names: "plain", "phrase", "websearch", "fts"
func ConvertQuery(query, queryType string) (string, error) {
	if query == "" {
		return "", fmt.Errorf("query cannot be empty")
	}

	var qt QueryType
	switch strings.ToLower(queryType) {
	case "plain", "plfts":
		qt = QueryTypePlain
	case "phrase", "phfts":
		qt = QueryTypePhrase
	case "websearch", "wfts":
		qt = QueryTypeWebsearch
	case "fts", "":
		qt = QueryTypeFTS
	default:
		return "", fmt.Errorf("invalid query type: %s (valid: plain, phrase, websearch, fts)", queryType)
	}

	return ToFTS5Query(qt, query), nil
}
