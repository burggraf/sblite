package dashboard

import (
	"path/filepath"
	"testing"
)

func TestValidateFunctionFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		// Valid paths
		{name: "simple ts file", path: "index.ts", wantErr: false},
		{name: "nested ts file", path: "utils/helper.ts", wantErr: false},
		{name: "json file", path: "config.json", wantErr: false},
		{name: "html file", path: "template.html", wantErr: false},
		{name: "css file", path: "style.css", wantErr: false},
		{name: "markdown file", path: "README.md", wantErr: false},
		{name: "js file", path: "main.js", wantErr: false},
		{name: "mjs file", path: "module.mjs", wantErr: false},
		{name: "tsx file", path: "component.tsx", wantErr: false},
		{name: "jsx file", path: "component.jsx", wantErr: false},
		{name: "txt file", path: "notes.txt", wantErr: false},
		{name: "deeply nested", path: "src/utils/helpers/format.ts", wantErr: false},

		// Invalid: empty path
		{name: "empty path", path: "", wantErr: true},

		// Invalid: absolute paths
		{name: "absolute unix path", path: "/etc/passwd", wantErr: true},
		{name: "absolute windows path", path: "C:\\Windows\\System32", wantErr: true},

		// Invalid: path traversal
		{name: "parent traversal", path: "../index.ts", wantErr: true},
		{name: "deep traversal", path: "utils/../../../etc/passwd", wantErr: true},
		{name: "windows traversal", path: "..\\index.ts", wantErr: true},
		{name: "hidden traversal", path: "utils/..\\..\\etc/passwd", wantErr: true},
		{name: "encoded traversal attempt", path: "utils/..%2F..%2Fetc/passwd", wantErr: true},

		// Invalid: hidden files
		{name: "dotenv file", path: ".env", wantErr: true},
		{name: "dotgitignore", path: ".gitignore", wantErr: true},
		{name: "hidden in subdir", path: "config/.env", wantErr: true},
		{name: "hidden directory", path: ".git/config", wantErr: true},

		// Invalid: disallowed extensions
		{name: "shell script", path: "script.sh", wantErr: true},
		{name: "python file", path: "script.py", wantErr: true},
		{name: "executable", path: "binary.exe", wantErr: true},
		{name: "no extension", path: "Makefile", wantErr: true},
		{name: "php file", path: "index.php", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFunctionFilePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFunctionFilePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestIsAllowedExtension(t *testing.T) {
	tests := []struct {
		ext     string
		allowed bool
	}{
		// Allowed extensions
		{ext: ".ts", allowed: true},
		{ext: ".js", allowed: true},
		{ext: ".json", allowed: true},
		{ext: ".mjs", allowed: true},
		{ext: ".tsx", allowed: true},
		{ext: ".jsx", allowed: true},
		{ext: ".html", allowed: true},
		{ext: ".css", allowed: true},
		{ext: ".md", allowed: true},
		{ext: ".txt", allowed: true},

		// Not allowed
		{ext: ".sh", allowed: false},
		{ext: ".py", allowed: false},
		{ext: ".exe", allowed: false},
		{ext: ".php", allowed: false},
		{ext: ".go", allowed: false},
		{ext: "", allowed: false},
		{ext: ".env", allowed: false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := IsAllowedExtension(tt.ext)
			if got != tt.allowed {
				t.Errorf("IsAllowedExtension(%q) = %v, want %v", tt.ext, got, tt.allowed)
			}
		})
	}
}

func TestSanitizePath(t *testing.T) {
	basePath := "/functions/test-function"

	tests := []struct {
		name         string
		basePath     string
		relativePath string
		wantPath     string
		wantErr      bool
	}{
		{
			name:         "simple file",
			basePath:     basePath,
			relativePath: "index.ts",
			wantPath:     filepath.Join(basePath, "index.ts"),
			wantErr:      false,
		},
		{
			name:         "nested file",
			basePath:     basePath,
			relativePath: "src/utils/helper.ts",
			wantPath:     filepath.Join(basePath, "src/utils/helper.ts"),
			wantErr:      false,
		},
		{
			name:         "traversal attempt",
			basePath:     basePath,
			relativePath: "../../../etc/passwd",
			wantPath:     "",
			wantErr:      true,
		},
		{
			name:         "absolute path",
			basePath:     basePath,
			relativePath: "/etc/passwd",
			wantPath:     "",
			wantErr:      true,
		},
		{
			name:         "hidden file",
			basePath:     basePath,
			relativePath: ".env",
			wantPath:     "",
			wantErr:      true,
		},
		{
			name:         "disallowed extension",
			basePath:     basePath,
			relativePath: "script.sh",
			wantPath:     "",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPath, err := SanitizePath(tt.basePath, tt.relativePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("SanitizePath(%q, %q) error = %v, wantErr %v", tt.basePath, tt.relativePath, err, tt.wantErr)
				return
			}
			if gotPath != tt.wantPath {
				t.Errorf("SanitizePath(%q, %q) = %q, want %q", tt.basePath, tt.relativePath, gotPath, tt.wantPath)
			}
		})
	}
}

func TestMaxFileSize(t *testing.T) {
	// Verify MaxFileSize constant is 1MB
	expected := int64(1 * 1024 * 1024)
	if MaxFileSize != expected {
		t.Errorf("MaxFileSize = %d, want %d (1MB)", MaxFileSize, expected)
	}
}
