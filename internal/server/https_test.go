package server

import (
	"testing"
)

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		domain  string
		wantErr bool
		errMsg  string
	}{
		// Valid domains
		{"example.com", false, ""},
		{"sub.example.com", false, ""},
		{"my-site.example.com", false, ""},

		// Invalid: empty
		{"", true, "domain required"},

		// Invalid: localhost
		{"localhost", true, "public domain"},
		{"LOCALHOST", true, "public domain"},

		// Invalid: IP addresses
		{"127.0.0.1", true, "domain name, not an IP"},
		{"192.168.1.1", true, "domain name, not an IP"},
		{"10.0.0.1", true, "domain name, not an IP"},
		{"::1", true, "domain name, not an IP"},
		{"2001:db8::1", true, "domain name, not an IP"},

		// Invalid: malformed
		{"example..com", true, "invalid domain"},
		{".example.com", true, "invalid domain"},
		{"example.com.", true, "invalid domain"},
		{"-example.com", true, "invalid domain"},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateDomain(%q) expected error containing %q, got nil", tt.domain, tt.errMsg)
				} else if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateDomain(%q) error = %q, want containing %q", tt.domain, err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateDomain(%q) unexpected error: %v", tt.domain, err)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchString(s, substr)))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
