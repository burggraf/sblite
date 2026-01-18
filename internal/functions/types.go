package functions

import "time"

// FunctionInfo contains metadata about a discovered function.
type FunctionInfo struct {
	Name       string    `json:"name"`
	Entrypoint string    `json:"entrypoint"`
	Path       string    `json:"path"`
	Status     string    `json:"status,omitempty"`
	VerifyJWT  bool      `json:"verify_jwt"`
	ModTime    time.Time `json:"mod_time"`
}

// FunctionInvokeRequest represents a function invocation request.
type FunctionInvokeRequest struct {
	Name    string            `json:"name"`
	Body    []byte            `json:"body,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
	Method  string            `json:"method"`
}

// FunctionInvokeResponse represents a function invocation response.
type FunctionInvokeResponse struct {
	StatusCode int               `json:"status_code"`
	Body       []byte            `json:"body,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// Secret represents a function secret (value never exposed via API).
type Secret struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FunctionsConfig represents the functions service configuration.
type FunctionsConfig struct {
	FunctionsDir     string `json:"functions_dir"`
	RuntimePath      string `json:"runtime_path,omitempty"`
	RuntimePort      int    `json:"runtime_port"`
	DefaultMemoryMB  int    `json:"default_memory_mb"`
	DefaultTimeoutMS int    `json:"default_timeout_ms"`
	VerifyJWT        bool   `json:"verify_jwt"`
}

// FunctionMetadata represents per-function configuration.
type FunctionMetadata struct {
	Name      string            `json:"name"`
	VerifyJWT bool              `json:"verify_jwt"`
	MemoryMB  int               `json:"memory_mb,omitempty"`
	TimeoutMS int               `json:"timeout_ms,omitempty"`
	ImportMap string            `json:"import_map,omitempty"`
	EnvVars   map[string]string `json:"env_vars,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}
