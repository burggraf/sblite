package dashboard

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLength = 8
	bcryptCost        = 10
)

var (
	ErrPasswordTooShort = errors.New("password must be at least 8 characters")
	ErrPasswordExists   = errors.New("password already set, use reset instead")
	ErrInvalidPassword  = errors.New("invalid password")
)

// Auth handles dashboard authentication.
type Auth struct {
	store *Store
}

// NewAuth creates a new Auth.
func NewAuth(store *Store) *Auth {
	return &Auth{store: store}
}

// NeedsSetup returns true if no password has been set.
func (a *Auth) NeedsSetup() bool {
	return !a.store.HasPassword()
}

// SetupPassword sets the initial password. Fails if password already exists.
func (a *Auth) SetupPassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}

	if a.store.HasPassword() {
		return ErrPasswordExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}

	return a.store.Set("password_hash", string(hash))
}

// ResetPassword changes the password (can be used anytime).
func (a *Auth) ResetPassword(password string) error {
	if len(password) < minPasswordLength {
		return ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}

	return a.store.Set("password_hash", string(hash))
}

// VerifyPassword checks if the provided password matches the stored hash.
func (a *Auth) VerifyPassword(password string) bool {
	hash, err := a.store.Get("password_hash")
	if err != nil || hash == "" {
		return false
	}

	err = bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
