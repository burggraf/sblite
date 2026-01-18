package functions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// FunctionsProxy proxies requests to the edge runtime.
type FunctionsProxy struct {
	runtimePort int
	proxy       *httputil.ReverseProxy
}

// NewFunctionsProxy creates a new functions proxy.
func NewFunctionsProxy(runtimePort int) *FunctionsProxy {
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", runtimePort))

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ModifyResponse = modifyProxyResponse
	proxy.ErrorHandler = handleProxyError

	return &FunctionsProxy{
		runtimePort: runtimePort,
		proxy:       proxy,
	}
}

// ServeHTTP proxies the request to the edge runtime.
func (fp *FunctionsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add X-Request-Id if not present
	if r.Header.Get("X-Request-Id") == "" {
		r.Header.Set("X-Request-Id", uuid.New().String())
	}

	// Add X-Forwarded-* headers
	if r.Header.Get("X-Forwarded-For") == "" {
		r.Header.Set("X-Forwarded-For", r.RemoteAddr)
	}
	if r.Header.Get("X-Forwarded-Host") == "" {
		r.Header.Set("X-Forwarded-Host", r.Host)
	}
	if r.Header.Get("X-Forwarded-Proto") == "" {
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		r.Header.Set("X-Forwarded-Proto", proto)
	}

	fp.proxy.ServeHTTP(w, r)
}

// modifyProxyResponse modifies the response from edge runtime.
func modifyProxyResponse(resp *http.Response) error {
	// Add CORS headers if not present
	if resp.Header.Get("Access-Control-Allow-Origin") == "" {
		resp.Header.Set("Access-Control-Allow-Origin", "*")
	}
	return nil
}

// handleProxyError handles errors when proxying to edge runtime.
func handleProxyError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")

	// Determine error type
	errType := "FunctionsRelayError"
	statusCode := http.StatusBadGateway
	message := "Edge runtime unavailable"

	if strings.Contains(err.Error(), "connection refused") {
		message = "Edge runtime is not running"
	} else if strings.Contains(err.Error(), "timeout") {
		statusCode = http.StatusGatewayTimeout
		message = "Function execution timed out"
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":   errType,
		"message": message,
	})
}

// FunctionsError represents an error response from the functions API.
type FunctionsError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// WriteError writes a functions error response.
func WriteError(w http.ResponseWriter, errorType string, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(FunctionsError{
		Error:   errorType,
		Message: message,
	})
}
