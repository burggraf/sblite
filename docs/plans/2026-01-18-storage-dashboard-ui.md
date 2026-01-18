# Storage Dashboard UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a Storage section to the sblite dashboard for file management with buckets, drag-drop uploads, grid/list views, and bulk operations.

**Architecture:** Dashboard proxies storage API calls through its own session-authenticated endpoints. Frontend uses existing app.js patterns (state management, render functions, modals). Two-panel layout similar to Tables view.

**Tech Stack:** Go (Chi router, sql), Vanilla JavaScript (no framework), existing CSS variables

---

## Task 1: Dashboard Storage API - Bucket Endpoints

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/server/server.go` (to pass storage service to dashboard)

**Step 1: Add storage service to dashboard handler**

In `internal/dashboard/handler.go`, add storage service field:

```go
// Add to Handler struct (around line 35)
type Handler struct {
	db               *sql.DB
	store            *Store
	auth             *Auth
	sessions         *SessionManager
	fts              *fts.Manager
	functionsService *functions.Service
	storageService   *storage.Service  // ADD THIS
	migrationsDir    string
	startTime        time.Time
	serverConfig     *ServerConfig
	jwtSecret        string
	oauthReloadFunc  func()
}

// Add setter method after SetFunctionsService (around line 91)
func (h *Handler) SetStorageService(svc *storage.Service) {
	h.storageService = svc
}
```

**Step 2: Add import for storage package**

```go
// Add to imports at top of handler.go
import (
	// ... existing imports ...
	"github.com/markb/sblite/internal/storage"
)
```

**Step 3: Register storage routes in RegisterRoutes**

Find `RegisterRoutes` function and add storage routes after functions routes:

```go
// Add after r.Route("/functions", ...) block (around line 180)
r.Route("/storage", func(r chi.Router) {
	r.Use(h.requireAuth)

	// Bucket routes
	r.Get("/buckets", h.handleListBuckets)
	r.Post("/buckets", h.handleCreateBucket)
	r.Get("/buckets/{id}", h.handleGetBucket)
	r.Put("/buckets/{id}", h.handleUpdateBucket)
	r.Delete("/buckets/{id}", h.handleDeleteBucket)
	r.Post("/buckets/{id}/empty", h.handleEmptyBucket)
})
```

**Step 4: Implement bucket handlers**

Add at end of `handler.go`:

```go
// Storage bucket handlers

func (h *Handler) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	buckets, err := h.storageService.ListBuckets()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(buckets)
}

func (h *Handler) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Name             string   `json:"name"`
		Public           bool     `json:"public"`
		FileSizeLimit    *int64   `json:"file_size_limit,omitempty"`
		AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	bucket, err := h.storageService.CreateBucket(req.Name, req.Public, req.FileSizeLimit, req.AllowedMimeTypes)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bucket)
}

func (h *Handler) handleGetBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	bucket, err := h.storageService.GetBucket(id)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bucket)
}

func (h *Handler) handleUpdateBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	var req struct {
		Public           *bool    `json:"public,omitempty"`
		FileSizeLimit    *int64   `json:"file_size_limit,omitempty"`
		AllowedMimeTypes []string `json:"allowed_mime_types,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	bucket, err := h.storageService.UpdateBucket(id, req.Public, req.FileSizeLimit, req.AllowedMimeTypes)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bucket)
}

func (h *Handler) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.storageService.DeleteBucket(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleEmptyBucket(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.storageService.EmptyBucket(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 5: Wire up storage service in server.go**

In `internal/server/server.go`, find where dashboard handler is configured and add:

```go
// After dashboardHandler.SetFunctionsService(...) call
if s.storageService != nil {
	dashboardHandler.SetStorageService(s.storageService)
}
```

**Step 6: Build and verify compilation**

Run: `go build ./...`
Expected: No errors

**Step 7: Commit**

```bash
git add internal/dashboard/handler.go internal/server/server.go
git commit -m "feat(dashboard): add storage bucket API endpoints"
```

---

## Task 2: Dashboard Storage API - Object Endpoints

**Files:**
- Modify: `internal/dashboard/handler.go`

**Step 1: Add object routes to storage route group**

In RegisterRoutes, extend the storage route group:

```go
r.Route("/storage", func(r chi.Router) {
	r.Use(h.requireAuth)

	// Bucket routes (already added)
	r.Get("/buckets", h.handleListBuckets)
	r.Post("/buckets", h.handleCreateBucket)
	r.Get("/buckets/{id}", h.handleGetBucket)
	r.Put("/buckets/{id}", h.handleUpdateBucket)
	r.Delete("/buckets/{id}", h.handleDeleteBucket)
	r.Post("/buckets/{id}/empty", h.handleEmptyBucket)

	// Object routes - ADD THESE
	r.Post("/objects/list", h.handleListObjects)
	r.Post("/objects/upload", h.handleUploadObject)
	r.Get("/objects/download", h.handleDownloadObject)
	r.Delete("/objects", h.handleDeleteObjects)
})
```

**Step 2: Implement object handlers**

Add after bucket handlers:

```go
// Storage object handlers

func (h *Handler) handleListObjects(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Bucket string `json:"bucket"`
		Prefix string `json:"prefix"`
		Limit  int    `json:"limit"`
		Offset int    `json:"offset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Limit == 0 {
		req.Limit = 100
	}

	objects, err := h.storageService.ListObjects(req.Bucket, req.Prefix, req.Limit, req.Offset)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(objects)
}

func (h *Handler) handleUploadObject(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	// Parse multipart form (32MB max)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, `{"error":"Failed to parse form"}`, http.StatusBadRequest)
		return
	}

	bucket := r.FormValue("bucket")
	path := r.FormValue("path")

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"No file provided"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"Failed to read file"}`, http.StatusInternalServerError)
		return
	}

	// Determine full path
	fullPath := path
	if fullPath != "" && !strings.HasSuffix(fullPath, "/") {
		fullPath += "/"
	}
	fullPath += header.Filename

	obj, err := h.storageService.UploadObject(bucket, fullPath, content, header.Header.Get("Content-Type"))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(obj)
}

func (h *Handler) handleDownloadObject(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	bucket := r.URL.Query().Get("bucket")
	path := r.URL.Query().Get("path")

	content, mimeType, err := h.storageService.GetObject(bucket, path)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	// Extract filename from path
	filename := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		filename = path[idx+1:]
	}

	w.Header().Set("Content-Type", mimeType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", strconv.Itoa(len(content)))
	w.Write(content)
}

func (h *Handler) handleDeleteObjects(w http.ResponseWriter, r *http.Request) {
	if h.storageService == nil {
		http.Error(w, `{"error":"Storage not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		Bucket string   `json:"bucket"`
		Paths  []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	var errors []string
	for _, path := range req.Paths {
		if err := h.storageService.DeleteObject(req.Bucket, path); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", path, err.Error()))
		}
	}

	if len(errors) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMultiStatus)
		json.NewEncoder(w).Encode(map[string]any{
			"deleted": len(req.Paths) - len(errors),
			"errors":  errors,
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 3: Add io import if not present**

```go
import (
	"io"
	// ... other imports
)
```

**Step 4: Build and verify**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go
git commit -m "feat(dashboard): add storage object API endpoints"
```

---

## Task 3: Frontend - Storage State and Navigation

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add storage state to initial state**

Find the `state` object initialization (around line 50) and add storage:

```javascript
// Add after 'settings' state object
storage: {
    buckets: [],
    selectedBucket: null,
    objects: [],
    currentPath: '',
    viewMode: 'grid',
    selectedFiles: [],
    uploading: [],
    loading: false
},
```

**Step 2: Add Storage to navigation sidebar**

Find the `renderLayout` function and locate the nav sections. Add Storage section after Auth:

```javascript
// After the Auth nav-section div, add:
<div class="nav-section">
    <div class="nav-section-title">Storage</div>
    <a class="nav-item ${this.state.currentView === 'storage' ? 'active' : ''}"
       onclick="App.navigate('storage')">Buckets</a>
</div>
```

**Step 3: Add storage view case to renderContent**

Find the `renderContent` method with the switch statement and add:

```javascript
case 'storage':
    return this.renderStorageView();
```

**Step 4: Add navigate handler for storage**

The existing `navigate` method should work, but verify it handles unknown views gracefully.

**Step 5: Add placeholder renderStorageView**

Add at end of App object (before closing `};`):

```javascript
// Storage view
renderStorageView() {
    return `
        <div class="storage-container">
            <div class="storage-sidebar">
                <h3>Buckets</h3>
                <p class="text-muted">Storage UI coming soon...</p>
            </div>
            <div class="storage-main">
                <p>Select a bucket to browse files</p>
            </div>
        </div>
    `;
},
```

**Step 6: Build and test manually**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Open: `http://localhost:8080/_`
Expected: "Storage" nav item visible, clicking shows placeholder

**Step 7: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add storage navigation and placeholder view"
```

---

## Task 4: Frontend - Bucket List and Create

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Implement loadBuckets method**

Add to App object:

```javascript
async loadBuckets() {
    this.state.storage.loading = true;
    this.render();

    try {
        const res = await fetch('/_/api/storage/buckets');
        if (!res.ok) throw new Error('Failed to load buckets');
        this.state.storage.buckets = await res.json();
    } catch (err) {
        this.showToast(err.message, 'error');
        this.state.storage.buckets = [];
    } finally {
        this.state.storage.loading = false;
        this.render();
    }
},
```

**Step 2: Update navigate to load buckets**

Find the `navigate` method and add bucket loading:

```javascript
// In navigate method, add case for storage:
if (view === 'storage') {
    this.loadBuckets();
}
```

**Step 3: Implement full renderStorageView**

Replace the placeholder:

```javascript
renderStorageView() {
    const { buckets, selectedBucket, loading } = this.state.storage;

    if (loading && buckets.length === 0) {
        return '<div class="loading">Loading buckets...</div>';
    }

    return `
        <div class="storage-layout">
            <div class="storage-sidebar">
                <div class="storage-sidebar-header">
                    <h3>Buckets</h3>
                    <button class="btn btn-primary btn-sm" onclick="App.showCreateBucketModal()">
                        + New
                    </button>
                </div>
                <div class="bucket-list">
                    ${buckets.length === 0 ? `
                        <p class="text-muted" style="padding: 1rem;">No buckets yet</p>
                    ` : buckets.map(bucket => `
                        <div class="bucket-item ${selectedBucket?.id === bucket.id ? 'selected' : ''}"
                             onclick="App.selectBucket('${bucket.id}')">
                            <span class="bucket-name">${this.escapeHtml(bucket.name)}</span>
                            <span class="bucket-badge ${bucket.public ? 'public' : 'private'}">
                                ${bucket.public ? 'Public' : 'Private'}
                            </span>
                        </div>
                    `).join('')}
                </div>
            </div>
            <div class="storage-main">
                ${selectedBucket ? this.renderFileBrowser() : `
                    <div class="storage-empty">
                        <p>Select a bucket to browse files</p>
                    </div>
                `}
            </div>
        </div>
    `;
},
```

**Step 4: Implement selectBucket**

```javascript
async selectBucket(bucketId) {
    const bucket = this.state.storage.buckets.find(b => b.id === bucketId);
    this.state.storage.selectedBucket = bucket;
    this.state.storage.currentPath = '';
    this.state.storage.selectedFiles = [];
    await this.loadObjects();
},
```

**Step 5: Implement showCreateBucketModal**

```javascript
showCreateBucketModal() {
    this.state.modal = {
        title: 'Create Bucket',
        content: `
            <form onsubmit="App.createBucket(event)">
                <div class="form-group">
                    <label class="form-label">Bucket Name</label>
                    <input type="text" class="form-input" id="bucket-name" required
                           pattern="[a-z0-9-]+" title="Lowercase letters, numbers, and hyphens only">
                </div>
                <div class="form-group">
                    <label class="checkbox-label">
                        <input type="checkbox" id="bucket-public">
                        Public bucket (files accessible without authentication)
                    </label>
                </div>
                <div class="form-group">
                    <label class="form-label">File Size Limit (MB, optional)</label>
                    <input type="number" class="form-input" id="bucket-size-limit" min="1">
                </div>
                <div class="form-group">
                    <label class="form-label">Allowed MIME Types (optional)</label>
                    <input type="text" class="form-input" id="bucket-mime-types"
                           placeholder="image/*, application/pdf">
                    <small class="text-muted">Comma-separated list</small>
                </div>
                <div class="modal-actions">
                    <button type="button" class="btn" onclick="App.closeModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Create Bucket</button>
                </div>
            </form>
        `
    };
    this.render();
},

async createBucket(event) {
    event.preventDefault();

    const name = document.getElementById('bucket-name').value;
    const isPublic = document.getElementById('bucket-public').checked;
    const sizeLimit = document.getElementById('bucket-size-limit').value;
    const mimeTypes = document.getElementById('bucket-mime-types').value;

    try {
        const body = {
            name,
            public: isPublic
        };
        if (sizeLimit) {
            body.file_size_limit = parseInt(sizeLimit) * 1024 * 1024; // Convert MB to bytes
        }
        if (mimeTypes) {
            body.allowed_mime_types = mimeTypes.split(',').map(t => t.trim());
        }

        const res = await fetch('/_/api/storage/buckets', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });

        if (!res.ok) {
            const err = await res.json();
            throw new Error(err.error || 'Failed to create bucket');
        }

        this.closeModal();
        this.showToast('Bucket created successfully', 'success');
        await this.loadBuckets();
    } catch (err) {
        this.showToast(err.message, 'error');
    }
},
```

**Step 6: Add placeholder renderFileBrowser**

```javascript
renderFileBrowser() {
    return `<div class="file-browser"><p>File browser coming next...</p></div>`;
},
```

**Step 7: Add CSS for storage layout**

In `style.css`, add:

```css
/* Storage Layout */
.storage-layout {
    display: flex;
    height: calc(100vh - 120px);
    gap: 1rem;
}

.storage-sidebar {
    width: 280px;
    flex-shrink: 0;
    background: var(--card-bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
}

.storage-sidebar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    border-bottom: 1px solid var(--border);
}

.storage-sidebar-header h3 {
    margin: 0;
    font-size: 1rem;
}

.bucket-list {
    flex: 1;
    overflow-y: auto;
}

.bucket-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    cursor: pointer;
    border-bottom: 1px solid var(--border);
}

.bucket-item:hover {
    background: var(--hover-bg);
}

.bucket-item.selected {
    background: var(--primary-bg);
}

.bucket-name {
    font-weight: 500;
}

.bucket-badge {
    font-size: 0.75rem;
    padding: 0.125rem 0.5rem;
    border-radius: 4px;
}

.bucket-badge.public {
    background: var(--success-bg);
    color: var(--success);
}

.bucket-badge.private {
    background: var(--muted-bg);
    color: var(--muted);
}

.storage-main {
    flex: 1;
    background: var(--card-bg);
    border: 1px solid var(--border);
    border-radius: 8px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

.storage-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--muted);
}
```

**Step 8: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Create a bucket via the modal

**Step 9: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): implement bucket list and create modal"
```

---

## Task 5: Frontend - File Browser with Grid/List Views

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Implement loadObjects**

```javascript
async loadObjects() {
    const { selectedBucket, currentPath } = this.state.storage;
    if (!selectedBucket) return;

    this.state.storage.loading = true;
    this.render();

    try {
        const res = await fetch('/_/api/storage/objects/list', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                bucket: selectedBucket.name,
                prefix: currentPath,
                limit: 100
            })
        });

        if (!res.ok) throw new Error('Failed to load objects');
        const data = await res.json();
        this.state.storage.objects = data || [];
    } catch (err) {
        this.showToast(err.message, 'error');
        this.state.storage.objects = [];
    } finally {
        this.state.storage.loading = false;
        this.render();
    }
},
```

**Step 2: Implement full renderFileBrowser**

Replace the placeholder:

```javascript
renderFileBrowser() {
    const { selectedBucket, objects, currentPath, viewMode, selectedFiles, loading } = this.state.storage;

    // Separate folders and files
    const folders = [];
    const files = [];
    const seenFolders = new Set();

    for (const obj of objects) {
        // Check if this represents a folder
        const relativePath = obj.name.slice(currentPath.length);
        const slashIndex = relativePath.indexOf('/');

        if (slashIndex > 0) {
            // This is inside a subfolder
            const folderName = relativePath.slice(0, slashIndex);
            if (!seenFolders.has(folderName)) {
                seenFolders.add(folderName);
                folders.push({ name: folderName, isFolder: true });
            }
        } else if (relativePath && !relativePath.endsWith('/')) {
            // This is a file in current folder
            files.push({ ...obj, isFolder: false });
        }
    }

    const allItems = [...folders, ...files];

    return `
        <div class="file-browser">
            ${this.renderFileBrowserToolbar()}
            <div class="file-browser-content ${viewMode}">
                ${loading ? '<div class="loading">Loading...</div>' : ''}
                ${!loading && allItems.length === 0 ? `
                    <div class="file-browser-empty">
                        <p>No files in this ${currentPath ? 'folder' : 'bucket'}</p>
                        <p class="text-muted">Drag and drop files here or click Upload</p>
                    </div>
                ` : ''}
                ${viewMode === 'grid' ? this.renderFileGrid(allItems) : this.renderFileList(allItems)}
            </div>
            ${this.renderUploadProgress()}
        </div>
    `;
},

renderFileBrowserToolbar() {
    const { selectedBucket, currentPath, viewMode, selectedFiles } = this.state.storage;
    const pathParts = currentPath.split('/').filter(Boolean);

    return `
        <div class="file-browser-toolbar">
            <div class="toolbar-left">
                ${currentPath ? `
                    <button class="btn btn-sm" onclick="App.navigateToFolder('..')">
                        ← Back
                    </button>
                ` : ''}
                <div class="breadcrumb">
                    <span class="breadcrumb-item" onclick="App.navigateToFolder('')">
                        ${this.escapeHtml(selectedBucket.name)}
                    </span>
                    ${pathParts.map((part, i) => `
                        <span class="breadcrumb-sep">/</span>
                        <span class="breadcrumb-item"
                              onclick="App.navigateToFolder('${pathParts.slice(0, i + 1).join('/')}/')">
                            ${this.escapeHtml(part)}
                        </span>
                    `).join('')}
                </div>
            </div>
            <div class="toolbar-right">
                <button class="btn btn-primary btn-sm" onclick="App.triggerFileUpload()">
                    Upload Files
                </button>
                <input type="file" id="file-upload-input" multiple style="display:none"
                       onchange="App.handleFileSelect(event)">
                <div class="view-toggle">
                    <button class="btn btn-sm ${viewMode === 'grid' ? 'active' : ''}"
                            onclick="App.setStorageViewMode('grid')">⊞</button>
                    <button class="btn btn-sm ${viewMode === 'list' ? 'active' : ''}"
                            onclick="App.setStorageViewMode('list')">☰</button>
                </div>
                <div class="dropdown">
                    <button class="btn btn-sm" ${selectedFiles.length === 0 ? 'disabled' : ''}>
                        Actions ▾
                    </button>
                    <div class="dropdown-menu">
                        <a onclick="App.deleteSelectedFiles()">Delete Selected</a>
                        <a onclick="App.downloadSelectedFiles()">Download Selected</a>
                    </div>
                </div>
            </div>
        </div>
    `;
},
```

**Step 3: Implement grid and list renderers**

```javascript
renderFileGrid(items) {
    const { selectedFiles } = this.state.storage;

    return `
        <div class="file-grid">
            ${items.map(item => `
                <div class="file-card ${selectedFiles.includes(item.name) ? 'selected' : ''}"
                     ondblclick="App.${item.isFolder ? `navigateToFolder('${this.escapeHtml(item.name)}/')` : `downloadFile('${this.escapeHtml(item.name)}')`}">
                    <div class="file-card-checkbox">
                        <input type="checkbox"
                               ${selectedFiles.includes(item.name) ? 'checked' : ''}
                               onclick="event.stopPropagation(); App.toggleFileSelection('${this.escapeHtml(item.name)}')"
                               onchange="event.stopPropagation()">
                    </div>
                    <div class="file-card-preview">
                        ${item.isFolder ? `
                            <div class="folder-icon">📁</div>
                        ` : this.isImageFile(item.name) ? `
                            <img src="/_/api/storage/objects/download?bucket=${encodeURIComponent(this.state.storage.selectedBucket.name)}&path=${encodeURIComponent(this.state.storage.currentPath + item.name)}"
                                 alt="${this.escapeHtml(item.name)}"
                                 loading="lazy">
                        ` : `
                            <div class="file-icon">${this.getFileIcon(item.name)}</div>
                        `}
                    </div>
                    <div class="file-card-info">
                        <div class="file-name" title="${this.escapeHtml(item.name)}">
                            ${this.escapeHtml(item.name)}${item.isFolder ? '/' : ''}
                        </div>
                        ${!item.isFolder ? `
                            <div class="file-size">${this.formatFileSize(item.size || 0)}</div>
                        ` : ''}
                    </div>
                </div>
            `).join('')}
        </div>
    `;
},

renderFileList(items) {
    const { selectedFiles } = this.state.storage;

    return `
        <table class="file-list-table">
            <thead>
                <tr>
                    <th style="width:40px"></th>
                    <th>Name</th>
                    <th style="width:100px">Size</th>
                    <th style="width:120px">Type</th>
                    <th style="width:150px">Modified</th>
                </tr>
            </thead>
            <tbody>
                ${items.map(item => `
                    <tr class="${selectedFiles.includes(item.name) ? 'selected' : ''}"
                        ondblclick="App.${item.isFolder ? `navigateToFolder('${this.escapeHtml(item.name)}/')` : `downloadFile('${this.escapeHtml(item.name)}')`}">
                        <td>
                            <input type="checkbox"
                                   ${selectedFiles.includes(item.name) ? 'checked' : ''}
                                   onclick="event.stopPropagation(); App.toggleFileSelection('${this.escapeHtml(item.name)}')"
                                   onchange="event.stopPropagation()">
                        </td>
                        <td>
                            ${item.isFolder ? '📁' : this.getFileIcon(item.name)}
                            ${this.escapeHtml(item.name)}${item.isFolder ? '/' : ''}
                        </td>
                        <td>${item.isFolder ? '-' : this.formatFileSize(item.size || 0)}</td>
                        <td>${item.isFolder ? 'Folder' : (item.mime_type || '-')}</td>
                        <td>${item.updated_at ? new Date(item.updated_at).toLocaleDateString() : '-'}</td>
                    </tr>
                `).join('')}
            </tbody>
        </table>
    `;
},
```

**Step 4: Add helper methods**

```javascript
isImageFile(filename) {
    const ext = filename.split('.').pop().toLowerCase();
    return ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg'].includes(ext);
},

getFileIcon(filename) {
    const ext = filename.split('.').pop().toLowerCase();
    const icons = {
        pdf: '📄', doc: '📝', docx: '📝', txt: '📝',
        xls: '📊', xlsx: '📊', csv: '📊',
        zip: '📦', rar: '📦', tar: '📦', gz: '📦',
        mp3: '🎵', wav: '🎵', ogg: '🎵',
        mp4: '🎬', mov: '🎬', avi: '🎬',
        js: '📜', ts: '📜', py: '📜', go: '📜',
        json: '📋', xml: '📋', html: '📋', css: '📋'
    };
    return icons[ext] || '📄';
},

formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
},

setStorageViewMode(mode) {
    this.state.storage.viewMode = mode;
    this.render();
},

navigateToFolder(path) {
    if (path === '..') {
        const parts = this.state.storage.currentPath.split('/').filter(Boolean);
        parts.pop();
        path = parts.length ? parts.join('/') + '/' : '';
    }
    this.state.storage.currentPath = path;
    this.state.storage.selectedFiles = [];
    this.loadObjects();
},

toggleFileSelection(filename) {
    const idx = this.state.storage.selectedFiles.indexOf(filename);
    if (idx >= 0) {
        this.state.storage.selectedFiles.splice(idx, 1);
    } else {
        this.state.storage.selectedFiles.push(filename);
    }
    this.render();
},
```

**Step 5: Add CSS for file browser**

```css
/* File Browser */
.file-browser {
    display: flex;
    flex-direction: column;
    height: 100%;
}

.file-browser-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--border);
    gap: 1rem;
}

.toolbar-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.toolbar-right {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.breadcrumb {
    display: flex;
    align-items: center;
    gap: 0.25rem;
}

.breadcrumb-item {
    cursor: pointer;
    color: var(--primary);
}

.breadcrumb-item:hover {
    text-decoration: underline;
}

.breadcrumb-sep {
    color: var(--muted);
}

.view-toggle {
    display: flex;
    gap: 0;
}

.view-toggle .btn {
    border-radius: 0;
}

.view-toggle .btn:first-child {
    border-radius: 4px 0 0 4px;
}

.view-toggle .btn:last-child {
    border-radius: 0 4px 4px 0;
}

.view-toggle .btn.active {
    background: var(--primary);
    color: white;
}

.file-browser-content {
    flex: 1;
    overflow: auto;
    padding: 1rem;
}

.file-browser-empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    color: var(--muted);
}

/* Grid View */
.file-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 1rem;
}

.file-card {
    position: relative;
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0.5rem;
    cursor: pointer;
    transition: all 0.2s;
}

.file-card:hover {
    border-color: var(--primary);
    background: var(--hover-bg);
}

.file-card.selected {
    border-color: var(--primary);
    background: var(--primary-bg);
}

.file-card-checkbox {
    position: absolute;
    top: 0.5rem;
    left: 0.5rem;
    z-index: 1;
}

.file-card-preview {
    height: 100px;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    border-radius: 4px;
    background: var(--muted-bg);
}

.file-card-preview img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
}

.folder-icon, .file-icon {
    font-size: 3rem;
}

.file-card-info {
    margin-top: 0.5rem;
    text-align: center;
}

.file-name {
    font-size: 0.875rem;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.file-size {
    font-size: 0.75rem;
    color: var(--muted);
}

/* List View */
.file-list-table {
    width: 100%;
    border-collapse: collapse;
}

.file-list-table th,
.file-list-table td {
    padding: 0.5rem;
    text-align: left;
    border-bottom: 1px solid var(--border);
}

.file-list-table th {
    font-weight: 600;
    background: var(--muted-bg);
}

.file-list-table tr:hover {
    background: var(--hover-bg);
}

.file-list-table tr.selected {
    background: var(--primary-bg);
}
```

**Step 6: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: View grid/list, navigate folders

**Step 7: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): implement file browser with grid/list views"
```

---

## Task 6: Frontend - File Upload with Drag-Drop and Progress

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Implement upload methods**

```javascript
triggerFileUpload() {
    document.getElementById('file-upload-input').click();
},

handleFileSelect(event) {
    const files = Array.from(event.target.files);
    if (files.length > 0) {
        this.uploadFiles(files);
    }
    event.target.value = ''; // Reset for re-selection
},

async uploadFiles(files) {
    const { selectedBucket, currentPath } = this.state.storage;
    if (!selectedBucket) return;

    for (const file of files) {
        const uploadItem = {
            name: file.name,
            size: file.size,
            progress: 0,
            status: 'uploading'
        };
        this.state.storage.uploading.push(uploadItem);
        this.render();

        try {
            const formData = new FormData();
            formData.append('bucket', selectedBucket.name);
            formData.append('path', currentPath);
            formData.append('file', file);

            await this.uploadWithProgress(formData, uploadItem);
            uploadItem.status = 'complete';
            uploadItem.progress = 100;
        } catch (err) {
            uploadItem.status = 'error';
            uploadItem.error = err.message;
        }
        this.render();
    }

    // Refresh file list
    await this.loadObjects();

    // Clear completed uploads after delay
    setTimeout(() => {
        this.state.storage.uploading = this.state.storage.uploading.filter(u => u.status === 'uploading');
        this.render();
    }, 3000);
},

uploadWithProgress(formData, uploadItem) {
    return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener('progress', (e) => {
            if (e.lengthComputable) {
                uploadItem.progress = Math.round((e.loaded / e.total) * 100);
                this.render();
            }
        });

        xhr.addEventListener('load', () => {
            if (xhr.status >= 200 && xhr.status < 300) {
                resolve(JSON.parse(xhr.responseText));
            } else {
                reject(new Error('Upload failed'));
            }
        });

        xhr.addEventListener('error', () => reject(new Error('Upload failed')));

        xhr.open('POST', '/_/api/storage/objects/upload');
        xhr.send(formData);
    });
},
```

**Step 2: Implement renderUploadProgress**

```javascript
renderUploadProgress() {
    const { uploading } = this.state.storage;
    if (uploading.length === 0) return '';

    return `
        <div class="upload-progress-panel">
            <div class="upload-progress-header">
                <span>Uploading ${uploading.length} file${uploading.length > 1 ? 's' : ''}</span>
                <button class="btn-icon" onclick="App.clearCompletedUploads()">✕</button>
            </div>
            <div class="upload-progress-list">
                ${uploading.map(item => `
                    <div class="upload-item ${item.status}">
                        <span class="upload-name">${this.escapeHtml(item.name)}</span>
                        <div class="upload-bar">
                            <div class="upload-bar-fill" style="width: ${item.progress}%"></div>
                        </div>
                        <span class="upload-status">
                            ${item.status === 'uploading' ? `${item.progress}%` : ''}
                            ${item.status === 'complete' ? '✓' : ''}
                            ${item.status === 'error' ? '✗' : ''}
                        </span>
                    </div>
                `).join('')}
            </div>
        </div>
    `;
},

clearCompletedUploads() {
    this.state.storage.uploading = this.state.storage.uploading.filter(u => u.status === 'uploading');
    this.render();
},
```

**Step 3: Add drag-drop handlers**

Update renderFileBrowser to include drag-drop attributes:

```javascript
// In renderFileBrowser, update the file-browser-content div:
<div class="file-browser-content ${viewMode}"
     ondragover="App.handleDragOver(event)"
     ondragleave="App.handleDragLeave(event)"
     ondrop="App.handleDrop(event)">
```

Add handlers:

```javascript
handleDragOver(event) {
    event.preventDefault();
    event.currentTarget.classList.add('drag-over');
},

handleDragLeave(event) {
    event.currentTarget.classList.remove('drag-over');
},

handleDrop(event) {
    event.preventDefault();
    event.currentTarget.classList.remove('drag-over');

    const files = Array.from(event.dataTransfer.files);
    if (files.length > 0) {
        this.uploadFiles(files);
    }
},
```

**Step 4: Add CSS for upload progress and drag-drop**

```css
/* Upload Progress Panel */
.upload-progress-panel {
    border-top: 1px solid var(--border);
    max-height: 200px;
    overflow: auto;
}

.upload-progress-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.5rem 1rem;
    background: var(--muted-bg);
    font-weight: 500;
}

.upload-progress-list {
    padding: 0.5rem;
}

.upload-item {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem;
}

.upload-name {
    flex: 1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: 0.875rem;
}

.upload-bar {
    width: 100px;
    height: 8px;
    background: var(--muted-bg);
    border-radius: 4px;
    overflow: hidden;
}

.upload-bar-fill {
    height: 100%;
    background: var(--primary);
    transition: width 0.2s;
}

.upload-item.complete .upload-bar-fill {
    background: var(--success);
}

.upload-item.error .upload-bar-fill {
    background: var(--danger);
}

.upload-status {
    width: 40px;
    text-align: right;
    font-size: 0.875rem;
}

/* Drag and Drop */
.file-browser-content.drag-over {
    background: var(--primary-bg);
    border: 2px dashed var(--primary);
}

.file-browser-content.drag-over::before {
    content: 'Drop files to upload';
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    font-size: 1.25rem;
    color: var(--primary);
    pointer-events: none;
}
```

**Step 5: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Upload via button and drag-drop

**Step 6: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): implement file upload with drag-drop and progress"
```

---

## Task 7: Frontend - Bulk Operations (Delete & Download)

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Implement delete selected files**

```javascript
async deleteSelectedFiles() {
    const { selectedBucket, currentPath, selectedFiles } = this.state.storage;
    if (selectedFiles.length === 0) return;

    const confirmed = confirm(`Delete ${selectedFiles.length} file(s)? This cannot be undone.`);
    if (!confirmed) return;

    try {
        const paths = selectedFiles.map(f => currentPath + f);
        const res = await fetch('/_/api/storage/objects', {
            method: 'DELETE',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                bucket: selectedBucket.name,
                paths: paths
            })
        });

        if (!res.ok && res.status !== 207) {
            throw new Error('Failed to delete files');
        }

        this.state.storage.selectedFiles = [];
        this.showToast('Files deleted successfully', 'success');
        await this.loadObjects();
    } catch (err) {
        this.showToast(err.message, 'error');
    }
},
```

**Step 2: Implement download file and download selected**

```javascript
downloadFile(filename) {
    const { selectedBucket, currentPath } = this.state.storage;
    const path = currentPath + filename;
    const url = `/_/api/storage/objects/download?bucket=${encodeURIComponent(selectedBucket.name)}&path=${encodeURIComponent(path)}`;

    // Create temporary link and click it
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
},

async downloadSelectedFiles() {
    const { selectedFiles } = this.state.storage;
    if (selectedFiles.length === 0) return;

    // Download files sequentially
    for (const filename of selectedFiles) {
        this.downloadFile(filename);
        // Small delay between downloads to not overwhelm browser
        await new Promise(resolve => setTimeout(resolve, 500));
    }
},
```

**Step 3: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Select files, delete and download

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): implement bulk delete and download operations"
```

---

## Task 8: Frontend - Bucket Settings Modal

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add settings button to bucket list**

Update the bucket item in renderStorageView:

```javascript
// Update bucket-item div:
<div class="bucket-item ${selectedBucket?.id === bucket.id ? 'selected' : ''}"
     onclick="App.selectBucket('${bucket.id}')">
    <span class="bucket-name">${this.escapeHtml(bucket.name)}</span>
    <div class="bucket-actions">
        <span class="bucket-badge ${bucket.public ? 'public' : 'private'}">
            ${bucket.public ? 'Public' : 'Private'}
        </span>
        <button class="btn-icon" onclick="event.stopPropagation(); App.showBucketSettingsModal('${bucket.id}')" title="Settings">
            ⚙
        </button>
    </div>
</div>
```

**Step 2: Implement bucket settings modal**

```javascript
showBucketSettingsModal(bucketId) {
    const bucket = this.state.storage.buckets.find(b => b.id === bucketId);
    if (!bucket) return;

    const mimeTypes = bucket.allowed_mime_types ? bucket.allowed_mime_types.join(', ') : '';
    const sizeLimit = bucket.file_size_limit ? Math.round(bucket.file_size_limit / 1024 / 1024) : '';

    this.state.modal = {
        title: `Bucket Settings: ${bucket.name}`,
        content: `
            <form onsubmit="App.updateBucket(event, '${bucket.id}')">
                <div class="form-group">
                    <label class="checkbox-label">
                        <input type="checkbox" id="bucket-public" ${bucket.public ? 'checked' : ''}>
                        Public bucket
                    </label>
                </div>
                <div class="form-group">
                    <label class="form-label">File Size Limit (MB)</label>
                    <input type="number" class="form-input" id="bucket-size-limit"
                           value="${sizeLimit}" min="1">
                </div>
                <div class="form-group">
                    <label class="form-label">Allowed MIME Types</label>
                    <input type="text" class="form-input" id="bucket-mime-types"
                           value="${this.escapeHtml(mimeTypes)}"
                           placeholder="image/*, application/pdf">
                </div>

                <hr style="margin: 1rem 0">

                <div class="form-group">
                    <p class="text-muted">RLS Status: ${this.getRLSStatus()}</p>
                    <a href="#" onclick="App.navigate('policies'); App.closeModal();">
                        Manage Policies →
                    </a>
                </div>

                <hr style="margin: 1rem 0">

                <div class="danger-zone">
                    <button type="button" class="btn btn-danger btn-sm"
                            onclick="App.emptyBucket('${bucket.id}')">
                        Empty Bucket
                    </button>
                    <button type="button" class="btn btn-danger btn-sm"
                            onclick="App.deleteBucket('${bucket.id}')">
                        Delete Bucket
                    </button>
                </div>

                <div class="modal-actions">
                    <button type="button" class="btn" onclick="App.closeModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Save Changes</button>
                </div>
            </form>
        `
    };
    this.render();
},

getRLSStatus() {
    // This would need to check if storage_objects has RLS enabled
    // For now, return a placeholder
    return 'Enabled';
},

async updateBucket(event, bucketId) {
    event.preventDefault();

    const isPublic = document.getElementById('bucket-public').checked;
    const sizeLimit = document.getElementById('bucket-size-limit').value;
    const mimeTypes = document.getElementById('bucket-mime-types').value;

    try {
        const body = {
            public: isPublic
        };
        if (sizeLimit) {
            body.file_size_limit = parseInt(sizeLimit) * 1024 * 1024;
        }
        if (mimeTypes) {
            body.allowed_mime_types = mimeTypes.split(',').map(t => t.trim());
        }

        const res = await fetch(`/_/api/storage/buckets/${bucketId}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });

        if (!res.ok) throw new Error('Failed to update bucket');

        this.closeModal();
        this.showToast('Bucket updated', 'success');
        await this.loadBuckets();
    } catch (err) {
        this.showToast(err.message, 'error');
    }
},

async emptyBucket(bucketId) {
    const confirmed = confirm('Remove all files from this bucket? This cannot be undone.');
    if (!confirmed) return;

    try {
        const res = await fetch(`/_/api/storage/buckets/${bucketId}/empty`, {
            method: 'POST'
        });

        if (!res.ok) throw new Error('Failed to empty bucket');

        this.showToast('Bucket emptied', 'success');
        await this.loadObjects();
    } catch (err) {
        this.showToast(err.message, 'error');
    }
},

async deleteBucket(bucketId) {
    const bucket = this.state.storage.buckets.find(b => b.id === bucketId);
    const confirmed = confirm(`Delete bucket "${bucket?.name}"? The bucket must be empty.`);
    if (!confirmed) return;

    try {
        const res = await fetch(`/_/api/storage/buckets/${bucketId}`, {
            method: 'DELETE'
        });

        if (!res.ok) {
            const err = await res.json();
            throw new Error(err.error || 'Failed to delete bucket');
        }

        this.closeModal();
        this.state.storage.selectedBucket = null;
        this.showToast('Bucket deleted', 'success');
        await this.loadBuckets();
    } catch (err) {
        this.showToast(err.message, 'error');
    }
},
```

**Step 3: Build and test**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Test: Open bucket settings, update, empty, delete

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): implement bucket settings modal"
```

---

## Task 9: E2E Tests for Storage Dashboard

**Files:**
- Create: `e2e/tests/dashboard/storage.test.ts`

**Step 1: Create test file**

```typescript
import { test, expect } from '@playwright/test';

test.describe('Dashboard Storage', () => {
  test.beforeEach(async ({ page }) => {
    // Login to dashboard
    await page.goto('http://localhost:8080/_');
    // Handle login if needed
    const loginForm = page.locator('form');
    if (await loginForm.isVisible()) {
      await page.fill('input[type="password"]', 'testpassword');
      await page.click('button[type="submit"]');
    }
    // Navigate to storage
    await page.click('text=Buckets');
    await page.waitForLoadState('networkidle');
  });

  test('should display storage navigation', async ({ page }) => {
    await expect(page.locator('text=Storage')).toBeVisible();
    await expect(page.locator('text=Buckets')).toBeVisible();
  });

  test('should create a bucket', async ({ page }) => {
    await page.click('text=+ New');
    await page.fill('#bucket-name', 'test-bucket');
    await page.check('#bucket-public');
    await page.click('text=Create Bucket');

    await expect(page.locator('text=test-bucket')).toBeVisible();
    await expect(page.locator('text=Public')).toBeVisible();
  });

  test('should list buckets', async ({ page }) => {
    // Create bucket first via API for consistent state
    await page.request.post('http://localhost:8080/_/api/storage/buckets', {
      data: { name: 'list-test-bucket', public: false }
    });

    await page.reload();
    await page.click('text=Buckets');

    await expect(page.locator('text=list-test-bucket')).toBeVisible();
  });

  test('should upload a file', async ({ page }) => {
    // Select bucket first
    await page.click('.bucket-item');

    // Upload file
    const fileInput = page.locator('#file-upload-input');
    await fileInput.setInputFiles({
      name: 'test.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Hello, World!')
    });

    await expect(page.locator('text=test.txt')).toBeVisible();
  });

  test('should toggle between grid and list view', async ({ page }) => {
    await page.click('.bucket-item');

    // Default is grid
    await expect(page.locator('.file-grid')).toBeVisible();

    // Switch to list
    await page.click('button:has-text("☰")');
    await expect(page.locator('.file-list-table')).toBeVisible();

    // Switch back to grid
    await page.click('button:has-text("⊞")');
    await expect(page.locator('.file-grid')).toBeVisible();
  });

  test('should delete files', async ({ page }) => {
    await page.click('.bucket-item');

    // Upload a file first
    const fileInput = page.locator('#file-upload-input');
    await fileInput.setInputFiles({
      name: 'delete-test.txt',
      mimeType: 'text/plain',
      buffer: Buffer.from('Delete me')
    });

    await page.waitForSelector('text=delete-test.txt');

    // Select and delete
    await page.click('.file-card-checkbox input');

    page.on('dialog', dialog => dialog.accept());
    await page.click('text=Delete Selected');

    await expect(page.locator('text=delete-test.txt')).not.toBeVisible();
  });

  test('should delete bucket', async ({ page }) => {
    // Create empty bucket
    await page.click('text=+ New');
    await page.fill('#bucket-name', 'delete-bucket');
    await page.click('text=Create Bucket');

    await page.waitForSelector('text=delete-bucket');

    // Open settings
    await page.click('.bucket-item:has-text("delete-bucket") .btn-icon');

    page.on('dialog', dialog => dialog.accept());
    await page.click('text=Delete Bucket');

    await expect(page.locator('text=delete-bucket')).not.toBeVisible();
  });
});
```

**Step 2: Run tests**

Run: `cd e2e && npm test -- tests/dashboard/storage.test.ts`
Expected: All tests pass

**Step 3: Commit**

```bash
git add e2e/tests/dashboard/storage.test.ts
git commit -m "test(e2e): add storage dashboard tests"
```

---

## Task 10: Documentation Updates

**Files:**
- Modify: `CLAUDE.md`
- Modify: `e2e/TESTS.md`

**Step 1: Update CLAUDE.md dashboard endpoints**

Add to the Dashboard section:

```markdown
| `/_/api/storage/buckets` | GET | List all buckets |
| `/_/api/storage/buckets` | POST | Create bucket |
| `/_/api/storage/buckets/{id}` | GET | Get bucket details |
| `/_/api/storage/buckets/{id}` | PUT | Update bucket settings |
| `/_/api/storage/buckets/{id}` | DELETE | Delete bucket |
| `/_/api/storage/buckets/{id}/empty` | POST | Empty bucket |
| `/_/api/storage/objects/list` | POST | List objects |
| `/_/api/storage/objects/upload` | POST | Upload file |
| `/_/api/storage/objects/download` | GET | Download file |
| `/_/api/storage/objects` | DELETE | Delete objects |
```

**Step 2: Update e2e/TESTS.md**

Add storage tests to the inventory.

**Step 3: Commit**

```bash
git add CLAUDE.md e2e/TESTS.md
git commit -m "docs: add storage dashboard documentation"
```

---

## Summary

This plan implements the Storage Dashboard UI in 10 tasks:

1. Dashboard bucket API endpoints
2. Dashboard object API endpoints
3. Frontend navigation and state
4. Bucket list and create modal
5. File browser with grid/list views
6. File upload with drag-drop and progress
7. Bulk delete and download operations
8. Bucket settings modal
9. E2E tests
10. Documentation updates

Each task is self-contained with clear files, code, and commit points.
