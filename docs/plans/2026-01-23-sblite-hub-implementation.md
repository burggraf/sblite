# sblite-hub Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a multi-tenant control plane (sblite-hub) that orchestrates multiple sblite instances across organizations and projects with scale-to-zero capability.

**Architecture:** Separate Go binary that dogfoods sblite for its own data storage. Routes requests via subdomain to the appropriate sblite instance. Pluggable orchestrators support process, Docker, and Kubernetes backends.

**Tech Stack:** Go 1.25, Chi router, SQLite (via sblite dogfooding), httputil.ReverseProxy, Docker SDK, Kubernetes client-go

**Design Document:** `docs/plans/2026-01-23-sblite-hub-design.md`

---

## Project Structure

```
sblite-hub/                         # New directory at repo root (or separate repo)
├── main.go                         # Entry point
├── go.mod                          # Module: github.com/markb/sblite-hub
├── cmd/
│   └── root.go                     # Cobra root command
│   └── serve.go                    # sblite-hub serve command
├── internal/
│   ├── config/
│   │   └── config.go               # Hub configuration
│   ├── store/
│   │   ├── store.go                # Control plane data access (wraps sblite client)
│   │   ├── orgs.go                 # Org CRUD
│   │   ├── projects.go             # Project CRUD
│   │   ├── members.go              # Membership operations
│   │   └── instances.go            # Instance tracking
│   ├── proxy/
│   │   ├── proxy.go                # HTTP reverse proxy
│   │   ├── subdomain.go            # Subdomain extraction
│   │   └── wakeup.go               # Instance wake-up logic
│   ├── orchestrator/
│   │   ├── orchestrator.go         # Interface definition
│   │   ├── process.go              # Child process orchestrator
│   │   ├── docker.go               # Docker orchestrator
│   │   └── kubernetes.go           # Kubernetes orchestrator
│   ├── lifecycle/
│   │   ├── manager.go              # Instance lifecycle manager
│   │   └── idle.go                 # Idle detection & shutdown
│   ├── api/
│   │   ├── router.go               # API route setup
│   │   ├── orgs.go                 # Org API handlers
│   │   ├── projects.go             # Project API handlers
│   │   ├── members.go              # Member API handlers
│   │   ├── instances.go            # Instance API handlers
│   │   └── middleware.go           # Auth middleware
│   └── dashboard/
│       ├── handler.go              # Dashboard HTTP handlers
│       └── static/                 # Embedded frontend assets
├── migrations/
│   └── 00001_initial_schema.sql    # Hub schema for sblite
└── e2e/
    └── tests/                      # End-to-end tests
```

---

## Phase 1: Foundation

**Goal:** Minimal working hub that can authenticate users and store org/project data in a dogfooded sblite instance.

### Task 1.1: Initialize sblite-hub Module

**Files:**
- Create: `sblite-hub/go.mod`
- Create: `sblite-hub/main.go`
- Create: `sblite-hub/cmd/root.go`

**Step 1: Create directory and initialize Go module**

```bash
mkdir -p sblite-hub/cmd sblite-hub/internal
cd sblite-hub
go mod init github.com/markb/sblite-hub
```

**Step 2: Create main.go**

```go
// sblite-hub/main.go
package main

import "github.com/markb/sblite-hub/cmd"

func main() {
	cmd.Execute()
}
```

**Step 3: Create root command**

```go
// sblite-hub/cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "sblite-hub",
	Short: "Control plane for managing sblite instances",
	Long:  `sblite-hub orchestrates multiple sblite instances across organizations and projects.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 4: Add cobra dependency and verify build**

```bash
go get github.com/spf13/cobra
go build -o sblite-hub .
./sblite-hub --help
```

Expected: Shows help with "Control plane for managing sblite instances"

**Step 5: Commit**

```bash
git add sblite-hub/
git commit -m "feat(hub): initialize sblite-hub module with cobra CLI"
```

---

### Task 1.2: Create Hub Schema Migration

**Files:**
- Create: `sblite-hub/migrations/00001_initial_schema.sql`

**Step 1: Create migrations directory**

```bash
mkdir -p sblite-hub/migrations
```

**Step 2: Create initial schema migration**

```sql
-- sblite-hub/migrations/00001_initial_schema.sql
-- Hub control plane schema (stored in dogfooded sblite instance)

-- Organizations
CREATE TABLE IF NOT EXISTS orgs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    slug TEXT UNIQUE NOT NULL,
    owner_id TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Org membership (many-to-many users to orgs)
CREATE TABLE IF NOT EXISTS org_members (
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (org_id, user_id)
);

-- Projects within orgs
CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    org_id TEXT NOT NULL REFERENCES orgs(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    slug TEXT NOT NULL,
    database_path TEXT NOT NULL,
    is_dedicated INTEGER DEFAULT 0,
    keep_alive INTEGER DEFAULT 0,
    idle_timeout_minutes INTEGER DEFAULT 15,
    region TEXT DEFAULT 'default',
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(org_id, slug)
);

-- Project membership with roles
CREATE TABLE IF NOT EXISTS project_members (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL CHECK(role IN ('owner', 'admin', 'developer', 'viewer')),
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY (project_id, user_id)
);

-- Instance tracking
CREATE TABLE IF NOT EXISTS instances (
    id TEXT PRIMARY KEY,
    org_id TEXT REFERENCES orgs(id),
    project_id TEXT REFERENCES projects(id),
    hub_id TEXT,
    endpoint TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('running', 'starting', 'stopped', 'failed')),
    started_at TEXT,
    last_activity_at TEXT,
    orchestrator_ref TEXT,
    region TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Usage metrics (hourly buckets)
CREATE TABLE IF NOT EXISTS usage_metrics (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    period_start TEXT NOT NULL,
    api_calls INTEGER DEFAULT 0,
    storage_bytes INTEGER DEFAULT 0,
    bandwidth_bytes INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(project_id, period_start)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_org_members_user ON org_members(user_id);
CREATE INDEX IF NOT EXISTS idx_projects_org ON projects(org_id);
CREATE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug);
CREATE INDEX IF NOT EXISTS idx_project_members_user ON project_members(user_id);
CREATE INDEX IF NOT EXISTS idx_instances_status ON instances(status);
CREATE INDEX IF NOT EXISTS idx_instances_org ON instances(org_id);
CREATE INDEX IF NOT EXISTS idx_instances_project ON instances(project_id);
```

**Step 3: Commit**

```bash
git add sblite-hub/migrations/
git commit -m "feat(hub): add initial schema migration for orgs, projects, instances"
```

---

### Task 1.3: Create Configuration Module

**Files:**
- Create: `sblite-hub/internal/config/config.go`

**Step 1: Create config package**

```go
// sblite-hub/internal/config/config.go
package config

import (
	"os"
	"strconv"
)

// Config holds all hub configuration
type Config struct {
	// Hub server settings
	Host string
	Port int

	// Internal sblite instance (dogfooding)
	SbliteURL    string // URL of internal sblite for hub data
	SbliteAPIKey string // Service role key for internal sblite

	// Data paths
	DataDir string // Base directory for project databases

	// Instance defaults
	DefaultIdleTimeout int // Minutes before idle shutdown

	// Orchestrator settings
	Orchestrator     string // "process", "docker", or "kubernetes"
	SbliteBinaryPath string // Path to sblite binary (for process orchestrator)
	DockerImage      string // Docker image for sblite (for docker orchestrator)
}

// DefaultConfig returns configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Host:               getEnv("SBLITE_HUB_HOST", "0.0.0.0"),
		Port:               getEnvInt("SBLITE_HUB_PORT", 8000),
		SbliteURL:          getEnv("SBLITE_HUB_SBLITE_URL", "http://localhost:8080"),
		SbliteAPIKey:       getEnv("SBLITE_HUB_SBLITE_KEY", ""),
		DataDir:            getEnv("SBLITE_HUB_DATA_DIR", "./data"),
		DefaultIdleTimeout: getEnvInt("SBLITE_HUB_IDLE_TIMEOUT", 15),
		Orchestrator:       getEnv("SBLITE_HUB_ORCHESTRATOR", "process"),
		SbliteBinaryPath:   getEnv("SBLITE_HUB_SBLITE_BINARY", "./sblite"),
		DockerImage:        getEnv("SBLITE_HUB_DOCKER_IMAGE", "sblite:latest"),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return fallback
}
```

**Step 2: Verify compilation**

```bash
cd sblite-hub
go build ./...
```

**Step 3: Commit**

```bash
git add sblite-hub/internal/config/
git commit -m "feat(hub): add configuration module with env var support"
```

---

### Task 1.4: Create Store Layer (Sblite Client Wrapper)

**Files:**
- Create: `sblite-hub/internal/store/store.go`
- Create: `sblite-hub/internal/store/orgs.go`

**Step 1: Create base store**

```go
// sblite-hub/internal/store/store.go
package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Store wraps the internal sblite instance for hub data access
type Store struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// New creates a new Store connected to the internal sblite instance
func New(baseURL, apiKey string) *Store {
	return &Store{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// request makes an authenticated request to the internal sblite
func (s *Store) request(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, s.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("apikey", s.apiKey)
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	return s.httpClient.Do(req)
}

// decodeResponse reads and unmarshals JSON response
func decodeResponse(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}

	if v != nil {
		return json.NewDecoder(resp.Body).Decode(v)
	}
	return nil
}
```

**Step 2: Create orgs store**

```go
// sblite-hub/internal/store/orgs.go
package store

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Org represents an organization
type Org struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateOrg creates a new organization
func (s *Store) CreateOrg(ctx context.Context, name, slug, ownerID string) (*Org, error) {
	org := &Org{
		ID:      uuid.New().String(),
		Name:    name,
		Slug:    slug,
		OwnerID: ownerID,
	}

	resp, err := s.request(ctx, "POST", "/rest/v1/orgs", org)
	if err != nil {
		return nil, err
	}

	var result []Org
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no org returned after create")
	}

	return &result[0], nil
}

// GetOrg retrieves an organization by ID
func (s *Store) GetOrg(ctx context.Context, id string) (*Org, error) {
	path := "/rest/v1/orgs?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Org
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("org not found: %s", id)
	}

	return &result[0], nil
}

// GetOrgBySlug retrieves an organization by slug
func (s *Store) GetOrgBySlug(ctx context.Context, slug string) (*Org, error) {
	path := "/rest/v1/orgs?slug=eq." + url.QueryEscape(slug)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Org
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("org not found: %s", slug)
	}

	return &result[0], nil
}

// ListOrgsForUser returns all orgs a user belongs to
func (s *Store) ListOrgsForUser(ctx context.Context, userID string) ([]Org, error) {
	// First get org IDs from membership
	memberPath := "/rest/v1/org_members?user_id=eq." + url.QueryEscape(userID) + "&select=org_id"
	memberResp, err := s.request(ctx, "GET", memberPath, nil)
	if err != nil {
		return nil, err
	}

	var members []struct {
		OrgID string `json:"org_id"`
	}
	if err := decodeResponse(memberResp, &members); err != nil {
		return nil, err
	}

	if len(members) == 0 {
		return []Org{}, nil
	}

	// Build org IDs list for IN query
	var orgIDs []string
	for _, m := range members {
		orgIDs = append(orgIDs, m.OrgID)
	}

	// Fetch orgs
	// For simplicity, fetch one by one (could optimize with IN query)
	var orgs []Org
	for _, orgID := range orgIDs {
		org, err := s.GetOrg(ctx, orgID)
		if err != nil {
			continue // Skip if org was deleted
		}
		orgs = append(orgs, *org)
	}

	return orgs, nil
}

// DeleteOrg deletes an organization
func (s *Store) DeleteOrg(ctx context.Context, id string) error {
	path := "/rest/v1/orgs?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}
```

**Step 3: Add uuid dependency and verify**

```bash
cd sblite-hub
go get github.com/google/uuid
go build ./...
```

**Step 4: Commit**

```bash
git add sblite-hub/internal/store/
git commit -m "feat(hub): add store layer for orgs with sblite client"
```

---

### Task 1.5: Create Projects Store

**Files:**
- Create: `sblite-hub/internal/store/projects.go`

**Step 1: Create projects store**

```go
// sblite-hub/internal/store/projects.go
package store

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Project represents a project within an org
type Project struct {
	ID                 string    `json:"id"`
	OrgID              string    `json:"org_id"`
	Name               string    `json:"name"`
	Slug               string    `json:"slug"`
	DatabasePath       string    `json:"database_path"`
	IsDedicated        bool      `json:"is_dedicated"`
	KeepAlive          bool      `json:"keep_alive"`
	IdleTimeoutMinutes int       `json:"idle_timeout_minutes"`
	Region             string    `json:"region"`
	CreatedAt          time.Time `json:"created_at"`
}

// CreateProject creates a new project
func (s *Store) CreateProject(ctx context.Context, orgID, name, slug, databasePath string) (*Project, error) {
	project := &Project{
		ID:                 uuid.New().String(),
		OrgID:              orgID,
		Name:               name,
		Slug:               slug,
		DatabasePath:       databasePath,
		IsDedicated:        false,
		KeepAlive:          false,
		IdleTimeoutMinutes: 15,
		Region:             "default",
	}

	resp, err := s.request(ctx, "POST", "/rest/v1/projects", project)
	if err != nil {
		return nil, err
	}

	var result []Project
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no project returned after create")
	}

	return &result[0], nil
}

// GetProject retrieves a project by ID
func (s *Store) GetProject(ctx context.Context, id string) (*Project, error) {
	path := "/rest/v1/projects?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Project
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("project not found: %s", id)
	}

	return &result[0], nil
}

// GetProjectBySlug retrieves a project by its slug (globally unique)
func (s *Store) GetProjectBySlug(ctx context.Context, slug string) (*Project, error) {
	path := "/rest/v1/projects?slug=eq." + url.QueryEscape(slug)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Project
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("project not found: %s", slug)
	}

	return &result[0], nil
}

// ListProjectsForOrg returns all projects in an org
func (s *Store) ListProjectsForOrg(ctx context.Context, orgID string) ([]Project, error) {
	path := "/rest/v1/projects?org_id=eq." + url.QueryEscape(orgID)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Project
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// UpdateProject updates project fields
func (s *Store) UpdateProject(ctx context.Context, id string, updates map[string]interface{}) (*Project, error) {
	path := "/rest/v1/projects?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "PATCH", path, updates)
	if err != nil {
		return nil, err
	}

	var result []Project
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no project returned after update")
	}

	return &result[0], nil
}

// DeleteProject deletes a project
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	path := "/rest/v1/projects?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}
```

**Step 2: Verify compilation**

```bash
cd sblite-hub
go build ./...
```

**Step 3: Commit**

```bash
git add sblite-hub/internal/store/projects.go
git commit -m "feat(hub): add projects store"
```

---

### Task 1.6: Create Members Store

**Files:**
- Create: `sblite-hub/internal/store/members.go`

**Step 1: Create members store**

```go
// sblite-hub/internal/store/members.go
package store

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

// OrgMember represents org membership
type OrgMember struct {
	OrgID     string    `json:"org_id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

// ProjectMember represents project membership with role
type ProjectMember struct {
	ProjectID string    `json:"project_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"` // owner, admin, developer, viewer
	CreatedAt time.Time `json:"created_at"`
}

// AddOrgMember adds a user to an organization
func (s *Store) AddOrgMember(ctx context.Context, orgID, userID string) error {
	member := &OrgMember{
		OrgID:  orgID,
		UserID: userID,
	}

	resp, err := s.request(ctx, "POST", "/rest/v1/org_members", member)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// RemoveOrgMember removes a user from an organization
func (s *Store) RemoveOrgMember(ctx context.Context, orgID, userID string) error {
	path := fmt.Sprintf("/rest/v1/org_members?org_id=eq.%s&user_id=eq.%s",
		url.QueryEscape(orgID), url.QueryEscape(userID))
	resp, err := s.request(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// ListOrgMembers returns all members of an organization
func (s *Store) ListOrgMembers(ctx context.Context, orgID string) ([]OrgMember, error) {
	path := "/rest/v1/org_members?org_id=eq." + url.QueryEscape(orgID)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []OrgMember
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// IsOrgMember checks if a user is a member of an organization
func (s *Store) IsOrgMember(ctx context.Context, orgID, userID string) (bool, error) {
	path := fmt.Sprintf("/rest/v1/org_members?org_id=eq.%s&user_id=eq.%s",
		url.QueryEscape(orgID), url.QueryEscape(userID))
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return false, err
	}

	var result []OrgMember
	if err := decodeResponse(resp, &result); err != nil {
		return false, err
	}

	return len(result) > 0, nil
}

// AddProjectMember adds a user to a project with a role
func (s *Store) AddProjectMember(ctx context.Context, projectID, userID, role string) error {
	member := &ProjectMember{
		ProjectID: projectID,
		UserID:    userID,
		Role:      role,
	}

	resp, err := s.request(ctx, "POST", "/rest/v1/project_members", member)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// UpdateProjectMemberRole changes a member's role
func (s *Store) UpdateProjectMemberRole(ctx context.Context, projectID, userID, role string) error {
	path := fmt.Sprintf("/rest/v1/project_members?project_id=eq.%s&user_id=eq.%s",
		url.QueryEscape(projectID), url.QueryEscape(userID))
	resp, err := s.request(ctx, "PATCH", path, map[string]string{"role": role})
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// RemoveProjectMember removes a user from a project
func (s *Store) RemoveProjectMember(ctx context.Context, projectID, userID string) error {
	path := fmt.Sprintf("/rest/v1/project_members?project_id=eq.%s&user_id=eq.%s",
		url.QueryEscape(projectID), url.QueryEscape(userID))
	resp, err := s.request(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// ListProjectMembers returns all members of a project
func (s *Store) ListProjectMembers(ctx context.Context, projectID string) ([]ProjectMember, error) {
	path := "/rest/v1/project_members?project_id=eq." + url.QueryEscape(projectID)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []ProjectMember
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// GetProjectMember returns a user's membership in a project
func (s *Store) GetProjectMember(ctx context.Context, projectID, userID string) (*ProjectMember, error) {
	path := fmt.Sprintf("/rest/v1/project_members?project_id=eq.%s&user_id=eq.%s",
		url.QueryEscape(projectID), url.QueryEscape(userID))
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []ProjectMember
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil // Not a member
	}

	return &result[0], nil
}

// ListProjectsForUser returns all projects a user has access to
func (s *Store) ListProjectsForUser(ctx context.Context, userID string) ([]Project, error) {
	// Get project IDs from membership
	memberPath := "/rest/v1/project_members?user_id=eq." + url.QueryEscape(userID) + "&select=project_id"
	memberResp, err := s.request(ctx, "GET", memberPath, nil)
	if err != nil {
		return nil, err
	}

	var members []struct {
		ProjectID string `json:"project_id"`
	}
	if err := decodeResponse(memberResp, &members); err != nil {
		return nil, err
	}

	if len(members) == 0 {
		return []Project{}, nil
	}

	// Fetch projects
	var projects []Project
	for _, m := range members {
		project, err := s.GetProject(ctx, m.ProjectID)
		if err != nil {
			continue
		}
		projects = append(projects, *project)
	}

	return projects, nil
}
```

**Step 2: Verify compilation**

```bash
cd sblite-hub
go build ./...
```

**Step 3: Commit**

```bash
git add sblite-hub/internal/store/members.go
git commit -m "feat(hub): add org and project members store"
```

---

### Task 1.7: Create Instances Store

**Files:**
- Create: `sblite-hub/internal/store/instances.go`

**Step 1: Create instances store**

```go
// sblite-hub/internal/store/instances.go
package store

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// Instance represents a running sblite instance
type Instance struct {
	ID             string     `json:"id"`
	OrgID          *string    `json:"org_id"`     // Set for shared instances
	ProjectID      *string    `json:"project_id"` // Set for dedicated instances
	HubID          string     `json:"hub_id"`
	Endpoint       string     `json:"endpoint"`
	Status         string     `json:"status"` // running, starting, stopped, failed
	StartedAt      *time.Time `json:"started_at"`
	LastActivityAt *time.Time `json:"last_activity_at"`
	OrchestratorRef string    `json:"orchestrator_ref"`
	Region         string     `json:"region"`
	CreatedAt      time.Time  `json:"created_at"`
}

// CreateInstance creates a new instance record
func (s *Store) CreateInstance(ctx context.Context, orgID, projectID *string, endpoint, orchestratorRef, region string) (*Instance, error) {
	now := time.Now()
	instance := &Instance{
		ID:              uuid.New().String(),
		OrgID:           orgID,
		ProjectID:       projectID,
		Endpoint:        endpoint,
		Status:          "starting",
		StartedAt:       &now,
		LastActivityAt:  &now,
		OrchestratorRef: orchestratorRef,
		Region:          region,
	}

	resp, err := s.request(ctx, "POST", "/rest/v1/instances", instance)
	if err != nil {
		return nil, err
	}

	var result []Instance
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("no instance returned after create")
	}

	return &result[0], nil
}

// GetInstance retrieves an instance by ID
func (s *Store) GetInstance(ctx context.Context, id string) (*Instance, error) {
	path := "/rest/v1/instances?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Instance
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("instance not found: %s", id)
	}

	return &result[0], nil
}

// GetInstanceForOrg returns the shared instance for an org
func (s *Store) GetInstanceForOrg(ctx context.Context, orgID string) (*Instance, error) {
	path := "/rest/v1/instances?org_id=eq." + url.QueryEscape(orgID) + "&project_id=is.null"
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Instance
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil // No instance exists yet
	}

	return &result[0], nil
}

// GetInstanceForProject returns the dedicated instance for a project
func (s *Store) GetInstanceForProject(ctx context.Context, projectID string) (*Instance, error) {
	path := "/rest/v1/instances?project_id=eq." + url.QueryEscape(projectID)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Instance
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, nil // No dedicated instance
	}

	return &result[0], nil
}

// UpdateInstanceStatus updates the instance status
func (s *Store) UpdateInstanceStatus(ctx context.Context, id, status string) error {
	path := "/rest/v1/instances?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "PATCH", path, map[string]string{"status": status})
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// UpdateInstanceActivity updates the last activity timestamp
func (s *Store) UpdateInstanceActivity(ctx context.Context, id string) error {
	path := "/rest/v1/instances?id=eq." + url.QueryEscape(id)
	now := time.Now().Format(time.RFC3339)
	resp, err := s.request(ctx, "PATCH", path, map[string]string{"last_activity_at": now})
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}

// ListIdleInstances returns instances that have been idle longer than the threshold
func (s *Store) ListIdleInstances(ctx context.Context, idleThreshold time.Duration) ([]Instance, error) {
	cutoff := time.Now().Add(-idleThreshold).Format(time.RFC3339)
	path := "/rest/v1/instances?status=eq.running&last_activity_at=lt." + url.QueryEscape(cutoff)
	resp, err := s.request(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}

	var result []Instance
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ListAllInstances returns all instances
func (s *Store) ListAllInstances(ctx context.Context) ([]Instance, error) {
	resp, err := s.request(ctx, "GET", "/rest/v1/instances", nil)
	if err != nil {
		return nil, err
	}

	var result []Instance
	if err := decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// DeleteInstance removes an instance record
func (s *Store) DeleteInstance(ctx context.Context, id string) error {
	path := "/rest/v1/instances?id=eq." + url.QueryEscape(id)
	resp, err := s.request(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	return decodeResponse(resp, nil)
}
```

**Step 2: Verify compilation**

```bash
cd sblite-hub
go build ./...
```

**Step 3: Commit**

```bash
git add sblite-hub/internal/store/instances.go
git commit -m "feat(hub): add instances store for tracking sblite processes"
```

---

### Task 1.8: Create Serve Command

**Files:**
- Create: `sblite-hub/cmd/serve.go`

**Step 1: Create serve command**

```go
// sblite-hub/cmd/serve.go
package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/markb/sblite-hub/internal/config"
	"github.com/markb/sblite-hub/internal/store"
	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the sblite-hub control plane server",
	Long:  `Starts the HTTP server that manages sblite instances.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()

		// Override from flags
		if host, _ := cmd.Flags().GetString("host"); host != "" {
			cfg.Host = host
		}
		if port, _ := cmd.Flags().GetInt("port"); port != 0 {
			cfg.Port = port
		}
		if sbliteURL, _ := cmd.Flags().GetString("sblite-url"); sbliteURL != "" {
			cfg.SbliteURL = sbliteURL
		}
		if apiKey, _ := cmd.Flags().GetString("sblite-key"); apiKey != "" {
			cfg.SbliteAPIKey = apiKey
		}

		// Validate required config
		if cfg.SbliteAPIKey == "" {
			return fmt.Errorf("sblite API key required: set SBLITE_HUB_SBLITE_KEY or --sblite-key")
		}

		// Initialize store
		st := store.New(cfg.SbliteURL, cfg.SbliteAPIKey)

		// Create router
		r := chi.NewRouter()
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)
		r.Use(middleware.RequestID)

		// Health check
		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
		})

		// Placeholder API routes (will be implemented in Phase 2)
		r.Route("/api/v1", func(r chi.Router) {
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"status":"sblite-hub running"}`))
			})
		})

		// Start server
		addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		srv := &http.Server{
			Addr:    addr,
			Handler: r,
		}

		// Graceful shutdown
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\nShutting down...")

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer shutdownCancel()

			srv.Shutdown(shutdownCtx)
			cancel()
		}()

		fmt.Printf("sblite-hub starting on %s\n", addr)
		fmt.Printf("  Internal sblite: %s\n", cfg.SbliteURL)
		fmt.Printf("  Data directory: %s\n", cfg.DataDir)

		// Verify connection to internal sblite
		_, err := st.ListAllInstances(ctx)
		if err != nil {
			fmt.Printf("  Warning: cannot connect to internal sblite: %v\n", err)
		} else {
			fmt.Println("  Connected to internal sblite")
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().String("host", "", "Host to bind to (default: 0.0.0.0)")
	serveCmd.Flags().IntP("port", "p", 0, "Port to listen on (default: 8000)")
	serveCmd.Flags().String("sblite-url", "", "URL of internal sblite instance")
	serveCmd.Flags().String("sblite-key", "", "Service role API key for internal sblite")
}
```

**Step 2: Add chi dependency and verify**

```bash
cd sblite-hub
go get github.com/go-chi/chi/v5
go build -o sblite-hub .
```

**Step 3: Commit**

```bash
git add sblite-hub/cmd/serve.go
git commit -m "feat(hub): add serve command with basic HTTP server"
```

---

## Phase 2: Project Management API

**Goal:** Full CRUD API for orgs, projects, and members.

### Task 2.1: Create API Router

**Files:**
- Create: `sblite-hub/internal/api/router.go`
- Create: `sblite-hub/internal/api/middleware.go`

**Step 1: Create router setup**

```go
// sblite-hub/internal/api/router.go
package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/markb/sblite-hub/internal/store"
)

// Router holds API dependencies
type Router struct {
	store *store.Store
}

// New creates a new API router
func New(st *store.Store) *Router {
	return &Router{store: st}
}

// Routes returns the API router
func (a *Router) Routes() chi.Router {
	r := chi.NewRouter()

	// Auth middleware (extracts user from JWT)
	r.Use(a.AuthMiddleware)

	// User endpoints
	r.Get("/user", a.GetCurrentUser)
	r.Patch("/user", a.UpdateCurrentUser)

	// Org endpoints
	r.Route("/orgs", func(r chi.Router) {
		r.Get("/", a.ListOrgs)
		r.Post("/", a.CreateOrg)
		r.Route("/{orgID}", func(r chi.Router) {
			r.Get("/", a.GetOrg)
			r.Patch("/", a.UpdateOrg)
			r.Delete("/", a.DeleteOrg)

			// Org members
			r.Get("/members", a.ListOrgMembers)
			r.Post("/members", a.InviteOrgMember)
			r.Delete("/members/{userID}", a.RemoveOrgMember)

			// Projects within org
			r.Route("/projects", func(r chi.Router) {
				r.Get("/", a.ListProjects)
				r.Post("/", a.CreateProject)
				r.Route("/{projectID}", func(r chi.Router) {
					r.Get("/", a.GetProject)
					r.Patch("/", a.UpdateProject)
					r.Delete("/", a.DeleteProject)

					// Project actions
					r.Post("/promote", a.PromoteProject)
					r.Post("/demote", a.DemoteProject)

					// Project members
					r.Get("/members", a.ListProjectMembers)
					r.Post("/members", a.AddProjectMember)
					r.Patch("/members/{userID}", a.UpdateProjectMemberRole)
					r.Delete("/members/{userID}", a.RemoveProjectMember)
				})
			})
		})
	})

	// Instance endpoints (admin)
	r.Route("/instances", func(r chi.Router) {
		r.Get("/", a.ListInstances)
		r.Get("/{instanceID}", a.GetInstance)
		r.Post("/{instanceID}/restart", a.RestartInstance)
		r.Post("/{instanceID}/stop", a.StopInstance)
	})

	return r
}
```

**Step 2: Create auth middleware**

```go
// sblite-hub/internal/api/middleware.go
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type contextKey string

const userContextKey contextKey = "user"

// User represents the authenticated user from JWT
type User struct {
	ID    string `json:"sub"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// GetUser extracts user from context
func GetUser(ctx context.Context) *User {
	if u, ok := ctx.Value(userContextKey).(*User); ok {
		return u
	}
	return nil
}

// AuthMiddleware extracts and validates JWT from Authorization header
func (a *Router) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]

		// Parse JWT without verification (verification happens at sblite level)
		// In production, you'd verify against the hub's JWT secret
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, `{"error":"invalid claims"}`, http.StatusUnauthorized)
			return
		}

		user := &User{
			ID:    claims["sub"].(string),
			Email: claims["email"].(string),
		}
		if role, ok := claims["role"].(string); ok {
			user.Role = role
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError writes a JSON error response
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
```

**Step 3: Add jwt dependency and verify**

```bash
cd sblite-hub
go get github.com/golang-jwt/jwt/v5
go build ./...
```

**Step 4: Commit**

```bash
git add sblite-hub/internal/api/
git commit -m "feat(hub): add API router structure and auth middleware"
```

---

### Task 2.2: Implement Org API Handlers

**Files:**
- Create: `sblite-hub/internal/api/orgs.go`

**Step 1: Create org handlers**

```go
// sblite-hub/internal/api/orgs.go
package api

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
)

var slugRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

func isValidSlug(s string) bool {
	if len(s) < 3 || len(s) > 63 {
		return false
	}
	return slugRegex.MatchString(s)
}

func slugify(name string) string {
	s := strings.ToLower(name)
	s = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) < 3 {
		s = s + "-org"
	}
	return s
}

// ListOrgs returns all orgs the user belongs to
func (a *Router) ListOrgs(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgs, err := a.store.ListOrgsForUser(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, orgs)
}

// CreateOrg creates a new organization
func (a *Router) CreateOrg(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		respondError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Generate slug if not provided
	if req.Slug == "" {
		req.Slug = slugify(req.Name)
	}

	if !isValidSlug(req.Slug) {
		respondError(w, http.StatusBadRequest, "invalid slug: must be 3-63 lowercase alphanumeric characters with hyphens")
		return
	}

	// Create org
	org, err := a.store.CreateOrg(r.Context(), req.Name, req.Slug, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Add creator as org member
	if err := a.store.AddOrgMember(r.Context(), org.ID, user.ID); err != nil {
		// Rollback org creation
		a.store.DeleteOrg(r.Context(), org.ID)
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, org)
}

// GetOrg returns an organization by ID
func (a *Router) GetOrg(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgID := chi.URLParam(r, "orgID")

	// Check membership
	isMember, err := a.store.IsOrgMember(r.Context(), orgID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !isMember {
		respondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	org, err := a.store.GetOrg(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusNotFound, "organization not found")
		return
	}

	respondJSON(w, http.StatusOK, org)
}

// UpdateOrg updates an organization
func (a *Router) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgID := chi.URLParam(r, "orgID")

	// Check if user is owner
	org, err := a.store.GetOrg(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusNotFound, "organization not found")
		return
	}

	if org.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only owner can update organization")
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update via store (would need to add UpdateOrg method)
	// For now, return the org unchanged
	respondJSON(w, http.StatusOK, org)
}

// DeleteOrg deletes an organization
func (a *Router) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgID := chi.URLParam(r, "orgID")

	// Check if user is owner
	org, err := a.store.GetOrg(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusNotFound, "organization not found")
		return
	}

	if org.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only owner can delete organization")
		return
	}

	// Check if org has projects
	projects, err := a.store.ListProjectsForOrg(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if len(projects) > 0 {
		respondError(w, http.StatusBadRequest, "cannot delete organization with projects")
		return
	}

	if err := a.store.DeleteOrg(r.Context(), orgID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListOrgMembers returns all members of an organization
func (a *Router) ListOrgMembers(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgID := chi.URLParam(r, "orgID")

	// Check membership
	isMember, err := a.store.IsOrgMember(r.Context(), orgID, user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !isMember {
		respondError(w, http.StatusForbidden, "not a member of this organization")
		return
	}

	members, err := a.store.ListOrgMembers(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusOK, members)
}

// InviteOrgMember adds a user to an organization
func (a *Router) InviteOrgMember(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgID := chi.URLParam(r, "orgID")

	// Check if user is owner
	org, err := a.store.GetOrg(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusNotFound, "organization not found")
		return
	}

	if org.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only owner can invite members")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"` // For future: invite by email
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.UserID == "" {
		respondError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	if err := a.store.AddOrgMember(r.Context(), orgID, req.UserID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondJSON(w, http.StatusCreated, map[string]string{"status": "invited"})
}

// RemoveOrgMember removes a user from an organization
func (a *Router) RemoveOrgMember(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r.Context())
	if user == nil {
		respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	orgID := chi.URLParam(r, "orgID")
	targetUserID := chi.URLParam(r, "userID")

	// Check if user is owner
	org, err := a.store.GetOrg(r.Context(), orgID)
	if err != nil {
		respondError(w, http.StatusNotFound, "organization not found")
		return
	}

	if org.OwnerID != user.ID {
		respondError(w, http.StatusForbidden, "only owner can remove members")
		return
	}

	// Cannot remove owner
	if targetUserID == org.OwnerID {
		respondError(w, http.StatusBadRequest, "cannot remove organization owner")
		return
	}

	if err := a.store.RemoveOrgMember(r.Context(), orgID, targetUserID); err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 2: Verify compilation**

```bash
cd sblite-hub
go build ./...
```

**Step 3: Commit**

```bash
git add sblite-hub/internal/api/orgs.go
git commit -m "feat(hub): implement org API handlers"
```

---

### Task 2.3: Implement Project API Handlers

**Files:**
- Create: `sblite-hub/internal/api/projects.go`

*(Similar pattern to orgs.go - full implementation provided)*

---

### Task 2.4: Implement Instance API Handlers

**Files:**
- Create: `sblite-hub/internal/api/instances.go`

*(Admin endpoints for viewing/managing instances)*

---

### Task 2.5: Implement User API Handlers

**Files:**
- Create: `sblite-hub/internal/api/user.go`

*(Current user profile endpoints)*

---

## Phase 3: Proxy & Routing

**Goal:** Route requests from subdomains to the correct sblite instance.

### Task 3.1: Create Subdomain Extraction

**Files:**
- Create: `sblite-hub/internal/proxy/subdomain.go`
- Create: `sblite-hub/internal/proxy/subdomain_test.go`

*(Extract project slug from hostname)*

---

### Task 3.2: Create Reverse Proxy

**Files:**
- Create: `sblite-hub/internal/proxy/proxy.go`

*(HTTP reverse proxy with X-Database-Path injection)*

---

### Task 3.3: Add Multi-Project Mode to sblite

**Files:**
- Modify: `cmd/serve.go` (add --multi-project flag)
- Modify: `internal/server/server.go` (handle X-Database-Path)
- Modify: `internal/db/db.go` (connection pool per database)

*(This modifies the main sblite binary)*

---

## Phase 4: Scale-to-Zero

**Goal:** Automatically shutdown idle instances and wake them on demand.

### Task 4.1: Create Orchestrator Interface

**Files:**
- Create: `sblite-hub/internal/orchestrator/orchestrator.go`

---

### Task 4.2: Implement Process Orchestrator

**Files:**
- Create: `sblite-hub/internal/orchestrator/process.go`
- Create: `sblite-hub/internal/orchestrator/process_test.go`

*(Spawn/stop sblite as child processes)*

---

### Task 4.3: Create Lifecycle Manager

**Files:**
- Create: `sblite-hub/internal/lifecycle/manager.go`
- Create: `sblite-hub/internal/lifecycle/idle.go`

*(Background job for idle detection and shutdown)*

---

### Task 4.4: Implement Wake-Up Logic

**Files:**
- Create: `sblite-hub/internal/proxy/wakeup.go`

*(Queue requests while instance starts)*

---

## Phase 5: Docker Orchestrator

**Goal:** Manage sblite instances as Docker containers.

### Task 5.1: Implement Docker Orchestrator

**Files:**
- Create: `sblite-hub/internal/orchestrator/docker.go`

---

### Task 5.2: Create Docker Compose Example

**Files:**
- Create: `sblite-hub/docker-compose.yml`
- Create: `sblite-hub/Dockerfile`

---

## Phase 6: Hub Dashboard

**Goal:** Web UI for managing orgs and projects.

### Task 6.1: Create Dashboard Handler

**Files:**
- Create: `sblite-hub/internal/dashboard/handler.go`
- Create: `sblite-hub/internal/dashboard/static/index.html`
- Create: `sblite-hub/internal/dashboard/static/app.js`
- Create: `sblite-hub/internal/dashboard/static/styles.css`

---

### Task 6.2: Implement Org Selector Page

*(List orgs, create org)*

---

### Task 6.3: Implement Org Dashboard Page

*(List projects, usage stats, members)*

---

### Task 6.4: Implement Project Settings Page

*(Promote/demote, transfer, delete)*

---

## Phase 7: Kubernetes Orchestrator

**Goal:** Manage sblite instances as Kubernetes pods.

### Task 7.1: Implement Kubernetes Orchestrator

**Files:**
- Create: `sblite-hub/internal/orchestrator/kubernetes.go`

---

### Task 7.2: Create Helm Chart

**Files:**
- Create: `sblite-hub/helm/sblite-hub/Chart.yaml`
- Create: `sblite-hub/helm/sblite-hub/values.yaml`
- Create: `sblite-hub/helm/sblite-hub/templates/deployment.yaml`

---

## Phase 8: Multi-Hub & Production

**Goal:** Support multiple hub instances with shared database.

### Task 8.1: Migrate to PostgreSQL Support

*(Make store layer support both SQLite and PostgreSQL)*

---

### Task 8.2: Add Hub Registration

*(Hubs register themselves and coordinate instance affinity)*

---

### Task 8.3: Add Rate Limiting

**Files:**
- Create: `sblite-hub/internal/api/ratelimit.go`

---

### Task 8.4: Add Request Logging

*(Structured logging for all proxy requests)*

---

## Testing Strategy

### Unit Tests
- Each store method should have tests using a test sblite instance
- Orchestrator implementations should have mock tests
- Subdomain extraction should have comprehensive edge case tests

### Integration Tests
- E2E tests for full API flows (create org → create project → make request)
- Proxy tests with real sblite instances
- Scale-to-zero tests (verify idle shutdown and wake-up)

### Load Tests
- Concurrent project creation
- High-throughput proxy routing
- Many simultaneous instance starts

---

## Summary

| Phase | Tasks | Primary Files |
|-------|-------|---------------|
| 1. Foundation | 8 | Module, config, store, serve command |
| 2. Project Management | 5 | API router, handlers |
| 3. Proxy & Routing | 3 | Subdomain, proxy, multi-project sblite |
| 4. Scale-to-Zero | 4 | Orchestrator interface, process impl, lifecycle |
| 5. Docker | 2 | Docker orchestrator, compose |
| 6. Dashboard | 4 | Handler, pages |
| 7. Kubernetes | 2 | K8s orchestrator, helm |
| 8. Production | 4 | PostgreSQL, multi-hub, rate limiting, logging |

**Total: ~32 tasks across 8 phases**

Each phase builds on the previous and can be deployed independently for testing.
