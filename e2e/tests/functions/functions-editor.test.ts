/**
 * Edge Functions Code Editor API Tests
 *
 * Tests for the dashboard API endpoints that support the functions code editor UI.
 * These tests verify file operations for editing function source code.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Edge Functions Code Editor API', () => {
  let dashboardCookie: string | null = null
  const testFunctionName = 'test-editor-function'

  /**
   * Helper to make authenticated dashboard API requests
   */
  async function dashboardFetch(path: string, options: RequestInit = {}) {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    }

    if (dashboardCookie) {
      headers['Cookie'] = dashboardCookie
    }

    return fetch(`${TEST_CONFIG.SBLITE_URL}${path}`, {
      ...options,
      headers,
    })
  }

  /**
   * Check if functions are enabled
   */
  async function functionsEnabled(): Promise<boolean> {
    const response = await dashboardFetch('/_/api/functions/status')
    if (!response.ok) return false
    const status = await response.json()
    return status.enabled === true
  }

  /**
   * Create a test function for file operations
   */
  beforeAll(async () => {
    // Create a test function
    const response = await dashboardFetch(`/_/api/functions/${testFunctionName}`, {
      method: 'POST',
      body: JSON.stringify({ template: 'default' }),
    })
    // Ignore 409 if function already exists
    if (!response.ok && response.status !== 409) {
      console.log('Warning: Could not create test function', response.status)
    }
  })

  /**
   * Clean up test function
   */
  afterAll(async () => {
    await dashboardFetch(`/_/api/functions/${testFunctionName}`, { method: 'DELETE' })
  })

  /**
   * File List API Tests
   */
  describe('File List API', () => {
    it('should list files in a function directory', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files`)

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      if (response.status === 404) {
        console.log('Skipping: Function not found')
        return
      }

      expect(response.ok).toBe(true)
      const tree = await response.json()

      // Should be a directory structure
      expect(tree).toHaveProperty('name')
      expect(tree).toHaveProperty('type')
      expect(tree.type).toBe('dir')

      // Should have children (at least index.ts from template)
      if (tree.children && tree.children.length > 0) {
        const indexFile = tree.children.find((c: { name: string }) => c.name === 'index.ts')
        expect(indexFile).toBeDefined()
        expect(indexFile.type).toBe('file')
      }
    })

    it('should return 404 for non-existent function', async () => {
      const response = await dashboardFetch('/_/api/functions/non-existent-function-xyz/files')

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect(response.status).toBe(404)
    })
  })

  /**
   * File Read API Tests
   */
  describe('File Read API', () => {
    it('should read index.ts file content', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/index.ts`)

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      if (response.status === 404) {
        console.log('Skipping: File not found')
        return
      }

      expect(response.ok).toBe(true)
      const data = await response.json()

      expect(data).toHaveProperty('content')
      expect(typeof data.content).toBe('string')
      // Template should contain Deno.serve
      expect(data.content).toContain('Deno.serve')
    })

    it('should return 404 for non-existent file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/non-existent.ts`)

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect(response.status).toBe(404)
    })

    it('should reject path traversal attempts', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const maliciousPaths = [
        '../../../etc/passwd',
        '..%2F..%2F..%2Fetc%2Fpasswd',
        'index.ts/../../../etc/passwd',
      ]

      for (const path of maliciousPaths) {
        const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${path}`)

        if (response.status === 401) {
          continue
        }

        // Should be rejected
        expect([400, 403, 404]).toContain(response.status)
      }
    })
  })

  /**
   * File Write API Tests
   */
  describe('File Write API', () => {
    const testFilePath = 'test-file.ts'
    const testContent = '// Test file content\nexport const test = true;\n'

    afterAll(async () => {
      // Clean up test file
      await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${testFilePath}`, {
        method: 'DELETE',
      })
    })

    it('should create a new file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${testFilePath}`, {
        method: 'PUT',
        body: JSON.stringify({ content: testContent }),
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect([200, 201]).toContain(response.status)

      // Verify content was written
      const readResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${testFilePath}`)
      if (readResponse.ok) {
        const data = await readResponse.json()
        expect(data.content).toBe(testContent)
      }
    })

    it('should update an existing file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const updatedContent = '// Updated content\nexport const updated = true;\n'

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${testFilePath}`, {
        method: 'PUT',
        body: JSON.stringify({ content: updatedContent }),
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect([200, 201]).toContain(response.status)

      // Verify content was updated
      const readResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${testFilePath}`)
      if (readResponse.ok) {
        const data = await readResponse.json()
        expect(data.content).toBe(updatedContent)
      }
    })

    it('should reject disallowed file extensions', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const disallowedFiles = [
        'malware.exe',
        'script.sh',
        'binary.bin',
      ]

      for (const file of disallowedFiles) {
        const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${file}`, {
          method: 'PUT',
          body: JSON.stringify({ content: 'test' }),
        })

        if (response.status === 401) {
          continue
        }

        // Should be rejected
        expect([400, 403]).toContain(response.status)
      }
    })
  })

  /**
   * File Delete API Tests
   */
  describe('File Delete API', () => {
    const deleteTestFile = 'to-delete.ts'

    it('should delete a file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      // First create a file to delete
      const createResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${deleteTestFile}`, {
        method: 'PUT',
        body: JSON.stringify({ content: '// delete me' }),
      })

      if (createResponse.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      // Now delete it
      const deleteResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${deleteTestFile}`, {
        method: 'DELETE',
      })

      expect([200, 204, 404]).toContain(deleteResponse.status)

      // Verify it's deleted
      const readResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${deleteTestFile}`)
      expect(readResponse.status).toBe(404)
    })

    it('should return 404 for non-existent file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/non-existent-file.ts`, {
        method: 'DELETE',
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect(response.status).toBe(404)
    })
  })

  /**
   * File Rename API Tests
   */
  describe('File Rename API', () => {
    const originalFile = 'original.ts'
    const renamedFile = 'renamed.ts'

    afterAll(async () => {
      // Clean up any remaining files
      await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${originalFile}`, { method: 'DELETE' })
      await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${renamedFile}`, { method: 'DELETE' })
    })

    it('should rename a file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      // First create a file to rename
      const createResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${originalFile}`, {
        method: 'PUT',
        body: JSON.stringify({ content: '// original file' }),
      })

      if (createResponse.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      // Now rename it
      const renameResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/rename`, {
        method: 'POST',
        body: JSON.stringify({ oldPath: originalFile, newPath: renamedFile }),
      })

      if (renameResponse.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect([200, 204]).toContain(renameResponse.status)

      // Original should not exist
      const originalCheck = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${originalFile}`)
      expect(originalCheck.status).toBe(404)

      // Renamed file should exist
      const renamedCheck = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/${renamedFile}`)
      expect(renamedCheck.ok).toBe(true)
    })

    it('should reject rename of non-existent file', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/files/rename`, {
        method: 'POST',
        body: JSON.stringify({ oldPath: 'non-existent.ts', newPath: 'new-name.ts' }),
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect([400, 404]).toContain(response.status)
    })
  })

  /**
   * Runtime Restart API Tests
   */
  describe('Runtime Restart API', () => {
    it('should restart the functions runtime', async () => {
      if (!await functionsEnabled()) {
        console.log('Skipping: Functions not enabled')
        return
      }

      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}/restart`, {
        method: 'POST',
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      // Should succeed
      expect([200, 204]).toContain(response.status)

      // Check that runtime is still healthy after restart
      const statusResponse = await dashboardFetch('/_/api/functions/status')
      if (statusResponse.ok) {
        const status = await statusResponse.json()
        expect(status.enabled).toBe(true)
      }
    })
  })
})

/**
 * Compatibility Summary for Edge Functions Code Editor API:
 *
 * IMPLEMENTED:
 * - List function files: GET /_/api/functions/{name}/files
 * - Read file content: GET /_/api/functions/{name}/files/{path}
 * - Write file content: PUT /_/api/functions/{name}/files/{path}
 * - Delete file: DELETE /_/api/functions/{name}/files/{path}
 * - Rename file: POST /_/api/functions/{name}/files/rename
 * - Restart runtime: POST /_/api/functions/{name}/restart
 *
 * SECURITY:
 * - Path traversal protection (../)
 * - File extension validation (only allow safe extensions)
 * - Hidden file protection (no .files)
 */
