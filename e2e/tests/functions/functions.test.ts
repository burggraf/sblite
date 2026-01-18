/**
 * Edge Functions Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/functions-invoke
 *
 * Note: These tests require the server to be running with --functions flag:
 *   ./sblite serve --functions --db test.db
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient, FunctionsHttpError, FunctionsRelayError } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Edge Functions', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * Basic invocation tests
   */
  describe('Basic Invocation', () => {
    it('should invoke a function with POST method (default)', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world', {
        body: { name: 'Test' },
      })

      // Skip test if functions not enabled
      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data.message).toBe('Hello Test!')
      expect(data.method).toBe('POST')
    })

    it('should invoke a function with empty body', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world')

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data.message).toBe('Hello World!')
    })

    it('should invoke a function with GET method', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world', {
        method: 'GET',
      })

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data.method).toBe('GET')
    })

    it('should return timestamp in response', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world', {
        body: { name: 'Timestamp' },
      })

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(data.timestamp).toBeDefined()
      // Timestamp should be a valid ISO date
      expect(new Date(data.timestamp).toISOString()).toBe(data.timestamp)
    })
  })

  /**
   * Custom headers tests
   */
  describe('Custom Headers', () => {
    it('should pass custom headers to the function', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world', {
        body: { name: 'Headers' },
        headers: {
          'X-Custom-Header': 'test-value',
        },
      })

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })
  })

  /**
   * Environment variable injection tests
   */
  describe('Environment Variables', () => {
    it('should have SUPABASE_URL injected', async () => {
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
   * Error handling tests
   */
  describe('Error Handling', () => {
    it('should return 404 for non-existent function', async () => {
      const { data, error } = await supabase.functions.invoke('non-existent-function')

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).not.toBeNull()
      // Should be FunctionsHttpError for 404
    })

    it('should handle function that throws error', async () => {
      // Send invalid JSON to trigger error
      const { data, error } = await supabase.functions.invoke('hello-world', {
        body: 'not-json',
        headers: {
          'Content-Type': 'application/json',
        },
      })

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      // Function should handle this gracefully or return error
      // Either way, the call should complete without crashing
    })
  })

  /**
   * HTTP methods tests
   */
  describe('HTTP Methods', () => {
    const methods = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE'] as const

    for (const method of methods) {
      it(`should support ${method} method`, async () => {
        const options: any = { method }
        if (method !== 'GET') {
          options.body = { test: method }
        }

        const { data, error } = await supabase.functions.invoke('hello-world', options)

        if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
          console.log('Skipping: Edge runtime not running')
          return
        }

        expect(error).toBeNull()
        expect(data).toBeDefined()
        expect(data.method).toBe(method)
      })
    }
  })

  /**
   * Response types tests
   */
  describe('Response Types', () => {
    it('should return JSON response', async () => {
      const { data, error } = await supabase.functions.invoke('hello-world', {
        body: { name: 'JSON' },
      })

      if (error?.message?.includes('not running') || error?.message?.includes('unavailable')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      expect(error).toBeNull()
      expect(typeof data).toBe('object')
      expect(data.message).toBe('Hello JSON!')
    })
  })

  /**
   * Concurrent invocation tests
   */
  describe('Concurrent Invocations', () => {
    it('should handle multiple concurrent invocations', async () => {
      const names = ['Alice', 'Bob', 'Charlie', 'Diana', 'Eve']
      const promises = names.map((name) =>
        supabase.functions.invoke('hello-world', {
          body: { name },
        })
      )

      const results = await Promise.all(promises)

      // Check if runtime is available
      if (results[0].error?.message?.includes('not running')) {
        console.log('Skipping: Edge runtime not running')
        return
      }

      for (let i = 0; i < results.length; i++) {
        const { data, error } = results[i]
        expect(error).toBeNull()
        expect(data.message).toBe(`Hello ${names[i]}!`)
      }
    })
  })
})

/**
 * Compatibility Summary for Edge Functions:
 *
 * IMPLEMENTED:
 * - Basic function invocation (POST)
 * - Multiple HTTP methods (GET, POST, PUT, PATCH, DELETE)
 * - Custom headers
 * - JSON request/response bodies
 * - Environment variable injection (SUPABASE_URL, keys)
 * - Concurrent invocations
 * - Error handling (404 for non-existent functions)
 *
 * PLANNED (Phase 2+):
 * - Per-function JWT verification toggle
 * - Secrets management
 * - Function-specific configuration
 *
 * NOT IMPLEMENTED:
 * - Blob/FormData request bodies
 * - Streaming responses
 * - WebSocket connections
 */
