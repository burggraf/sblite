/**
 * Storage - TUS Resumable Uploads Tests
 *
 * Tests for TUS protocol implementation for resumable file uploads.
 * TUS protocol: https://tus.io/protocols/resumable-upload.html
 */

import { describe, it, expect, beforeAll, afterAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { createServiceRoleClient, uniqueId } from '../../setup/test-helpers'

// Base64 encode helper
function base64Encode(str: string): string {
  return Buffer.from(str).toString('base64')
}

// Build TUS metadata header
function buildMetadata(data: Record<string, string>): string {
  return Object.entries(data)
    .map(([key, value]) => `${key} ${base64Encode(value)}`)
    .join(',')
}

describe('Storage - TUS Resumable Uploads', () => {
  let supabase: SupabaseClient
  let serviceClient: SupabaseClient
  let testBucketName: string
  let authToken: string

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
    serviceClient = createServiceRoleClient()

    // Get service role token for auth
    authToken = TEST_CONFIG.SBLITE_SERVICE_KEY

    // Create a test bucket for all upload tests
    testBucketName = uniqueId('tus-test')
    await serviceClient.storage.createBucket(testBucketName, { public: true })
  })

  afterAll(async () => {
    // Clean up test bucket
    try {
      await serviceClient.storage.emptyBucket(testBucketName)
      await serviceClient.storage.deleteBucket(testBucketName)
    } catch (e) {
      // Ignore errors during cleanup
    }
  })

  afterEach(async () => {
    // Clean up files after each test
    try {
      const { data: files } = await serviceClient.storage.from(testBucketName).list()
      if (files && files.length > 0) {
        const paths = files.map(f => f.name)
        await serviceClient.storage.from(testBucketName).remove(paths)
      }
    } catch (e) {
      // Ignore errors during cleanup
    }
  })

  const baseUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/upload/resumable`

  describe('1. TUS Protocol Compliance', () => {
    it('should return TUS capabilities on OPTIONS', async () => {
      const response = await fetch(baseUrl, {
        method: 'OPTIONS',
        headers: {
          'Tus-Resumable': '1.0.0',
        },
      })

      expect(response.status).toBe(204)
      expect(response.headers.get('Tus-Resumable')).toBe('1.0.0')
      expect(response.headers.get('Tus-Version')).toBe('1.0.0')
      expect(response.headers.get('Tus-Extension')).toContain('creation')
      expect(response.headers.get('Tus-Extension')).toContain('termination')
    })

    it('should reject requests without Tus-Resumable header', async () => {
      const response = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Upload-Length': '100',
          'Upload-Metadata': buildMetadata({
            bucketName: testBucketName,
            objectName: 'test.txt',
          }),
        },
      })

      expect(response.status).toBe(412)
    })
  })

  describe('2. Create Upload Session', () => {
    it('should create an upload session with POST', async () => {
      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName: 'create-test.txt',
        contentType: 'text/plain',
      })

      const response = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': '100',
          'Upload-Metadata': metadata,
        },
      })

      expect(response.status).toBe(201)
      expect(response.headers.get('Tus-Resumable')).toBe('1.0.0')
      expect(response.headers.get('Location')).toBeTruthy()
      expect(response.headers.get('Upload-Offset')).toBe('0')
    })

    it('should require bucketName and objectName in metadata', async () => {
      const response = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': '100',
          'Upload-Metadata': buildMetadata({ bucketName: testBucketName }),
        },
      })

      expect(response.status).toBe(400)
    })

    it('should reject upload exceeding bucket size limit', async () => {
      // Create bucket with size limit
      const limitedBucket = uniqueId('limited')
      await serviceClient.storage.createBucket(limitedBucket, {
        public: true,
        fileSizeLimit: 1000, // 1KB limit
      })

      try {
        const metadata = buildMetadata({
          bucketName: limitedBucket,
          objectName: 'too-big.txt',
        })

        const response = await fetch(baseUrl, {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${authToken}`,
            'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
            'Tus-Resumable': '1.0.0',
            'Upload-Length': '10000', // 10KB - exceeds limit
            'Upload-Metadata': metadata,
          },
        })

        expect(response.status).toBe(413)
      } finally {
        await serviceClient.storage.deleteBucket(limitedBucket)
      }
    })
  })

  describe('3. Query Upload Progress', () => {
    it('should return upload progress with HEAD', async () => {
      // Create upload session
      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName: 'progress-test.txt',
      })

      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': '100',
          'Upload-Metadata': metadata,
        },
      })

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Query progress
      const headResponse = await fetch(uploadUrl, {
        method: 'HEAD',
        headers: {
          'Tus-Resumable': '1.0.0',
        },
      })

      expect(headResponse.status).toBe(200)
      expect(headResponse.headers.get('Upload-Offset')).toBe('0')
      expect(headResponse.headers.get('Upload-Length')).toBe('100')
      expect(headResponse.headers.get('Cache-Control')).toBe('no-store')

      // Clean up
      await fetch(uploadUrl, {
        method: 'DELETE',
        headers: { 'Tus-Resumable': '1.0.0' },
      })
    })

    it('should return 404 for non-existent upload', async () => {
      const response = await fetch(`${baseUrl}/nonexistent-upload-id`, {
        method: 'HEAD',
        headers: {
          'Tus-Resumable': '1.0.0',
        },
      })

      expect(response.status).toBe(404)
    })
  })

  describe('4. Upload Chunks', () => {
    it('should upload a complete file in one chunk', async () => {
      const content = 'Hello, World!'
      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName: 'single-chunk.txt',
        contentType: 'text/plain',
      })

      // Create upload
      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': String(content.length),
          'Upload-Metadata': metadata,
        },
      })

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Upload chunk
      const patchResponse = await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': '0',
          'Content-Type': 'application/offset+octet-stream',
        },
        body: content,
      })

      expect(patchResponse.status).toBe(204)
      expect(patchResponse.headers.get('Upload-Offset')).toBe(String(content.length))

      // Verify file was created
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download('single-chunk.txt')

      expect(error).toBeNull()
      expect(await data?.text()).toBe(content)
    })

    it('should upload a file in multiple chunks', async () => {
      const chunk1 = 'First chunk. '
      const chunk2 = 'Second chunk.'
      const totalLength = chunk1.length + chunk2.length
      const objectName = 'multi-chunk.txt'

      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName,
        contentType: 'text/plain',
      })

      // Create upload
      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': String(totalLength),
          'Upload-Metadata': metadata,
        },
      })

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Upload first chunk
      const patch1Response = await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': '0',
          'Content-Type': 'application/offset+octet-stream',
        },
        body: chunk1,
      })

      expect(patch1Response.status).toBe(204)
      expect(patch1Response.headers.get('Upload-Offset')).toBe(String(chunk1.length))

      // Upload second chunk
      const patch2Response = await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': String(chunk1.length),
          'Content-Type': 'application/offset+octet-stream',
        },
        body: chunk2,
      })

      expect(patch2Response.status).toBe(204)
      expect(patch2Response.headers.get('Upload-Offset')).toBe(String(totalLength))

      // Verify complete file
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download(objectName)

      expect(error).toBeNull()
      expect(await data?.text()).toBe(chunk1 + chunk2)
    })

    it('should return 409 on offset mismatch', async () => {
      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName: 'offset-test.txt',
      })

      // Create upload
      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': '100',
          'Upload-Metadata': metadata,
        },
      })

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Upload with wrong offset
      const patchResponse = await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': '50', // Wrong - should be 0
          'Content-Type': 'application/offset+octet-stream',
        },
        body: 'test data',
      })

      expect(patchResponse.status).toBe(409)

      // Clean up
      await fetch(uploadUrl, {
        method: 'DELETE',
        headers: { 'Tus-Resumable': '1.0.0' },
      })
    })
  })

  describe('5. Cancel Upload', () => {
    it('should cancel an upload with DELETE', async () => {
      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName: 'cancel-test.txt',
      })

      // Create upload
      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': '100',
          'Upload-Metadata': metadata,
        },
      })

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Cancel upload
      const deleteResponse = await fetch(uploadUrl, {
        method: 'DELETE',
        headers: {
          'Tus-Resumable': '1.0.0',
        },
      })

      expect(deleteResponse.status).toBe(204)

      // Verify upload is gone
      const headResponse = await fetch(uploadUrl, {
        method: 'HEAD',
        headers: { 'Tus-Resumable': '1.0.0' },
      })

      expect(headResponse.status).toBe(404)
    })
  })

  describe('6. Resume After Interruption', () => {
    it('should resume upload from last offset', async () => {
      const chunk1 = 'Chunk one data. '
      const chunk2 = 'Chunk two data.'
      const totalLength = chunk1.length + chunk2.length
      const objectName = 'resume-test.txt'

      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName,
        contentType: 'text/plain',
      })

      // Create upload
      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': String(totalLength),
          'Upload-Metadata': metadata,
        },
      })

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Upload first chunk
      await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': '0',
          'Content-Type': 'application/offset+octet-stream',
        },
        body: chunk1,
      })

      // Simulate resumption - query current offset
      const headResponse = await fetch(uploadUrl, {
        method: 'HEAD',
        headers: { 'Tus-Resumable': '1.0.0' },
      })

      const currentOffset = headResponse.headers.get('Upload-Offset')
      expect(currentOffset).toBe(String(chunk1.length))

      // Resume from current offset
      const patchResponse = await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': currentOffset!,
          'Content-Type': 'application/offset+octet-stream',
        },
        body: chunk2,
      })

      expect(patchResponse.status).toBe(204)
      expect(patchResponse.headers.get('Upload-Offset')).toBe(String(totalLength))

      // Verify complete file
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download(objectName)

      expect(error).toBeNull()
      expect(await data?.text()).toBe(chunk1 + chunk2)
    })
  })

  describe('7. Upsert Support', () => {
    it('should support x-upsert header for replacing files', async () => {
      const objectName = 'upsert-test.txt'

      // Upload initial file normally
      const file = new Blob(['original content'], { type: 'text/plain' })
      await serviceClient.storage.from(testBucketName).upload(objectName, file)

      // Use TUS to replace it with upsert
      const newContent = 'replaced content'
      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName,
        contentType: 'text/plain',
      })

      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': String(newContent.length),
          'Upload-Metadata': metadata,
          'x-upsert': 'true',
        },
      })

      expect(createResponse.status).toBe(201)

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Upload content
      await fetch(uploadUrl, {
        method: 'PATCH',
        headers: {
          'Tus-Resumable': '1.0.0',
          'Upload-Offset': '0',
          'Content-Type': 'application/offset+octet-stream',
        },
        body: newContent,
      })

      // Verify file was replaced
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download(objectName)

      expect(error).toBeNull()
      expect(await data?.text()).toBe(newContent)
    })
  })

  describe('8. Large File Upload', () => {
    it('should handle larger file uploads', async () => {
      // Create a ~100KB file
      const size = 100 * 1024
      const content = 'x'.repeat(size)
      const objectName = 'large-file.txt'

      const metadata = buildMetadata({
        bucketName: testBucketName,
        objectName,
        contentType: 'text/plain',
      })

      // Create upload
      const createResponse = await fetch(baseUrl, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${authToken}`,
          'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY,
          'Tus-Resumable': '1.0.0',
          'Upload-Length': String(size),
          'Upload-Metadata': metadata,
        },
      })

      expect(createResponse.status).toBe(201)

      const location = createResponse.headers.get('Location')!
      const uploadUrl = `${TEST_CONFIG.SBLITE_URL}${location}`

      // Upload in chunks of 32KB
      const chunkSize = 32 * 1024
      let offset = 0

      while (offset < size) {
        const chunk = content.slice(offset, offset + chunkSize)
        const patchResponse = await fetch(uploadUrl, {
          method: 'PATCH',
          headers: {
            'Tus-Resumable': '1.0.0',
            'Upload-Offset': String(offset),
            'Content-Type': 'application/offset+octet-stream',
          },
          body: chunk,
        })

        expect(patchResponse.status).toBe(204)
        offset += chunk.length
      }

      // Verify file
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download(objectName)

      expect(error).toBeNull()
      const downloadedContent = await data?.text()
      expect(downloadedContent?.length).toBe(size)
    })
  })
})
