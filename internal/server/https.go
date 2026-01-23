// internal/server/https.go
package server

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"

	"golang.org/x/crypto/acme/autocert"
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

// NewAutocertManager creates an autocert.Manager configured for the given domain.
// Certificates are cached in the specified directory.
func NewAutocertManager(domain, certDir string) *autocert.Manager {
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache(certDir),
	}
}

// NewTLSConfig creates a TLS config using the autocert manager.
func NewTLSConfig(manager *autocert.Manager) *tls.Config {
	return &tls.Config{
		GetCertificate: manager.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"}, // Enable HTTP/2
	}
}

// HTTPRedirectHandler returns a handler that redirects HTTP requests to HTTPS.
// ACME challenges (/.well-known/acme-challenge/) are not handled by this handler
// and should be wrapped by autocert.Manager.HTTPHandler().
func HTTPRedirectHandler(domain string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + domain + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}
