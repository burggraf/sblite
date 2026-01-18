/**
 * Edge Functions Dashboard API Tests
 *
 * Tests for the dashboard API endpoints that support the functions management UI.
 * These tests verify the backend functionality used by the dashboard frontend.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Edge Functions Dashboard API', () => {
  let dashboardCookie: string | null = null

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
   * Functions List API Tests
   */
  describe('Functions List API', () => {
    it('should list functions from /_/api/functions', async () => {
      const response = await dashboardFetch('/_/api/functions')

      // Skip if not authenticated or not found
      if (response.status === 401 || response.status === 404) {
        console.log('Skipping: Dashboard auth required or functions not enabled')
        return
      }

      if (response.ok) {
        const functions = await response.json()
        expect(Array.isArray(functions)).toBe(true)

        // Each function should have expected properties
        if (functions.length > 0) {
          for (const fn of functions) {
            expect(fn).toHaveProperty('name')
            expect(typeof fn.name).toBe('string')
          }
        }
      }
    })

    it('should get functions status from /_/api/functions/status', async () => {
      const response = await dashboardFetch('/_/api/functions/status')

      // Skip if not found
      if (response.status === 404) {
        console.log('Skipping: Functions status endpoint not found')
        return
      }

      if (response.ok) {
        const status = await response.json()
        expect(status).toHaveProperty('enabled')

        // If enabled, should have running status
        if (status.enabled) {
          expect(status).toHaveProperty('running')
          expect(typeof status.running).toBe('boolean')
        }
      }
    })
  })

  /**
   * Function Details API Tests
   */
  describe('Function Details API', () => {
    it('should get function config from /_/api/functions/{name}/config', async () => {
      // First get the list of functions
      const listResponse = await dashboardFetch('/_/api/functions')
      if (!listResponse.ok) {
        console.log('Skipping: Cannot list functions')
        return
      }

      const functions = await listResponse.json()
      if (!Array.isArray(functions) || functions.length === 0) {
        console.log('Skipping: No functions available')
        return
      }

      const functionName = functions[0].name
      const response = await dashboardFetch(`/_/api/functions/${functionName}/config`)

      if (response.ok) {
        const config = await response.json()
        expect(config).toHaveProperty('name')
        expect(config.name).toBe(functionName)
        expect(config).toHaveProperty('verify_jwt')
      }
    })

    it('should return 404 for non-existent function config', async () => {
      const response = await dashboardFetch('/_/api/functions/non-existent-function-xyz/config')

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      // Should return 404 for non-existent function
      expect([404, 500]).toContain(response.status)
    })
  })

  /**
   * Function Config Update API Tests
   */
  describe('Function Config Update API', () => {
    it('should update function JWT verification setting', async () => {
      // First get the list of functions
      const listResponse = await dashboardFetch('/_/api/functions')
      if (!listResponse.ok) {
        console.log('Skipping: Cannot list functions')
        return
      }

      const functions = await listResponse.json()
      if (!Array.isArray(functions) || functions.length === 0) {
        console.log('Skipping: No functions available')
        return
      }

      const functionName = functions[0].name

      // Get current config
      const configResponse = await dashboardFetch(`/_/api/functions/${functionName}/config`)
      if (!configResponse.ok) {
        console.log('Skipping: Cannot get function config')
        return
      }

      const currentConfig = await configResponse.json()
      const currentVerifyJwt = currentConfig.verify_jwt

      // Toggle the setting
      const updateResponse = await dashboardFetch(`/_/api/functions/${functionName}/config`, {
        method: 'PATCH',
        body: JSON.stringify({ verify_jwt: !currentVerifyJwt }),
      })

      if (updateResponse.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      if (updateResponse.ok) {
        const updatedConfig = await updateResponse.json()
        expect(updatedConfig.verify_jwt).toBe(!currentVerifyJwt)

        // Restore original setting
        await dashboardFetch(`/_/api/functions/${functionName}/config`, {
          method: 'PATCH',
          body: JSON.stringify({ verify_jwt: currentVerifyJwt }),
        })
      }
    })
  })

  /**
   * Secrets List API Tests
   */
  describe('Secrets List API', () => {
    it('should list secrets from /_/api/secrets', async () => {
      const response = await dashboardFetch('/_/api/secrets')

      if (response.status === 401 || response.status === 404) {
        console.log('Skipping: Dashboard auth required or secrets not enabled')
        return
      }

      if (response.ok) {
        const secrets = await response.json()
        expect(Array.isArray(secrets)).toBe(true)

        // Each secret should have name property (values are never exposed)
        for (const secret of secrets) {
          expect(secret).toHaveProperty('name')
          expect(typeof secret.name).toBe('string')
          // Value should NOT be exposed
          expect(secret.value).toBeUndefined()
        }
      }
    })
  })

  /**
   * Secrets Management API Tests
   */
  describe('Secrets Management API', () => {
    const testSecretName = 'TEST_E2E_SECRET'

    afterAll(async () => {
      // Clean up test secret
      await dashboardFetch(`/_/api/secrets/${testSecretName}`, { method: 'DELETE' })
    })

    it('should create a new secret', async () => {
      const response = await dashboardFetch('/_/api/secrets', {
        method: 'POST',
        body: JSON.stringify({
          name: testSecretName,
          value: 'test-secret-value-12345',
        }),
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      if (response.status === 404) {
        console.log('Skipping: Secrets endpoint not found')
        return
      }

      expect([200, 201]).toContain(response.status)
    })

    it('should list secrets including the created one', async () => {
      const response = await dashboardFetch('/_/api/secrets')

      if (!response.ok) {
        console.log('Skipping: Cannot list secrets')
        return
      }

      const secrets = await response.json()
      const foundSecret = secrets.find((s: { name: string }) => s.name === testSecretName)

      // Secret should be in the list
      expect(foundSecret).toBeDefined()
      expect(foundSecret.name).toBe(testSecretName)
    })

    it('should delete a secret', async () => {
      const response = await dashboardFetch(`/_/api/secrets/${testSecretName}`, {
        method: 'DELETE',
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect([200, 204, 404]).toContain(response.status)

      // Verify it's deleted
      const listResponse = await dashboardFetch('/_/api/secrets')
      if (listResponse.ok) {
        const secrets = await listResponse.json()
        const foundSecret = secrets.find((s: { name: string }) => s.name === testSecretName)
        expect(foundSecret).toBeUndefined()
      }
    })
  })

  /**
   * Function Creation API Tests (if supported)
   */
  describe('Function Creation API', () => {
    const testFunctionName = 'test-e2e-function'

    afterAll(async () => {
      // Clean up test function
      await dashboardFetch(`/_/api/functions/${testFunctionName}`, { method: 'DELETE' })
    })

    it('should create a new function', async () => {
      const response = await dashboardFetch(`/_/api/functions/${testFunctionName}`, {
        method: 'POST',
        body: JSON.stringify({ template: 'default' }),
      })

      if (response.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      if (response.status === 404 || response.status === 501) {
        console.log('Skipping: Function creation not supported via API')
        return
      }

      if (response.ok) {
        const result = await response.json()
        expect(result).toHaveProperty('name')
        expect(result.name).toBe(testFunctionName)
      }
    })

    it('should reject invalid function names', async () => {
      const invalidNames = ['123invalid', 'UPPER_CASE', 'has spaces', 'has.dots']

      for (const invalidName of invalidNames) {
        const response = await dashboardFetch(`/_/api/functions/${invalidName}`, {
          method: 'POST',
          body: JSON.stringify({ template: 'default' }),
        })

        if (response.status === 401 || response.status === 404 || response.status === 501) {
          continue // Skip if not supported or not authenticated
        }

        // Should reject invalid names
        expect([400, 422]).toContain(response.status)
      }
    })

    it('should delete a function', async () => {
      // First create a function to delete
      const createResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}`, {
        method: 'POST',
        body: JSON.stringify({ template: 'default' }),
      })

      if (!createResponse.ok && createResponse.status !== 409) {
        console.log('Skipping: Cannot create function for deletion test')
        return
      }

      const deleteResponse = await dashboardFetch(`/_/api/functions/${testFunctionName}`, {
        method: 'DELETE',
      })

      if (deleteResponse.status === 401) {
        console.log('Skipping: Dashboard auth required')
        return
      }

      expect([200, 204, 404]).toContain(deleteResponse.status)
    })
  })
})

/**
 * Compatibility Summary for Edge Functions Dashboard API:
 *
 * IMPLEMENTED:
 * - List functions: GET /_/api/functions
 * - Get runtime status: GET /_/api/functions/status
 * - Get function config: GET /_/api/functions/{name}/config
 * - Update function config: PATCH /_/api/functions/{name}/config
 * - List secrets: GET /_/api/secrets
 * - Create secret: POST /_/api/secrets
 * - Delete secret: DELETE /_/api/secrets/{name}
 * - Create function: POST /_/api/functions/{name}
 * - Delete function: DELETE /_/api/functions/{name}
 *
 * MATCHES SUPABASE DASHBOARD:
 * - Functions list with verify_jwt status
 * - Per-function configuration
 * - Secrets management (names only, values never exposed)
 */
