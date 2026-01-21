package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/markb/sblite/internal/storage/backend"
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

// StorageSettingsUpdate is the request body for PATCH /settings/storage.
type StorageSettingsUpdate struct {
	Backend   string           `json:"backend,omitempty"`
	LocalPath string           `json:"local_path,omitempty"`
	S3        *StorageS3Config `json:"s3,omitempty"`
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

// buildStorageConfig creates a StorageConfig from dashboard settings.
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
