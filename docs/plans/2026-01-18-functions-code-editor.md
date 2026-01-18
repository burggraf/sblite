# Edge Functions Code Editor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Monaco-based code editor to the dashboard for editing edge function source files with full file operations support.

**Architecture:** Backend API endpoints for file CRUD operations with path validation, Monaco Editor loaded from CDN in the frontend, file tree component for navigation, and restart confirmation dialog for runtime updates.

**Tech Stack:** Go (backend handlers), Monaco Editor (CDN), vanilla JavaScript (frontend), CSS (styling)

---

## Task 1: Backend - Path Validation Utility

**Files:**
- Create: `internal/dashboard/fileutil.go`
- Test: `internal/dashboard/fileutil_test.go`

**Step 1: Write the failing test**

Create `internal/dashboard/fileutil_test.go`:

```go
package dashboard

import (
	"testing"
)

func TestValidateFunctionFilePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid simple", "index.ts", false},
		{"valid nested", "utils/helper.ts", false},
		{"valid json", "config.json", false},
		{"invalid traversal", "../index.ts", true},
		{"invalid traversal nested", "utils/../../../etc/passwd", true},
		{"invalid absolute", "/etc/passwd", true},
		{"invalid extension", "script.sh", true},
		{"invalid hidden", ".env", true},
		{"valid html", "template.html", false},
		{"valid css", "style.css", false},
		{"valid md", "README.md", false},
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

func TestAllowedExtensions(t *testing.T) {
	allowed := []string{".ts", ".js", ".json", ".mjs", ".tsx", ".jsx", ".html", ".css", ".md", ".txt"}
	for _, ext := range allowed {
		if !IsAllowedExtension(ext) {
			t.Errorf("IsAllowedExtension(%q) = false, want true", ext)
		}
	}

	notAllowed := []string{".sh", ".exe", ".go", ".py", ".env"}
	for _, ext := range notAllowed {
		if IsAllowedExtension(ext) {
			t.Errorf("IsAllowedExtension(%q) = true, want false", ext)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run "TestValidateFunctionFilePath|TestAllowedExtensions" -v`
Expected: FAIL with "undefined: ValidateFunctionFilePath"

**Step 3: Write minimal implementation**

Create `internal/dashboard/fileutil.go`:

```go
package dashboard

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Allowed file extensions for function files
var allowedExtensions = map[string]bool{
	".ts":   true,
	".js":   true,
	".json": true,
	".mjs":  true,
	".tsx":  true,
	".jsx":  true,
	".html": true,
	".css":  true,
	".md":   true,
	".txt":  true,
}

// MaxFileSize is the maximum file size that can be edited (1MB)
const MaxFileSize = 1 * 1024 * 1024

// IsAllowedExtension returns true if the extension is allowed
func IsAllowedExtension(ext string) bool {
	return allowedExtensions[strings.ToLower(ext)]
}

// ValidateFunctionFilePath validates a file path for function editing
func ValidateFunctionFilePath(path string) error {
	// Check for empty path
	if path == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Check for absolute path
	if filepath.IsAbs(path) {
		return fmt.Errorf("absolute paths not allowed")
	}

	// Check for path traversal
	clean := filepath.Clean(path)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, "/../") {
		return fmt.Errorf("path traversal not allowed")
	}

	// Check for hidden files
	parts := strings.Split(clean, string(filepath.Separator))
	for _, part := range parts {
		if strings.HasPrefix(part, ".") && part != "." {
			return fmt.Errorf("hidden files not allowed")
		}
	}

	// Check extension
	ext := filepath.Ext(path)
	if ext != "" && !IsAllowedExtension(ext) {
		return fmt.Errorf("file type %q not allowed", ext)
	}

	return nil
}

// SanitizePath cleans and validates a path, returning error if invalid
func SanitizePath(basePath, relativePath string) (string, error) {
	if err := ValidateFunctionFilePath(relativePath); err != nil {
		return "", err
	}

	fullPath := filepath.Join(basePath, filepath.Clean(relativePath))

	// Ensure path is still within base
	if !strings.HasPrefix(fullPath, basePath) {
		return "", fmt.Errorf("path escapes base directory")
	}

	return fullPath, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run "TestValidateFunctionFilePath|TestAllowedExtensions" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/fileutil.go internal/dashboard/fileutil_test.go
git commit -m "feat(dashboard): add path validation utility for function files"
```

---

## Task 2: Backend - List Function Files API

**Files:**
- Modify: `internal/dashboard/handler.go`
- Test: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

Add to `internal/dashboard/handler_test.go`:

```go
func TestHandleListFunctionFiles(t *testing.T) {
	// Create temp directory with test files
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "test-func")
	os.MkdirAll(filepath.Join(funcDir, "utils"), 0755)
	os.WriteFile(filepath.Join(funcDir, "index.ts"), []byte("// test"), 0644)
	os.WriteFile(filepath.Join(funcDir, "utils", "helper.ts"), []byte("// helper"), 0644)

	// Create handler with mock functions service
	h := &Handler{
		functionsService: &mockFunctionsService{
			functionsDir: tmpDir,
		},
	}

	req := httptest.NewRequest("GET", "/_/api/functions/test-func/files", nil)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, &chi.Context{
		URLParams: chi.RouteParams{
			Keys:   []string{"name"},
			Values: []string{"test-func"},
		},
	}))
	w := httptest.NewRecorder()

	h.handleListFunctionFiles(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)

	if result["name"] != "test-func" {
		t.Errorf("expected name 'test-func', got %v", result["name"])
	}

	children := result["children"].([]interface{})
	if len(children) < 1 {
		t.Error("expected at least 1 child")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandleListFunctionFiles -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/dashboard/handler.go` in the functions route section:

```go
// In RegisterRoutes, add these routes under /functions:
r.Get("/{name}/files", h.handleListFunctionFiles)
r.Get("/{name}/files/*", h.handleReadFunctionFile)
r.Put("/{name}/files/*", h.handleWriteFunctionFile)
r.Delete("/{name}/files/*", h.handleDeleteFunctionFile)
r.Post("/{name}/files/rename", h.handleRenameFunctionFile)
r.Post("/{name}/restart", h.handleRestartFunctions)
```

Add handler implementation:

```go
// FileNode represents a file or directory in the tree
type FileNode struct {
	Name     string      `json:"name"`
	Type     string      `json:"type"` // "file" or "dir"
	Size     int64       `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

// handleListFunctionFiles returns the file tree for a function
func (h *Handler) handleListFunctionFiles(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	name := chi.URLParam(r, "name")
	funcDir := filepath.Join(h.functionsService.FunctionsDir(), name)

	// Check if function exists
	info, err := os.Stat(funcDir)
	if err != nil || !info.IsDir() {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Function not found"})
		return
	}

	// Build file tree
	tree, err := h.buildFileTree(funcDir, name)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(tree)
}

func (h *Handler) buildFileTree(dir, name string) (*FileNode, error) {
	node := &FileNode{
		Name: name,
		Type: "dir",
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		childPath := filepath.Join(dir, entry.Name())

		if entry.IsDir() {
			child, err := h.buildFileTree(childPath, entry.Name())
			if err != nil {
				continue
			}
			node.Children = append(node.Children, child)
		} else {
			// Check extension
			ext := filepath.Ext(entry.Name())
			if ext != "" && !IsAllowedExtension(ext) {
				continue
			}

			info, _ := entry.Info()
			child := &FileNode{
				Name: entry.Name(),
				Type: "file",
				Size: info.Size(),
			}
			node.Children = append(node.Children, child)
		}
	}

	return node, nil
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandleListFunctionFiles -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add list function files API endpoint"
```

---

## Task 3: Backend - Read/Write/Delete File APIs

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Write the failing tests**

Add to `internal/dashboard/handler_test.go`:

```go
func TestHandleReadFunctionFile(t *testing.T) {
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "test-func")
	os.MkdirAll(funcDir, 0755)
	os.WriteFile(filepath.Join(funcDir, "index.ts"), []byte("console.log('hello')"), 0644)

	h := &Handler{
		functionsService: &mockFunctionsService{functionsDir: tmpDir},
	}

	req := httptest.NewRequest("GET", "/_/api/functions/test-func/files/index.ts", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "test-func")
	rctx.URLParams.Add("*", "index.ts")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.handleReadFunctionFile(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var result map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["content"] != "console.log('hello')" {
		t.Errorf("unexpected content: %v", result["content"])
	}
}

func TestHandleWriteFunctionFile(t *testing.T) {
	tmpDir := t.TempDir()
	funcDir := filepath.Join(tmpDir, "test-func")
	os.MkdirAll(funcDir, 0755)

	h := &Handler{
		functionsService: &mockFunctionsService{functionsDir: tmpDir},
	}

	body := `{"content": "// new content"}`
	req := httptest.NewRequest("PUT", "/_/api/functions/test-func/files/index.ts", strings.NewReader(body))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "test-func")
	rctx.URLParams.Add("*", "index.ts")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.handleWriteFunctionFile(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify file was written
	content, _ := os.ReadFile(filepath.Join(funcDir, "index.ts"))
	if string(content) != "// new content" {
		t.Errorf("file content mismatch: %s", content)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/dashboard/... -run "TestHandleReadFunctionFile|TestHandleWriteFunctionFile" -v`
Expected: FAIL

**Step 3: Write minimal implementation**

Add to `internal/dashboard/handler.go`:

```go
// handleReadFunctionFile reads a file from a function directory
func (h *Handler) handleReadFunctionFile(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*")

	funcDir := filepath.Join(h.functionsService.FunctionsDir(), name)
	fullPath, err := SanitizePath(funcDir, filePath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Check file size
	info, err := os.Stat(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "File not found"})
		return
	}

	if info.Size() > MaxFileSize {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "File too large to edit"})
		return
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    filePath,
		"content": string(content),
		"size":    info.Size(),
	})
}

// handleWriteFunctionFile creates or updates a file in a function directory
func (h *Handler) handleWriteFunctionFile(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*")

	funcDir := filepath.Join(h.functionsService.FunctionsDir(), name)
	fullPath, err := SanitizePath(funcDir, filePath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if err := os.WriteFile(fullPath, []byte(req.Content), 0644); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "path": filePath})
}

// handleDeleteFunctionFile deletes a file or directory
func (h *Handler) handleDeleteFunctionFile(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	name := chi.URLParam(r, "name")
	filePath := chi.URLParam(r, "*")

	funcDir := filepath.Join(h.functionsService.FunctionsDir(), name)
	fullPath, err := SanitizePath(funcDir, filePath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if err := os.RemoveAll(fullPath); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRenameFunctionFile renames/moves a file or directory
func (h *Handler) handleRenameFunctionFile(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	name := chi.URLParam(r, "name")
	funcDir := filepath.Join(h.functionsService.FunctionsDir(), name)

	var req struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	oldFull, err := SanitizePath(funcDir, req.OldPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid old path: " + err.Error()})
		return
	}

	newFull, err := SanitizePath(funcDir, req.NewPath)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid new path: " + err.Error()})
		return
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(newFull), 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if err := os.Rename(oldFull, newFull); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleRestartFunctions restarts the edge runtime
func (h *Handler) handleRestartFunctions(w http.ResponseWriter, r *http.Request) {
	if h.functionsService == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"error": "Edge functions not enabled"})
		return
	}

	ctx := r.Context()
	if err := h.functionsService.Stop(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to stop: " + err.Error()})
		return
	}

	if err := h.functionsService.Start(ctx); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start: " + err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "Runtime restarted"})
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/dashboard/... -run "TestHandleReadFunctionFile|TestHandleWriteFunctionFile" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add read/write/delete/rename file API endpoints"
```

---

## Task 4: Frontend - Monaco Editor Setup

**Files:**
- Modify: `internal/dashboard/static/index.html`
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add Monaco Editor script to index.html**

In `internal/dashboard/static/index.html`, add before `</head>`:

```html
<!-- Monaco Editor -->
<script src="https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs/loader.js"></script>
<script>
  require.config({ paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' } });
</script>
```

**Step 2: Add editor state to app.js**

In `internal/dashboard/static/app.js`, add to state.functions:

```javascript
// In state.functions, add:
editor: {
    currentFile: null,      // Currently open file path
    content: '',            // File content in editor
    originalContent: '',    // For dirty detection
    isDirty: false,         // Has unsaved changes
    tree: null,             // File tree structure
    expandedFolders: {},    // Which folders are expanded
    isExpanded: false,      // Full-width mode
    monacoEditor: null,     // Monaco editor instance
    loading: false          // Loading state
}
```

**Step 3: Commit**

```bash
git add internal/dashboard/static/index.html internal/dashboard/static/app.js
git commit -m "feat(dashboard): add Monaco editor setup"
```

---

## Task 5: Frontend - Editor State and Methods

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add editor methods**

Add these methods to the App object in `app.js`:

```javascript
// Load file tree for a function
async loadFunctionFiles(name) {
    this.state.functions.editor.loading = true;
    this.render();

    try {
        const res = await fetch(`/_/api/functions/${name}/files`);
        if (res.ok) {
            const tree = await res.json();
            this.state.functions.editor.tree = tree;

            // Auto-open index.ts if exists
            const indexFile = this.findFileInTree(tree, 'index.ts');
            if (indexFile) {
                await this.openFunctionFile('index.ts');
            }
        }
    } catch (err) {
        console.error('Failed to load function files:', err);
    }

    this.state.functions.editor.loading = false;
    this.render();
},

findFileInTree(node, filename) {
    if (node.type === 'file' && node.name === filename) return node;
    if (node.children) {
        for (const child of node.children) {
            const found = this.findFileInTree(child, filename);
            if (found) return found;
        }
    }
    return null;
},

async openFunctionFile(path) {
    const name = this.state.functions.selected;
    if (!name) return;

    // Check for unsaved changes
    if (this.state.functions.editor.isDirty) {
        if (!confirm('You have unsaved changes. Discard them?')) return;
    }

    try {
        const res = await fetch(`/_/api/functions/${name}/files/${path}`);
        if (res.ok) {
            const data = await res.json();
            this.state.functions.editor.currentFile = path;
            this.state.functions.editor.content = data.content;
            this.state.functions.editor.originalContent = data.content;
            this.state.functions.editor.isDirty = false;

            // Update Monaco editor if it exists
            if (this.state.functions.editor.monacoEditor) {
                this.state.functions.editor.monacoEditor.setValue(data.content);
                this.setMonacoLanguage(path);
            }

            this.render();
        }
    } catch (err) {
        console.error('Failed to open file:', err);
    }
},

async saveFunctionFile() {
    const { selected } = this.state.functions;
    const { currentFile, monacoEditor } = this.state.functions.editor;
    if (!selected || !currentFile) return;

    const content = monacoEditor ? monacoEditor.getValue() : this.state.functions.editor.content;

    try {
        const res = await fetch(`/_/api/functions/${selected}/files/${currentFile}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content })
        });

        if (res.ok) {
            this.state.functions.editor.originalContent = content;
            this.state.functions.editor.content = content;
            this.state.functions.editor.isDirty = false;
            this.render();

            // Ask about restart
            if (confirm('File saved. Restart edge runtime to apply changes?')) {
                await this.restartFunctionsRuntime();
            }
        }
    } catch (err) {
        console.error('Failed to save file:', err);
        alert('Failed to save file');
    }
},

async restartFunctionsRuntime() {
    const { selected } = this.state.functions;
    try {
        const res = await fetch(`/_/api/functions/${selected}/restart`, { method: 'POST' });
        if (res.ok) {
            await this.loadFunctionsStatus();
            this.render();
        } else {
            alert('Failed to restart runtime');
        }
    } catch (err) {
        console.error('Failed to restart runtime:', err);
    }
},

setMonacoLanguage(path) {
    const editor = this.state.functions.editor.monacoEditor;
    if (!editor) return;

    const ext = path.split('.').pop();
    const languageMap = {
        'ts': 'typescript',
        'tsx': 'typescript',
        'js': 'javascript',
        'jsx': 'javascript',
        'mjs': 'javascript',
        'json': 'json',
        'html': 'html',
        'css': 'css',
        'md': 'markdown',
        'txt': 'plaintext'
    };

    const language = languageMap[ext] || 'plaintext';
    monaco.editor.setModelLanguage(editor.getModel(), language);
},

toggleEditorExpand() {
    this.state.functions.editor.isExpanded = !this.state.functions.editor.isExpanded;
    this.render();

    // Resize Monaco editor
    if (this.state.functions.editor.monacoEditor) {
        setTimeout(() => this.state.functions.editor.monacoEditor.layout(), 100);
    }
},

toggleFolder(path) {
    const expanded = this.state.functions.editor.expandedFolders;
    expanded[path] = !expanded[path];
    this.render();
},
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add editor state management methods"
```

---

## Task 6: Frontend - File Tree Component

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add file tree render method**

```javascript
renderFileTree(node, path = '') {
    const currentPath = path ? `${path}/${node.name}` : node.name;
    const isRoot = path === '';
    const isExpanded = isRoot || this.state.functions.editor.expandedFolders[currentPath];

    if (node.type === 'file') {
        const isActive = this.state.functions.editor.currentFile === currentPath;
        const isDirty = isActive && this.state.functions.editor.isDirty;
        return `
            <div class="file-tree-item file ${isActive ? 'active' : ''}"
                 onclick="App.openFunctionFile('${currentPath}')"
                 oncontextmenu="App.showFileContextMenu(event, '${currentPath}', 'file')">
                <span class="file-icon">${this.getFileIcon(node.name)}</span>
                <span class="file-name">${this.escapeHtml(node.name)}${isDirty ? ' ●' : ''}</span>
            </div>
        `;
    }

    // Directory
    const children = node.children || [];
    return `
        <div class="file-tree-item dir ${isRoot ? 'root' : ''}">
            <div class="dir-header" onclick="App.toggleFolder('${currentPath}')"
                 oncontextmenu="App.showFileContextMenu(event, '${currentPath}', 'dir')">
                <span class="expand-icon">${isExpanded ? '▼' : '▶'}</span>
                <span class="dir-name">${this.escapeHtml(node.name)}</span>
            </div>
            ${isExpanded ? `
                <div class="dir-children">
                    ${children.map(child => this.renderFileTree(child, isRoot ? '' : currentPath)).join('')}
                </div>
            ` : ''}
        </div>
    `;
},

getFileIcon(filename) {
    const ext = filename.split('.').pop();
    const icons = {
        'ts': '📘', 'tsx': '📘',
        'js': '📙', 'jsx': '📙', 'mjs': '📙',
        'json': '📋',
        'html': '🌐',
        'css': '🎨',
        'md': '📝',
        'txt': '📄'
    };
    return icons[ext] || '📄';
},

showFileContextMenu(event, path, type) {
    event.preventDefault();
    // Store for context menu actions
    this._contextMenuPath = path;
    this._contextMenuType = type;

    const menu = document.getElementById('file-context-menu');
    if (menu) {
        menu.style.display = 'block';
        menu.style.left = event.pageX + 'px';
        menu.style.top = event.pageY + 'px';

        // Show/hide options based on type
        menu.querySelector('.ctx-new-file').style.display = type === 'dir' ? 'block' : 'none';
        menu.querySelector('.ctx-new-folder').style.display = type === 'dir' ? 'block' : 'none';
    }
},

hideContextMenu() {
    const menu = document.getElementById('file-context-menu');
    if (menu) menu.style.display = 'none';
},
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add file tree component"
```

---

## Task 7: Frontend - Editor Panel Render

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Update renderFunctionDetail to include editor**

Replace the existing `renderFunctionDetail` method:

```javascript
renderFunctionDetail() {
    const { selected, config } = this.state.functions;
    const { tree, currentFile, isDirty, isExpanded, loading } = this.state.functions.editor;
    const fn = this.state.functions.list.find(f => f.name === selected);

    if (!fn) return '';

    if (loading) {
        return `<div class="function-detail loading">Loading files...</div>`;
    }

    return `
        <div class="function-detail ${isExpanded ? 'expanded' : ''}">
            <div class="function-detail-header">
                <div class="header-left">
                    <button class="btn btn-link" onclick="App.deselectFunction()">← Back</button>
                    <h2>${this.escapeHtml(selected)}</h2>
                </div>
                <div class="header-right">
                    <div class="dropdown">
                        <button class="btn btn-secondary btn-sm dropdown-toggle" onclick="this.nextElementSibling.classList.toggle('show')">
                            Config ▼
                        </button>
                        <div class="dropdown-menu">
                            <label class="dropdown-item">
                                <input type="checkbox" ${fn.verify_jwt !== false ? 'checked' : ''}
                                    onchange="App.toggleFunctionJWT('${selected}', this.checked)">
                                Require JWT
                            </label>
                        </div>
                    </div>
                    <div class="dropdown">
                        <button class="btn btn-secondary btn-sm dropdown-toggle" onclick="this.nextElementSibling.classList.toggle('show')">
                            Test ▼
                        </button>
                        <div class="dropdown-menu test-dropdown">
                            ${this.renderFunctionTestConsole()}
                        </div>
                    </div>
                    <button class="btn btn-secondary btn-sm" onclick="App.toggleEditorExpand()" title="Toggle expand">
                        ${isExpanded ? '⛶' : '⛶'}
                    </button>
                    <button class="btn btn-danger btn-sm" onclick="App.deleteFunction('${selected}')">Delete</button>
                </div>
            </div>

            <div class="editor-container">
                ${!isExpanded ? `
                    <div class="file-tree-panel">
                        <div class="file-tree-header">
                            <span>Files</span>
                            <div class="file-tree-actions">
                                <button class="btn btn-xs" onclick="App.createNewFile()" title="New File">+ File</button>
                                <button class="btn btn-xs" onclick="App.createNewFolder()" title="New Folder">+ Folder</button>
                            </div>
                        </div>
                        <div class="file-tree">
                            ${tree ? this.renderFileTree(tree) : '<div class="empty">No files</div>'}
                        </div>
                    </div>
                ` : ''}

                <div class="editor-panel">
                    <div class="editor-header">
                        <span class="current-file">
                            ${currentFile ? this.escapeHtml(currentFile) : 'No file selected'}
                            ${isDirty ? ' ●' : ''}
                        </span>
                        <div class="editor-actions">
                            <button class="btn btn-primary btn-sm" onclick="App.saveFunctionFile()"
                                ${!currentFile || !isDirty ? 'disabled' : ''}>Save</button>
                            <button class="btn btn-secondary btn-sm" onclick="App.restartFunctionsRuntime()"
                                title="Restart Runtime">⟳</button>
                        </div>
                    </div>
                    <div id="monaco-editor-container" class="editor-content"></div>
                </div>
            </div>
        </div>

        <!-- Context Menu -->
        <div id="file-context-menu" class="context-menu" style="display: none;">
            <div class="ctx-item ctx-new-file" onclick="App.createNewFile()">New File</div>
            <div class="ctx-item ctx-new-folder" onclick="App.createNewFolder()">New Folder</div>
            <div class="ctx-item" onclick="App.renameFile()">Rename</div>
            <div class="ctx-item ctx-delete" onclick="App.deleteFile()">Delete</div>
        </div>
    `;
},
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add editor panel rendering"
```

---

## Task 8: Frontend - Monaco Initialization

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add Monaco initialization**

Add method to initialize Monaco after render:

```javascript
initMonacoEditor() {
    const container = document.getElementById('monaco-editor-container');
    if (!container || this.state.functions.editor.monacoEditor) return;

    // Wait for Monaco to load
    if (typeof monaco === 'undefined') {
        require(['vs/editor/editor.main'], () => this.initMonacoEditor());
        return;
    }

    const isDark = document.body.classList.contains('dark-theme');

    const editor = monaco.editor.create(container, {
        value: this.state.functions.editor.content || '// Select a file to edit',
        language: 'typescript',
        theme: isDark ? 'vs-dark' : 'vs',
        automaticLayout: true,
        minimap: { enabled: true },
        fontSize: 14,
        tabSize: 2,
        scrollBeyondLastLine: false,
        wordWrap: 'on'
    });

    this.state.functions.editor.monacoEditor = editor;

    // Track changes for dirty state
    editor.onDidChangeModelContent(() => {
        const current = editor.getValue();
        const original = this.state.functions.editor.originalContent;
        const wasDirty = this.state.functions.editor.isDirty;
        const isDirty = current !== original;

        if (wasDirty !== isDirty) {
            this.state.functions.editor.isDirty = isDirty;
            this.state.functions.editor.content = current;
            // Update just the header, not full render
            this.updateEditorHeader();
        }
    });

    // Keyboard shortcut for save
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
        this.saveFunctionFile();
    });
},

updateEditorHeader() {
    const { currentFile, isDirty } = this.state.functions.editor;
    const fileSpan = document.querySelector('.editor-header .current-file');
    const saveBtn = document.querySelector('.editor-header .btn-primary');

    if (fileSpan) {
        fileSpan.textContent = currentFile ? `${currentFile}${isDirty ? ' ●' : ''}` : 'No file selected';
    }
    if (saveBtn) {
        saveBtn.disabled = !currentFile || !isDirty;
    }

    // Update file tree item
    const activeItem = document.querySelector('.file-tree-item.file.active .file-name');
    if (activeItem && currentFile) {
        const filename = currentFile.split('/').pop();
        activeItem.textContent = `${filename}${isDirty ? ' ●' : ''}`;
    }
},

destroyMonacoEditor() {
    if (this.state.functions.editor.monacoEditor) {
        this.state.functions.editor.monacoEditor.dispose();
        this.state.functions.editor.monacoEditor = null;
    }
},
```

**Step 2: Update selectFunction to load files and init editor**

Modify the `selectFunction` method:

```javascript
async selectFunction(name) {
    this.state.functions.selected = name;
    this.state.functions.config = null;
    this.state.functions.editor = {
        currentFile: null,
        content: '',
        originalContent: '',
        isDirty: false,
        tree: null,
        expandedFolders: {},
        isExpanded: false,
        monacoEditor: null,
        loading: true
    };

    this.render();

    // Load function config
    try {
        const res = await fetch(`/_/api/functions/${name}/config`);
        if (res.ok) {
            this.state.functions.config = await res.json();
        }
    } catch (err) {
        console.error('Failed to load function config:', err);
    }

    // Load file tree
    await this.loadFunctionFiles(name);

    // Initialize Monaco after render
    setTimeout(() => this.initMonacoEditor(), 100);
},
```

**Step 3: Update deselectFunction to clean up editor**

```javascript
deselectFunction() {
    this.destroyMonacoEditor();
    this.state.functions.selected = null;
    this.state.functions.config = null;
    this.state.functions.editor = {
        currentFile: null,
        content: '',
        originalContent: '',
        isDirty: false,
        tree: null,
        expandedFolders: {},
        isExpanded: false,
        monacoEditor: null,
        loading: false
    };
    this.render();
},
```

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add Monaco editor initialization"
```

---

## Task 9: Frontend - File Operations (Create/Rename/Delete)

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add file operation methods**

```javascript
async createNewFile() {
    const name = this.state.functions.selected;
    if (!name) return;

    const path = this._contextMenuPath || '';
    const filename = prompt('Enter file name (e.g., utils.ts):');
    if (!filename) return;

    // Validate extension
    const ext = filename.split('.').pop();
    const allowed = ['ts', 'js', 'json', 'mjs', 'tsx', 'jsx', 'html', 'css', 'md', 'txt'];
    if (!allowed.includes(ext)) {
        alert(`File type .${ext} not allowed. Use: ${allowed.join(', ')}`);
        return;
    }

    const fullPath = path && this._contextMenuType === 'dir' ? `${path}/${filename}` : filename;

    try {
        const res = await fetch(`/_/api/functions/${name}/files/${fullPath}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content: '' })
        });

        if (res.ok) {
            await this.loadFunctionFiles(name);
            await this.openFunctionFile(fullPath);
        } else {
            const err = await res.json();
            alert(err.error || 'Failed to create file');
        }
    } catch (err) {
        console.error('Failed to create file:', err);
    }

    this.hideContextMenu();
},

async createNewFolder() {
    const name = this.state.functions.selected;
    if (!name) return;

    const path = this._contextMenuPath || '';
    const dirname = prompt('Enter folder name:');
    if (!dirname) return;

    const fullPath = path && this._contextMenuType === 'dir' ? `${path}/${dirname}` : dirname;

    // Create folder by creating a placeholder file
    try {
        const res = await fetch(`/_/api/functions/${name}/files/${fullPath}/.gitkeep`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ content: '' })
        });

        if (res.ok) {
            await this.loadFunctionFiles(name);
        } else {
            alert('Failed to create folder');
        }
    } catch (err) {
        console.error('Failed to create folder:', err);
    }

    this.hideContextMenu();
},

async renameFile() {
    const name = this.state.functions.selected;
    const oldPath = this._contextMenuPath;
    if (!name || !oldPath) return;

    const oldName = oldPath.split('/').pop();
    const newName = prompt('Enter new name:', oldName);
    if (!newName || newName === oldName) return;

    const newPath = oldPath.replace(/[^/]+$/, newName);

    try {
        const res = await fetch(`/_/api/functions/${name}/files/rename`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ oldPath, newPath })
        });

        if (res.ok) {
            // Update current file if it was renamed
            if (this.state.functions.editor.currentFile === oldPath) {
                this.state.functions.editor.currentFile = newPath;
            }
            await this.loadFunctionFiles(name);
        } else {
            alert('Failed to rename');
        }
    } catch (err) {
        console.error('Failed to rename:', err);
    }

    this.hideContextMenu();
},

async deleteFile() {
    const name = this.state.functions.selected;
    const path = this._contextMenuPath;
    if (!name || !path) return;

    if (!confirm(`Delete ${path}?`)) return;

    try {
        const res = await fetch(`/_/api/functions/${name}/files/${path}`, {
            method: 'DELETE'
        });

        if (res.ok) {
            // Clear editor if deleted file was open
            if (this.state.functions.editor.currentFile === path) {
                this.state.functions.editor.currentFile = null;
                this.state.functions.editor.content = '';
                this.state.functions.editor.originalContent = '';
                this.state.functions.editor.isDirty = false;
                if (this.state.functions.editor.monacoEditor) {
                    this.state.functions.editor.monacoEditor.setValue('// Select a file to edit');
                }
            }
            await this.loadFunctionFiles(name);
        } else {
            alert('Failed to delete');
        }
    } catch (err) {
        console.error('Failed to delete:', err);
    }

    this.hideContextMenu();
},
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add file create/rename/delete operations"
```

---

## Task 10: Frontend - CSS Styles

**Files:**
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add editor styles**

Add to `style.css`:

```css
/* Function Editor Layout */
.function-detail.expanded .editor-container {
    display: block;
}

.function-detail.expanded .file-tree-panel {
    display: none;
}

.editor-container {
    display: flex;
    height: calc(100vh - 200px);
    min-height: 400px;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    overflow: hidden;
}

.file-tree-panel {
    width: 250px;
    min-width: 200px;
    border-right: 1px solid var(--border-color);
    display: flex;
    flex-direction: column;
    background: var(--sidebar-bg);
}

.file-tree-header {
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-weight: 500;
}

.file-tree-actions {
    display: flex;
    gap: 4px;
}

.file-tree {
    flex: 1;
    overflow: auto;
    padding: 8px 0;
}

.file-tree-item {
    cursor: pointer;
    user-select: none;
}

.file-tree-item.file {
    padding: 4px 12px 4px 24px;
    display: flex;
    align-items: center;
    gap: 6px;
}

.file-tree-item.file:hover {
    background: var(--hover-bg);
}

.file-tree-item.file.active {
    background: var(--active-bg);
    color: var(--primary-color);
}

.dir-header {
    padding: 4px 12px;
    display: flex;
    align-items: center;
    gap: 4px;
}

.dir-header:hover {
    background: var(--hover-bg);
}

.expand-icon {
    font-size: 10px;
    width: 12px;
}

.dir-children {
    padding-left: 12px;
}

.file-icon {
    font-size: 14px;
}

/* Editor Panel */
.editor-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.editor-header {
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color);
    display: flex;
    justify-content: space-between;
    align-items: center;
    background: var(--bg-color);
}

.current-file {
    font-family: monospace;
    font-size: 13px;
}

.editor-actions {
    display: flex;
    gap: 8px;
}

.editor-content {
    flex: 1;
    min-height: 300px;
}

#monaco-editor-container {
    width: 100%;
    height: 100%;
}

/* Context Menu */
.context-menu {
    position: fixed;
    background: var(--bg-color);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 1000;
    min-width: 150px;
}

.ctx-item {
    padding: 8px 12px;
    cursor: pointer;
}

.ctx-item:hover {
    background: var(--hover-bg);
}

.ctx-item.ctx-delete {
    color: var(--error-color);
}

/* Dropdown menus */
.dropdown {
    position: relative;
    display: inline-block;
}

.dropdown-menu {
    display: none;
    position: absolute;
    right: 0;
    top: 100%;
    background: var(--bg-color);
    border: 1px solid var(--border-color);
    border-radius: 6px;
    box-shadow: 0 4px 12px rgba(0,0,0,0.15);
    z-index: 100;
    min-width: 200px;
    padding: 8px 0;
}

.dropdown-menu.show {
    display: block;
}

.dropdown-item {
    display: block;
    padding: 8px 12px;
    cursor: pointer;
}

.dropdown-item:hover {
    background: var(--hover-bg);
}

.test-dropdown {
    width: 400px;
    max-height: 500px;
    overflow: auto;
    padding: 12px;
}

/* Header layout */
.function-detail-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
    flex-wrap: wrap;
    gap: 12px;
}

.header-left {
    display: flex;
    align-items: center;
    gap: 12px;
}

.header-right {
    display: flex;
    align-items: center;
    gap: 8px;
}

.btn-link {
    background: none;
    border: none;
    color: var(--primary-color);
    cursor: pointer;
    padding: 0;
}

.btn-link:hover {
    text-decoration: underline;
}
```

**Step 2: Commit**

```bash
git add internal/dashboard/static/style.css
git commit -m "feat(dashboard): add editor CSS styles"
```

---

## Task 11: Register Routes and Final Integration

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Register the new routes**

In `RegisterRoutes` function, update the `/functions` route section:

```go
// Functions management routes (require auth)
r.Route("/functions", func(r chi.Router) {
    r.Use(h.authMiddleware)
    r.Get("/", h.handleListFunctions)
    r.Get("/status", h.handleGetFunctionsStatus)
    r.Get("/{name}", h.handleGetFunction)
    r.Post("/{name}", h.handleCreateFunction)
    r.Delete("/{name}", h.handleDeleteFunction)
    r.Get("/{name}/config", h.handleGetFunctionConfig)
    r.Patch("/{name}/config", h.handleUpdateFunctionConfig)

    // File operations
    r.Get("/{name}/files", h.handleListFunctionFiles)
    r.Get("/{name}/files/*", h.handleReadFunctionFile)
    r.Put("/{name}/files/*", h.handleWriteFunctionFile)
    r.Delete("/{name}/files/*", h.handleDeleteFunctionFile)
    r.Post("/{name}/files/rename", h.handleRenameFunctionFile)
    r.Post("/{name}/restart", h.handleRestartFunctions)
})
```

**Step 2: Run all tests**

Run: `go test ./... -v`
Expected: All tests pass

**Step 3: Build and manual test**

```bash
go build -o sblite .
./sblite serve --db test.db --functions
# Open http://localhost:8080/_/ and test the editor
```

**Step 4: Final commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(dashboard): register file operation routes"
```

---

## Task 12: Add Restart Method to Functions Service

**Files:**
- Modify: `internal/functions/functions.go`

**Step 1: Add Restart method if not exists**

Check if `Restart` method exists. If not, add:

```go
// Restart restarts the edge runtime (stops and starts).
func (s *Service) Restart(ctx context.Context) error {
    if err := s.Stop(); err != nil {
        return err
    }
    return s.Start(ctx)
}
```

**Step 2: Commit**

```bash
git add internal/functions/functions.go
git commit -m "feat(functions): add Restart method to service"
```

---

## Summary

After completing all tasks:

1. Run full test suite: `go test ./...`
2. Build: `go build -o sblite .`
3. Manual test the editor in the dashboard
4. Push branch: `git push -u origin feature/functions-code-editor`
5. Create PR when ready

**Total commits:** 12
**Estimated tasks:** 12 steps with TDD approach
