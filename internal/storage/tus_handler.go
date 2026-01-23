package storage

import (
	"encoding/base64"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite/internal/rls"
)

// TUSHandler handles TUS protocol HTTP requests.
type TUSHandler struct {
	service    *TUSService
	storageSvc *Service
	rlsService *rls.Service
	rlsEnforcer *rls.Enforcer
	jwtSecret  string
}

// NewTUSHandler creates a new TUS handler.
func NewTUSHandler(tusSvc *TUSService, storageSvc *Service, jwtSecret string) *TUSHandler {
	return &TUSHandler{
		service:    tusSvc,
		storageSvc: storageSvc,
		jwtSecret:  jwtSecret,
	}
}

// SetRLSEnforcer sets the RLS service and enforcer.
func (h *TUSHandler) SetRLSEnforcer(rlsService *rls.Service, rlsEnforcer *rls.Enforcer) {
	h.rlsService = rlsService
	h.rlsEnforcer = rlsEnforcer
}

// RegisterRoutes registers TUS routes on the given router.
// Mounts at /storage/v1/upload/resumable
func (h *TUSHandler) RegisterRoutes(r chi.Router) {
	r.Route("/upload/resumable", func(r chi.Router) {
		r.Options("/", h.HandleOptions)
		r.Options("/*", h.HandleOptions)
		r.Post("/", h.HandleCreate)
		r.Head("/*", h.HandleHead)
		r.Patch("/*", h.HandlePatch)
		r.Put("/*", h.HandlePatch) // Alternate method
		r.Delete("/*", h.HandleDelete)
	})
}

// setTUSHeaders adds standard TUS response headers.
func (h *TUSHandler) setTUSHeaders(w http.ResponseWriter) {
	w.Header().Set("Tus-Resumable", TUSVersion)
	w.Header().Set("Tus-Version", TUSVersion)
	w.Header().Set("Tus-Extension", TUSExtensions)
	w.Header().Set("Tus-Max-Size", strconv.FormatInt(h.service.config.MaxSize, 10))
}

// checkTUSVersion validates the Tus-Resumable header.
func (h *TUSHandler) checkTUSVersion(r *http.Request) bool {
	version := r.Header.Get("Tus-Resumable")
	return version == TUSVersion
}

// tusError sends a TUS error response.
func (h *TUSHandler) tusError(w http.ResponseWriter, err error) {
	h.setTUSHeaders(w)
	if te, ok := err.(*TUSError); ok {
		w.WriteHeader(te.StatusCode)
		w.Write([]byte(te.Message))
		return
	}
	if se, ok := err.(*StorageError); ok {
		w.WriteHeader(se.StatusCode)
		w.Write([]byte(se.Message))
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
	w.Write([]byte(err.Error()))
}

// parseMetadata parses the Upload-Metadata header.
// Format: key1 base64value1,key2 base64value2
func (h *TUSHandler) parseMetadata(header string) map[string]string {
	result := make(map[string]string)
	if header == "" {
		return result
	}

	pairs := strings.Split(header, ",")
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		parts := strings.SplitN(pair, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		encodedValue := strings.TrimSpace(parts[1])

		// Decode base64 value
		decoded, err := base64.StdEncoding.DecodeString(encodedValue)
		if err != nil {
			continue
		}

		result[key] = string(decoded)
	}

	return result
}

// getUploadID extracts the upload ID from the URL path.
func (h *TUSHandler) getUploadID(r *http.Request) string {
	path := chi.URLParam(r, "*")
	// Remove any leading slashes
	return strings.TrimPrefix(path, "/")
}

// HandleOptions handles OPTIONS requests for TUS capabilities.
// OPTIONS /storage/v1/upload/resumable
func (h *TUSHandler) HandleOptions(w http.ResponseWriter, r *http.Request) {
	h.setTUSHeaders(w)
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, HEAD, PATCH, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Upload-Length, Upload-Offset, Upload-Metadata, Tus-Resumable, X-Upsert, X-HTTP-Method-Override")
	w.Header().Set("Access-Control-Expose-Headers", "Upload-Offset, Upload-Length, Location, Tus-Resumable, Tus-Version, Tus-Extension, Tus-Max-Size")
	w.WriteHeader(http.StatusNoContent)
}

// HandleCreate creates a new upload session.
// POST /storage/v1/upload/resumable
func (h *TUSHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	// Validate TUS version
	if !h.checkTUSVersion(r) {
		h.tusError(w, ErrTUSVersionMismatch)
		return
	}

	// Parse Upload-Length
	lengthStr := r.Header.Get("Upload-Length")
	if lengthStr == "" {
		h.tusError(w, &TUSError{StatusCode: 400, Message: "Missing Upload-Length header"})
		return
	}
	uploadLength, err := strconv.ParseInt(lengthStr, 10, 64)
	if err != nil || uploadLength < 0 {
		h.tusError(w, ErrTUSInvalidLength)
		return
	}

	// Parse Upload-Metadata
	metadata := h.parseMetadata(r.Header.Get("Upload-Metadata"))

	bucketName := metadata["bucketName"]
	objectName := metadata["objectName"]
	if bucketName == "" || objectName == "" {
		h.tusError(w, ErrTUSMissingMetadata)
		return
	}

	contentType := metadata["contentType"]
	cacheControl := metadata["cacheControl"]

	// Check for upsert header
	upsert := r.Header.Get("x-upsert") == "true"

	// Get bucket by name to validate and get ID
	bucket, err := h.storageSvc.GetBucketByName(bucketName)
	if err != nil {
		h.tusError(w, err)
		return
	}

	// Validate file size limit
	if bucket.FileSizeLimit != nil && uploadLength > *bucket.FileSizeLimit {
		h.tusError(w, &TUSError{
			StatusCode: 413,
			Message:    "File size exceeds bucket limit",
		})
		return
	}

	// Validate MIME type
	if len(bucket.AllowedMimeTypes) > 0 && contentType != "" {
		allowed := false
		for _, mt := range bucket.AllowedMimeTypes {
			if mt == contentType || strings.HasPrefix(contentType, strings.TrimSuffix(mt, "*")) {
				allowed = true
				break
			}
		}
		if !allowed {
			h.tusError(w, &TUSError{StatusCode: 415, Message: "MIME type not allowed"})
			return
		}
	}

	// Get owner ID from auth context
	ownerID := getUserID(r)

	// Check RLS for INSERT
	if h.rlsService != nil && h.rlsEnforcer != nil {
		if err := h.checkStorageInsertRLS(r, bucket.ID, objectName, ownerID); err != nil {
			h.tusError(w, err)
			return
		}
	}

	// Create upload session
	req := CreateUploadRequest{
		BucketID:     bucket.ID,
		ObjectName:   objectName,
		UploadLength: uploadLength,
		ContentType:  contentType,
		CacheControl: cacheControl,
		Metadata:     metadata,
		OwnerID:      ownerID,
		Upsert:       upsert,
	}

	session, err := h.service.CreateUpload(r.Context(), req)
	if err != nil {
		h.tusError(w, err)
		return
	}

	// Set response headers
	h.setTUSHeaders(w)
	w.Header().Set("Location", "/storage/v1/upload/resumable/"+session.ID)
	w.Header().Set("Upload-Offset", "0")
	w.WriteHeader(http.StatusCreated)
}

// HandleHead returns the current upload progress.
// HEAD /storage/v1/upload/resumable/{id}
func (h *TUSHandler) HandleHead(w http.ResponseWriter, r *http.Request) {
	// Validate TUS version
	if !h.checkTUSVersion(r) {
		h.tusError(w, ErrTUSVersionMismatch)
		return
	}

	uploadID := h.getUploadID(r)
	if uploadID == "" {
		h.tusError(w, ErrTUSSessionNotFound)
		return
	}

	session, err := h.service.GetUpload(r.Context(), uploadID)
	if err != nil {
		h.tusError(w, err)
		return
	}

	h.setTUSHeaders(w)
	w.Header().Set("Upload-Offset", strconv.FormatInt(session.UploadOffset, 10))
	w.Header().Set("Upload-Length", strconv.FormatInt(session.UploadLength, 10))
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}

// HandlePatch uploads a chunk of data.
// PATCH/PUT /storage/v1/upload/resumable/{id}
func (h *TUSHandler) HandlePatch(w http.ResponseWriter, r *http.Request) {
	// Validate TUS version
	if !h.checkTUSVersion(r) {
		h.tusError(w, ErrTUSVersionMismatch)
		return
	}

	// Validate Content-Type
	contentType := r.Header.Get("Content-Type")
	if contentType != "" && contentType != "application/offset+octet-stream" {
		h.tusError(w, &TUSError{StatusCode: 415, Message: "Content-Type must be application/offset+octet-stream"})
		return
	}

	uploadID := h.getUploadID(r)
	if uploadID == "" {
		h.tusError(w, ErrTUSSessionNotFound)
		return
	}

	// Parse Upload-Offset
	offsetStr := r.Header.Get("Upload-Offset")
	if offsetStr == "" {
		h.tusError(w, &TUSError{StatusCode: 400, Message: "Missing Upload-Offset header"})
		return
	}
	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		h.tusError(w, &TUSError{StatusCode: 400, Message: "Invalid Upload-Offset"})
		return
	}

	// Write chunk
	newOffset, err := h.service.WriteChunk(r.Context(), uploadID, offset, r.Body)
	if err != nil {
		h.tusError(w, err)
		return
	}

	// Check if upload is complete
	session, err := h.service.GetUpload(r.Context(), uploadID)
	if err != nil {
		h.tusError(w, err)
		return
	}

	h.setTUSHeaders(w)
	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))

	if session.IsComplete() {
		// Finalize the upload
		resp, err := h.service.FinalizeUpload(r.Context(), uploadID, h.storageSvc)
		if err != nil {
			h.tusError(w, err)
			return
		}

		// Return success with key
		w.Header().Set("Upload-Length", strconv.FormatInt(session.UploadLength, 10))
		w.WriteHeader(http.StatusNoContent)

		// The key is available in resp but TUS doesn't typically return it in headers
		_ = resp
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

// HandleDelete cancels an upload.
// DELETE /storage/v1/upload/resumable/{id}
func (h *TUSHandler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	// Validate TUS version
	if !h.checkTUSVersion(r) {
		h.tusError(w, ErrTUSVersionMismatch)
		return
	}

	uploadID := h.getUploadID(r)
	if uploadID == "" {
		h.tusError(w, ErrTUSSessionNotFound)
		return
	}

	if err := h.service.CancelUpload(r.Context(), uploadID); err != nil {
		h.tusError(w, err)
		return
	}

	h.setTUSHeaders(w)
	w.WriteHeader(http.StatusNoContent)
}

// checkStorageInsertRLS checks if the user is allowed to insert an object.
func (h *TUSHandler) checkStorageInsertRLS(r *http.Request, bucketID, objectPath, ownerID string) error {
	if h.rlsService == nil || h.rlsEnforcer == nil {
		return nil
	}

	ctx := h.getAuthContext(r)

	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return nil
	}

	// Check if RLS is enabled for storage_objects
	enabled, err := h.rlsService.IsRLSEnabled("storage_objects")
	if err != nil || !enabled {
		return nil
	}

	// Get INSERT check conditions
	checkConditions, err := h.rlsEnforcer.GetInsertConditions("storage_objects", ctx)
	if err != nil {
		return &TUSError{StatusCode: 500, Message: "Failed to evaluate RLS policies"}
	}

	// If no policies defined and RLS is enabled, deny access
	if checkConditions == "" {
		return &TUSError{StatusCode: 403, Message: "Access denied by RLS policy"}
	}

	// Evaluate the CHECK expression against the proposed values
	query := `SELECT CASE WHEN (` + checkConditions + `) THEN 1 ELSE 0 END FROM (
		SELECT ? as bucket_id, ? as name, ? as owner_id
	)`

	var allowed int
	err = h.storageSvc.DB().QueryRow(query, bucketID, objectPath, ownerID).Scan(&allowed)
	if err != nil || allowed != 1 {
		return &TUSError{StatusCode: 403, Message: "Access denied by RLS policy"}
	}

	return nil
}

// getAuthContext extracts RLS auth context from the request.
func (h *TUSHandler) getAuthContext(r *http.Request) *rls.AuthContext {
	// Check if service_role API key was used
	if role := r.Context().Value("apikey_role"); role != nil {
		if role.(string) == "service_role" {
			return &rls.AuthContext{BypassRLS: true}
		}
	}

	// Extract JWT claims if present
	if claims := r.Context().Value("claims"); claims != nil {
		ctx := &rls.AuthContext{}
		ctx.Claims = make(map[string]any)

		// Try various claim formats
		switch c := claims.(type) {
		case map[string]interface{}:
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
		}
		return ctx
	}

	return nil
}

// CreationWithUploadHandler handles creation with inline upload data.
// This is an optimization for small files.
func (h *TUSHandler) HandleCreateWithUpload(w http.ResponseWriter, r *http.Request) {
	// Check if there's body content
	if r.ContentLength == 0 {
		h.HandleCreate(w, r)
		return
	}

	// Create the upload first
	h.HandleCreate(w, r)
	if w.Header().Get("Location") == "" {
		return // Creation failed
	}

	// Extract upload ID from Location header
	location := w.Header().Get("Location")
	uploadID := strings.TrimPrefix(location, "/storage/v1/upload/resumable/")

	// Write the initial chunk
	newOffset, err := h.service.WriteChunk(r.Context(), uploadID, 0, r.Body)
	if err != nil {
		h.tusError(w, err)
		return
	}

	// Update offset in response
	w.Header().Set("Upload-Offset", strconv.FormatInt(newOffset, 10))

	// Check if complete
	session, err := h.service.GetUpload(r.Context(), uploadID)
	if err == nil && session.IsComplete() {
		h.service.FinalizeUpload(r.Context(), uploadID, h.storageSvc)
	}
}

// drainBody reads and discards the remaining body content.
func drainBody(body io.ReadCloser) {
	io.Copy(io.Discard, body)
	body.Close()
}
