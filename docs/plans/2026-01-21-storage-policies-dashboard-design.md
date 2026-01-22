# Storage Policies Dashboard Design

## Overview

Add a dedicated Storage Policies section within Settings → Storage to manage RLS policies for `storage_objects` table, organized by bucket.

## Architecture

**Location:** Inside the existing Settings → Storage expandable section, as a new "Policies" subsection after backend configuration.

**Data Flow:**
- Frontend fetches policies for `storage_objects` table via existing `/_/api/policies?table_name=storage_objects` endpoint
- Groups policies by bucket using the `bucket_id` condition in their expressions
- Creates/updates/deletes policies via existing `/_/api/policies` endpoints

**State Structure:**
```javascript
storageSettings: {
    // existing fields...
    policies: {
        loading: false,
        list: [],           // all storage_objects policies
        buckets: [],        // list of buckets for grouping
        selectedBucket: null,
        showModal: false,
        editingPolicy: null
    }
}
```

**No new backend endpoints needed** - reuses existing policy and bucket APIs.

## UI Layout

Within the Storage settings section, after backend config:

```
┌─ Storage ──────────────────────────────────────────────────┐
│ ▼ Storage                                                  │
│                                                            │
│   [Backend selector - existing]                            │
│   [Local/S3 settings - existing]                           │
│                                                            │
│   ─────────────────────────────────────────────────────    │
│                                                            │
│   Storage Policies                                         │
│   ┌─────────────────┬──────────────────────────────────┐   │
│   │ Buckets         │ product-images                   │   │
│   │                 │ ┌────────────────────────────┐   │   │
│   │ • product-images│ │ storage_public_read SELECT │   │   │
│   │   (4 policies)  │ │ USING: bucket_id IN (...)  │   │   │
│   │                 │ ├────────────────────────────┤   │   │
│   │ • avatars       │ │ storage_admin_insert INSERT│   │   │
│   │   (2 policies)  │ │ CHECK: bucket_id = '...'   │   │   │
│   │                 │ └────────────────────────────┘   │   │
│   │ + All buckets   │                                  │   │
│   │                 │ [+ New Policy]                   │   │
│   └─────────────────┴──────────────────────────────────┘   │
│                                                            │
│   Helper Functions Reference (collapsible)                 │
│   • storage.filename() - returns object filename           │
│   • storage.foldername() - returns folder path             │
│   • storage.extension() - returns file extension           │
└────────────────────────────────────────────────────────────┘
```

**Bucket list shows:**
- Each bucket from `storage_buckets` table
- Policy count badge
- "All buckets" option to see policies not scoped to a specific bucket

## Policy Templates

When clicking "+ New Policy", show a modal with template selection:

| Template | Command | Generated Expression |
|----------|---------|---------------------|
| Public read | SELECT | `bucket_id = '{bucket}'` |
| Authenticated read | SELECT | `bucket_id = '{bucket}' AND auth.uid() IS NOT NULL` |
| Authenticated upload | INSERT | `bucket_id = '{bucket}' AND auth.uid() IS NOT NULL` |
| Owner only | ALL | `bucket_id = '{bucket}' AND storage.foldername(name) = auth.uid()` |
| Custom policy | (user choice) | (empty, user writes) |

After template selection, show the full policy form with pre-filled expressions (editable).

## Policy Form

Fields:
- **Policy Name** - text input
- **Command** - dropdown: SELECT, INSERT, UPDATE, DELETE, ALL
- **USING Expression** - textarea (for SELECT/UPDATE/DELETE)
- **CHECK Expression** - textarea (for INSERT/UPDATE)

Buttons:
- **Cancel** - close modal
- **Test** - uses existing `/_/api/policies/test` endpoint
- **Create Policy** / **Save Changes** - submit

## Helper Functions Reference

Collapsible section shown below the policy form or in sidebar:

| Function | Description | Example |
|----------|-------------|---------|
| `storage.filename(name)` | Returns filename without path | `'uploads/photo.jpg'` → `'photo.jpg'` |
| `storage.foldername(name)` | Returns folder path | `'user123/photos/img.png'` → `'user123/photos'` |
| `storage.extension(name)` | Returns file extension | `'document.pdf'` → `'pdf'` |
| `auth.uid()` | Current user's ID or NULL | - |

## Files to Modify

1. **internal/dashboard/static/app.js**
   - Add `policies` sub-state to `storageSettings`
   - Add `loadStoragePolicies()`, `selectStorageBucket()`, `showStoragePolicyModal()`, etc.
   - Add `renderStoragePoliciesSection()` within `renderStorageSettingsSection()`
   - Add `renderStoragePolicyModal()` for create/edit form

2. **internal/dashboard/static/style.css**
   - Styles for storage policies layout (bucket list + policy list)
   - Styles for policy template selector
   - Styles for helper functions reference

## Design Decisions

- **Per-bucket organization** - More intuitive than flat list of all storage policies
- **Reuse existing APIs** - No backend changes needed
- **Policy templates** - Speed up common access patterns
- **SQL command names** - Keep SELECT/INSERT/UPDATE/DELETE for consistency with main policy editor
- **Helper function reference** - Help users write correct expressions
