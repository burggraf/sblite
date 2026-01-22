package vector

import (
	"testing"
)

func TestParseSearchParams(t *testing.T) {
	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
	}{
		{
			name: "valid minimal params",
			args: map[string]any{
				"table_name":       "documents",
				"embedding_column": "embedding",
				"query_embedding":  []float64{0.1, 0.2, 0.3},
			},
		},
		{
			name: "valid full params",
			args: map[string]any{
				"table_name":       "documents",
				"embedding_column": "embedding",
				"query_embedding":  []float64{0.1, 0.2, 0.3},
				"match_count":      float64(20),
				"match_threshold":  float64(0.7),
				"metric":           "cosine",
				"select_columns":   []any{"id", "content"},
			},
		},
		{
			name: "query_embedding as interface slice",
			args: map[string]any{
				"table_name":       "documents",
				"embedding_column": "embedding",
				"query_embedding":  []interface{}{0.1, 0.2, 0.3},
			},
		},
		{
			name: "missing table_name",
			args: map[string]any{
				"embedding_column": "embedding",
				"query_embedding":  []float64{0.1, 0.2, 0.3},
			},
			wantErr: true,
		},
		{
			name: "missing embedding_column",
			args: map[string]any{
				"table_name":      "documents",
				"query_embedding": []float64{0.1, 0.2, 0.3},
			},
			wantErr: true,
		},
		{
			name: "missing query_embedding",
			args: map[string]any{
				"table_name":       "documents",
				"embedding_column": "embedding",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := ParseSearchParams(tt.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSearchParams() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if params.TableName == "" {
					t.Error("ParseSearchParams() TableName is empty")
				}
				if params.EmbeddingColumn == "" {
					t.Error("ParseSearchParams() EmbeddingColumn is empty")
				}
				if len(params.QueryEmbedding) == 0 {
					t.Error("ParseSearchParams() QueryEmbedding is empty")
				}
			}
		})
	}
}

func TestFormatSearchResults(t *testing.T) {
	results := []SearchResult{
		{
			Row:        map[string]any{"id": "1", "content": "hello"},
			Similarity: 0.95,
		},
		{
			Row:        map[string]any{"id": "2", "content": "world"},
			Similarity: 0.85,
		},
	}

	formatted := FormatSearchResults(results)

	if len(formatted) != 2 {
		t.Errorf("FormatSearchResults() returned %d results, want 2", len(formatted))
	}

	// Check first result
	if formatted[0]["id"] != "1" {
		t.Errorf("FormatSearchResults()[0][id] = %v, want 1", formatted[0]["id"])
	}
	if formatted[0]["similarity"] != 0.95 {
		t.Errorf("FormatSearchResults()[0][similarity] = %v, want 0.95", formatted[0]["similarity"])
	}

	// Check second result
	if formatted[1]["content"] != "world" {
		t.Errorf("FormatSearchResults()[1][content] = %v, want world", formatted[1]["content"])
	}
}

func TestIsBuiltinFunction(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"vector_search", true},
		{"other_function", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsBuiltinFunction(tt.name); got != tt.want {
				t.Errorf("IsBuiltinFunction(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
