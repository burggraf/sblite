package storage

import (
	"context"
	"database/sql"
	"sync"

	"github.com/markb/sblite/internal/storage/backend"
)

// Service provides storage operations.
type Service struct {
	db         *sql.DB
	backend    backend.Backend
	ctx        context.Context
	mu         sync.RWMutex
	tusService *TUSService
}

// Config holds configuration for the storage service.
type Config struct {
	// Backend specifies the storage backend type: "local" or "s3"
	Backend string

	// LocalPath is the base directory for local storage.
	LocalPath string

	// S3 configuration (for future use)
	S3Endpoint        string
	S3Region          string
	S3Bucket          string
	S3AccessKey       string
	S3SecretKey       string
	S3ForcePathStyle  bool
	S3UseSSL          bool

	// TUS resumable uploads configuration
	TUSMaxSize int64 // Maximum upload size for TUS (0 = unlimited)
}

// NewService creates a new storage service.
func NewService(db *sql.DB, cfg Config) (*Service, error) {
	var b backend.Backend
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
		b, err = backend.NewS3(context.Background(), s3Cfg)
		if err != nil {
			return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: "Failed to initialize S3 storage: " + err.Error()}
		}
	default:
		// Default to local storage
		localPath := cfg.LocalPath
		if localPath == "" {
			localPath = "./storage"
		}
		b, err = backend.NewLocal(localPath)
		if err != nil {
			return nil, &StorageError{StatusCode: 500, ErrorCode: "internal", Message: "Failed to initialize local storage: " + err.Error()}
		}
	}

	return &Service{
		db:      db,
		backend: b,
		ctx:     context.Background(),
	}, nil
}

// Close releases resources held by the service.
func (s *Service) Close() error {
	if s.backend != nil {
		return s.backend.Close()
	}
	return nil
}

// WithContext returns a copy of the service with the given context.
func (s *Service) WithContext(ctx context.Context) *Service {
	return &Service{
		db:      s.db,
		backend: s.backend,
		ctx:     ctx,
	}
}

// DB returns the database connection.
func (s *Service) DB() *sql.DB {
	return s.db
}

// Backend returns the storage backend.
func (s *Service) Backend() backend.Backend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backend
}

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

// InitTUS initializes the TUS resumable upload service.
// uploadsDir is the directory for temporary upload files.
func (s *Service) InitTUS(uploadsDir string, maxSize int64) *TUSService {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.tusService != nil {
		return s.tusService
	}

	cfg := DefaultTUSConfig()
	if maxSize > 0 {
		cfg.MaxSize = maxSize
	}

	s.tusService = NewTUSService(s.db, s.backend, cfg, uploadsDir)
	return s.tusService
}

// TUSService returns the TUS service, initializing it if needed.
func (s *Service) TUSService(uploadsDir string) *TUSService {
	s.mu.RLock()
	if s.tusService != nil {
		svc := s.tusService
		s.mu.RUnlock()
		return svc
	}
	s.mu.RUnlock()

	return s.InitTUS(uploadsDir, 0)
}
