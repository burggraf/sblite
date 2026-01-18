# Storage Dashboard UI Design

**Date:** 2026-01-18
**Status:** Approved

## Overview

Add a Storage section to the sblite dashboard for end-user file management. Provides a full file manager experience with drag-drop uploads, image previews, and bulk operations.

## Requirements

- Full file manager experience (not just developer debugging)
- Grid + List toggle view
- Image-only previews (jpg, png, gif, webp, svg)
- Drag-drop + click upload with progress indicators
- Bulk delete and download operations
- Full bucket settings (create/delete, public/private, size limits, MIME types)
- Link to existing Policies page for RLS management

## Navigation Structure

New "Storage" nav section added after "Auth" in sidebar:

```
Database
  └─ Tables
Auth
  └─ Users
Storage          ← NEW
  └─ Buckets     ← NEW (default view)
Security
  └─ Policies
...
```

## View Architecture

Two-panel layout:
- **Left panel (30%)**: Bucket list with create button
- **Right panel (70%)**: File browser for selected bucket

Mirrors existing Tables view pattern.

## State Management

New state keys in `app.js`:

```javascript
storage: {
  buckets: [],           // All buckets
  selectedBucket: null,  // Currently selected bucket
  objects: [],           // Files in selected bucket
  currentPath: '',       // Current folder path (e.g., "images/2024/")
  viewMode: 'grid',      // 'grid' or 'list'
  selectedFiles: [],     // Multi-select for bulk operations
  uploading: [],         // Files currently uploading with progress
  loading: false
}
```

## Bucket Management

### Bucket List Panel

Displays all buckets in a vertical list with:
- Bucket name
- Public/private badge
- Object count (if available)
- Click to select and browse files

### Create Bucket Modal

Triggered by "+ New Bucket" button:

```
┌─────────────────────────────────────┐
│ Create Bucket                    ✕  │
├─────────────────────────────────────┤
│ Name: [________________]            │
│                                     │
│ ☐ Public bucket                     │
│   (Files accessible without auth)   │
│                                     │
│ File size limit (optional):         │
│ [________] MB                       │
│                                     │
│ Allowed file types (optional):      │
│ [image/*, application/pdf______]    │
│ (Comma-separated MIME types)        │
│                                     │
│         [Cancel]  [Create Bucket]   │
└─────────────────────────────────────┘
```

### Bucket Settings

Accessible via gear icon on selected bucket. Same fields as create, plus:
- "Empty Bucket" button (delete all files, keep bucket)
- "Delete Bucket" button (must be empty first)
- RLS status indicator with link: "RLS: Enabled → Manage Policies"

## File Browser

### Toolbar

```
[← Back] [/images/2024/] [Upload Files] [⊞ Grid | ☰ List] [⋯ Actions ▼]
```

- **Back**: Navigate to parent folder
- **Breadcrumb path**: Clickable segments
- **Upload Files**: Opens file picker (also accepts drag-drop anywhere)
- **View toggle**: Switch between grid and list
- **Actions menu**: "Delete Selected", "Download Selected" (disabled when nothing selected)

### Grid View

Files displayed as cards in a responsive grid:

```
┌──────────┐  ┌──────────┐  ┌──────────┐
│  [IMG]   │  │  [IMG]   │  │   📁     │
│ thumb    │  │ thumb    │  │          │
├──────────┤  ├──────────┤  ├──────────┤
│ photo.jpg│  │ logo.png │  │ avatars/ │
│ 245 KB   │  │ 12 KB    │  │ 8 items  │
└──────────┘  └──────────┘  └──────────┘
```

- Images show thumbnail preview
- Folders show folder icon
- Non-images show file type icon
- Checkbox overlay in corner for multi-select

### List View

Traditional table layout:

```
☐ | Name          | Size    | Type       | Modified
──┼───────────────┼─────────┼────────────┼──────────
☐ | 📁 avatars/   | -       | folder     | -
☐ | photo.jpg     | 245 KB  | image/jpeg | Jan 18, 2026
☐ | document.pdf  | 1.2 MB  | application| Jan 17, 2026
```

### Folder Navigation

- Double-click folder to enter
- Path stored in `currentPath` state
- API uses prefix parameter for listing

## Upload Experience

### Drag-Drop Zone

Covers entire file browser area:
- On drag enter: Blue dashed border, overlay text "Drop files to upload"
- On drop: Files added to upload queue
- Click button: Opens native file picker (multi-select enabled)

### Upload Progress Panel

Collapsible panel at bottom of file browser:

```
┌─────────────────────────────────────────────────┐
│ Uploading 3 files                          [✕]  │
├─────────────────────────────────────────────────┤
│ photo1.jpg    [████████████░░░░] 75%  245 KB    │
│ photo2.jpg    [████░░░░░░░░░░░░] 25%  1.2 MB    │
│ photo3.jpg    [░░░░░░░░░░░░░░░░] Queued         │
└─────────────────────────────────────────────────┘
```

- Progress tracked via XHR `upload.onprogress` event
- Files uploaded to current path
- On completion: File appears in browser, success toast

## Bulk Operations

### Bulk Delete

1. Select files via checkboxes
2. Click "Actions → Delete Selected"
3. Confirmation modal: "Delete 5 files? This cannot be undone."
4. Uses batch delete API endpoint

### Bulk Download

1. Select files via checkboxes
2. Click "Actions → Download Selected"
3. Files downloaded individually in sequence (no server-side zip)
4. Browser's native download behavior handles save dialog

## Dashboard API Endpoints

New endpoints proxying storage operations through dashboard auth:

```
GET    /_/api/storage/buckets              List all buckets
POST   /_/api/storage/buckets              Create bucket
GET    /_/api/storage/buckets/{id}         Get bucket details
PUT    /_/api/storage/buckets/{id}         Update bucket settings
DELETE /_/api/storage/buckets/{id}         Delete bucket
POST   /_/api/storage/buckets/{id}/empty   Empty bucket

POST   /_/api/storage/objects/list         List objects (bucket + prefix in body)
POST   /_/api/storage/objects/upload       Upload file (multipart form)
GET    /_/api/storage/objects/download     Download file (bucket + path in query)
DELETE /_/api/storage/objects              Delete objects (bucket + paths in body)
```

### Why Proxy Through Dashboard?

- Dashboard uses session auth (cookie), not JWT
- Avoids exposing service_role key to browser
- Consistent with existing dashboard patterns

## Storage API Endpoints Used

```
GET    /storage/v1/bucket                  List buckets
POST   /storage/v1/bucket                  Create bucket
PUT    /storage/v1/bucket/{id}             Update settings
DELETE /storage/v1/bucket/{id}             Delete bucket
POST   /storage/v1/bucket/{id}/empty       Empty bucket

POST   /storage/v1/object/list/{bucket}    List objects
POST   /storage/v1/object/{bucket}/*       Upload file
GET    /storage/v1/object/{bucket}/*       Download file
DELETE /storage/v1/object/{bucket}         Batch delete
DELETE /storage/v1/object/{bucket}/*       Single delete
```

## E2E Tests

New file: `e2e/tests/dashboard/storage.test.ts`

Tests:
- Bucket CRUD operations
- File upload and list
- File download
- Bulk delete
- View toggle persistence
- Drag-drop upload

## Documentation Updates

- Update `CLAUDE.md` with new dashboard endpoints
- Update `e2e/TESTS.md` with new test inventory
