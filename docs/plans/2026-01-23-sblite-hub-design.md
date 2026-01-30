# sblite-hub Design Document

**Date**: 2026-01-23
**Status**: Draft
**Author**: Claude (with Mark)

## Overview

**sblite-hub** is a control plane for managing multiple sblite instances across organizations and projects. It enables multi-tenancy at the org level, with the ability to promote high-traffic projects to dedicated instances.

### Goals

- Enable thousands of sblite projects on shared infrastructure
- Support organizations with multiple projects and team members
- Implement scale-to-zero for cost efficiency
- Provide seamless promotion from shared to dedicated instances
- Support self-hosted, managed SaaS, and simple local deployments

### Deployment Tiers

| Tier | Binary | Use Case |
|------|--------|----------|
| **Simple** | `sblite` only | Local dev, single project, no orchestration |
| **Self-hosted** | `sblite` + `sblite-hub` | VPS/Docker/k8s, multi-org, scale-to-zero |
| **Managed SaaS** | `sblite-hub` (hosted) | Fully managed, users just create projects |

## Architecture

### System Overview

```
┌─────────────────────────────────────────────────────────┐
│                     sblite-hub                          │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │  Dashboard  │  │  Proxy/Router│  │  Orchestrator │  │
│  │  (Web UI)   │  │  (HTTP)      │  │  (Docker/k8s) │  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
│                          │                              │
│  ┌───────────────────────▼────────────────────────┐    │
│  │  Internal sblite instance (dogfooding)          │    │
│  │  - users, orgs, org_members, projects           │    │
│  │  - instances, usage_metrics                     │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
                          │
          ┌───────────────┼───────────────┐
          ▼               ▼               ▼
   ┌────────────┐  ┌────────────┐  ┌────────────┐
   │  sblite    │  │  sblite    │  │  sblite    │
   │  Org A     │  │  Org B     │  │  Dedicated │
   │  (shared)  │  │  (shared)  │  │  Project X │
   └────────────┘  └────────────┘  └────────────┘
```

### Key Design Decisions

1. **Separate binary**: sblite-hub is its own Go binary, keeping sblite focused on being a Supabase-lite runtime
2. **Dogfooding**: Hub uses its own internal sblite instance for user/org/project data
3. **Full proxy**: All traffic flows through the hub - simpler client config, enables transparent scale-to-zero
4. **Subdomain routing**: Projects identified by subdomain (`myproject.hub.example.com`)
5. **HTTP-only IPC**: Hub communicates with instances via existing HTTP APIs
6. **Per-project roles**: Permissions assigned at project level, not org level

### Request Flow

```
1. Request: GET https://myproject.hub.example.com/rest/v1/todos
                      └────────┘
                       subdomain = project slug

2. Proxy extracts subdomain, looks up project:
   SELECT p.id, p.database_path, i.endpoint, i.status
   FROM projects p
   LEFT JOIN instances i ON (
     (i.project_id = p.id) OR
     (i.org_id = p.org_id AND p.is_dedicated = FALSE)
   )
   WHERE p.slug = 'myproject'

3. If instance.status = 'stopped':
   - Queue request
   - Trigger instance start
   - Wait for health check (with 30s timeout)
   - If timeout: return 503 Service Unavailable

4. If instance.status = 'running':
   - Proxy request to instance.endpoint
   - Add X-Database-Path header for multi-project routing
   - Update last_activity_at
   - Return response to client
```

## Data Model

The internal sblite instance stores all control plane data.

### Schema

```sql
-- Control plane users (distinct from project users)
CREATE TABLE users (
  id UUID PRIMARY KEY,
  email TEXT UNIQUE NOT NULL,
  name TEXT,
  avatar_url TEXT,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Organizations
CREATE TABLE orgs (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,  -- URL-safe identifier
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  owner_id UUID REFERENCES users(id)  -- original creator
);

-- Many-to-many: users belong to orgs
CREATE TABLE org_members (
  org_id UUID REFERENCES orgs(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (org_id, user_id)
);

-- Projects within orgs
CREATE TABLE projects (
  id UUID PRIMARY KEY,
  org_id UUID REFERENCES orgs(id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  slug TEXT NOT NULL,            -- subdomain: slug.hub.example.com
  database_path TEXT NOT NULL,   -- /data/orgs/{org_id}/projects/{id}/data.db
  is_dedicated BOOLEAN DEFAULT FALSE,
  keep_alive BOOLEAN DEFAULT FALSE,
  idle_timeout_minutes INTEGER DEFAULT 15,
  region TEXT DEFAULT 'default',
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(org_id, slug)
);

-- Project-level roles: owner/admin/developer/viewer
CREATE TABLE project_members (
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  user_id UUID REFERENCES users(id) ON DELETE CASCADE,
  role TEXT CHECK(role IN ('owner', 'admin', 'developer', 'viewer')),
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (project_id, user_id)
);

-- sblite instances (processes/containers)
CREATE TABLE instances (
  id UUID PRIMARY KEY,
  org_id UUID REFERENCES orgs(id),      -- NULL for dedicated instances
  project_id UUID REFERENCES projects(id), -- NULL for shared instances
  hub_id TEXT,                           -- which hub manages this instance
  endpoint TEXT NOT NULL,                -- internal URL: http://10.0.0.5:8080
  status TEXT CHECK(status IN ('running', 'starting', 'stopped', 'failed')),
  started_at TIMESTAMPTZ,
  last_activity_at TIMESTAMPTZ,
  orchestrator_ref TEXT,                 -- docker container ID, k8s pod name, etc.
  region TEXT,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Usage tracking
CREATE TABLE usage_metrics (
  id UUID PRIMARY KEY,
  project_id UUID REFERENCES projects(id) ON DELETE CASCADE,
  period_start TIMESTAMPTZ NOT NULL,    -- hourly buckets
  api_calls INTEGER DEFAULT 0,
  storage_bytes BIGINT DEFAULT 0,
  bandwidth_bytes BIGINT DEFAULT 0,
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(project_id, period_start)
);

-- Multi-hub support (Phase 8+)
CREATE TABLE hubs (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  endpoint TEXT NOT NULL,       -- https://hub-west.example.com
  region TEXT NOT NULL,
  status TEXT DEFAULT 'active',
  capacity INTEGER DEFAULT 1000, -- max instances
  created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);
```

### Permission Model

| Role | View Project | Edit Data | Manage Schema | Manage Members | Delete Project |
|------|--------------|-----------|---------------|----------------|----------------|
| viewer | ✓ | - | - | - | - |
| developer | ✓ | ✓ | ✓ | - | - |
| admin | ✓ | ✓ | ✓ | ✓ | - |
| owner | ✓ | ✓ | ✓ | ✓ | ✓ |

## Instance Lifecycle

### Instance Types

| Type | Serves | Scale-to-Zero | Use Case |
|------|--------|---------------|----------|
| **Shared** | All projects in an org | Yes (configurable timeout) | Default for most projects |
| **Dedicated** | Single project | Yes (unless keep-alive) | High-traffic or isolated projects |

### Lifecycle States

```
                    ┌─────────┐
         ┌─────────►│ stopped │◄───────────┐
         │          └────┬────┘            │
         │               │ request arrives │ idle timeout
         │               ▼                 │
         │          ┌─────────┐            │
         │          │starting │            │
         │          └────┬────┘            │
         │               │ health check OK │
         │               ▼                 │
         │          ┌─────────┐            │
         └──────────│ running │────────────┘
         crash/fail └─────────┘
```

### Scale-to-Zero Flow

1. **Activity tracking**: Proxy updates `instances.last_activity_at` on each request
2. **Idle check**: Background job runs every minute, finds instances where `now() - last_activity_at > idle_timeout`
3. **Shutdown**:
   - If keep-alive projects exist in instance → skip
   - Otherwise: graceful shutdown via orchestrator
   - Update status to `stopped`
4. **Wake-up**: On next request to stopped instance:
   - Proxy holds request (with timeout)
   - Orchestrator starts instance
   - Health check loop
   - Proxy releases held requests

### Orchestrator Interface

```go
type Orchestrator interface {
    // Start an instance, return internal endpoint
    Start(ctx context.Context, config InstanceConfig) (endpoint string, ref string, error)

    // Stop an instance gracefully
    Stop(ctx context.Context, ref string) error

    // Kill an instance immediately
    Kill(ctx context.Context, ref string) error

    // Check if instance is healthy
    Health(ctx context.Context, ref string) (bool, error)

    // Get resource usage (CPU, memory)
    Stats(ctx context.Context, ref string) (*ResourceStats, error)
}
```

**Implementations:**
- `ProcessOrchestrator` - spawns `sblite serve` as child processes (dev/single-machine)
- `DockerOrchestrator` - manages containers via Docker API
- `KubernetesOrchestrator` - creates pods via k8s API

## Proxy & Routing

### Multi-Project Mode for sblite

sblite needs a new flag to accept database path per-request:

```bash
sblite serve --multi-project --data-dir /data/orgs/abc/projects/
```

In multi-project mode:
- `X-Database-Path` header specifies which database file to use
- Connections are pooled per database file
- Lazy-loaded: database only opened on first request
- Idle databases closed after inactivity

### Proxy Implementation

```go
type Proxy struct {
    store     *Store           // control plane database
    orch      Orchestrator     // instance management
    transport *http.Transport  // connection pooling
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract project from subdomain
    slug := extractSubdomain(r.Host)

    project, instance, err := p.store.GetProjectInstance(r.Context(), slug)
    if err != nil {
        http.Error(w, "Project not found", 404)
        return
    }

    // Wake instance if needed
    if instance.Status == "stopped" {
        if err := p.wakeInstance(r.Context(), instance); err != nil {
            http.Error(w, "Service starting, retry in a moment", 503)
            return
        }
    }

    // Add project context header
    r.Header.Set("X-Database-Path", project.DatabasePath)

    // Proxy to instance
    proxy := httputil.NewSingleHostReverseProxy(instance.Endpoint)
    proxy.Transport = p.transport
    proxy.ServeHTTP(w, r)

    // Update activity (async)
    go p.store.UpdateActivity(instance.ID)
}
```

## Observability & Metrics

sblite-hub leverages OpenTelemetry for comprehensive observability, enabling rate limiting, resource quotas, and operational monitoring across all tenant instances.

### OTel Integration

Each sblite instance runs with OpenTelemetry enabled, sending metrics and traces to the hub's central collector:

```bash
sblite serve \
  --otel-exporter otlp \
  --otel-endpoint hub-collector:4317 \
  --otel-service-name "sblite-{org}-{project}" \
  --otel-sample-rate 0.1
```

### Available Metrics

#### Instance Metrics (from sblite)

| Metric | Type | Attributes | Description |
|--------|------|------------|-------------|
| `http.server.request_count` | Counter | `tenant.id`, `project.id`, `http.method`, `http.status_code` | HTTP request count per tenant |
| `http.server.request_duration` | Histogram | `tenant.id`, `project.id` | Request latency (p50, p95, p99) |
| `http.server.response_size` | Histogram | `tenant.id`, `project.id` | Response body size |
| `db.connection.count` | Gauge | `tenant.id`, `project.id`, `db.name` | Active database connections |
| `db.query.duration` | Histogram | `tenant.id`, `project.id`, `db.table` | Query execution time |

#### Hub Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `hub.instance.count` | Gauge | Number of running instances per org |
| `hub.instance.wakeups` | Counter | Number of scale-up events |
| `hub.proxy.request_count` | Counter | Total proxied requests |
| `hub.proxy.queue_duration` | Histogram | Time spent waiting for instance wake-up |

### Multi-Tenant Attribute Enrichment

The hub automatically adds tenant context to all metrics from downstream sblite instances:

```go
// Hub proxy enriches OTel context
func (p *Proxy) enrichSpan(ctx context.Context, project *Project) {
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.String("tenant.id", project.OrgID),
        attribute.String("tenant.slug", project.OrgSlug),
        attribute.String("project.id", project.ID),
        attribute.String("project.slug", project.Slug),
        attribute.String("project.tier", project.Tier), // "shared" or "dedicated"
    )
}
```

### Rate Limiting & Quotas

Metrics drive per-tenant rate limiting:

```go
type RateLimiter struct {
    meter    metric.Meter
    requests metric.Int64Counter

    // Per-tenant quotas
    quotas   map[string]*TenantQuota  // tenantID -> quota
}

type TenantQuota struct {
    OrgID        string
    ProjectID    string

    // Rate limits
    RequestsPerSecond int
    Concurrency      int

    // Resource quotas
    MaxCPU          float64
    MaxMemory       int64
    MaxConnections  int
}

func (rl *RateLimiter) Check(ctx context.Context, tenantID string) (bool, error) {
    quota := rl.quotas[tenantID]

    // Read current usage from OTel metrics
    usage := rl.getUsage(ctx, tenantID)

    // Check against quota
    if usage.RequestsPerSecond >= quota.RequestsPerSecond {
        return false, nil  // Rate limit exceeded
    }

    return true, nil
}
```

### Quota Enforcement Points

| Enforcement Point | Metric Used | Action |
|-------------------|-------------|--------|
| **Proxy** | `http.server.request_count` | Reject requests over quota |
| **Orchestrator** | Resource usage | Block new instances if org at capacity |
| **sblite instance** | `db.connection.count` | Reject new DB connections over limit |

### Dashboard Metrics Display

The hub dashboard shows per-project metrics:

```
┌─────────────────────────────────────────────────────────────┐
│ Project: myapp                    Org: acme-corp             │
├─────────────────────────────────────────────────────────────┤
│ Rate Limits (Last 24h)                                     │
│ ├─ Requests:     1.2M / 10M (12%)                         │
│ ├─ Concurrency: 45 / 100                                   │
│ ├─ CPU:           0.8 / 2.0 cores                          │
│ └─ Memory:        512 MB / 4 GB                             │
│                                                             │
│ Current Usage (Real-time)                                   │
│ ├─ Requests/sec: 42 (p95: 87ms)                           │
│ ├─ Active connections: 12                                  │
│ └─ Instance: running (uptime: 2h 34m)                     │
└─────────────────────────────────────────────────────────────┘
```

### Alerting

Alerts fire on quota thresholds:

```yaml
# Alert rules for hub
alerts:
  - name: HighRequestRate
    condition: rate(http_server_request_count[5m]) > quota * 0.9
    action: Notify org admin, consider scaling

  - name: InstanceMemoryLimit
    condition: process_memory_usage > max_memory * 0.95
    action: Block new connections, alert ops team

  - name: TooManyWakeups
    condition: rate(hub_instance_wakeups[1h]) > 10
    action: Increase idle timeout, suggest dedicated instance
```

### Trace Correlation

Traces flow from sblite → hub → central collector, maintaining context:

```
sblite instance (myapp):
  └─ GET /rest/v1/posts [tenant.id=acme, project.id=myapp]
      └─ db.query: SELECT * FROM posts [2ms]

Hub proxy:
  └─ Proxy request to myapp [tenant.id=acme, project.id=myapp]
      └─ (propagated) GET /rest/v1/posts
          └─ (propagated) db.query: SELECT * FROM posts
```

This enables:
- **End-to-end tracing**: See full request path from client → hub → sblite → database
- **Performance debugging**: Identify bottlenecks at any layer
- **Cost allocation**: Attribute resource usage to specific tenants

### Implementation Timeline

| Phase | OTel Features | Status |
|-------|--------------|--------|
| **sblite v0.5** | HTTP metrics, traces, stdout/OTLP exporters | ✅ Planned (this doc) |
| **sblite-hub Phase 3** | Proxy span enrichment, metrics aggregation | Planned |
| **sblite-hub Phase 4** | Rate limiting based on metrics | Planned |
| **sblite-hub Phase 8** | Per-tenant quotas, billing integration | Planned |

### See Also

- [OpenTelemetry Implementation Plan](2026-01-30-opentelemetry-implementation.md)
- [Observability Documentation](../observability.md)

## Dashboard UI

### sblite-hub Dashboard

Separate from project dashboards, focused on org/project management.

**URL**: `https://hub.example.com/_/`

### Pages

**1. Login/Signup**
- Email/password, magic link, OAuth (Google/GitHub)
- Uses dogfooded sblite auth

**2. Org Selector** (after login)
- List orgs user belongs to
- "Create Organization" button
- Each org shows project count, member count

**3. Org Dashboard** (after selecting org)
```
┌─────────────────────────────────────────────────────────┐
│  Acme Corp                               [Settings] [+] │
├─────────────────────────────────────────────────────────┤
│  Projects                                               │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐    │
│  │ 🟢 myapp     │ │ 🔴 staging  │ │ ⚪ internal  │    │
│  │ 1.2k calls/h │ │ stopped     │ │ idle         │    │
│  │ [Dashboard]  │ │ [Dashboard] │ │ [Dashboard]  │    │
│  └──────────────┘ └──────────────┘ └──────────────┘    │
│                                                         │
│  Members (3)                                            │
│  ┌─────────────────────────────────────────────────┐   │
│  │ alice@acme.com (you)     │ -                    │   │
│  │ bob@acme.com             │ 2 projects           │   │
│  │ carol@acme.com           │ 1 project            │   │
│  └─────────────────────────────────────────────────┘   │
│  [+ Invite Member]                                      │
│                                                         │
│  Usage This Month                                       │
│  API Calls: 45,231  │  Storage: 128 MB  │  BW: 2.1 GB  │
└─────────────────────────────────────────────────────────┘
```

**4. Project Settings**
- General: name, slug, keep-alive toggle
- Members: add/remove, role assignment
- Danger zone: promote to dedicated, transfer to another org, delete

**5. Org Settings**
- General: name, slug
- Members: invite, remove
- Billing: usage details (future: payment methods)

### Project Dashboard Access

From hub, clicking "Dashboard" on a project opens the project's own sblite dashboard at `https://myproject.hub.example.com/_/`. Hub generates a short-lived dashboard token for SSO.

## Project Operations

### Creating a Project

1. User selects org, clicks "New Project"
2. Enters project name → auto-generates slug
3. Hub creates project record in database
4. Hub creates database directory: `/data/orgs/{org_id}/projects/{project_id}/`
5. Hub runs `sblite init` to create `data.db` in that directory
6. Project is now available (will start instance on first request)

### Promoting to Dedicated Instance

When project owner clicks "Promote to Dedicated":

1. Hub marks project as `is_dedicated = TRUE`
2. If org instance is running:
   - Wait for requests to that project to drain (30s)
   - Instance stops serving that project
3. Next request to project triggers dedicated instance start
4. Project now has its own sblite process

### Transferring Project Between Orgs

1. User must be owner of project AND member of target org
2. Hub workflow:
   - Stop any instance serving the project
   - Move database files
   - Update project.org_id
   - Clear project_members
   - Add transferring user as owner
3. Brief downtime during file move

### Deleting a Project

1. Confirmation required (type project name)
2. Stop any dedicated instance
3. Delete project record (cascades to project_members)
4. Delete database files

## Hub API

### Base URL

`https://hub.example.com/api/v1/`

### Authentication

JWT from the dogfooded sblite instance:
```
Authorization: Bearer <hub-jwt>
```

### Endpoints

**Organizations**
```
GET    /api/v1/orgs                    # List user's orgs
POST   /api/v1/orgs                    # Create org
GET    /api/v1/orgs/:id                # Get org details
PATCH  /api/v1/orgs/:id                # Update org
DELETE /api/v1/orgs/:id                # Delete org (must be empty)

GET    /api/v1/orgs/:id/members        # List org members
POST   /api/v1/orgs/:id/members        # Invite member
DELETE /api/v1/orgs/:id/members/:uid   # Remove member
```

**Projects**
```
GET    /api/v1/orgs/:oid/projects              # List org's projects
POST   /api/v1/orgs/:oid/projects              # Create project
GET    /api/v1/orgs/:oid/projects/:pid         # Get project details
PATCH  /api/v1/orgs/:oid/projects/:pid         # Update project
DELETE /api/v1/orgs/:oid/projects/:pid         # Delete project

POST   /api/v1/orgs/:oid/projects/:pid/promote # Promote to dedicated
POST   /api/v1/orgs/:oid/projects/:pid/demote  # Demote to shared

GET    /api/v1/orgs/:oid/projects/:pid/members # List project members
POST   /api/v1/orgs/:oid/projects/:pid/members # Add member with role
PATCH  /api/v1/orgs/:oid/projects/:pid/members/:uid  # Update role
DELETE /api/v1/orgs/:oid/projects/:pid/members/:uid  # Remove member
```

**Instances (admin)**
```
GET    /api/v1/instances               # List all instances
GET    /api/v1/instances/:id           # Get instance details
POST   /api/v1/instances/:id/restart   # Restart instance
POST   /api/v1/instances/:id/stop      # Force stop instance
```

**Usage**
```
GET    /api/v1/orgs/:id/usage          # Org usage summary
GET    /api/v1/projects/:id/usage      # Project usage details
```

**User**
```
GET    /api/v1/user                    # Current user profile
PATCH  /api/v1/user                    # Update profile
```

## Multi-Hub Federation

For scaling beyond a single VPS.

### Architecture

**Phase 1 (Shared Database):**
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Hub #1    │     │   Hub #2    │     │   Hub #3    │
│  VPS West   │     │  VPS East   │     │  VPS EU     │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       └───────────────────┼───────────────────┘
                           │
                    ┌──────▼──────┐
                    │  PostgreSQL │
                    │  (shared)   │
                    └─────────────┘
```

- Hubs share a PostgreSQL database
- Any hub can serve any request
- Load balancer routes to nearest/healthiest hub
- Instances table includes `hub_id` to track which hub manages which instance

**Phase 2 (Regional Hubs):**
```
┌─────────────────────────────────────────────────────────┐
│                    Global Coordinator                    │
│          (routes to correct regional hub)               │
└───────────────────────┬─────────────────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼               ▼               ▼
┌───────────────┐ ┌───────────────┐ ┌───────────────┐
│   Hub West    │ │   Hub East    │ │   Hub EU      │
│ + own sblite  │ │ + own sblite  │ │ + own sblite  │
│ + own DB      │ │ + own DB      │ │ + own DB      │
└───────────────┘ └───────────────┘ └───────────────┘
```

- Each regional hub is independent
- Global coordinator routes based on project→region mapping
- User accounts sync across regions

### Instance Affinity

When starting an instance, hub prefers:
1. Same hub that received the request
2. Hub with lowest load in same region
3. Any hub with capacity

## Implementation Phases

### Phase 1: Foundation
- Create `sblite-hub` repository/directory structure
- Set up internal sblite instance (dogfooding)
- Create schema migrations for users, orgs, projects tables
- Basic CLI: `sblite-hub serve`
- Authentication (email/password via internal sblite)

### Phase 2: Project Management
- Hub API: org and project CRUD
- Hub dashboard: org selector, project list
- Project creation (init database directory)
- `ProcessOrchestrator` (child process management)

### Phase 3: Proxy & Routing
- Subdomain extraction
- Request proxying to instances
- Multi-project mode for sblite (`--multi-project`, `X-Database-Path`)
- Activity tracking

### Phase 4: Scale-to-Zero
- Idle detection background job
- Instance shutdown on idle
- Request queuing during wake-up
- Keep-alive flag support

### Phase 5: Docker Orchestrator
- `DockerOrchestrator` implementation
- Docker Compose example deployment
- Health checking
- Resource stats collection

### Phase 6: Hub Dashboard Polish
- Member invitation flow
- Project settings (promote/demote, transfer)
- Usage metrics display
- Project dashboard SSO (token generation)

### Phase 7: Kubernetes Orchestrator
- `KubernetesOrchestrator` implementation
- Helm chart
- Pod lifecycle management

### Phase 8: Multi-Hub & Production
- Migrate control plane to PostgreSQL
- Multi-hub support with shared database
- Rate limiting
- Request logging
- Error handling & retries
- Backup/restore workflows

## Open Questions

1. **Wildcard DNS/SSL**: Self-hosted users need wildcard DNS and certificates for subdomain routing. Document setup for Let's Encrypt wildcard certs.

2. **Project slug uniqueness**: Should slugs be globally unique or per-org unique? Currently designed as per-org unique with subdomain format `{project}.{org}.hub.example.com` or just `{project}.hub.example.com` with global uniqueness.

3. **Instance memory limits**: How to prevent a single project from consuming all memory in a shared instance? Go doesn't have great memory isolation.

4. **Database connection limits**: In multi-project mode, how many SQLite connections should be kept open? Need to balance memory vs connection overhead.

5. **Backup strategy**: How to back up thousands of small SQLite databases efficiently?
