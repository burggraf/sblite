# Storage RLS Policies Design

## Overview

Implement Supabase-compatible Row Level Security (RLS) policies for the storage system. Policies apply to the `storage_objects` table to control file access based on user identity, file metadata, and custom expressions.

## Architecture

### RLS Context Flow

```
Storage Request → Extract Auth Context → Check RLS Enabled → Apply Policies → Execute Operation
                  (JWT/API Key)         (storage_objects)    (using/check)
```

### Key Components

1. **Storage Helper Functions** - SQL functions for policy expressions
2. **Extended RLS Enforcer** - Apply policies to storage operations
3. **Storage Handler Integration** - Pass auth context through operations
4. **Default RLS State** - Enabled by default (matches Supabase)

## Storage Helper Functions

Add to `internal/rls/rewriter.go`:

| Function | Description | Example |
|----------|-------------|---------|
| `storage.filename(name)` | Extract filename from path | `'folder/image.png'` → `'image.png'` |
| `storage.foldername(name)` | Extract folder path | `'folder/sub/file.txt'` → `'folder/sub'` |
| `storage.extension(name)` | Extract file extension | `'image.png'` → `'png'` |

These functions are substituted during policy evaluation, similar to `auth.uid()`.

## Storage Operation Mapping

| Storage Operation | RLS Command | Relevant Columns |
|-------------------|-------------|------------------|
| Upload (POST/PUT) | INSERT | bucket_id, name, owner_id, mime_type |
| Download (GET) | SELECT | bucket_id, name, owner_id |
| Delete (DELETE) | DELETE | bucket_id, name, owner_id |
| List (POST /list) | SELECT | bucket_id, name, owner_id |
| Copy | SELECT (source) + INSERT (dest) | bucket_id, name, owner_id |
| Move | SELECT + DELETE (source) + INSERT (dest) | bucket_id, name, owner_id |

## Default Behavior

- **RLS Enabled by Default** on `storage_objects` table (matches Supabase)
- **No Default Policies** - must create policies to allow access
- **Service Role Bypasses RLS** - administrative access always works
- **Public Bucket** - only affects `/object/public/*` endpoint (no auth required)

## Common Policy Patterns

### Owner-based access (users can only access their own files):
```sql
-- SELECT policy
CREATE POLICY "Users can view own files" ON storage_objects
FOR SELECT USING (owner_id = auth.uid());

-- INSERT policy
CREATE POLICY "Users can upload own files" ON storage_objects
FOR INSERT WITH CHECK (owner_id = auth.uid());

-- DELETE policy
CREATE POLICY "Users can delete own files" ON storage_objects
FOR DELETE USING (owner_id = auth.uid());
```

### Bucket-specific access:
```sql
CREATE POLICY "Allow public bucket access" ON storage_objects
FOR SELECT USING (bucket_id = 'public-assets');
```

### File type restrictions:
```sql
CREATE POLICY "Only images allowed" ON storage_objects
FOR INSERT WITH CHECK (storage.extension(name) IN ('png', 'jpg', 'gif'));
```

### Folder-based access:
```sql
CREATE POLICY "Users can access their folder" ON storage_objects
FOR SELECT USING (storage.foldername(name) = auth.uid());
```

## Error Handling

| Scenario | HTTP Status | Error Code | Message |
|----------|-------------|------------|---------|
| RLS denies access | 403 | `access_denied` | "Access denied by RLS policy" |
| No matching policy | 403 | `access_denied` | "Access denied by RLS policy" |
| Object not found (after RLS) | 404 | `not_found` | "Object not found" |

Note: 404 returned for both "doesn't exist" and "exists but RLS denies" to prevent information leakage.

## Implementation Tasks

1. Add storage helper function substitution to `internal/rls/rewriter.go`
2. Enable RLS by default on `storage_objects` in migrations
3. Integrate RLS checks into storage handler operations
4. Add E2E tests for storage RLS policies
5. Update documentation

## Files to Modify

- `internal/rls/rewriter.go` - Add storage.* function substitution
- `internal/storage/handler.go` - Integrate RLS checks
- `internal/db/migrations.go` - Enable RLS on storage_objects by default
- `e2e/tests/storage/rls.test.ts` - Add RLS tests
- `docs/STORAGE.md` - Document RLS policies
