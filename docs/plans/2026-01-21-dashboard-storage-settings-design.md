# Dashboard Storage Settings Design

Add S3 configuration to the dashboard settings so users can configure storage backend credentials through the UI.

## Requirements

- Hot-reload: Changes take effect immediately without server restart
- Allow switching between local and S3 backends at runtime
- New "Storage" tab in Settings UI
- "Test Connection" button to validate S3 credentials before saving

## Data Model

Storage configuration stored in `_dashboard` key-value table (same pattern as OAuth settings):

| Key | Description |
|-----|-------------|
| `storage_backend` | "local" or "s3" |
| `storage_local_path` | Path for local storage (default: "./storage") |
| `storage_s3_endpoint` | S3 endpoint URL |
| `storage_s3_region` | AWS region |
| `storage_s3_bucket` | Bucket name |
| `storage_s3_access_key` | Access key ID |
| `storage_s3_secret_key` | Secret key (masked in API responses) |
| `storage_s3_path_style` | "true" or "false" for path-style addressing |

**Priority order** (highest to lowest):
1. Dashboard-configured values (from `_dashboard` table)
2. CLI flags
3. Environment variables
4. Defaults

## API Endpoints

### GET `/_/api/settings/storage`

Returns current storage configuration:

```json
{
  "backend": "local",
  "local_path": "./storage",
  "s3": {
    "endpoint": "https://s3.amazonaws.com",
    "region": "us-east-1",
    "bucket": "my-bucket",
    "access_key": "AKIA...",
    "secret_key": "********",
    "path_style": false
  },
  "active": "local"
}
```

### PATCH `/_/api/settings/storage`

Updates storage configuration. Accepts partial updates:

```json
{
  "backend": "s3",
  "s3": {
    "endpoint": "...",
    "bucket": "...",
    "access_key": "...",
    "secret_key": "..."
  }
}
```

On success, hot-reloads the storage backend.

### POST `/_/api/settings/storage/test`

Tests S3 connection without saving:

```json
{
  "endpoint": "...",
  "region": "...",
  "bucket": "...",
  "access_key": "...",
  "secret_key": "..."
}
```

Returns `{"success": true}` or `{"success": false, "error": "..."}`.

## Hot-Reload Mechanism

Following the pattern used for OAuth settings:

1. Add `onStorageReload func(*storage.Config)` callback to `Handler`
2. Add `SetStorageReloadFunc(f func(*storage.Config))` method
3. In `cmd/serve.go`, register callback that:
   - Creates new `storage.Service` with updated config
   - Swaps it into the server
   - Closes old backend gracefully
4. Add `Reconfigure(cfg Config) error` method to `storage.Service`:
   - Creates new backend
   - Tests it works (HeadBucket for S3)
   - Atomically swaps backend
   - Closes old backend

If new backend fails to initialize, old one stays active and error is returned.

## UI Design

New "Storage" tab in Settings:

- Radio buttons: "Local filesystem" / "S3-compatible"
- Local Settings section: Storage Path field
- S3 Settings section: Endpoint, Region, Bucket, Access Key, Secret Key, Path-style checkbox
- "Test Connection" button for S3 validation
- Warning banner when switching backend types (files don't migrate)
- Cancel / Save Changes buttons

**Behavior:**
- S3 fields grayed out when "Local" selected (and vice versa)
- Test Connection shows success/error inline
- Warning appears only when changing backend type, not just updating credentials

## Implementation Files

- `internal/dashboard/storage_settings.go` - API handlers
- `internal/dashboard/handler.go` - Route registration, callback fields
- `internal/dashboard/static/app.js` - UI components
- `internal/storage/storage.go` - Add Reconfigure method
- `cmd/serve.go` - Register reload callback, update config loading priority
