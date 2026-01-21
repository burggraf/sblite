package functions

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFunctionName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "hello", false},
		{"valid with hyphen", "hello-world", false},
		{"valid with underscore", "hello_world", false},
		{"valid with numbers", "function123", false},
		{"empty name", "", true},
		{"starts with dot", ".hidden", true},
		{"starts with underscore", "_private", true},
		{"contains slash", "path/to/func", true},
		{"contains backslash", "path\\to\\func", true},
		{"reserved name _shared", "_shared", true},
		{"reserved name node_modules", "node_modules", true},
		{"too long", "a" + string(make([]byte, 65)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFunctionName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFunctionName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestGetTemplate(t *testing.T) {
	tests := []struct {
		name         string
		templateType TemplateType
		funcName     string
		wantContains string
	}{
		{"default template", TemplateDefault, "test-func", "serve(async (req: Request)"},
		{"supabase template", TemplateSupabase, "my-func", "createClient"},
		{"cors template", TemplateCORS, "cors-func", "corsHeaders"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetTemplate(tt.templateType, tt.funcName)
			if result == "" {
				t.Error("GetTemplate returned empty string")
			}
			if len(result) < 100 {
				t.Error("GetTemplate returned suspiciously short template")
			}
		})
	}
}

func TestAvailableTemplates(t *testing.T) {
	templates := AvailableTemplates()
	if len(templates) == 0 {
		t.Error("AvailableTemplates returned empty list")
	}

	// Check that default is included
	found := false
	for _, tmpl := range templates {
		if tmpl == "default" {
			found = true
			break
		}
	}
	if !found {
		t.Error("AvailableTemplates does not include 'default'")
	}
}

func TestServiceListFunctions(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "functions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test function
	funcDir := filepath.Join(tmpDir, "test-func")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, "index.ts"), []byte("// test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewService(nil, &Config{FunctionsDir: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	// List functions
	funcs, err := svc.ListFunctions()
	if err != nil {
		t.Fatal(err)
	}

	if len(funcs) != 1 {
		t.Errorf("expected 1 function, got %d", len(funcs))
	}

	if funcs[0].Name != "test-func" {
		t.Errorf("expected function name 'test-func', got %q", funcs[0].Name)
	}

	if funcs[0].Entrypoint != "index.ts" {
		t.Errorf("expected entrypoint 'index.ts', got %q", funcs[0].Entrypoint)
	}
}

func TestServiceFunctionExists(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "functions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test function with index.ts
	funcDir := filepath.Join(tmpDir, "exists-func")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, "index.ts"), []byte("// test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewService(nil, &Config{FunctionsDir: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	// Test existing function
	if !svc.FunctionExists("exists-func") {
		t.Error("FunctionExists returned false for existing function")
	}

	// Test non-existing function
	if svc.FunctionExists("not-exists") {
		t.Error("FunctionExists returned true for non-existing function")
	}
}

func TestServiceCreateFunction(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "functions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service
	svc, err := NewService(nil, &Config{FunctionsDir: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	// Create function with default template
	if err := svc.CreateFunction("new-func", "default"); err != nil {
		t.Fatal(err)
	}

	// Verify function exists
	if !svc.FunctionExists("new-func") {
		t.Error("Created function does not exist")
	}

	// Verify index.ts was created
	indexPath := filepath.Join(tmpDir, "new-func", "index.ts")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.ts was not created")
	}

	// Creating same function again should fail
	if err := svc.CreateFunction("new-func", "default"); err == nil {
		t.Error("Creating duplicate function should fail")
	}
}

func TestServiceDeleteFunction(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "functions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create service
	svc, err := NewService(nil, &Config{FunctionsDir: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	// Create function
	if err := svc.CreateFunction("delete-me", "default"); err != nil {
		t.Fatal(err)
	}

	// Delete function
	if err := svc.DeleteFunction("delete-me"); err != nil {
		t.Fatal(err)
	}

	// Verify function no longer exists
	if svc.FunctionExists("delete-me") {
		t.Error("Deleted function still exists")
	}

	// Deleting non-existent function should fail
	if err := svc.DeleteFunction("not-exists"); err == nil {
		t.Error("Deleting non-existent function should fail")
	}
}

func TestServiceGetFunction(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "functions-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test function
	funcDir := filepath.Join(tmpDir, "get-func")
	if err := os.MkdirAll(funcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(funcDir, "index.ts"), []byte("// test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create service
	svc, err := NewService(nil, &Config{FunctionsDir: tmpDir})
	if err != nil {
		t.Fatal(err)
	}

	// Get function
	fn, err := svc.GetFunction("get-func")
	if err != nil {
		t.Fatal(err)
	}

	if fn.Name != "get-func" {
		t.Errorf("expected name 'get-func', got %q", fn.Name)
	}

	if fn.Entrypoint != "index.ts" {
		t.Errorf("expected entrypoint 'index.ts', got %q", fn.Entrypoint)
	}

	// Get non-existent function
	_, err = svc.GetFunction("not-exists")
	if err == nil {
		t.Error("GetFunction should fail for non-existent function")
	}
}

func TestDefaultDownloadDir(t *testing.T) {
	// Test with empty dbPath (fallback behavior)
	dir := DefaultDownloadDir("")
	if dir == "" {
		t.Error("DefaultDownloadDir returned empty string")
	}
	if !filepath.IsAbs(dir) {
		t.Error("DefaultDownloadDir should return absolute path")
	}

	// Test with dbPath (db-relative behavior)
	dbPath := "/some/path/data.db"
	dir = DefaultDownloadDir(dbPath)
	expected := "/some/path/edge-runtime"
	if dir != expected {
		t.Errorf("DefaultDownloadDir(%q) = %q, want %q", dbPath, dir, expected)
	}
}

func TestIsSupported(t *testing.T) {
	// This just tests that the function doesn't panic
	supported := IsSupported()
	// On darwin/linux with amd64/arm64, should be true
	t.Logf("Platform %s supported: %v", PlatformString(), supported)
}

func TestPlatformString(t *testing.T) {
	platform := PlatformString()
	if platform == "" {
		t.Error("PlatformString returned empty string")
	}
	if platform == "/" {
		t.Error("PlatformString returned invalid format")
	}
}
