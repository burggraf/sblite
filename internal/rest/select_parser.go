// internal/rest/select_parser.go
package rest

import (
	"fmt"
	"strings"
)

// SelectColumn represents a column in a parsed select statement.
// It can be a simple column, an aliased column, or a relation.
type SelectColumn struct {
	Name     string          // Column name or "*"
	Alias    string          // Optional alias (customName:columnName)
	Relation *SelectRelation // If this is a relation
}

// SelectRelation represents a nested relation in a select statement.
// Relations are specified as "table(columns)" or "alias:table!inner(columns)".
type SelectRelation struct {
	Name    string         // Relation/table name
	Alias   string         // Optional alias
	Inner   bool           // !inner join modifier
	Columns []SelectColumn // Nested columns
}

// ParsedSelect represents a fully parsed select string.
type ParsedSelect struct {
	Columns []SelectColumn
}

// ParseSelectString parses a PostgREST-style select string into a structured format.
// It handles:
// - Simple columns: "id, name, email"
// - Aliases: "customName:actualColumn"
// - Relations: "table(col1, col2)"
// - Aliased relations: "alias:table(col1, col2)"
// - Inner joins: "table!inner(col1, col2)"
// - Nested relations: "table(col1, nested(col2, col3))"
func ParseSelectString(input string) (*ParsedSelect, error) {
	input = strings.TrimSpace(input)

	if input == "" || input == "*" {
		return &ParsedSelect{Columns: []SelectColumn{{Name: "*"}}}, nil
	}

	// Split by comma, respecting parentheses
	parts := splitRespectingParens(input)

	var columns []SelectColumn
	for _, part := range parts {
		col, err := parseSelectPart(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}

	return &ParsedSelect{Columns: columns}, nil
}

// parseSelectPart parses a single part of a select string.
// It could be a simple column, aliased column, or a relation.
func parseSelectPart(part string) (SelectColumn, error) {
	if part == "" {
		return SelectColumn{}, fmt.Errorf("empty select part")
	}

	// Check for relation: "table(cols)" or "alias:table(cols)" or "table!inner(cols)"
	if idx := strings.Index(part, "("); idx != -1 {
		return parseRelationSelect(part)
	}

	// Check for alias: "alias:column"
	if idx := strings.Index(part, ":"); idx != -1 {
		alias := part[:idx]
		name := part[idx+1:]
		if alias == "" || name == "" {
			return SelectColumn{}, fmt.Errorf("invalid alias format: %s", part)
		}
		return SelectColumn{
			Alias: alias,
			Name:  name,
		}, nil
	}

	// Simple column
	return SelectColumn{Name: part}, nil
}

// parseRelationSelect parses a relation select part.
// Formats:
// - "table(cols)"
// - "alias:table(cols)"
// - "table!inner(cols)"
// - "alias:table!inner(cols)"
func parseRelationSelect(part string) (SelectColumn, error) {
	parenIdx := strings.Index(part, "(")
	if parenIdx == -1 {
		return SelectColumn{}, fmt.Errorf("invalid relation format: %s", part)
	}

	// Ensure closing parenthesis exists
	if !strings.HasSuffix(part, ")") {
		return SelectColumn{}, fmt.Errorf("missing closing parenthesis: %s", part)
	}

	prefix := part[:parenIdx]

	// Check for !inner modifier
	inner := strings.Contains(prefix, "!inner")
	prefix = strings.Replace(prefix, "!inner", "", 1)

	var alias, name string
	if colonIdx := strings.Index(prefix, ":"); colonIdx != -1 {
		alias = prefix[:colonIdx]
		name = prefix[colonIdx+1:]
		if alias == "" || name == "" {
			return SelectColumn{}, fmt.Errorf("invalid relation alias format: %s", part)
		}
	} else {
		name = prefix
	}

	if name == "" {
		return SelectColumn{}, fmt.Errorf("empty relation name: %s", part)
	}

	// Parse nested columns
	nestedStr := part[parenIdx+1 : len(part)-1]
	nested, err := ParseSelectString(nestedStr)
	if err != nil {
		return SelectColumn{}, fmt.Errorf("error parsing nested select for %s: %w", name, err)
	}

	return SelectColumn{
		Alias: alias,
		Relation: &SelectRelation{
			Name:    name,
			Alias:   alias,
			Inner:   inner,
			Columns: nested.Columns,
		},
	}, nil
}

// splitRespectingParens splits a string by commas while respecting nested parentheses.
// For example: "a, b(c, d), e" -> ["a", "b(c, d)", "e"]
func splitRespectingParens(input string) []string {
	var parts []string
	var current strings.Builder
	depth := 0

	for _, char := range input {
		switch char {
		case '(':
			depth++
			current.WriteRune(char)
		case ')':
			depth--
			current.WriteRune(char)
		case ',':
			if depth == 0 {
				if current.Len() > 0 {
					parts = append(parts, strings.TrimSpace(current.String()))
					current.Reset()
				}
			} else {
				current.WriteRune(char)
			}
		default:
			current.WriteRune(char)
		}
	}

	// Don't forget the last part
	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}

	return parts
}

// HasRelations returns true if the parsed select contains any relations.
func (ps *ParsedSelect) HasRelations() bool {
	for _, col := range ps.Columns {
		if col.Relation != nil {
			return true
		}
	}
	return false
}

// GetBaseColumns returns only the non-relation columns.
func (ps *ParsedSelect) GetBaseColumns() []SelectColumn {
	var cols []SelectColumn
	for _, col := range ps.Columns {
		if col.Relation == nil {
			cols = append(cols, col)
		}
	}
	return cols
}

// GetRelations returns only the relation columns.
func (ps *ParsedSelect) GetRelations() []SelectColumn {
	var cols []SelectColumn
	for _, col := range ps.Columns {
		if col.Relation != nil {
			cols = append(cols, col)
		}
	}
	return cols
}

// ToColumnNames returns a slice of column names for SQL select.
// Relations are not included; use GetRelations() to handle them separately.
func (ps *ParsedSelect) ToColumnNames() []string {
	var names []string
	for _, col := range ps.Columns {
		if col.Relation == nil {
			names = append(names, col.Name)
		}
	}
	return names
}
