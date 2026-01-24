package pgwire

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	_ "modernc.org/sqlite"
)

func TestNewServer(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "basic config",
			config: Config{
				Address: ":5432",
			},
			wantErr: false,
		},
		{
			name: "with password auth",
			config: Config{
				Address:  ":5433",
				Password: "secret",
			},
			wantErr: false,
		},
		{
			name: "with no auth",
			config: Config{
				Address: ":5434",
				NoAuth:  true,
			},
			wantErr: false,
		},
		{
			name: "with logger",
			config: Config{
				Address: ":5435",
				Logger:  slog.New(slog.NewTextHandler(os.Stderr, nil)),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := NewServer(db, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && srv == nil {
				t.Error("NewServer() returned nil server without error")
			}
		})
	}
}

func TestServer_PasswordAuth(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	cfg := Config{
		Address:  ":5436",
		Password: "correctpassword",
	}

	srv, err := NewServer(db, cfg)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	tests := []struct {
		name     string
		password string
		wantOK   bool
	}{
		{"correct password", "correctpassword", true},
		{"wrong password", "wrongpassword", false},
		{"empty password", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok, err := srv.passwordAuth(nil, "sblite", "user", tt.password)
			if err != nil {
				t.Errorf("passwordAuth() error = %v", err)
				return
			}
			if ok != tt.wantOK {
				t.Errorf("passwordAuth() = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}
