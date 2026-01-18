/**
 * Storage - Object Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/storage-from-upload
 */

import { describe, it, expect, beforeAll, afterAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { createServiceRoleClient, uniqueId } from '../../setup/test-helpers'

describe('Storage - Objects', () => {
  let supabase: SupabaseClient
  let serviceClient: SupabaseClient
  let testBucketName: string

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
    serviceClient = createServiceRoleClient()

    // Create a test bucket for all object tests
    testBucketName = uniqueId('objects-test')
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

  /**
   * Upload a file
   * https://supabase.com/docs/reference/javascript/storage-from-upload
   */
  describe('1. Upload file', () => {
    it('should upload a file', async () => {
      const file = new Blob(['Hello, World!'], { type: 'text/plain' })
      const path = 'hello.txt'

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .upload(path, file)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.path).toBe(path)
    })

    it('should upload a file to a folder', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      const path = 'folder/subfolder/file.txt'

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .upload(path, file)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.path).toBe(path)
    })

    it('should fail to upload duplicate without upsert', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      const path = 'duplicate.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .upload(path, file)

      expect(error).not.toBeNull()
    })

    it('should upsert a file', async () => {
      const file1 = new Blob(['original content'], { type: 'text/plain' })
      const file2 = new Blob(['updated content'], { type: 'text/plain' })
      const path = 'upsert.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file1)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .upload(path, file2, { upsert: true })

      expect(error).toBeNull()
      expect(data).toBeDefined()

      // Verify content was updated
      const { data: downloadData } = await serviceClient.storage
        .from(testBucketName)
        .download(path)
      const text = await downloadData?.text()
      expect(text).toBe('updated content')
    })

    it('should upload with custom content type', async () => {
      const jsonContent = JSON.stringify({ key: 'value' })
      const file = new Blob([jsonContent], { type: 'application/json' })
      const path = 'data.json'

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .upload(path, file, {
          contentType: 'application/json',
        })

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })
  })

  /**
   * Download a file
   * https://supabase.com/docs/reference/javascript/storage-from-download
   */
  describe('2. Download file', () => {
    it('should download a file', async () => {
      const content = 'Download test content'
      const file = new Blob([content], { type: 'text/plain' })
      const path = 'download-test.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download(path)

      expect(error).toBeNull()
      expect(data).toBeDefined()

      const text = await data?.text()
      expect(text).toBe(content)
    })

    it('should return error for non-existent file', async () => {
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .download('non-existent-file.txt')

      expect(error).not.toBeNull()
    })
  })

  /**
   * List files
   * https://supabase.com/docs/reference/javascript/storage-from-list
   */
  describe('3. List files', () => {
    it('should list all files in bucket root', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      await serviceClient.storage.from(testBucketName).upload('file1.txt', file)
      await serviceClient.storage.from(testBucketName).upload('file2.txt', file)

      const { data, error } = await serviceClient.storage.from(testBucketName).list()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.length).toBeGreaterThanOrEqual(2)

      const names = data?.map(f => f.name) || []
      expect(names).toContain('file1.txt')
      expect(names).toContain('file2.txt')
    })

    it('should list files in a folder', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      await serviceClient.storage.from(testBucketName).upload('folder/a.txt', file)
      await serviceClient.storage.from(testBucketName).upload('folder/b.txt', file)
      await serviceClient.storage.from(testBucketName).upload('other/c.txt', file)

      const { data, error } = await serviceClient.storage.from(testBucketName).list('folder')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.length).toBe(2)

      const names = data?.map(f => f.name) || []
      expect(names).toContain('a.txt')
      expect(names).toContain('b.txt')
    })

    it('should list with limit', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      for (let i = 0; i < 5; i++) {
        await serviceClient.storage.from(testBucketName).upload(`limited-${i}.txt`, file)
      }

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .list('', { limit: 3 })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.length).toBeLessThanOrEqual(3)
    })

    it('should list with search', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      await serviceClient.storage.from(testBucketName).upload('searchable-file.txt', file)
      await serviceClient.storage.from(testBucketName).upload('other-file.txt', file)

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .list('', { search: 'searchable' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.some(f => f.name.includes('searchable'))).toBe(true)
    })

    it('should list with sorting', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      await serviceClient.storage.from(testBucketName).upload('z-file.txt', file)
      await serviceClient.storage.from(testBucketName).upload('a-file.txt', file)
      await serviceClient.storage.from(testBucketName).upload('m-file.txt', file)

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .list('', { sortBy: { column: 'name', order: 'asc' } })

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })
  })

  /**
   * Remove files
   * https://supabase.com/docs/reference/javascript/storage-from-remove
   */
  describe('4. Remove files', () => {
    it('should remove a single file', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      const path = 'to-remove.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .remove([path])

      expect(error).toBeNull()

      // Verify file is removed
      const { error: downloadError } = await serviceClient.storage
        .from(testBucketName)
        .download(path)
      expect(downloadError).not.toBeNull()
    })

    it('should remove multiple files', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      const paths = ['remove1.txt', 'remove2.txt', 'remove3.txt']

      for (const path of paths) {
        await serviceClient.storage.from(testBucketName).upload(path, file)
      }

      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .remove(paths)

      expect(error).toBeNull()

      // Verify all files are removed
      const { data: files } = await serviceClient.storage.from(testBucketName).list()
      const remainingNames = files?.map(f => f.name) || []
      for (const path of paths) {
        expect(remainingNames).not.toContain(path)
      }
    })
  })

  /**
   * Move a file
   * https://supabase.com/docs/reference/javascript/storage-from-move
   */
  describe('5. Move file', () => {
    it('should move a file to a new location', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      const srcPath = 'source.txt'
      const dstPath = 'destination.txt'

      await serviceClient.storage.from(testBucketName).upload(srcPath, file)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .move(srcPath, dstPath)

      expect(error).toBeNull()

      // Verify source is gone
      const { error: srcError } = await serviceClient.storage
        .from(testBucketName)
        .download(srcPath)
      expect(srcError).not.toBeNull()

      // Verify destination exists
      const { data: dstData, error: dstError } = await serviceClient.storage
        .from(testBucketName)
        .download(dstPath)
      expect(dstError).toBeNull()
      expect(dstData).toBeDefined()
    })

    it('should move a file to a folder', async () => {
      const file = new Blob(['content'], { type: 'text/plain' })
      const srcPath = 'root-file.txt'
      const dstPath = 'folder/moved-file.txt'

      await serviceClient.storage.from(testBucketName).upload(srcPath, file)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .move(srcPath, dstPath)

      expect(error).toBeNull()

      // Verify destination exists
      const { data: dstData, error: dstError } = await serviceClient.storage
        .from(testBucketName)
        .download(dstPath)
      expect(dstError).toBeNull()
    })
  })

  /**
   * Copy a file
   * https://supabase.com/docs/reference/javascript/storage-from-copy
   */
  describe('6. Copy file', () => {
    it('should copy a file', async () => {
      const content = 'copy test content'
      const file = new Blob([content], { type: 'text/plain' })
      const srcPath = 'original.txt'
      const dstPath = 'copied.txt'

      await serviceClient.storage.from(testBucketName).upload(srcPath, file)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .copy(srcPath, dstPath)

      expect(error).toBeNull()

      // Verify both files exist with same content
      const { data: srcData } = await serviceClient.storage
        .from(testBucketName)
        .download(srcPath)
      const { data: dstData } = await serviceClient.storage
        .from(testBucketName)
        .download(dstPath)

      expect(srcData).toBeDefined()
      expect(dstData).toBeDefined()
      expect(await srcData?.text()).toBe(content)
      expect(await dstData?.text()).toBe(content)
    })
  })

  /**
   * Get public URL
   * https://supabase.com/docs/reference/javascript/storage-from-getpublicurl
   */
  describe('7. Get public URL', () => {
    it('should get public URL for a file', async () => {
      const file = new Blob(['public content'], { type: 'text/plain' })
      const path = 'public-file.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)
      const { data } = serviceClient.storage
        .from(testBucketName)
        .getPublicUrl(path)

      expect(data).toBeDefined()
      expect(data.publicUrl).toContain(testBucketName)
      expect(data.publicUrl).toContain(path)
    })

    it('should include download query param', async () => {
      const file = new Blob(['download content'], { type: 'text/plain' })
      const path = 'downloadable.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)
      const { data } = serviceClient.storage
        .from(testBucketName)
        .getPublicUrl(path, { download: 'custom-filename.txt' })

      expect(data.publicUrl).toContain('download=')
    })
  })
})
