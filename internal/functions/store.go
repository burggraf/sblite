package functions

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// Store handles database operations for functions configuration, secrets, and metadata.
type Store struct {
	db        *sql.DB
	secretKey []byte // Derived from JWT secret for encrypting secrets
}

// NewStore creates a new functions store.
func NewStore(db *sql.DB, jwtSecret string) *Store {
	// Derive a 32-byte key from JWT secret using SHA-256
	hash := sha256.Sum256([]byte(jwtSecret))
	return &Store{
		db:        db,
		secretKey: hash[:],
	}
}

// Config operations

// GetConfig retrieves a configuration value.
func (s *Store) GetConfig(key string) (string, error) {
	var value string
	err := s.db.QueryRow("SELECT value FROM _functions_config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SetConfig sets a configuration value.
func (s *Store) SetConfig(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO _functions_config (key, value, updated_at)
		VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value)
	return err
}

// GetAllConfig retrieves all configuration values.
func (s *Store) GetAllConfig() (map[string]string, error) {
	rows, err := s.db.Query("SELECT key, value FROM _functions_config")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	config := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		config[key] = value
	}
	return config, rows.Err()
}

// Secrets operations

// GetSecret retrieves and decrypts a secret value.
func (s *Store) GetSecret(name string) (string, error) {
	var encrypted string
	err := s.db.QueryRow("SELECT value FROM _functions_secrets WHERE name = ?", name).Scan(&encrypted)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("secret %q not found", name)
	}
	if err != nil {
		return "", err
	}

	return s.decrypt(encrypted)
}

// SetSecret encrypts and stores a secret.
func (s *Store) SetSecret(name, value string) error {
	encrypted, err := s.encrypt(value)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	_, err = s.db.Exec(`
		INSERT INTO _functions_secrets (name, value, created_at, updated_at)
		VALUES (?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, name, encrypted)
	return err
}

// DeleteSecret deletes a secret.
func (s *Store) DeleteSecret(name string) error {
	result, err := s.db.Exec("DELETE FROM _functions_secrets WHERE name = ?", name)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return fmt.Errorf("secret %q not found", name)
	}
	return nil
}

// ListSecrets returns the names of all secrets (values are never exposed).
func (s *Store) ListSecrets() ([]Secret, error) {
	rows, err := s.db.Query("SELECT name, created_at, updated_at FROM _functions_secrets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []Secret
	for rows.Next() {
		var name, createdAt, updatedAt string
		if err := rows.Scan(&name, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		created, _ := time.Parse(time.RFC3339, createdAt)
		updated, _ := time.Parse(time.RFC3339, updatedAt)
		secrets = append(secrets, Secret{
			Name:      name,
			CreatedAt: created,
			UpdatedAt: updated,
		})
	}
	return secrets, rows.Err()
}

// GetAllSecrets retrieves all secrets as a map (for env var injection).
func (s *Store) GetAllSecrets() (map[string]string, error) {
	rows, err := s.db.Query("SELECT name, value FROM _functions_secrets")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	secrets := make(map[string]string)
	for rows.Next() {
		var name, encrypted string
		if err := rows.Scan(&name, &encrypted); err != nil {
			return nil, err
		}
		value, err := s.decrypt(encrypted)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt secret %q: %w", name, err)
		}
		secrets[name] = value
	}
	return secrets, rows.Err()
}

// Metadata operations

// GetMetadata retrieves metadata for a function.
func (s *Store) GetMetadata(name string) (*FunctionMetadata, error) {
	var (
		verifyJWT int
		memoryMB  sql.NullInt64
		timeoutMS sql.NullInt64
		importMap sql.NullString
		envVars   string
		createdAt string
		updatedAt string
	)

	err := s.db.QueryRow(`
		SELECT verify_jwt, memory_mb, timeout_ms, import_map, env_vars, created_at, updated_at
		FROM _functions_metadata WHERE name = ?
	`, name).Scan(&verifyJWT, &memoryMB, &timeoutMS, &importMap, &envVars, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		// Return default metadata
		return &FunctionMetadata{
			Name:      name,
			VerifyJWT: true,
			EnvVars:   make(map[string]string),
		}, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse env vars JSON
	env := make(map[string]string)
	if envVars != "" && envVars != "{}" {
		json.Unmarshal([]byte(envVars), &env)
	}

	created, _ := time.Parse(time.RFC3339, createdAt)
	updated, _ := time.Parse(time.RFC3339, updatedAt)

	return &FunctionMetadata{
		Name:      name,
		VerifyJWT: verifyJWT == 1,
		MemoryMB:  int(memoryMB.Int64),
		TimeoutMS: int(timeoutMS.Int64),
		ImportMap: importMap.String,
		EnvVars:   env,
		CreatedAt: created,
		UpdatedAt: updated,
	}, nil
}

// SetMetadata saves or updates function metadata.
func (s *Store) SetMetadata(meta *FunctionMetadata) error {
	envVars, err := json.Marshal(meta.EnvVars)
	if err != nil {
		return fmt.Errorf("failed to serialize env vars: %w", err)
	}

	verifyJWT := 0
	if meta.VerifyJWT {
		verifyJWT = 1
	}

	var memoryMB, timeoutMS interface{}
	if meta.MemoryMB > 0 {
		memoryMB = meta.MemoryMB
	}
	if meta.TimeoutMS > 0 {
		timeoutMS = meta.TimeoutMS
	}

	var importMap interface{}
	if meta.ImportMap != "" {
		importMap = meta.ImportMap
	}

	_, err = s.db.Exec(`
		INSERT INTO _functions_metadata (name, verify_jwt, memory_mb, timeout_ms, import_map, env_vars, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
		ON CONFLICT(name) DO UPDATE SET
			verify_jwt = excluded.verify_jwt,
			memory_mb = excluded.memory_mb,
			timeout_ms = excluded.timeout_ms,
			import_map = excluded.import_map,
			env_vars = excluded.env_vars,
			updated_at = excluded.updated_at
	`, meta.Name, verifyJWT, memoryMB, timeoutMS, importMap, string(envVars))
	return err
}

// DeleteMetadata deletes function metadata.
func (s *Store) DeleteMetadata(name string) error {
	_, err := s.db.Exec("DELETE FROM _functions_metadata WHERE name = ?", name)
	return err
}

// Encryption helpers

// encrypt encrypts a value using AES-GCM.
func (s *Store) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.secretKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt decrypts a value using AES-GCM.
func (s *Store) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.secretKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
