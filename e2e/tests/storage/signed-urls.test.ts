/**
 * Storage - Signed URLs Tests
 *
 * Tests for signed URL generation and usage:
 * 1. createSignedUrl - Create time-limited download URL
 * 2. createSignedUrls - Batch create download URLs
 * 3. Download via signed URL
 * 4. createSignedUploadUrl - Create time-limited upload URL
 * 5. uploadToSignedUrl - Upload via signed URL
 * 6. Token expiration
 * 7. Path validation
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { createServiceRoleClient, uniqueId, uniqueEmail } from '../../setup/test-helpers'

// Helper to toggle email confirmation requirement
async function setEmailConfirmationRequired(serviceClient: SupabaseClient, required: boolean): Promise<void> {
  const { error } = await serviceClient
    .from('_dashboard')
    .upsert({ key: 'auth_require_email_confirmation', value: required ? 'true' : 'false' })
  if (error) {
    console.warn(`Failed to set email confirmation setting: ${error.message}`)
  }
}

async function getEmailConfirmationRequired(serviceClient: SupabaseClient): Promise<boolean> {
  const { data, error } = await serviceClient
    .from('_dashboard')
    .select('value')
    .eq('key', 'auth_require_email_confirmation')
    .single()
  if (error || !data) {
    return true
  }
  return data.value === 'true'
}

describe('Storage - Signed URLs', () => {
  let serviceClient: SupabaseClient
  let userClient: SupabaseClient
  let testBucketName: string
  let originalEmailConfirmSetting: boolean
  let userId: string

  beforeAll(async () => {
    serviceClient = createServiceRoleClient()

    // Save and disable email confirmation for tests
    originalEmailConfirmSetting = await getEmailConfirmationRequired(serviceClient)
    await setEmailConfirmationRequired(serviceClient, false)

    // Create a test bucket
    testBucketName = uniqueId('signed-url-test')
    await serviceClient.storage.createBucket(testBucketName, { public: false })

    // Create a test user
    const email = uniqueEmail()
    const anonClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    const { data: signup, error: signupError } = await anonClient.auth.signUp({
      email,
      password: 'password123',
    })
    if (signupError) throw new Error(`Failed to sign up: ${signupError.message}`)
    userId = signup.user!.id

    const { data: signin, error: signinError } = await anonClient.auth.signInWithPassword({
      email,
      password: 'password123',
    })
    if (signinError || !signin.session?.access_token) {
      throw new Error(`Failed to sign in: ${signinError?.message || 'No session'}`)
    }

    userClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      global: {
        headers: {
          Authorization: `Bearer ${signin.session.access_token}`,
        },
      },
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  afterAll(async () => {
    // Restore email confirmation setting
    await setEmailConfirmationRequired(serviceClient, originalEmailConfirmSetting)

    // Clean up bucket
    try {
      await serviceClient.storage.emptyBucket(testBucketName)
      await serviceClient.storage.deleteBucket(testBucketName)
    } catch (e) {
      // Ignore
    }
  })

  describe('1. createSignedUrl', () => {
    it('should create a signed URL for an existing file', async () => {
      // Upload a file first
      const content = 'test content for signed url'
      const file = new Blob([content], { type: 'text/plain' })
      const path = 'signed-test.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)

      // Create signed URL
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrl(path, 60) // 60 seconds

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.signedUrl).toBeDefined()
      expect(data?.signedUrl).toContain('/storage/v1/object/sign/')
      expect(data?.signedUrl).toContain('token=')

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })

    it('should be able to download file using signed URL', async () => {
      // Upload a file first
      const content = 'download via signed url'
      const file = new Blob([content], { type: 'text/plain' })
      const path = 'download-signed.txt'

      await serviceClient.storage.from(testBucketName).upload(path, file)

      // Create signed URL
      const { data: signedData } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrl(path, 60)

      expect(signedData?.signedUrl).toBeDefined()

      // Download using the signed URL directly via fetch
      // SDK returns full URL, no need to prepend base URL
      const response = await fetch(signedData!.signedUrl)
      const responseText = await response.text()

      expect(response.ok).toBe(true)
      expect(responseText).toBe(content)

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })

    it('should reject invalid token', async () => {
      const invalidUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/object/sign/${testBucketName}/fake-file.txt?token=invalid-token`
      const response = await fetch(invalidUrl)

      expect(response.ok).toBe(false)
      expect(response.status).toBe(401)
    })

    it('should reject token for wrong path', async () => {
      // Upload a file
      const file = new Blob(['content'], { type: 'text/plain' })
      const path = 'path-check.txt'
      await serviceClient.storage.from(testBucketName).upload(path, file)

      // Create signed URL for this file
      const { data: signedData } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrl(path, 60)

      // Try to use the token for a different path
      // SDK returns full URL
      const token = new URL(signedData!.signedUrl).searchParams.get('token')
      const wrongPathUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/object/sign/${testBucketName}/different-file.txt?token=${token}`

      const response = await fetch(wrongPathUrl)
      expect(response.ok).toBe(false)
      expect(response.status).toBe(403)

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })
  })

  describe('2. createSignedUrls (batch)', () => {
    it('should create multiple signed URLs', async () => {
      // Upload multiple files
      const files = ['batch1.txt', 'batch2.txt', 'batch3.txt']
      for (const path of files) {
        const file = new Blob([`content of ${path}`], { type: 'text/plain' })
        await serviceClient.storage.from(testBucketName).upload(path, file)
      }

      // Create signed URLs for all files
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrls(files, 60)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data).toHaveLength(3)

      for (let i = 0; i < files.length; i++) {
        expect(data![i].path).toBe(files[i])
        expect(data![i].signedUrl).toContain('/storage/v1/object/sign/')
        expect(data![i].error).toBeNull()
      }

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove(files)
    })
  })

  describe('3. createSignedUploadUrl', () => {
    it('should create a signed upload URL', async () => {
      const path = 'upload-via-signed.txt'

      // Create signed upload URL
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUploadUrl(path)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data?.signedUrl).toBeDefined()
      expect(data?.signedUrl).toContain('/storage/v1/object/upload/sign/')
      expect(data?.signedUrl).toContain('token=')
      expect(data?.token).toBeDefined()
      expect(data?.path).toBe(path)
    })

    it('should be able to upload using signed URL', async () => {
      const path = 'upload-test-' + Date.now() + '.txt'
      const content = 'uploaded via signed url'

      // Create signed upload URL
      const { data: signedData } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUploadUrl(path)

      expect(signedData).toBeDefined()

      // Upload using the signed URL
      const { data: uploadData, error: uploadError } = await serviceClient.storage
        .from(testBucketName)
        .uploadToSignedUrl(path, signedData!.token, new Blob([content], { type: 'text/plain' }))

      expect(uploadError).toBeNull()
      expect(uploadData).toBeDefined()

      // Verify the file was uploaded
      const { data: downloadData } = await serviceClient.storage
        .from(testBucketName)
        .download(path)

      expect(downloadData).toBeDefined()
      const downloadedContent = await downloadData!.text()
      expect(downloadedContent).toBe(content)

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })

    it('should reject upload with invalid token', async () => {
      const invalidUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/object/upload/sign/${testBucketName}/fake.txt?token=invalid`
      const response = await fetch(invalidUrl, {
        method: 'PUT',
        headers: { 'Content-Type': 'text/plain' },
        body: 'test content',
      })

      expect(response.ok).toBe(false)
      expect(response.status).toBe(401)
    })

    it('should reject upload to wrong path', async () => {
      const path = 'correct-path.txt'

      // Create signed upload URL
      const { data: signedData } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUploadUrl(path)

      // Try to upload to a different path using the same token
      const wrongUrl = `${TEST_CONFIG.SBLITE_URL}/storage/v1/object/upload/sign/${testBucketName}/wrong-path.txt?token=${signedData!.token}`
      const response = await fetch(wrongUrl, {
        method: 'PUT',
        headers: { 'Content-Type': 'text/plain' },
        body: 'test content',
      })

      expect(response.ok).toBe(false)
      expect(response.status).toBe(403)
    })
  })

  describe('4. Token expiration', () => {
    it('should reject expired download token', async () => {
      // Upload a file
      const file = new Blob(['expiry test'], { type: 'text/plain' })
      const path = 'expiry-test.txt'
      await serviceClient.storage.from(testBucketName).upload(path, file)

      // Create signed URL with 1 second expiry
      const { data: signedData } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrl(path, 1)

      // Wait for expiration
      await new Promise(resolve => setTimeout(resolve, 1500))

      // Try to download - SDK returns full URL
      const response = await fetch(signedData!.signedUrl)

      expect(response.ok).toBe(false)
      expect(response.status).toBe(401)

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })
  })

  describe('5. Download with options', () => {
    it('should support download query parameter', async () => {
      // Upload a file
      const content = 'download with custom name'
      const file = new Blob([content], { type: 'text/plain' })
      const path = 'custom-download.txt'
      await serviceClient.storage.from(testBucketName).upload(path, file)

      // Create signed URL with download option
      const { data: signedData } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrl(path, 60, { download: 'custom-name.txt' })

      expect(signedData?.signedUrl).toContain('download=')

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })
  })

  describe('6. Service role access', () => {
    it('service role should be able to create signed URLs for any file', async () => {
      // Upload a file
      const file = new Blob(['service role test'], { type: 'text/plain' })
      const path = 'service-role-signed.txt'
      await serviceClient.storage.from(testBucketName).upload(path, file)

      // Create signed URL with service role (bypasses RLS)
      const { data, error } = await serviceClient.storage
        .from(testBucketName)
        .createSignedUrl(path, 60)

      expect(error).toBeNull()
      expect(data?.signedUrl).toBeDefined()

      // Cleanup
      await serviceClient.storage.from(testBucketName).remove([path])
    })
  })
})
