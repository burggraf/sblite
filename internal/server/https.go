// internal/server/https.go
package server

import (
	"fmt"
	"net"
	"strings"
)

// HTTPSConfig holds HTTPS/TLS configuration.
type HTTPSConfig struct {
	Domain   string // Domain for Let's Encrypt certificate
	CertDir  string // Directory to cache certificates
	HTTPAddr string // Address for HTTP server (ACME challenges + redirect)
}

// ValidateDomain checks if the domain is valid for Let's Encrypt.
// Returns an error if the domain is localhost, an IP address, or malformed.
func ValidateDomain(domain string) error {
	if domain == "" {
		return fmt.Errorf("domain required for HTTPS")
	}

	// Check for localhost
	lower := strings.ToLower(domain)
	if lower == "localhost" {
		return fmt.Errorf("Let's Encrypt requires a public domain, not localhost. Use a reverse proxy for local HTTPS")
	}

	// Check if it's an IP address
	if ip := net.ParseIP(domain); ip != nil {
		return fmt.Errorf("Let's Encrypt requires a domain name, not an IP address")
	}

	// Check for IPv6 with brackets
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		return fmt.Errorf("Let's Encrypt requires a domain name, not an IP address")
	}

	// Basic domain format validation
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("invalid domain format: %s", domain)
	}
	if strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") {
		return fmt.Errorf("invalid domain format: %s", domain)
	}
	if strings.Contains(domain, "..") {
		return fmt.Errorf("invalid domain format: %s", domain)
	}

	return nil
}
