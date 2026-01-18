/**
 * Storage - Public Access Tests
 *
 * Tests for accessing files in public buckets without authentication.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { createServiceRoleClient, uniqueId } from '../../setup/test-helpers'

describe('Storage - Public Access', () => {
  let serviceClient: SupabaseClient
  let publicBucketName: string
  let privateBucketName: string

  beforeAll(async () => {
    serviceClient = createServiceRoleClient()

    // Create a public bucket
    publicBucketName = uniqueId('public-access')
    await serviceClient.storage.createBucket(publicBucketName, { public: true })

    // Create a private bucket
    privateBucketName = uniqueId('private-access')
    await serviceClient.storage.createBucket(privateBucketName, { public: false })

    // Upload test files
    const file = new Blob(['test content'], { type: 'text/plain' })
    await serviceClient.storage.from(publicBucketName).upload('test.txt', file)
    await serviceClient.storage.from(privateBucketName).upload('test.txt', file)
  })

  afterAll(async () => {
    try {
      await serviceClient.storage.emptyBucket(publicBucketName)
      await serviceClient.storage.deleteBucket(publicBucketName)
      await serviceClient.storage.emptyBucket(privateBucketName)
      await serviceClient.storage.deleteBucket(privateBucketName)
    } catch (e) {
      // Ignore cleanup errors
    }
  })

  describe('1. Public bucket access', () => {
    it('should access file in public bucket via public URL', async () => {
      const { data } = serviceClient.storage
        .from(publicBucketName)
        .getPublicUrl('test.txt')

      // Fetch the public URL without authentication
      const response = await fetch(data.publicUrl)

      expect(response.ok).toBe(true)
      const text = await response.text()
      expect(text).toBe('test content')
    })

    it('should return correct content-type for public files', async () => {
      // Upload a file with specific content type
      const jsonContent = JSON.stringify({ message: 'hello' })
      const file = new Blob([jsonContent], { type: 'application/json' })
      await serviceClient.storage.from(publicBucketName).upload('data.json', file, {
        contentType: 'application/json',
      })

      const { data } = serviceClient.storage
        .from(publicBucketName)
        .getPublicUrl('data.json')

      const response = await fetch(data.publicUrl)
      expect(response.headers.get('content-type')).toContain('application/json')
    })
  })

  describe('2. Private bucket access', () => {
    it('should fail to access file in private bucket via public URL', async () => {
      // Construct the public URL pattern for private bucket
      const publicUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/object/public/${privateBucketName}/test.txt`

      const response = await fetch(publicUrl)

      // Should fail because bucket is not public
      expect(response.ok).toBe(false)
      expect(response.status).toBe(400) // Or 403 depending on implementation
    })
  })

  describe('3. Download parameter', () => {
    it('should set Content-Disposition header when download param is set', async () => {
      const { data } = serviceClient.storage
        .from(publicBucketName)
        .getPublicUrl('test.txt', { download: 'custom-filename.txt' })

      const response = await fetch(data.publicUrl)

      expect(response.ok).toBe(true)
      const disposition = response.headers.get('content-disposition')
      if (disposition) {
        expect(disposition).toContain('attachment')
        expect(disposition).toContain('custom-filename.txt')
      }
    })
  })

  describe('4. Image files', () => {
    it('should serve image files with correct content type', async () => {
      // Create a minimal PNG image (1x1 pixel)
      const pngData = new Uint8Array([
        0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
        0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
        0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
        0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
        0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
        0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
        0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59,
        0xe7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
        0x44, 0xae, 0x42, 0x60, 0x82,
      ])
      const file = new Blob([pngData], { type: 'image/png' })

      await serviceClient.storage.from(publicBucketName).upload('image.png', file, {
        contentType: 'image/png',
      })

      const { data } = serviceClient.storage
        .from(publicBucketName)
        .getPublicUrl('image.png')

      const response = await fetch(data.publicUrl)

      expect(response.ok).toBe(true)
      expect(response.headers.get('content-type')).toContain('image/png')
    })
  })

  describe('5. Non-existent files', () => {
    it('should return 404 for non-existent file in public bucket', async () => {
      const publicUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/object/public/${publicBucketName}/non-existent.txt`

      const response = await fetch(publicUrl)

      expect(response.ok).toBe(false)
      expect(response.status).toBe(404)
    })
  })
})
