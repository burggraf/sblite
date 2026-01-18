# Signed URLs Implementation Design

## Overview

Implement Supabase-compatible signed URLs for sblite storage. Signed URLs allow time-limited access to files without requiring authentication, enabling secure file sharing and client-side uploads.

## API Endpoints

### Download Signed URLs

#### Create Single Signed URL
```
POST /storage/v1/object/sign/{bucket}/{*path}
```

**Request:**
```json
{
  "expiresIn": 3600
}
```

**Response:**
```json
{
  "signedURL": "/storage/v1/object/sign/avatars/folder/cat.png?token=eyJ..."
}
```

#### Create Multiple Signed URLs
```
POST /storage/v1/object/sign/{bucket}
```

**Request:**
```json
{
  "expiresIn": 3600,
  "paths": ["folder/file1.png", "folder/file2.png"]
}
```

**Response:**
```json
[
  {
    "path": "folder/file1.png",
    "signedURL": "/storage/v1/object/sign/avatars/folder/file1.png?token=eyJ...",
    "error": null
  },
  {
    "path": "folder/file2.png",
    "signedURL": "/storage/v1/object/sign/avatars/folder/file2.png?token=eyJ...",
    "error": null
  }
]
```

#### Download via Signed URL
```
GET /storage/v1/object/sign/{bucket}/{*path}?token={jwt}
```

**Query Parameters:**
- `token` (required): JWT token from createSignedUrl
- `download` (optional): Filename for Content-Disposition header

**Response:** File content with appropriate headers

### Upload Signed URLs

#### Create Signed Upload URL
```
POST /storage/v1/object/upload/sign/{bucket}/{*path}
```

**Headers:**
- `x-upsert: true` (optional): Allow overwriting existing files

**Response:**
```json
{
  "signedUrl": "/storage/v1/object/upload/sign/avatars/folder/cat.png?token=eyJ...",
  "token": "eyJ...",
  "path": "folder/cat.png"
}
```

#### Upload via Signed URL
```
PUT /storage/v1/object/upload/sign/{bucket}/{*path}?token={jwt}
```

**Headers:**
- `x-upsert: true` (optional)
- `content-type`: File MIME type
- `cache-control` (optional)

**Response:**
```json
{
  "Key": "avatars/folder/cat.png",
  "path": "folder/cat.png"
}
```

## Token Format

Signed URL tokens are JWTs signed with the same `SBLITE_JWT_SECRET` used for auth tokens.

### Download Token Claims
```json
{
  "url": "bucket/path/to/file.png",
  "iat": 1617726273,
  "exp": 1617729873,
  "type": "storage-download"
}
```

### Upload Token Claims
```json
{
  "url": "bucket/path/to/file.png",
  "iat": 1617726273,
  "exp": 1617733473,
  "type": "storage-upload",
  "owner_id": "user-uuid",
  "upsert": false
}
```

**Notes:**
- Upload tokens have fixed 2-hour expiry (7200 seconds)
- Download tokens use user-specified `expiresIn`
- `type` claim distinguishes storage tokens from auth tokens
- `owner_id` in upload tokens sets file ownership

## Security Model

### RLS Enforcement

**Critical:** RLS is checked at **signed URL creation time**, not at access time.

1. When `createSignedUrl` is called:
   - Validate user authentication
   - Check RLS SELECT policy for the object
   - If allowed, generate signed token

2. When `createSignedUploadUrl` is called:
   - Validate user authentication
   - Check RLS INSERT policy for the path
   - If allowed, generate signed token with owner_id

3. When accessing signed URL:
   - Validate JWT signature and expiry
   - **Bypass RLS** - token already authorized
   - Serve/accept file directly

### Token Validation

1. Verify JWT signature using `jwtSecret`
2. Check `exp` claim for expiry
3. Verify `type` claim matches operation (download vs upload)
4. Extract `url` claim and verify it matches request path

## Implementation Plan

### Phase 1: Download Signed URLs

1. **Add token generation** in `internal/storage/signed.go`:
   - `GenerateDownloadToken(bucket, path, expiresIn, secret) string`
   - `ValidateDownloadToken(token, secret) (*DownloadClaims, error)`

2. **Update handler.go**:
   - Modify `CreateSignedURL` to generate tokens
   - Add `CreateSignedURLs` for batch operation
   - Add `GetSignedObject` handler for token-based downloads

3. **Update routes** in `RegisterRoutes()`:
   - `POST /object/sign/{bucket}/*` → CreateSignedURL
   - `POST /object/sign/{bucket}` → CreateSignedURLs
   - `GET /object/sign/{bucket}/*` → GetSignedObject

4. **Add E2E tests** in `e2e/tests/storage/signed-urls.test.ts`

### Phase 2: Upload Signed URLs

1. **Add upload token generation**:
   - `GenerateUploadToken(bucket, path, ownerID, upsert, secret) string`
   - `ValidateUploadToken(token, secret) (*UploadClaims, error)`

2. **Update handler.go**:
   - Add `CreateSignedUploadURL` handler
   - Add `UploadToSignedURL` handler

3. **Update routes**:
   - `POST /object/upload/sign/{bucket}/*` → CreateSignedUploadURL
   - `PUT /object/upload/sign/{bucket}/*` → UploadToSignedURL

4. **Add E2E tests** for upload signed URLs

### Phase 3: Documentation & Polish

1. Update `docs/STORAGE.md` with signed URL documentation
2. Update CLAUDE.md with new endpoints
3. Remove "No signed URLs" from limitations section

## File Changes

| File | Changes |
|------|---------|
| `internal/storage/signed.go` | New file: token generation/validation |
| `internal/storage/handler.go` | Add 5 new handlers |
| `internal/storage/types.go` | Add request/response types |
| `e2e/tests/storage/signed-urls.test.ts` | New E2E test file |
| `docs/STORAGE.md` | Document signed URLs |

## Testing Strategy

### Unit Tests
- Token generation with various expiry times
- Token validation (valid, expired, wrong type, tampered)
- Path matching validation

### E2E Tests
1. Create signed download URL and download file
2. Signed URL expires after expiresIn
3. Signed URL with wrong path fails
4. Batch signed URLs work
5. Create signed upload URL and upload file
6. Upload signed URL expires after 2 hours
7. Upload with upsert flag works
8. RLS is enforced at creation time
9. Service role can create signed URLs for any file

## Compatibility Notes

- Token format matches Supabase (JWT with `url`, `iat`, `exp`)
- Response format matches Supabase exactly
- `signedURL` in response is relative path (not absolute URL)
- Client constructs full URL by prepending base URL
