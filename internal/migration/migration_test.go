// Package migration provides database migration tracking and execution for sblite.
package migration

import (
	"testing"
	"time"
)

func TestMigrationVersion(t *testing.T) {
	// Test that GenerateVersion produces expected format
	version := GenerateVersion()

	// Should be 14 digits: YYYYMMDDHHmmss
	if len(version) != 14 {
		t.Errorf("expected version length 14, got %d: %s", len(version), version)
	}

	// Should be parseable as a timestamp
	_, err := time.Parse("20060102150405", version)
	if err != nil {
		t.Errorf("version not parseable as timestamp: %v", err)
	}
}

func TestMigrationFilename(t *testing.T) {
	m := Migration{
		Version: "20260117143022",
		Name:    "create_posts",
	}

	expected := "20260117143022_create_posts.sql"
	if m.Filename() != expected {
		t.Errorf("expected %s, got %s", expected, m.Filename())
	}
}

func TestParseMigrationFilename(t *testing.T) {
	tests := []struct {
		filename string
		version  string
		name     string
		valid    bool
	}{
		{"20260117143022_create_posts.sql", "20260117143022", "create_posts", true},
		{"20260117143022_add_user_id_to_posts.sql", "20260117143022", "add_user_id_to_posts", true},
		{"invalid.sql", "", "", false},
		{"20260117143022.sql", "", "", false},
		{"not_a_migration.txt", "", "", false},
	}

	for _, tt := range tests {
		m, err := ParseFilename(tt.filename)
		if tt.valid {
			if err != nil {
				t.Errorf("ParseFilename(%q) unexpected error: %v", tt.filename, err)
				continue
			}
			if m.Version != tt.version {
				t.Errorf("ParseFilename(%q) version = %q, want %q", tt.filename, m.Version, tt.version)
			}
			if m.Name != tt.name {
				t.Errorf("ParseFilename(%q) name = %q, want %q", tt.filename, m.Name, tt.name)
			}
		} else {
			if err == nil {
				t.Errorf("ParseFilename(%q) expected error, got nil", tt.filename)
			}
		}
	}
}
