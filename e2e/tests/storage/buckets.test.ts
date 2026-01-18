/**
 * Storage - Bucket Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/storage-createbucket
 */

import { describe, it, expect, beforeAll, afterAll, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { createServiceRoleClient, uniqueId } from '../../setup/test-helpers'

describe('Storage - Buckets', () => {
  let supabase: SupabaseClient
  let serviceClient: SupabaseClient
  const createdBuckets: string[] = []

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
    serviceClient = createServiceRoleClient()
  })

  afterEach(async () => {
    // Clean up any buckets created during tests
    for (const bucketId of createdBuckets) {
      try {
        await serviceClient.storage.emptyBucket(bucketId)
        await serviceClient.storage.deleteBucket(bucketId)
      } catch (e) {
        // Ignore errors during cleanup
      }
    }
    createdBuckets.length = 0
  })

  /**
   * Create a bucket
   * https://supabase.com/docs/reference/javascript/storage-createbucket
   */
  describe('1. Create bucket', () => {
    it('should create a new bucket', async () => {
      const bucketName = uniqueId('bucket')
      createdBuckets.push(bucketName)

      const { data, error } = await serviceClient.storage.createBucket(bucketName)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.name).toBe(bucketName)
    })

    it('should create a public bucket', async () => {
      const bucketName = uniqueId('public-bucket')
      createdBuckets.push(bucketName)

      const { data, error } = await serviceClient.storage.createBucket(bucketName, {
        public: true,
      })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.name).toBe(bucketName)
    })

    it('should create a bucket with file size limit', async () => {
      const bucketName = uniqueId('limited-bucket')
      createdBuckets.push(bucketName)

      const { data, error } = await serviceClient.storage.createBucket(bucketName, {
        public: false,
        fileSizeLimit: 1024 * 1024, // 1MB
      })

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })

    it('should create a bucket with allowed MIME types', async () => {
      const bucketName = uniqueId('mime-bucket')
      createdBuckets.push(bucketName)

      const { data, error } = await serviceClient.storage.createBucket(bucketName, {
        public: false,
        allowedMimeTypes: ['image/png', 'image/jpeg'],
      })

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })

    it('should fail to create duplicate bucket', async () => {
      const bucketName = uniqueId('dup-bucket')
      createdBuckets.push(bucketName)

      await serviceClient.storage.createBucket(bucketName)
      const { data, error } = await serviceClient.storage.createBucket(bucketName)

      expect(error).not.toBeNull()
      expect(data).toBeNull()
    })
  })

  /**
   * Get a bucket
   * https://supabase.com/docs/reference/javascript/storage-getbucket
   */
  describe('2. Get bucket', () => {
    it('should retrieve a bucket by id', async () => {
      const bucketName = uniqueId('get-bucket')
      createdBuckets.push(bucketName)

      await serviceClient.storage.createBucket(bucketName, { public: true })
      const { data, error } = await serviceClient.storage.getBucket(bucketName)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.name).toBe(bucketName)
      expect(data?.public).toBe(true)
    })

    it('should return error for non-existent bucket', async () => {
      const { data, error } = await serviceClient.storage.getBucket('non-existent-bucket')

      expect(error).not.toBeNull()
      expect(data).toBeNull()
    })
  })

  /**
   * List buckets
   * https://supabase.com/docs/reference/javascript/storage-listbuckets
   */
  describe('3. List buckets', () => {
    it('should list all buckets', async () => {
      const bucketName1 = uniqueId('list-bucket-1')
      const bucketName2 = uniqueId('list-bucket-2')
      createdBuckets.push(bucketName1, bucketName2)

      await serviceClient.storage.createBucket(bucketName1)
      await serviceClient.storage.createBucket(bucketName2)

      const { data, error } = await serviceClient.storage.listBuckets()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(true)
      expect(data?.length).toBeGreaterThanOrEqual(2)

      const bucketNames = data?.map(b => b.name) || []
      expect(bucketNames).toContain(bucketName1)
      expect(bucketNames).toContain(bucketName2)
    })
  })

  /**
   * Update a bucket
   * https://supabase.com/docs/reference/javascript/storage-updatebucket
   */
  describe('4. Update bucket', () => {
    it('should update bucket to public', async () => {
      const bucketName = uniqueId('update-bucket')
      createdBuckets.push(bucketName)

      await serviceClient.storage.createBucket(bucketName, { public: false })
      const { data, error } = await serviceClient.storage.updateBucket(bucketName, {
        public: true,
      })

      expect(error).toBeNull()

      // Verify the update
      const { data: bucket } = await serviceClient.storage.getBucket(bucketName)
      expect(bucket?.public).toBe(true)
    })

    it('should update bucket file size limit', async () => {
      const bucketName = uniqueId('limit-update')
      createdBuckets.push(bucketName)

      await serviceClient.storage.createBucket(bucketName)
      const { data, error } = await serviceClient.storage.updateBucket(bucketName, {
        fileSizeLimit: 2 * 1024 * 1024, // 2MB
      })

      expect(error).toBeNull()
    })
  })

  /**
   * Delete a bucket
   * https://supabase.com/docs/reference/javascript/storage-deletebucket
   */
  describe('5. Delete bucket', () => {
    it('should delete an empty bucket', async () => {
      const bucketName = uniqueId('delete-bucket')

      await serviceClient.storage.createBucket(bucketName)
      const { data, error } = await serviceClient.storage.deleteBucket(bucketName)

      expect(error).toBeNull()

      // Verify bucket is deleted
      const { data: bucket, error: getError } = await serviceClient.storage.getBucket(bucketName)
      expect(getError).not.toBeNull()
      expect(bucket).toBeNull()
    })

    it('should fail to delete non-empty bucket', async () => {
      const bucketName = uniqueId('nonempty-bucket')
      createdBuckets.push(bucketName)

      await serviceClient.storage.createBucket(bucketName)

      // Upload a file to make it non-empty
      const file = new Blob(['test content'], { type: 'text/plain' })
      await serviceClient.storage.from(bucketName).upload('test.txt', file)

      const { data, error } = await serviceClient.storage.deleteBucket(bucketName)

      expect(error).not.toBeNull()
    })
  })

  /**
   * Empty a bucket
   * https://supabase.com/docs/reference/javascript/storage-emptybucket
   */
  describe('6. Empty bucket', () => {
    it('should empty a bucket with files', async () => {
      const bucketName = uniqueId('empty-bucket')
      createdBuckets.push(bucketName)

      await serviceClient.storage.createBucket(bucketName)

      // Upload some files
      const file = new Blob(['test content'], { type: 'text/plain' })
      await serviceClient.storage.from(bucketName).upload('file1.txt', file)
      await serviceClient.storage.from(bucketName).upload('file2.txt', file)
      await serviceClient.storage.from(bucketName).upload('folder/file3.txt', file)

      // Empty the bucket
      const { data, error } = await serviceClient.storage.emptyBucket(bucketName)

      expect(error).toBeNull()

      // Verify bucket is empty
      const { data: files } = await serviceClient.storage.from(bucketName).list()
      expect(files?.length).toBe(0)
    })
  })
})
