package fts

import (
	"testing"
)

func TestParseQueryType(t *testing.T) {
	tests := []struct {
		input       string
		wantType    QueryType
		wantQuery   string
		shouldError bool
	}{
		{"fts.hello world", QueryTypeFTS, "hello world", false},
		{"plfts.fat cat", QueryTypePlain, "fat cat", false},
		{"phfts.exact phrase", QueryTypePhrase, "exact phrase", false},
		{"wfts.fat or cat", QueryTypeWebsearch, "fat or cat", false},
		{"eq.value", "", "", true},    // Not an FTS operator
		{"invalid", "", "", true},     // Missing dot
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			qType, query, err := ParseQueryType(tc.input)
			if tc.shouldError {
				if err == nil {
					t.Errorf("expected error for %q", tc.input)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if qType != tc.wantType {
				t.Errorf("type: got %q, want %q", qType, tc.wantType)
			}
			if query != tc.wantQuery {
				t.Errorf("query: got %q, want %q", query, tc.wantQuery)
			}
		})
	}
}

func TestPlainToFTS5(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", "hello AND world"},
		{"single", "single"},
		{"  spaces  between  ", "spaces AND between"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := plainToFTS5(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPhraseToFTS5(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello world", `"hello world"`},
		{"single", `"single"`},
		{"  spaces  ", `"spaces"`},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := phraseToFTS5(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWebsearchToFTS5(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// OR handling (case insensitive)
		{"cat or dog", "cat OR dog"},
		{"cat OR dog", "cat OR dog"},
		// Negation
		{"-cat", "NOT cat"},
		{"dog -cat", "dog NOT cat"},
		// Quoted phrases
		{`"fat cat"`, `"fat cat"`},
		{`"fat cat" dog`, `"fat cat" dog`},
		// Combined
		{`"fat cat" or dog -mouse`, `"fat cat" OR dog NOT mouse`},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := websearchToFTS5(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFTSToFTS5(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// Basic operators
		{"'cat' & 'dog'", "cat AND dog"},
		{"'cat' | 'dog'", "cat OR dog"},
		{"!'cat'", "NOT cat"},
		// Prefix matching
		{"'cat':*", "cat*"},
		// Combined
		{"'fat' & 'cat' | 'dog'", "fat AND cat OR dog"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ftsToFTS5(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestToFTS5Query(t *testing.T) {
	tests := []struct {
		queryType QueryType
		input     string
		want      string
	}{
		{QueryTypePlain, "fat cat", "fat AND cat"},
		{QueryTypePhrase, "fat cat", `"fat cat"`},
		{QueryTypeWebsearch, "fat or cat", "fat OR cat"},
		{QueryTypeFTS, "'fat' & 'cat'", "fat AND cat"},
	}

	for _, tc := range tests {
		name := string(tc.queryType) + ":" + tc.input
		t.Run(name, func(t *testing.T) {
			got := ToFTS5Query(tc.queryType, tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSplitPreservingQuotes(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"hello world", []string{"hello", "world"}},
		{`"hello world"`, []string{`"hello world"`}},
		{`"hello world" foo`, []string{`"hello world"`, "foo"}},
		{`foo "hello world" bar`, []string{"foo", `"hello world"`, "bar"}},
		{"", nil},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := splitPreservingQuotes(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("length: got %d, want %d", len(got), len(tc.want))
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestBuildMatchCondition(t *testing.T) {
	cond := BuildMatchCondition("posts_fts_search", "test query")
	expected := `rowid IN (SELECT rowid FROM "posts_fts_search" WHERE "posts_fts_search" MATCH ?)`
	if cond != expected {
		t.Errorf("got %q, want %q", cond, expected)
	}
}
