package storage

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/rls"
)

// Handler provides HTTP handlers for storage operations.
type Handler struct {
	service    *Service
	rlsService *rls.Service
	rlsEnforcer *rls.Enforcer
}

// NewHandler creates a new storage handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// SetRLSEnforcer sets the RLS service and enforcer for storage operations.
func (h *Handler) SetRLSEnforcer(rlsService *rls.Service, rlsEnforcer *rls.Enforcer) {
	h.rlsService = rlsService
	h.rlsEnforcer = rlsEnforcer
}

// RegisterRoutes registers all storage routes on the given router.
// Mounts at /storage/v1
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Bucket routes
	r.Route("/bucket", func(r chi.Router) {
		r.Get("/", h.ListBuckets)
		r.Post("/", h.CreateBucket)
		r.Get("/{bucketId}", h.GetBucket)
		r.Put("/{bucketId}", h.UpdateBucket)
		r.Delete("/{bucketId}", h.DeleteBucket)
		r.Post("/{bucketId}/empty", h.EmptyBucket)
	})

	// Object routes
	r.Route("/object", func(r chi.Router) {
		// List objects
		r.Post("/list/{bucketName}", h.ListObjects)

		// Copy and move
		r.Post("/copy", h.CopyObject)
		r.Post("/move", h.MoveObject)

		// Signed URLs
		r.Post("/sign/{bucketName}/*", h.CreateSignedURL)

		// Public objects (no auth required for public buckets)
		r.Get("/public/{bucketName}/*", h.GetPublicObject)

		// Authenticated object operations
		r.Get("/authenticated/{bucketName}/*", h.GetObject)
		r.Get("/{bucketName}/*", h.GetObject)
		r.Head("/{bucketName}/*", h.GetObjectInfo)
		r.Post("/{bucketName}/*", h.UploadObject)
		r.Put("/{bucketName}/*", h.UploadObject)
		r.Delete("/{bucketName}", h.BatchDeleteObjects) // Batch delete with JSON body
		r.Delete("/{bucketName}/*", h.DeleteObject)     // Single file delete
	})
}

// Error response helper
func (h *Handler) jsonError(w http.ResponseWriter, err error) {
	if se, ok := err.(*StorageError); ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(se.StatusCode)
		json.NewEncoder(w).Encode(se)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(&StorageError{
		StatusCode: 500,
		ErrorCode:  "internal",
		Message:    err.Error(),
	})
}

// JSON response helper
func (h *Handler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Get user ID from request context (set by auth middleware)
func getUserID(r *http.Request) string {
	if userID := r.Context().Value("user_id"); userID != nil {
		return userID.(string)
	}
	// Try to get from JWT claims - handle various claim types
	if claims := r.Context().Value("claims"); claims != nil {
		// Try as *jwt.MapClaims (what the middleware stores)
		if c, ok := claims.(*jwt.MapClaims); ok && c != nil {
			if sub, ok := (*c)["sub"].(string); ok {
				return sub
			}
		}
		// Try as jwt.MapClaims (non-pointer)
		if c, ok := claims.(jwt.MapClaims); ok {
			if sub, ok := c["sub"].(string); ok {
				return sub
			}
		}
		// Try as generic map[string]interface{} (fallback)
		if c, ok := claims.(map[string]interface{}); ok {
			if sub, ok := c["sub"].(string); ok {
				return sub
			}
		}
	}
	return ""
}

// getAuthContext extracts RLS auth context from the request
func (h *Handler) getAuthContext(r *http.Request) *rls.AuthContext {
	// Check if service_role API key was used (bypasses RLS)
	if role := r.Context().Value("apikey_role"); role != nil {
		if role.(string) == "service_role" {
			return &rls.AuthContext{BypassRLS: true}
		}
	}

	// Extract JWT claims if present - handle various claim types
	if claims := r.Context().Value("claims"); claims != nil {
		ctx := &rls.AuthContext{}
		ctx.Claims = make(map[string]any)

		// Try as *jwt.MapClaims (what the middleware stores)
		if c, ok := claims.(*jwt.MapClaims); ok && c != nil {
			if sub, ok := (*c)["sub"].(string); ok {
				ctx.UserID = sub
			}
			if email, ok := (*c)["email"].(string); ok {
				ctx.Email = email
			}
			if role, ok := (*c)["role"].(string); ok {
				ctx.Role = role
			}
			for k, v := range *c {
				ctx.Claims[k] = v
			}
			return ctx
		}

		// Try as jwt.MapClaims (non-pointer)
		if c, ok := claims.(jwt.MapClaims); ok {
			if sub, ok := c["sub"].(string); ok {
				ctx.UserID = sub
			}
			if email, ok := c["email"].(string); ok {
				ctx.Email = email
			}
			if role, ok := c["role"].(string); ok {
				ctx.Role = role
			}
			for k, v := range c {
				ctx.Claims[k] = v
			}
			return ctx
		}

		// Try as generic map[string]interface{} (fallback)
		if c, ok := claims.(map[string]interface{}); ok {
			if sub, ok := c["sub"].(string); ok {
				ctx.UserID = sub
			}
			if email, ok := c["email"].(string); ok {
				ctx.Email = email
			}
			if role, ok := c["role"].(string); ok {
				ctx.Role = role
			}
			for k, v := range c {
				ctx.Claims[k] = v
			}
			return ctx
		}
	}

	return nil
}

// checkStorageRLS checks if the user is allowed to perform the operation on the object.
// Returns an error if access is denied.
func (h *Handler) checkStorageRLS(r *http.Request, bucketName, objectPath, command string) error {
	if h.rlsService == nil || h.rlsEnforcer == nil {
		return nil // RLS not configured
	}

	ctx := h.getAuthContext(r)

	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return nil
	}

	// Check if RLS is enabled for storage_objects
	enabled, err := h.rlsService.IsRLSEnabled("storage_objects")
	if err != nil || !enabled {
		return nil // RLS not enabled, allow all
	}

	// Build a WHERE condition that matches the specific object
	var whereConditions string
	switch command {
	case "SELECT":
		whereConditions, err = h.rlsEnforcer.GetSelectConditions("storage_objects", ctx)
	case "INSERT":
		whereConditions, err = h.rlsEnforcer.GetInsertConditions("storage_objects", ctx)
	case "DELETE":
		whereConditions, err = h.rlsEnforcer.GetDeleteConditions("storage_objects", ctx)
	default:
		return nil
	}
	if err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "rls_error", Message: "Failed to evaluate RLS policies"}
	}

	// If no policies defined and RLS is enabled, deny access
	if whereConditions == "" {
		return &StorageError{StatusCode: 403, ErrorCode: "access_denied", Message: "Access denied by RLS policy"}
	}

	// Check if the specific object matches the RLS conditions
	// We need to query with the object's bucket_id and name
	query := `SELECT 1 FROM storage_objects WHERE bucket_id = ? AND name = ? AND (` + whereConditions + `) LIMIT 1`

	var exists int
	err = h.service.DB().QueryRow(query, bucketName, objectPath).Scan(&exists)
	if err != nil {
		// Object doesn't exist or doesn't match RLS - return 404 to prevent information leakage
		return &StorageError{StatusCode: 404, ErrorCode: "not_found", Message: "Object not found"}
	}

	return nil
}

// checkStorageInsertRLS checks if the user is allowed to insert an object.
// For INSERT, we need to evaluate the CHECK expression against the proposed values.
func (h *Handler) checkStorageInsertRLS(r *http.Request, bucketName, objectPath, ownerID string) error {
	if h.rlsService == nil || h.rlsEnforcer == nil {
		return nil // RLS not configured
	}

	ctx := h.getAuthContext(r)

	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return nil
	}

	// Check if RLS is enabled for storage_objects
	enabled, err := h.rlsService.IsRLSEnabled("storage_objects")
	if err != nil || !enabled {
		return nil // RLS not enabled, allow all
	}

	// Get INSERT check conditions
	checkConditions, err := h.rlsEnforcer.GetInsertConditions("storage_objects", ctx)
	if err != nil {
		return &StorageError{StatusCode: 500, ErrorCode: "rls_error", Message: "Failed to evaluate RLS policies"}
	}

	// If no policies defined and RLS is enabled, deny access
	if checkConditions == "" {
		return &StorageError{StatusCode: 403, ErrorCode: "access_denied", Message: "Access denied by RLS policy"}
	}

	// Evaluate the CHECK expression against the proposed values
	// Create a temporary SELECT to evaluate the conditions
	query := `SELECT CASE WHEN (` + checkConditions + `) THEN 1 ELSE 0 END FROM (
		SELECT ? as bucket_id, ? as name, ? as owner_id
	)`

	var allowed int
	err = h.service.DB().QueryRow(query, bucketName, objectPath, ownerID).Scan(&allowed)
	if err != nil || allowed != 1 {
		return &StorageError{StatusCode: 403, ErrorCode: "access_denied", Message: "Access denied by RLS policy"}
	}

	return nil
}

// Bucket Handlers

// ListBuckets returns all buckets.
// GET /storage/v1/bucket
func (h *Handler) ListBuckets(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	search := r.URL.Query().Get("search")

	buckets, err := h.service.ListBuckets(limit, offset, search)
	if err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, buckets)
}

// CreateBucket creates a new bucket.
// POST /storage/v1/bucket
func (h *Handler) CreateBucket(w http.ResponseWriter, r *http.Request) {
	var req CreateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	bucket, err := h.service.CreateBucket(req, getUserID(r))
	if err != nil {
		h.jsonError(w, err)
		return
	}

	// Supabase returns just the name on success
	h.jsonResponse(w, http.StatusOK, map[string]string{"name": bucket.Name})
}

// GetBucket retrieves a bucket by ID.
// GET /storage/v1/bucket/{bucketId}
func (h *Handler) GetBucket(w http.ResponseWriter, r *http.Request) {
	bucketID := chi.URLParam(r, "bucketId")

	bucket, err := h.service.GetBucket(bucketID)
	if err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, bucket)
}

// UpdateBucket updates a bucket.
// PUT /storage/v1/bucket/{bucketId}
func (h *Handler) UpdateBucket(w http.ResponseWriter, r *http.Request) {
	bucketID := chi.URLParam(r, "bucketId")

	var req UpdateBucketRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	bucket, err := h.service.UpdateBucket(bucketID, req)
	if err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{"message": "Successfully updated"})
	_ = bucket // bucket is used for side effects
}

// DeleteBucket deletes a bucket.
// DELETE /storage/v1/bucket/{bucketId}
func (h *Handler) DeleteBucket(w http.ResponseWriter, r *http.Request) {
	bucketID := chi.URLParam(r, "bucketId")

	if err := h.service.DeleteBucket(bucketID, false); err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{"message": "Successfully deleted"})
}

// EmptyBucket removes all objects from a bucket.
// POST /storage/v1/bucket/{bucketId}/empty
func (h *Handler) EmptyBucket(w http.ResponseWriter, r *http.Request) {
	bucketID := chi.URLParam(r, "bucketId")

	if err := h.service.EmptyBucket(bucketID); err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{"message": "Successfully emptied"})
}

// Object Handlers

// ListObjects lists objects in a bucket.
// POST /storage/v1/object/list/{bucketName}
func (h *Handler) ListObjects(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")

	var req ListObjectsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	objects, err := h.service.ListObjects(bucketName, req)
	if err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, objects)
}

// UploadObject uploads a file.
// POST/PUT /storage/v1/object/{bucketName}/*
func (h *Handler) UploadObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	objectPath := chi.URLParam(r, "*")

	// Check RLS policies for INSERT
	ownerID := getUserID(r)
	if err := h.checkStorageInsertRLS(r, bucketName, objectPath, ownerID); err != nil {
		h.jsonError(w, err)
		return
	}

	// Check for upsert header
	upsert := r.Header.Get("x-upsert") == "true"

	// Get content type
	contentType := r.Header.Get("Content-Type")

	// Debug: Log content type
	// log.Printf("Upload Content-Type: %s", contentType)

	// Handle multipart form data
	if strings.HasPrefix(contentType, "multipart/form-data") {
		// Use multipart.Reader for more control
		reader, err := r.MultipartReader()
		if err != nil {
			h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Failed to parse multipart: " + err.Error()})
			return
		}

		var fileData []byte
		var fileContentType string
		var fileName string

		// Read all parts
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Failed to read multipart part: " + err.Error()})
				return
			}

			// Check if this is the file data (empty form name or named "file")
			formName := part.FormName()
			if formName == "" || formName == "file" {
				// This is likely the file content
				fileData, err = io.ReadAll(part)
				if err != nil {
					h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Failed to read file data: " + err.Error()})
					return
				}
				fileContentType = part.Header.Get("Content-Type")
				fileName = part.FileName()
			}
			part.Close()
		}

		if len(fileData) == 0 {
			h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "No file provided in multipart form"})
			return
		}

		// Detect content type if not provided
		if fileContentType == "" {
			fileContentType = DetectContentType(fileName, fileData)
		}

		resp, err := h.service.UploadObject(bucketName, objectPath, io.NopCloser(bytes.NewReader(fileData)), int64(len(fileData)), fileContentType, getUserID(r), upsert)
		if err != nil {
			h.jsonError(w, err)
			return
		}

		h.jsonResponse(w, http.StatusOK, resp)
		return
	}

	// Handle raw body upload
	size := r.ContentLength
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	resp, err := h.service.UploadObject(bucketName, objectPath, r.Body, size, contentType, getUserID(r), upsert)
	if err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, resp)
}

// GetObject downloads a file (authenticated).
// GET /storage/v1/object/{bucketName}/* or /storage/v1/object/authenticated/{bucketName}/*
func (h *Handler) GetObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	objectPath := chi.URLParam(r, "*")

	// Check RLS policies for SELECT
	if err := h.checkStorageRLS(r, bucketName, objectPath, "SELECT"); err != nil {
		h.jsonError(w, err)
		return
	}

	reader, contentType, size, err := h.service.GetObject(bucketName, objectPath)
	if err != nil {
		h.jsonError(w, err)
		return
	}
	defer reader.Close()

	// Set headers
	w.Header().Set("Content-Type", contentType)
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}

	// Check for download query param
	if download := r.URL.Query().Get("download"); download != "" {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+download+"\"")
	}

	// Stream content
	io.Copy(w, reader)
}

// GetPublicObject downloads a file from a public bucket.
// GET /storage/v1/object/public/{bucketName}/*
func (h *Handler) GetPublicObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	objectPath := chi.URLParam(r, "*")

	// Check bucket is public
	isPublic, err := h.service.IsBucketPublic(bucketName)
	if err != nil {
		h.jsonError(w, err)
		return
	}
	if !isPublic {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "not_public", Message: "Bucket is not public"})
		return
	}

	reader, contentType, size, err := h.service.GetObject(bucketName, objectPath)
	if err != nil {
		h.jsonError(w, err)
		return
	}
	defer reader.Close()

	// Set headers
	w.Header().Set("Content-Type", contentType)
	if size > 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(size, 10))
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")

	// Check for download query param
	if download := r.URL.Query().Get("download"); download != "" {
		w.Header().Set("Content-Disposition", "attachment; filename=\""+download+"\"")
	}

	io.Copy(w, reader)
}

// GetObjectInfo returns object metadata.
// HEAD /storage/v1/object/{bucketName}/*
func (h *Handler) GetObjectInfo(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	objectPath := chi.URLParam(r, "*")

	obj, err := h.service.GetObjectInfo(bucketName, objectPath)
	if err != nil {
		h.jsonError(w, err)
		return
	}

	w.Header().Set("Content-Type", obj.MimeType)
	w.Header().Set("Content-Length", strconv.FormatInt(obj.Size, 10))
	w.Header().Set("ETag", "\""+obj.ETag+"\"")
	w.Header().Set("Last-Modified", obj.UpdatedAt)
	w.WriteHeader(http.StatusOK)
}

// DeleteObject deletes a file.
// DELETE /storage/v1/object/{bucketName}/*
func (h *Handler) DeleteObject(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")
	objectPath := chi.URLParam(r, "*")

	// Check RLS policies for DELETE
	if err := h.checkStorageRLS(r, bucketName, objectPath, "DELETE"); err != nil {
		h.jsonError(w, err)
		return
	}

	if err := h.service.DeleteObject(bucketName, objectPath); err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, &DeleteResponse{Message: "Successfully deleted"})
}

// BatchDeleteObjects deletes multiple files.
// DELETE /storage/v1/object/{bucketName}
func (h *Handler) BatchDeleteObjects(w http.ResponseWriter, r *http.Request) {
	bucketName := chi.URLParam(r, "bucketName")

	var req struct {
		Prefixes []string `json:"prefixes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	var deleted []DeletedObject
	for _, path := range req.Prefixes {
		// Check RLS policies for DELETE on each file
		if err := h.checkStorageRLS(r, bucketName, path, "DELETE"); err != nil {
			continue // Skip files that fail RLS check
		}
		if err := h.service.DeleteObject(bucketName, path); err == nil {
			deleted = append(deleted, DeletedObject{
				BucketID: bucketName,
				Name:     path,
			})
		}
	}

	h.jsonResponse(w, http.StatusOK, deleted)
}

// CopyObject copies an object.
// POST /storage/v1/object/copy
func (h *Handler) CopyObject(w http.ResponseWriter, r *http.Request) {
	var req CopyObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	resp, err := h.service.CopyObject(req, getUserID(r))
	if err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, resp)
}

// MoveObject moves an object.
// POST /storage/v1/object/move
func (h *Handler) MoveObject(w http.ResponseWriter, r *http.Request) {
	var req MoveObjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	if err := h.service.MoveObject(req, getUserID(r)); err != nil {
		h.jsonError(w, err)
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]string{"message": "Successfully moved"})
}

// CreateSignedURL creates a signed URL for downloading.
// POST /storage/v1/object/sign/{bucketName}/*
func (h *Handler) CreateSignedURL(w http.ResponseWriter, r *http.Request) {
	// bucketName := chi.URLParam(r, "bucketName")
	// objectPath := chi.URLParam(r, "*")

	var req SignedURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, &StorageError{StatusCode: 400, ErrorCode: "invalid_request", Message: "Invalid JSON body"})
		return
	}

	// TODO: Implement signed URL generation in Phase 2
	h.jsonError(w, &StorageError{StatusCode: 501, ErrorCode: "not_implemented", Message: "Signed URLs not yet implemented"})
}
