# Dashboard Storage Settings Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add S3 storage configuration to the dashboard settings UI with hot-reload support.

**Architecture:** Store S3 credentials in `_dashboard` key-value table, expose via REST API following OAuth settings pattern, add callback mechanism for hot-reloading storage backend, build UI as new Settings tab.

**Tech Stack:** Go (backend), vanilla JS (frontend), SQLite (`_dashboard` table), AWS SDK v2 (S3 testing)

---

## Task 1: Add Storage Settings API Handlers

**Files:**
- Create: `internal/dashboard/storage_settings.go`
- Modify: `internal/dashboard/handler.go` (route registration)

**Step 1: Create storage_settings.go with types and GET handler**

```go
package dashboard

import (
	"encoding/json"
	"net/http"
)

// StorageS3Config holds S3 configuration.
type StorageS3Config struct {
	Endpoint  string `json:"endpoint"`
	Region    string `json:"region"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	PathStyle bool   `json:"path_style"`
}

// StorageSettingsResponse is returned by GET /settings/storage.
type StorageSettingsResponse struct {
	Backend   string          `json:"backend"`
	LocalPath string          `json:"local_path"`
	S3        StorageS3Config `json:"s3"`
	Active    string          `json:"active"`
}

// handleGetStorageSettings returns storage configuration.
// GET /_/api/settings/storage
func (h *Handler) handleGetStorageSettings(w http.ResponseWriter, r *http.Request) {
	backend, _ := h.store.Get("storage_backend")
	if backend == "" {
		backend = "local"
	}
	localPath, _ := h.store.Get("storage_local_path")
	if localPath == "" {
		localPath = "./storage"
	}

	s3Endpoint, _ := h.store.Get("storage_s3_endpoint")
	s3Region, _ := h.store.Get("storage_s3_region")
	s3Bucket, _ := h.store.Get("storage_s3_bucket")
	s3AccessKey, _ := h.store.Get("storage_s3_access_key")
	s3SecretKey, _ := h.store.Get("storage_s3_secret_key")
	s3PathStyle, _ := h.store.Get("storage_s3_path_style")

	resp := StorageSettingsResponse{
		Backend:   backend,
		LocalPath: localPath,
		S3: StorageS3Config{
			Endpoint:  s3Endpoint,
			Region:    s3Region,
			Bucket:    s3Bucket,
			AccessKey: s3AccessKey,
			SecretKey: maskSecret(s3SecretKey),
			PathStyle: s3PathStyle == "true",
		},
		Active: backend,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
```

**Step 2: Add PATCH handler for updating storage settings**

Add to `internal/dashboard/storage_settings.go`:

```go
// StorageSettingsUpdate is the request body for PATCH /settings/storage.
type StorageSettingsUpdate struct {
	Backend   string           `json:"backend,omitempty"`
	LocalPath string           `json:"local_path,omitempty"`
	S3        *StorageS3Config `json:"s3,omitempty"`
}

// handleUpdateStorageSettings updates storage configuration.
// PATCH /_/api/settings/storage
func (h *Handler) handleUpdateStorageSettings(w http.ResponseWriter, r *http.Request) {
	var req StorageSettingsUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Update backend type
	if req.Backend != "" {
		if req.Backend != "local" && req.Backend != "s3" {
			http.Error(w, "backend must be 'local' or 's3'", http.StatusBadRequest)
			return
		}
		h.store.Set("storage_backend", req.Backend)
	}

	// Update local path
	if req.LocalPath != "" {
		h.store.Set("storage_local_path", req.LocalPath)
	}

	// Update S3 settings
	if req.S3 != nil {
		if req.S3.Endpoint != "" {
			h.store.Set("storage_s3_endpoint", req.S3.Endpoint)
		}
		if req.S3.Region != "" {
			h.store.Set("storage_s3_region", req.S3.Region)
		}
		if req.S3.Bucket != "" {
			h.store.Set("storage_s3_bucket", req.S3.Bucket)
		}
		if req.S3.AccessKey != "" {
			h.store.Set("storage_s3_access_key", req.S3.AccessKey)
		}
		// Only update secret if not masked
		if req.S3.SecretKey != "" && req.S3.SecretKey != "********" {
			h.store.Set("storage_s3_secret_key", req.S3.SecretKey)
		}
		h.store.Set("storage_s3_path_style", boolToString(req.S3.PathStyle))
	}

	// Trigger hot-reload if callback registered
	if h.onStorageReload != nil {
		cfg := h.buildStorageConfig()
		if err := h.onStorageReload(cfg); err != nil {
			http.Error(w, "failed to reload storage: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// buildStorageConfig creates a storage.Config from dashboard settings.
func (h *Handler) buildStorageConfig() *StorageConfig {
	backend, _ := h.store.Get("storage_backend")
	if backend == "" {
		backend = "local"
	}
	localPath, _ := h.store.Get("storage_local_path")
	if localPath == "" {
		localPath = "./storage"
	}

	s3Endpoint, _ := h.store.Get("storage_s3_endpoint")
	s3Region, _ := h.store.Get("storage_s3_region")
	s3Bucket, _ := h.store.Get("storage_s3_bucket")
	s3AccessKey, _ := h.store.Get("storage_s3_access_key")
	s3SecretKey, _ := h.store.Get("storage_s3_secret_key")
	s3PathStyle, _ := h.store.Get("storage_s3_path_style")

	return &StorageConfig{
		Backend:          backend,
		LocalPath:        localPath,
		S3Endpoint:       s3Endpoint,
		S3Region:         s3Region,
		S3Bucket:         s3Bucket,
		S3AccessKey:      s3AccessKey,
		S3SecretKey:      s3SecretKey,
		S3ForcePathStyle: s3PathStyle == "true",
	}
}

// StorageConfig mirrors storage.Config for the reload callback.
type StorageConfig struct {
	Backend          string
	LocalPath        string
	S3Endpoint       string
	S3Region         string
	S3Bucket         string
	S3AccessKey      string
	S3SecretKey      string
	S3ForcePathStyle bool
}
```

**Step 3: Add test connection handler**

Add to `internal/dashboard/storage_settings.go`:

```go
import (
	"context"
	"time"

	"github.com/markb/sblite/internal/storage/backend"
)

// handleTestStorageConnection tests S3 connection without saving.
// POST /_/api/settings/storage/test
func (h *Handler) handleTestStorageConnection(w http.ResponseWriter, r *http.Request) {
	var req StorageS3Config
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Bucket == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "bucket is required",
		})
		return
	}

	// If secret is masked, use stored secret
	secretKey := req.SecretKey
	if secretKey == "********" || secretKey == "" {
		secretKey, _ = h.store.Get("storage_s3_secret_key")
	}

	// Create S3 backend to test connection
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	s3Cfg := backend.S3Config{
		Bucket:          req.Bucket,
		Region:          req.Region,
		Endpoint:        req.Endpoint,
		AccessKeyID:     req.AccessKey,
		SecretAccessKey: secretKey,
		UsePathStyle:    req.PathStyle,
	}

	_, err := backend.NewS3(ctx, s3Cfg)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}
```

**Step 4: Add callback field and setter to Handler**

Modify `internal/dashboard/handler.go`, add to Handler struct:

```go
onStorageReload func(*StorageConfig) error
```

Add setter method:

```go
// SetStorageReloadFunc sets the callback function for storage configuration changes.
func (h *Handler) SetStorageReloadFunc(f func(*StorageConfig) error) {
	h.onStorageReload = f
}
```

**Step 5: Register routes in handler.go**

In `RegisterRoutes`, inside the `/settings` route group, add:

```go
r.Get("/storage", h.handleGetStorageSettings)
r.Patch("/storage", h.handleUpdateStorageSettings)
r.Post("/storage/test", h.handleTestStorageConnection)
```

**Step 6: Build and verify compilation**

Run: `go build ./...`
Expected: No errors

**Step 7: Commit**

```bash
git add internal/dashboard/storage_settings.go internal/dashboard/handler.go
git commit -m "feat(dashboard): add storage settings API endpoints

- GET /_/api/settings/storage - retrieve storage config
- PATCH /_/api/settings/storage - update storage config with hot-reload
- POST /_/api/settings/storage/test - test S3 connection"
```

---

## Task 2: Add Storage Service Reconfigure Method

**Files:**
- Modify: `internal/storage/storage.go`

**Step 1: Add sync import and mutex to Service**

Modify `internal/storage/storage.go`:

```go
import (
	"context"
	"database/sql"
	"sync"

	"github.com/markb/sblite/internal/storage/backend"
)

// Service provides storage operations.
type Service struct {
	db      *sql.DB
	backend backend.Backend
	ctx     context.Context
	mu      sync.RWMutex
}
```

**Step 2: Add Reconfigure method**

Add to `internal/storage/storage.go`:

```go
// Reconfigure switches the storage backend to use the new configuration.
// If the new backend fails to initialize, the old backend remains active.
func (s *Service) Reconfigure(cfg Config) error {
	var newBackend backend.Backend
	var err error

	switch cfg.Backend {
	case "s3":
		s3Cfg := backend.S3Config{
			Bucket:          cfg.S3Bucket,
			Region:          cfg.S3Region,
			Endpoint:        cfg.S3Endpoint,
			AccessKeyID:     cfg.S3AccessKey,
			SecretAccessKey: cfg.S3SecretKey,
			UsePathStyle:    cfg.S3ForcePathStyle,
		}
		newBackend, err = backend.NewS3(context.Background(), s3Cfg)
		if err != nil {
			return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: "Failed to initialize S3 storage: " + err.Error()}
		}
	default:
		localPath := cfg.LocalPath
		if localPath == "" {
			localPath = "./storage"
		}
		newBackend, err = backend.NewLocal(localPath)
		if err != nil {
			return &StorageError{StatusCode: 500, ErrorCode: "internal", Message: "Failed to initialize local storage: " + err.Error()}
		}
	}

	// Swap backends atomically
	s.mu.Lock()
	oldBackend := s.backend
	s.backend = newBackend
	s.mu.Unlock()

	// Close old backend
	if oldBackend != nil {
		oldBackend.Close()
	}

	return nil
}
```

**Step 3: Update Backend() to use mutex**

Update the `Backend()` method:

```go
// Backend returns the storage backend.
func (s *Service) Backend() backend.Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend
}
```

**Step 4: Build and verify compilation**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add internal/storage/storage.go
git commit -m "feat(storage): add Reconfigure method for hot-reload

Allows switching storage backend at runtime with atomic swap
and graceful cleanup of old backend."
```

---

## Task 3: Wire Up Hot-Reload in Server

**Files:**
- Modify: `cmd/serve.go`

**Step 1: Add storage reload callback registration**

In `cmd/serve.go`, find where `dashboardHandler.SetStorageService(storageSvc)` is called, and add after it:

```go
// Register storage reload callback
dashboardHandler.SetStorageReloadFunc(func(cfg *dashboard.StorageConfig) error {
	storageCfg := storage.Config{
		Backend:          cfg.Backend,
		LocalPath:        cfg.LocalPath,
		S3Endpoint:       cfg.S3Endpoint,
		S3Region:         cfg.S3Region,
		S3Bucket:         cfg.S3Bucket,
		S3AccessKey:      cfg.S3AccessKey,
		S3SecretKey:      cfg.S3SecretKey,
		S3ForcePathStyle: cfg.S3ForcePathStyle,
	}
	return storageSvc.Reconfigure(storageCfg)
})
```

**Step 2: Update buildStorageConfig to check dashboard settings first**

Modify `buildStorageConfig` function to load from dashboard store first:

```go
// buildStorageConfig creates a storage.Config from dashboard settings, environment variables, and CLI flags.
// Priority: Dashboard settings > CLI flags > environment variables > defaults
func buildStorageConfig(cmd *cobra.Command, db *sql.DB) *storage.Config {
	cfg := &storage.Config{
		Backend:   "local",
		LocalPath: "./storage",
	}

	// Read environment variables first (lowest priority after defaults)
	if backend := os.Getenv("SBLITE_STORAGE_BACKEND"); backend != "" {
		cfg.Backend = backend
	}
	if localPath := os.Getenv("SBLITE_STORAGE_PATH"); localPath != "" {
		cfg.LocalPath = localPath
	}
	if s3Endpoint := os.Getenv("SBLITE_S3_ENDPOINT"); s3Endpoint != "" {
		cfg.S3Endpoint = s3Endpoint
	}
	if s3Region := os.Getenv("SBLITE_S3_REGION"); s3Region != "" {
		cfg.S3Region = s3Region
	}
	if s3Bucket := os.Getenv("SBLITE_S3_BUCKET"); s3Bucket != "" {
		cfg.S3Bucket = s3Bucket
	}
	if s3AccessKey := os.Getenv("SBLITE_S3_ACCESS_KEY"); s3AccessKey != "" {
		cfg.S3AccessKey = s3AccessKey
	}
	if s3SecretKey := os.Getenv("SBLITE_S3_SECRET_KEY"); s3SecretKey != "" {
		cfg.S3SecretKey = s3SecretKey
	}
	if pathStyle := os.Getenv("SBLITE_S3_PATH_STYLE"); pathStyle == "true" || pathStyle == "1" {
		cfg.S3ForcePathStyle = true
	}

	// CLI flags override environment variables
	if backend, _ := cmd.Flags().GetString("storage-backend"); backend != "" {
		cfg.Backend = backend
	}
	if localPath, _ := cmd.Flags().GetString("storage-path"); localPath != "" {
		cfg.LocalPath = localPath
	}
	if s3Endpoint, _ := cmd.Flags().GetString("s3-endpoint"); s3Endpoint != "" {
		cfg.S3Endpoint = s3Endpoint
	}
	if s3Region, _ := cmd.Flags().GetString("s3-region"); s3Region != "" {
		cfg.S3Region = s3Region
	}
	if s3Bucket, _ := cmd.Flags().GetString("s3-bucket"); s3Bucket != "" {
		cfg.S3Bucket = s3Bucket
	}
	if s3AccessKey, _ := cmd.Flags().GetString("s3-access-key"); s3AccessKey != "" {
		cfg.S3AccessKey = s3AccessKey
	}
	if s3SecretKey, _ := cmd.Flags().GetString("s3-secret-key"); s3SecretKey != "" {
		cfg.S3SecretKey = s3SecretKey
	}
	if pathStyle, _ := cmd.Flags().GetBool("s3-path-style"); pathStyle {
		cfg.S3ForcePathStyle = true
	}

	// Dashboard settings have highest priority (if db is available)
	if db != nil {
		store := dashboard.NewStore(db)
		if backend, _ := store.Get("storage_backend"); backend != "" {
			cfg.Backend = backend
		}
		if localPath, _ := store.Get("storage_local_path"); localPath != "" {
			cfg.LocalPath = localPath
		}
		if s3Endpoint, _ := store.Get("storage_s3_endpoint"); s3Endpoint != "" {
			cfg.S3Endpoint = s3Endpoint
		}
		if s3Region, _ := store.Get("storage_s3_region"); s3Region != "" {
			cfg.S3Region = s3Region
		}
		if s3Bucket, _ := store.Get("storage_s3_bucket"); s3Bucket != "" {
			cfg.S3Bucket = s3Bucket
		}
		if s3AccessKey, _ := store.Get("storage_s3_access_key"); s3AccessKey != "" {
			cfg.S3AccessKey = s3AccessKey
		}
		if s3SecretKey, _ := store.Get("storage_s3_secret_key"); s3SecretKey != "" {
			cfg.S3SecretKey = s3SecretKey
		}
		if s3PathStyle, _ := store.Get("storage_s3_path_style"); s3PathStyle == "true" {
			cfg.S3ForcePathStyle = true
		}
	}

	return cfg
}
```

**Step 3: Update call site to pass db**

Find where `buildStorageConfig(cmd)` is called and update to `buildStorageConfig(cmd, db)`.

**Step 4: Build and verify compilation**

Run: `go build ./...`
Expected: No errors

**Step 5: Commit**

```bash
git add cmd/serve.go
git commit -m "feat(server): wire up storage hot-reload and dashboard config priority

Dashboard settings now have highest priority for storage configuration.
Storage backend can be reconfigured at runtime via dashboard API."
```

---

## Task 4: Add Dashboard UI for Storage Settings

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add storage settings state**

In `app.js`, find the `state` object initialization and add:

```javascript
storageSettings: {
    backend: 'local',
    localPath: './storage',
    s3: {
        endpoint: '',
        region: '',
        bucket: '',
        accessKey: '',
        secretKey: '',
        pathStyle: false
    },
    loading: false,
    testing: false,
    testResult: null,
    saving: false,
    dirty: false,
    originalBackend: 'local'
},
```

**Step 2: Add loadStorageSettings method**

Add method to App:

```javascript
async loadStorageSettings() {
    this.state.storageSettings.loading = true;
    this.render();
    try {
        const res = await fetch('/_/api/settings/storage');
        if (res.ok) {
            const data = await res.json();
            this.state.storageSettings = {
                ...this.state.storageSettings,
                backend: data.backend,
                localPath: data.local_path,
                s3: {
                    endpoint: data.s3.endpoint || '',
                    region: data.s3.region || '',
                    bucket: data.s3.bucket || '',
                    accessKey: data.s3.access_key || '',
                    secretKey: data.s3.secret_key || '',
                    pathStyle: data.s3.path_style || false
                },
                originalBackend: data.backend,
                loading: false,
                dirty: false
            };
        }
    } catch (err) {
        console.error('Failed to load storage settings:', err);
        this.state.storageSettings.loading = false;
    }
    this.render();
},
```

**Step 3: Add saveStorageSettings method**

```javascript
async saveStorageSettings() {
    this.state.storageSettings.saving = true;
    this.state.storageSettings.testResult = null;
    this.render();

    const { backend, localPath, s3 } = this.state.storageSettings;
    const body = {
        backend,
        local_path: localPath,
        s3: {
            endpoint: s3.endpoint,
            region: s3.region,
            bucket: s3.bucket,
            access_key: s3.accessKey,
            secret_key: s3.secretKey,
            path_style: s3.pathStyle
        }
    };

    try {
        const res = await fetch('/_/api/settings/storage', {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });

        if (res.ok) {
            this.state.storageSettings.originalBackend = backend;
            this.state.storageSettings.dirty = false;
            this.showNotification('Storage settings saved successfully', 'success');
        } else {
            const err = await res.text();
            this.showNotification('Failed to save: ' + err, 'error');
        }
    } catch (err) {
        this.showNotification('Failed to save storage settings', 'error');
    }

    this.state.storageSettings.saving = false;
    this.render();
},
```

**Step 4: Add testS3Connection method**

```javascript
async testS3Connection() {
    this.state.storageSettings.testing = true;
    this.state.storageSettings.testResult = null;
    this.render();

    const { s3 } = this.state.storageSettings;

    try {
        const res = await fetch('/_/api/settings/storage/test', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                endpoint: s3.endpoint,
                region: s3.region,
                bucket: s3.bucket,
                access_key: s3.accessKey,
                secret_key: s3.secretKey,
                path_style: s3.pathStyle
            })
        });

        const data = await res.json();
        this.state.storageSettings.testResult = data;
    } catch (err) {
        this.state.storageSettings.testResult = { success: false, error: err.message };
    }

    this.state.storageSettings.testing = false;
    this.render();
},
```

**Step 5: Add updateStorageField helper**

```javascript
updateStorageField(field, value) {
    if (field.startsWith('s3.')) {
        const s3Field = field.substring(3);
        this.state.storageSettings.s3[s3Field] = value;
    } else {
        this.state.storageSettings[field] = value;
    }
    this.state.storageSettings.dirty = true;
    this.state.storageSettings.testResult = null;
    this.render();
},
```

**Step 6: Add renderStorageSettings method**

```javascript
renderStorageSettings() {
    const { backend, localPath, s3, loading, testing, testResult, saving, dirty, originalBackend } = this.state.storageSettings;
    const backendChanged = backend !== originalBackend;

    if (loading) {
        return '<div class="loading">Loading storage settings...</div>';
    }

    return `
        <div class="settings-section">
            <h3>Storage Configuration</h3>

            <div class="form-group">
                <label>Backend</label>
                <div class="radio-group">
                    <label class="radio-label">
                        <input type="radio" name="storage-backend" value="local"
                            ${backend === 'local' ? 'checked' : ''}
                            onchange="App.updateStorageField('backend', 'local')">
                        Local filesystem
                    </label>
                    <label class="radio-label">
                        <input type="radio" name="storage-backend" value="s3"
                            ${backend === 's3' ? 'checked' : ''}
                            onchange="App.updateStorageField('backend', 's3')">
                        S3-compatible
                    </label>
                </div>
            </div>

            <div class="settings-subsection ${backend !== 'local' ? 'disabled' : ''}">
                <h4>Local Settings</h4>
                <div class="form-group">
                    <label>Storage Path</label>
                    <input type="text" value="${this.escapeHtml(localPath)}"
                        ${backend !== 'local' ? 'disabled' : ''}
                        onchange="App.updateStorageField('localPath', this.value)">
                </div>
            </div>

            <div class="settings-subsection ${backend !== 's3' ? 'disabled' : ''}">
                <h4>S3 Settings</h4>
                <div class="form-group">
                    <label>Endpoint</label>
                    <input type="text" value="${this.escapeHtml(s3.endpoint)}"
                        placeholder="https://s3.amazonaws.com"
                        ${backend !== 's3' ? 'disabled' : ''}
                        onchange="App.updateStorageField('s3.endpoint', this.value)">
                    <small>Leave empty for AWS S3, or enter custom endpoint for MinIO, R2, etc.</small>
                </div>
                <div class="form-group">
                    <label>Region</label>
                    <input type="text" value="${this.escapeHtml(s3.region)}"
                        placeholder="us-east-1"
                        ${backend !== 's3' ? 'disabled' : ''}
                        onchange="App.updateStorageField('s3.region', this.value)">
                </div>
                <div class="form-group">
                    <label>Bucket</label>
                    <input type="text" value="${this.escapeHtml(s3.bucket)}"
                        placeholder="my-bucket"
                        ${backend !== 's3' ? 'disabled' : ''}
                        onchange="App.updateStorageField('s3.bucket', this.value)">
                </div>
                <div class="form-group">
                    <label>Access Key</label>
                    <input type="text" value="${this.escapeHtml(s3.accessKey)}"
                        placeholder="AKIAIOSFODNN7EXAMPLE"
                        ${backend !== 's3' ? 'disabled' : ''}
                        onchange="App.updateStorageField('s3.accessKey', this.value)">
                </div>
                <div class="form-group">
                    <label>Secret Key</label>
                    <input type="password" value="${this.escapeHtml(s3.secretKey)}"
                        placeholder="Enter secret key"
                        ${backend !== 's3' ? 'disabled' : ''}
                        onchange="App.updateStorageField('s3.secretKey', this.value)">
                </div>
                <div class="form-group">
                    <label class="checkbox-label">
                        <input type="checkbox" ${s3.pathStyle ? 'checked' : ''}
                            ${backend !== 's3' ? 'disabled' : ''}
                            onchange="App.updateStorageField('s3.pathStyle', this.checked)">
                        Use path-style addressing (required for MinIO)
                    </label>
                </div>

                <div class="form-group">
                    <button class="btn btn-secondary" onclick="App.testS3Connection()"
                        ${backend !== 's3' || testing ? 'disabled' : ''}>
                        ${testing ? 'Testing...' : 'Test Connection'}
                    </button>
                    ${testResult ? `
                        <span class="test-result ${testResult.success ? 'success' : 'error'}">
                            ${testResult.success ? 'Connection successful!' : 'Error: ' + this.escapeHtml(testResult.error)}
                        </span>
                    ` : ''}
                </div>
            </div>

            ${backendChanged ? `
                <div class="warning-banner">
                    <strong>Warning:</strong> Switching backends does not migrate existing files.
                    Files stored in the previous backend will not be accessible after switching.
                </div>
            ` : ''}

            <div class="form-actions">
                <button class="btn btn-secondary" onclick="App.loadStorageSettings()" ${saving ? 'disabled' : ''}>
                    Cancel
                </button>
                <button class="btn btn-primary" onclick="App.saveStorageSettings()" ${!dirty || saving ? 'disabled' : ''}>
                    ${saving ? 'Saving...' : 'Save Changes'}
                </button>
            </div>
        </div>
    `;
},
```

**Step 7: Add settings-storage navigation and view**

Find the settings navigation in `renderSettingsView` and add a Storage tab. Find where the settings tabs are rendered and add:

```javascript
<a class="tab ${this.state.settingsTab === 'storage' ? 'active' : ''}"
   onclick="App.setSettingsTab('storage')">Storage</a>
```

In the tab content rendering, add:

```javascript
case 'storage':
    return this.renderStorageSettings();
```

**Step 8: Load storage settings when tab is selected**

In `setSettingsTab` method, add:

```javascript
if (tab === 'storage') {
    this.loadStorageSettings();
}
```

**Step 9: Add CSS for new components**

Find the `<style>` section and add:

```css
.settings-subsection {
    margin: 1rem 0;
    padding: 1rem;
    border: 1px solid var(--border);
    border-radius: 4px;
}

.settings-subsection.disabled {
    opacity: 0.5;
    pointer-events: none;
}

.settings-subsection h4 {
    margin-top: 0;
    margin-bottom: 1rem;
    font-size: 0.9rem;
    color: var(--text-secondary);
}

.radio-group {
    display: flex;
    gap: 1.5rem;
}

.radio-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
}

.checkbox-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    cursor: pointer;
}

.test-result {
    margin-left: 1rem;
    padding: 0.25rem 0.5rem;
    border-radius: 4px;
    font-size: 0.875rem;
}

.test-result.success {
    background: var(--success-bg);
    color: var(--success);
}

.test-result.error {
    background: var(--error-bg);
    color: var(--error);
}

.warning-banner {
    margin: 1rem 0;
    padding: 1rem;
    background: var(--warning-bg);
    border: 1px solid var(--warning);
    border-radius: 4px;
    color: var(--warning);
}

.form-actions {
    margin-top: 1.5rem;
    display: flex;
    gap: 0.5rem;
    justify-content: flex-end;
}
```

**Step 10: Build and test manually**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Navigate to `http://localhost:8080/_` → Settings → Storage
Verify the UI renders correctly.

**Step 11: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add Storage settings UI

- New Storage tab in Settings
- Toggle between Local and S3 backends
- S3 credential configuration form
- Test Connection button
- Warning when switching backend types"
```

---

## Task 5: Update Documentation

**Files:**
- Modify: `CLAUDE.md`

**Step 1: Add storage settings endpoints to CLAUDE.md**

In the Dashboard API endpoints table, add:

```markdown
| `/_/api/settings/storage` | GET | Get storage configuration |
| `/_/api/settings/storage` | PATCH | Update storage configuration |
| `/_/api/settings/storage/test` | POST | Test S3 connection |
```

**Step 2: Add storage dashboard configuration section**

Add a new section under Environment Variables:

```markdown
### Dashboard Storage Configuration

Storage can be configured via the dashboard Settings → Storage tab. Dashboard settings take priority over CLI flags and environment variables.

**Configurable options:**
- Backend type (local or S3)
- Local storage path
- S3 endpoint, region, bucket, credentials
- Path-style addressing for S3-compatible services

Changes take effect immediately without server restart.
```

**Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add storage settings API documentation"
```

---

## Task 6: Final Testing and Cleanup

**Step 1: Run all tests**

Run: `go test ./...`
Expected: All tests pass (except pre-existing mail test failure)

**Step 2: Manual integration test**

1. Start server: `./sblite serve --db test.db`
2. Open dashboard: `http://localhost:8080/_`
3. Go to Settings → Storage
4. Verify local backend is selected by default
5. Switch to S3, enter test credentials
6. Click Test Connection
7. Save settings
8. Restart server, verify settings persist

**Step 3: Commit any final fixes**

```bash
git add -A
git commit -m "fix: address any issues found during testing"
```

---

Plan complete and saved to `docs/plans/2026-01-21-dashboard-storage-settings.md`. Two execution options:

**1. Subagent-Driven (this session)** - I dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session in worktree with executing-plans, batch execution with checkpoints

Which approach?
