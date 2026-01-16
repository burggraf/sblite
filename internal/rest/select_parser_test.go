// internal/rest/select_parser_test.go
package rest

import (
	"testing"
)

func TestParseSelectEmpty(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"star", "*"},
		{"star with spaces", "  *  "},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := ParseSelectString(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(p.Columns) != 1 {
				t.Fatalf("expected 1 column, got %d", len(p.Columns))
			}
			if p.Columns[0].Name != "*" {
				t.Errorf("expected column name '*', got '%s'", p.Columns[0].Name)
			}
		})
	}
}

func TestParseSelectSimple(t *testing.T) {
	p, err := ParseSelectString("id, name, email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(p.Columns))
	}

	expected := []string{"id", "name", "email"}
	for i, col := range p.Columns {
		if col.Name != expected[i] {
			t.Errorf("column %d: expected name '%s', got '%s'", i, expected[i], col.Name)
		}
		if col.Alias != "" {
			t.Errorf("column %d: expected no alias, got '%s'", i, col.Alias)
		}
		if col.Relation != nil {
			t.Errorf("column %d: expected no relation, got %+v", i, col.Relation)
		}
	}
}

func TestParseSelectNoSpaces(t *testing.T) {
	p, err := ParseSelectString("id,name,email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(p.Columns))
	}

	expected := []string{"id", "name", "email"}
	for i, col := range p.Columns {
		if col.Name != expected[i] {
			t.Errorf("column %d: expected name '%s', got '%s'", i, expected[i], col.Name)
		}
	}
}

func TestParseSelectWithAlias(t *testing.T) {
	p, err := ParseSelectString("userId:id, userName:name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	tests := []struct {
		alias string
		name  string
	}{
		{"userId", "id"},
		{"userName", "name"},
	}

	for i, tt := range tests {
		if p.Columns[i].Alias != tt.alias {
			t.Errorf("column %d: expected alias '%s', got '%s'", i, tt.alias, p.Columns[i].Alias)
		}
		if p.Columns[i].Name != tt.name {
			t.Errorf("column %d: expected name '%s', got '%s'", i, tt.name, p.Columns[i].Name)
		}
	}
}

func TestParseSelectWithRelation(t *testing.T) {
	p, err := ParseSelectString("id, name, country(id, name)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(p.Columns))
	}

	// Check first two columns are simple
	if p.Columns[0].Name != "id" {
		t.Errorf("column 0: expected 'id', got '%s'", p.Columns[0].Name)
	}
	if p.Columns[1].Name != "name" {
		t.Errorf("column 1: expected 'name', got '%s'", p.Columns[1].Name)
	}

	// Check relation
	rel := p.Columns[2].Relation
	if rel == nil {
		t.Fatal("column 2: expected relation, got nil")
	}
	if rel.Name != "country" {
		t.Errorf("relation name: expected 'country', got '%s'", rel.Name)
	}
	if rel.Inner {
		t.Error("relation should not be inner join")
	}
	if len(rel.Columns) != 2 {
		t.Fatalf("relation columns: expected 2, got %d", len(rel.Columns))
	}
	if rel.Columns[0].Name != "id" {
		t.Errorf("relation column 0: expected 'id', got '%s'", rel.Columns[0].Name)
	}
	if rel.Columns[1].Name != "name" {
		t.Errorf("relation column 1: expected 'name', got '%s'", rel.Columns[1].Name)
	}
}

func TestParseSelectWithAliasedRelation(t *testing.T) {
	p, err := ParseSelectString("from:sender_id(name), to:receiver_id(name)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	// First aliased relation
	if p.Columns[0].Alias != "from" {
		t.Errorf("column 0 alias: expected 'from', got '%s'", p.Columns[0].Alias)
	}
	if p.Columns[0].Relation == nil {
		t.Fatal("column 0: expected relation, got nil")
	}
	if p.Columns[0].Relation.Name != "sender_id" {
		t.Errorf("column 0 relation name: expected 'sender_id', got '%s'", p.Columns[0].Relation.Name)
	}
	if p.Columns[0].Relation.Alias != "from" {
		t.Errorf("column 0 relation alias: expected 'from', got '%s'", p.Columns[0].Relation.Alias)
	}

	// Second aliased relation
	if p.Columns[1].Alias != "to" {
		t.Errorf("column 1 alias: expected 'to', got '%s'", p.Columns[1].Alias)
	}
	if p.Columns[1].Relation == nil {
		t.Fatal("column 1: expected relation, got nil")
	}
	if p.Columns[1].Relation.Name != "receiver_id" {
		t.Errorf("column 1 relation name: expected 'receiver_id', got '%s'", p.Columns[1].Relation.Name)
	}
	if p.Columns[1].Relation.Alias != "to" {
		t.Errorf("column 1 relation alias: expected 'to', got '%s'", p.Columns[1].Relation.Alias)
	}
}

func TestParseSelectInnerJoin(t *testing.T) {
	p, err := ParseSelectString("name, country!inner(name)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	rel := p.Columns[1].Relation
	if rel == nil {
		t.Fatal("column 1: expected relation, got nil")
	}
	if !rel.Inner {
		t.Error("relation should be inner join")
	}
	if rel.Name != "country" {
		t.Errorf("relation name: expected 'country', got '%s'", rel.Name)
	}
}

func TestParseSelectAliasedInnerJoin(t *testing.T) {
	p, err := ParseSelectString("name, origin:country!inner(name)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	rel := p.Columns[1].Relation
	if rel == nil {
		t.Fatal("column 1: expected relation, got nil")
	}
	if !rel.Inner {
		t.Error("relation should be inner join")
	}
	if rel.Name != "country" {
		t.Errorf("relation name: expected 'country', got '%s'", rel.Name)
	}
	if rel.Alias != "origin" {
		t.Errorf("relation alias: expected 'origin', got '%s'", rel.Alias)
	}
}

func TestParseSelectTwoLevel(t *testing.T) {
	p, err := ParseSelectString("name, country(name, continent(name))")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	// Check outer relation
	rel := p.Columns[1].Relation
	if rel == nil {
		t.Fatal("column 1: expected relation, got nil")
	}
	if rel.Name != "country" {
		t.Errorf("relation name: expected 'country', got '%s'", rel.Name)
	}
	if len(rel.Columns) != 2 {
		t.Fatalf("relation columns: expected 2, got %d", len(rel.Columns))
	}

	// Check nested relation
	nestedRel := rel.Columns[1].Relation
	if nestedRel == nil {
		t.Fatal("nested column: expected relation, got nil")
	}
	if nestedRel.Name != "continent" {
		t.Errorf("nested relation name: expected 'continent', got '%s'", nestedRel.Name)
	}
	if len(nestedRel.Columns) != 1 {
		t.Fatalf("nested relation columns: expected 1, got %d", len(nestedRel.Columns))
	}
	if nestedRel.Columns[0].Name != "name" {
		t.Errorf("nested relation column: expected 'name', got '%s'", nestedRel.Columns[0].Name)
	}
}

func TestParseSelectThreeLevel(t *testing.T) {
	p, err := ParseSelectString("id, author(name, country(name, continent(name)))")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	// Level 1: author
	author := p.Columns[1].Relation
	if author == nil || author.Name != "author" {
		t.Fatal("expected author relation")
	}

	// Level 2: country
	country := author.Columns[1].Relation
	if country == nil || country.Name != "country" {
		t.Fatal("expected country relation")
	}

	// Level 3: continent
	continent := country.Columns[1].Relation
	if continent == nil || continent.Name != "continent" {
		t.Fatal("expected continent relation")
	}
}

func TestParseSelectMultipleRelations(t *testing.T) {
	p, err := ParseSelectString("id, author(name), category(title)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(p.Columns))
	}

	// Check author relation
	if p.Columns[1].Relation == nil || p.Columns[1].Relation.Name != "author" {
		t.Error("expected author relation at index 1")
	}

	// Check category relation
	if p.Columns[2].Relation == nil || p.Columns[2].Relation.Name != "category" {
		t.Error("expected category relation at index 2")
	}
}

func TestParseSelectRelationWithStar(t *testing.T) {
	p, err := ParseSelectString("id, country(*)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(p.Columns) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(p.Columns))
	}

	rel := p.Columns[1].Relation
	if rel == nil {
		t.Fatal("expected relation, got nil")
	}
	if len(rel.Columns) != 1 {
		t.Fatalf("expected 1 column in relation, got %d", len(rel.Columns))
	}
	if rel.Columns[0].Name != "*" {
		t.Errorf("expected '*' column, got '%s'", rel.Columns[0].Name)
	}
}

func TestParseSelectInvalidFormats(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty alias", ":column"},
		{"empty column after alias", "alias:"},
		{"empty relation alias", ":table(col)"},
		{"empty relation name", "alias:(col)"},
		{"missing closing paren", "table(col"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSelectString(tt.input)
			if err == nil {
				t.Errorf("expected error for input '%s', got nil", tt.input)
			}
		})
	}
}

func TestSplitRespectingParens(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "simple",
			input:    "a, b, c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "with nested parens",
			input:    "a, b(c, d), e",
			expected: []string{"a", "b(c, d)", "e"},
		},
		{
			name:     "deep nested parens",
			input:    "a, b(c, d(e, f)), g",
			expected: []string{"a", "b(c, d(e, f))", "g"},
		},
		{
			name:     "no spaces",
			input:    "a,b,c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "single item",
			input:    "a",
			expected: []string{"a"},
		},
		{
			name:     "multiple nested",
			input:    "a(b, c), d(e, f)",
			expected: []string{"a(b, c)", "d(e, f)"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitRespectingParens(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d parts, got %d: %v", len(tt.expected), len(result), result)
			}
			for i, expected := range tt.expected {
				if result[i] != expected {
					t.Errorf("part %d: expected '%s', got '%s'", i, expected, result[i])
				}
			}
		})
	}
}

func TestParsedSelectHelperMethods(t *testing.T) {
	p, err := ParseSelectString("id, name, country(name), category(title)")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Test HasRelations
	if !p.HasRelations() {
		t.Error("expected HasRelations() to return true")
	}

	// Test GetBaseColumns
	baseCols := p.GetBaseColumns()
	if len(baseCols) != 2 {
		t.Fatalf("expected 2 base columns, got %d", len(baseCols))
	}
	if baseCols[0].Name != "id" || baseCols[1].Name != "name" {
		t.Error("unexpected base columns")
	}

	// Test GetRelations
	rels := p.GetRelations()
	if len(rels) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(rels))
	}
	if rels[0].Relation.Name != "country" || rels[1].Relation.Name != "category" {
		t.Error("unexpected relations")
	}

	// Test ToColumnNames
	names := p.ToColumnNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 column names, got %d", len(names))
	}
	if names[0] != "id" || names[1] != "name" {
		t.Error("unexpected column names")
	}
}

func TestParsedSelectNoRelations(t *testing.T) {
	p, err := ParseSelectString("id, name, email")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if p.HasRelations() {
		t.Error("expected HasRelations() to return false")
	}

	baseCols := p.GetBaseColumns()
	if len(baseCols) != 3 {
		t.Errorf("expected 3 base columns, got %d", len(baseCols))
	}

	rels := p.GetRelations()
	if len(rels) != 0 {
		t.Errorf("expected 0 relations, got %d", len(rels))
	}
}

func TestParseSelectComplexRealWorld(t *testing.T) {
	// Test a complex real-world example
	input := "id, title, created_at, author:user_id(id, name, avatar_url), comments(id, body, author:user_id(name))"

	p, err := ParseSelectString(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(p.Columns) != 5 {
		t.Fatalf("expected 5 columns, got %d", len(p.Columns))
	}

	// Check basic columns
	if p.Columns[0].Name != "id" {
		t.Errorf("expected 'id', got '%s'", p.Columns[0].Name)
	}
	if p.Columns[1].Name != "title" {
		t.Errorf("expected 'title', got '%s'", p.Columns[1].Name)
	}
	if p.Columns[2].Name != "created_at" {
		t.Errorf("expected 'created_at', got '%s'", p.Columns[2].Name)
	}

	// Check author relation
	author := p.Columns[3].Relation
	if author == nil {
		t.Fatal("expected author relation")
	}
	if author.Name != "user_id" {
		t.Errorf("expected relation name 'user_id', got '%s'", author.Name)
	}
	if author.Alias != "author" {
		t.Errorf("expected alias 'author', got '%s'", author.Alias)
	}
	if len(author.Columns) != 3 {
		t.Errorf("expected 3 columns in author, got %d", len(author.Columns))
	}

	// Check comments relation
	comments := p.Columns[4].Relation
	if comments == nil {
		t.Fatal("expected comments relation")
	}
	if comments.Name != "comments" {
		t.Errorf("expected relation name 'comments', got '%s'", comments.Name)
	}

	// Check nested author in comments
	nestedAuthor := comments.Columns[2].Relation
	if nestedAuthor == nil {
		t.Fatal("expected nested author relation in comments")
	}
	if nestedAuthor.Name != "user_id" {
		t.Errorf("expected nested relation name 'user_id', got '%s'", nestedAuthor.Name)
	}
	if nestedAuthor.Alias != "author" {
		t.Errorf("expected nested alias 'author', got '%s'", nestedAuthor.Alias)
	}
}
