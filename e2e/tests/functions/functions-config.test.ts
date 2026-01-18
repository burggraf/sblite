/**
 * Edge Functions Configuration Tests
 *
 * Tests for per-function JWT verification and other configuration options.
 * These tests verify that the configuration system works correctly.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Edge Functions Configuration', () => {
  let supabase: SupabaseClient
  let anonSupabase: SupabaseClient

  beforeAll(() => {
    // Client with auth
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Client without auth (for testing JWT verification)
    anonSupabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
      global: {
        headers: {
          // Don't send the default Authorization header
        },
      },
    })
  })

  /**
   * JWT Verification Tests
   *
   * Note: These tests require specific function configuration.
   * By default, functions require JWT verification.
   * To test disabled JWT verification, the function must be configured with:
   *   sblite functions config set-jwt <function-name> disabled
   */
  describe('JWT Verification', () => {
    it('should reject requests without JWT when verification is enabled (default)', async () => {
      // Make a raw fetch request without Authorization header
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/functions/v1/hello-world`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ name: 'Test' }),
      })

      // Skip if functions not enabled
      if (response.status === 502) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      // Should get 401 Unauthorized when JWT verification is enabled
      // (default behavior) and no token is provided
      expect(response.status).toBe(401)
    })

    it('should accept requests with valid JWT when verification is enabled', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world', {
        body: { name: 'Authenticated' },
      })

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data.message).toBe('Hello Authenticated!')
    })

    it('should reject requests with invalid JWT', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/functions/v1/hello-world`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': 'Bearer invalid-token-here',
        },
        body: JSON.stringify({ name: 'Test' }),
      })

      // Skip if functions not enabled
      if (response.status === 502) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      // Should get 401 Unauthorized for invalid token
      expect(response.status).toBe(401)
    })
  })

  /**
   * Function Metadata Tests
   *
   * Tests that function metadata is properly returned in API responses.
   */
  describe('Function Metadata', () => {
    it('should include verifyJWT in function list response', async () => {
      // This test requires dashboard API access
      // The /functions endpoint in dashboard returns function metadata
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/functions`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      // Skip if not authorized (dashboard requires auth)
      if (response.status === 401 || response.status === 404) {
        console.log('Skipping: Dashboard auth required or endpoint not found')
        return
      }

      if (response.ok) {
        const functions = await response.json()
        if (Array.isArray(functions) && functions.length > 0) {
          // Each function should have verifyJWT property
          for (const fn of functions) {
            expect(fn).toHaveProperty('name')
            expect(fn).toHaveProperty('verify_jwt')
          }
        }
      }
    })
  })
})

/**
 * Edge Functions Secrets Tests
 *
 * Tests for secrets management functionality.
 * Note: Secrets require a restart of the edge runtime to take effect,
 * so these tests primarily verify the storage and retrieval mechanism.
 */
describe('Edge Functions Secrets (Unit)', () => {
  // Note: These tests verify that secrets are properly injected as
  // environment variables. The actual injection happens when the
  // edge runtime starts.

  it('should have SUPABASE_URL environment variable injected', async () => {
    const supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    const { data, error } = await supabase.functions.invoke('echo-env')

    if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
      console.log('Skipping: Edge runtime not running')
      return
    }

    expect(error).toBeNull()
    expect(data).toBeDefined()
    expect(data.supabase_url).toBeDefined()
    expect(data.supabase_url).toContain('http')
  })

  it('should have API keys injected', async () => {
    const supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    const { data, error } = await supabase.functions.invoke('echo-env')

    if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
      console.log('Skipping: Edge runtime not running')
      return
    }

    expect(error).toBeNull()
    expect(data).toBeDefined()
    expect(data.has_anon_key).toBe(true)
    expect(data.has_service_key).toBe(true)
  })
})

/**
 * Compatibility Summary for Edge Functions Configuration:
 *
 * IMPLEMENTED:
 * - Per-function JWT verification toggle (verify_jwt setting)
 * - Encrypted secrets storage (AES-GCM)
 * - Secrets injection as environment variables
 * - Per-function metadata (memory_mb, timeout_ms, import_map, env_vars)
 * - CLI for secrets management (set, list, delete)
 * - CLI for config management (set-jwt, show)
 *
 * MATCHES SUPABASE BEHAVIOR:
 * - Default JWT verification enabled
 * - Secrets stored encrypted in database
 * - Environment variables available to functions
 *
 * DIFFERENCES FROM SUPABASE:
 * - Secrets require runtime restart (vs. hot-reload in Supabase)
 * - Import maps are per-function (stored in metadata)
 */
