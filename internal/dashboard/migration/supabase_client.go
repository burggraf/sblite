// Package migration provides tools for migrating sblite databases to Supabase.
package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

const supabaseAPIBaseURL = "https://api.supabase.com"

// SupabaseClient is a client for the Supabase Management API.
type SupabaseClient struct {
	token      string
	httpClient *http.Client
	baseURL    string
}

// Project represents a Supabase project.
type Project struct {
	ID             string `json:"id"`
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Region         string `json:"region"`
	CreatedAt      string `json:"created_at"`
	Database       struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"database"`
}

// APIKey represents an API key for a Supabase project.
type APIKey struct {
	Name   string `json:"name"`
	APIKey string `json:"api_key"`
}

// Secret represents a secret for edge functions.
type Secret struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// FunctionMetadata contains metadata for deploying an edge function.
type FunctionMetadata struct {
	Name           string `json:"name"`
	EntrypointPath string `json:"entrypoint_path"`
	VerifyJWT      *bool  `json:"verify_jwt,omitempty"`
}

// AuthConfig represents the authentication configuration for a project.
type AuthConfig map[string]interface{}

// NewSupabaseClient creates a new Supabase Management API client.
func NewSupabaseClient(token string) *SupabaseClient {
	return &SupabaseClient{
		token: token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: supabaseAPIBaseURL,
	}
}

// doRequest performs an HTTP request to the Supabase Management API.
func (c *SupabaseClient) doRequest(method, path string, body []byte) (*http.Response, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// ValidateToken validates the API token by attempting to list projects.
func (c *SupabaseClient) ValidateToken() error {
	_, err := c.ListProjects()
	if err != nil {
		return fmt.Errorf("invalid token: %w", err)
	}
	return nil
}

// ListProjects returns all projects accessible with the current token.
func (c *SupabaseClient) ListProjects() ([]Project, error) {
	resp, err := c.doRequest(http.MethodGet, "/v1/projects", nil)
	if err != nil {
		return nil, fmt.Errorf("listing projects: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("listing projects: status %d: %s", resp.StatusCode, string(body))
	}

	var projects []Project
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		return nil, fmt.Errorf("decoding projects: %w", err)
	}

	return projects, nil
}

// GetProject returns details for a specific project.
func (c *SupabaseClient) GetProject(ref string) (*Project, error) {
	resp, err := c.doRequest(http.MethodGet, "/v1/projects/"+ref, nil)
	if err != nil {
		return nil, fmt.Errorf("getting project: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting project: status %d: %s", resp.StatusCode, string(body))
	}

	var project Project
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		return nil, fmt.Errorf("decoding project: %w", err)
	}

	return &project, nil
}

// GetAPIKeys returns all API keys for a project.
func (c *SupabaseClient) GetAPIKeys(projectRef string) ([]APIKey, error) {
	resp, err := c.doRequest(http.MethodGet, "/v1/projects/"+projectRef+"/api-keys", nil)
	if err != nil {
		return nil, fmt.Errorf("getting API keys: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting API keys: status %d: %s", resp.StatusCode, string(body))
	}

	var keys []APIKey
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return nil, fmt.Errorf("decoding API keys: %w", err)
	}

	return keys, nil
}

// CreateSecrets creates or updates secrets for edge functions in a project.
func (c *SupabaseClient) CreateSecrets(projectRef string, secrets []Secret) error {
	body, err := json.Marshal(secrets)
	if err != nil {
		return fmt.Errorf("marshaling secrets: %w", err)
	}

	resp, err := c.doRequest(http.MethodPost, "/v1/projects/"+projectRef+"/secrets", body)
	if err != nil {
		return fmt.Errorf("creating secrets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("creating secrets: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteSecrets deletes secrets by name from a project.
func (c *SupabaseClient) DeleteSecrets(projectRef string, names []string) error {
	body, err := json.Marshal(names)
	if err != nil {
		return fmt.Errorf("marshaling secret names: %w", err)
	}

	resp, err := c.doRequest(http.MethodDelete, "/v1/projects/"+projectRef+"/secrets", body)
	if err != nil {
		return fmt.Errorf("deleting secrets: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting secrets: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeployFunction deploys an edge function to a Supabase project.
func (c *SupabaseClient) DeployFunction(projectRef, slug string, metadata FunctionMetadata, fileContent []byte) error {
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add metadata field as JSON
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshaling function metadata: %w", err)
	}
	if err := writer.WriteField("metadata", string(metadataJSON)); err != nil {
		return fmt.Errorf("writing metadata field: %w", err)
	}

	// Add file field
	filePart, err := writer.CreateFormFile("file", slug+".tar.gz")
	if err != nil {
		return fmt.Errorf("creating file field: %w", err)
	}
	if _, err := filePart.Write(fileContent); err != nil {
		return fmt.Errorf("writing file content: %w", err)
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing multipart writer: %w", err)
	}

	// Create request
	url := c.baseURL + "/v1/projects/" + projectRef + "/functions/deploy"
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return fmt.Errorf("creating deploy request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("deploying function: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deploying function: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// DeleteFunction deletes an edge function from a Supabase project.
func (c *SupabaseClient) DeleteFunction(projectRef, slug string) error {
	resp, err := c.doRequest(http.MethodDelete, "/v1/projects/"+projectRef+"/functions/"+slug, nil)
	if err != nil {
		return fmt.Errorf("deleting function: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("deleting function: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// GetAuthConfig returns the authentication configuration for a project.
func (c *SupabaseClient) GetAuthConfig(projectRef string) (AuthConfig, error) {
	resp, err := c.doRequest(http.MethodGet, "/v1/projects/"+projectRef+"/config/auth", nil)
	if err != nil {
		return nil, fmt.Errorf("getting auth config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("getting auth config: status %d: %s", resp.StatusCode, string(body))
	}

	var config AuthConfig
	if err := json.NewDecoder(resp.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("decoding auth config: %w", err)
	}

	return config, nil
}

// UpdateAuthConfig updates the authentication configuration for a project.
func (c *SupabaseClient) UpdateAuthConfig(projectRef string, config AuthConfig) error {
	body, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling auth config: %w", err)
	}

	resp, err := c.doRequest(http.MethodPatch, "/v1/projects/"+projectRef+"/config/auth", body)
	if err != nil {
		return fmt.Errorf("updating auth config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("updating auth config: status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
