// internal/rest/openapi_types.go
package rest

// OpenAPISpec represents an OpenAPI 3.0 specification document.
type OpenAPISpec struct {
	OpenAPI    string               `json:"openapi"`
	Info       OpenAPIInfo          `json:"info"`
	Paths      map[string]PathItem  `json:"paths"`
	Components OpenAPIComponents    `json:"components"`
	Servers    []OpenAPIServer      `json:"servers,omitempty"`
}

// OpenAPIInfo contains metadata about the API.
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// OpenAPIServer describes a server URL.
type OpenAPIServer struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// OpenAPIComponents holds reusable components.
type OpenAPIComponents struct {
	Schemas         map[string]Schema         `json:"schemas,omitempty"`
	SecuritySchemes map[string]SecurityScheme `json:"securitySchemes,omitempty"`
}

// PathItem describes the operations available on a single path.
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
}

// Operation describes a single API operation on a path.
type Operation struct {
	Summary     string              `json:"summary"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
	Security    []SecurityReq       `json:"security,omitempty"`
}

// Parameter describes a single operation parameter.
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // "query", "header", "path", "cookie"
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody describes a single request body.
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// Response describes a single response from an API operation.
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
	Headers     map[string]Header    `json:"headers,omitempty"`
}

// MediaType provides schema for the media type.
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Header describes a single header.
type Header struct {
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// Schema represents a JSON Schema object.
type Schema struct {
	Type        string            `json:"type,omitempty"`
	Format      string            `json:"format,omitempty"`
	Description string            `json:"description,omitempty"`
	Properties  map[string]Schema `json:"properties,omitempty"`
	Items       *Schema           `json:"items,omitempty"`
	Required    []string          `json:"required,omitempty"`
	Nullable    bool              `json:"nullable,omitempty"`
	Ref         string            `json:"$ref,omitempty"`
	Default     any               `json:"default,omitempty"`
	Enum        []any             `json:"enum,omitempty"`
}

// SecurityScheme defines a security scheme that can be used by the operations.
type SecurityScheme struct {
	Type         string `json:"type"`
	Description  string `json:"description,omitempty"`
	Name         string `json:"name,omitempty"`         // For apiKey
	In           string `json:"in,omitempty"`           // For apiKey: "query", "header", "cookie"
	Scheme       string `json:"scheme,omitempty"`       // For http: "basic", "bearer"
	BearerFormat string `json:"bearerFormat,omitempty"` // For http bearer
}

// SecurityReq represents a security requirement.
// Map keys are security scheme names, values are lists of required scopes.
type SecurityReq map[string][]string
