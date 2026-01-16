/**
 * API Key Validation Tests
 *
 * Tests for Supabase-compatible API key validation in sblite.
 * Validates that:
 * - Requests without apikey header are rejected
 * - Requests with invalid apikey are rejected
 * - Valid anon and service_role keys are accepted
 * - service_role bypasses RLS while anon respects it
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { createServiceRoleClient, uniqueEmail } from '../../setup/test-helpers'

describe('API Key Validation', () => {
  describe('Request without API key', () => {
    it('should reject REST requests without apikey header', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters`)

      expect(response.status).toBe(401)
      const body = await response.json()
      expect(body.error).toBe('no_api_key')
      expect(body.message).toBe('API key is required')
    })

    it('should reject POST requests without apikey header', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'Test', homeworld: 'Test' }),
      })

      expect(response.status).toBe(401)
      const body = await response.json()
      expect(body.error).toBe('no_api_key')
    })
  })

  describe('Request with invalid API key', () => {
    it('should reject requests with malformed apikey', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters`, {
        headers: { 'apikey': 'invalid-not-a-jwt' },
      })

      expect(response.status).toBe(401)
      const body = await response.json()
      expect(body.error).toBe('invalid_api_key')
    })

    it('should reject requests with JWT signed by wrong secret', async () => {
      // This is a valid JWT structure but signed with a different secret
      const wrongSecretJwt = 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYW5vbiIsImlzcyI6InNibGl0ZSIsImlhdCI6MTcwMDAwMDAwMH0.wrong_signature_here'

      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters`, {
        headers: { 'apikey': wrongSecretJwt },
      })

      expect(response.status).toBe(401)
      const body = await response.json()
      expect(body.error).toBe('invalid_api_key')
    })
  })

  describe('Request with valid anon key', () => {
    let supabase: SupabaseClient

    beforeAll(() => {
      supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
        auth: { autoRefreshToken: false, persistSession: false },
      })
    })

    it('should accept requests with valid anon key', async () => {
      const { data, error } = await supabase.from('characters').select('*')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(true)
    })

    it('should allow CRUD operations with anon key', async () => {
      // Insert
      const { data: inserted, error: insertError } = await supabase
        .from('countries')
        .insert({ id: 999, name: 'Test Country', code: 'TC' })
        .select()

      expect(insertError).toBeNull()

      // Update
      const { error: updateError } = await supabase
        .from('countries')
        .update({ name: 'Updated Country' })
        .eq('id', 999)

      expect(updateError).toBeNull()

      // Delete
      const { error: deleteError } = await supabase
        .from('countries')
        .delete()
        .eq('id', 999)

      expect(deleteError).toBeNull()
    })
  })

  describe('Request with valid service_role key', () => {
    let serviceClient: SupabaseClient

    beforeAll(() => {
      serviceClient = createServiceRoleClient()
    })

    it('should accept requests with valid service_role key', async () => {
      const { data, error } = await serviceClient.from('characters').select('*')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(true)
    })

    it('should allow CRUD operations with service_role key', async () => {
      // Insert
      const { error: insertError } = await serviceClient
        .from('countries')
        .insert({ id: 998, name: 'Service Country', code: 'SC' })

      expect(insertError).toBeNull()

      // Delete
      const { error: deleteError } = await serviceClient
        .from('countries')
        .delete()
        .eq('id', 998)

      expect(deleteError).toBeNull()
    })
  })

  describe('service_role bypasses RLS', () => {
    let anonClient: SupabaseClient
    let serviceClient: SupabaseClient
    let userClient: SupabaseClient
    let testUserId: string
    let testEmail: string

    beforeAll(async () => {
      anonClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
        auth: { autoRefreshToken: false, persistSession: false },
      })
      serviceClient = createServiceRoleClient()

      // Create a test user and insert data into rls_test table
      testEmail = uniqueEmail()
      const { data: signup, error: signupError } = await anonClient.auth.signUp({
        email: testEmail,
        password: 'password123',
      })

      if (signupError) {
        throw new Error(`Failed to sign up test user: ${signupError.message}`)
      }
      testUserId = signup.user!.id

      // Sign in as the user
      userClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
        auth: { autoRefreshToken: false, persistSession: false },
      })
      await userClient.auth.signInWithPassword({
        email: testEmail,
        password: 'password123',
      })

      // Insert test data using authenticated user
      const { error: insertError } = await userClient
        .from('rls_test')
        .insert({ user_id: testUserId, data: 'apikey test data' })

      if (insertError) {
        throw new Error(`Failed to insert test data: ${insertError.message}. Make sure you ran 'npm run setup' first.`)
      }
    })

    afterAll(async () => {
      // Clean up using service_role (bypasses RLS)
      if (serviceClient) {
        await serviceClient.from('rls_test').delete().eq('data', 'apikey test data')
      }
      if (userClient) {
        await userClient.auth.signOut()
      }
    })

    it('service_role should see all rows in RLS-protected table', async () => {
      // Service role can see all data regardless of RLS
      const { data, error } = await serviceClient.from('rls_test').select('*')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should see at least the row we inserted
      const testRow = data!.find((row: any) => row.data === 'apikey test data')
      expect(testRow).toBeDefined()
    })

    it('anon (unauthenticated) should not see rows in RLS-protected table', async () => {
      // Fresh anon client without user auth
      const freshAnonClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
        auth: { autoRefreshToken: false, persistSession: false },
      })

      const { data, error } = await freshAnonClient.from('rls_test').select('*')

      // RLS should filter out all rows for unauthenticated requests
      // Either returns empty array or error
      if (error) {
        expect(error).toBeDefined()
      } else {
        expect(data).toHaveLength(0)
      }
    })

    it('service_role should be able to delete any row regardless of RLS', async () => {
      // Insert a row as user
      const { error: insertError } = await userClient
        .from('rls_test')
        .insert({ user_id: testUserId, data: 'to delete via service' })

      expect(insertError).toBeNull()

      // Delete using service_role (should bypass RLS)
      const { error: deleteError } = await serviceClient
        .from('rls_test')
        .delete()
        .eq('data', 'to delete via service')

      expect(deleteError).toBeNull()

      // Verify deletion
      const { data } = await serviceClient
        .from('rls_test')
        .select('*')
        .eq('data', 'to delete via service')

      expect(data).toHaveLength(0)
    })

    it('service_role should be able to update any row regardless of RLS', async () => {
      // Insert a row as user
      const { error: insertError } = await userClient
        .from('rls_test')
        .insert({ user_id: testUserId, data: 'original data' })

      expect(insertError).toBeNull()

      // Update using service_role (should bypass RLS)
      const { error: updateError } = await serviceClient
        .from('rls_test')
        .update({ data: 'updated by service' })
        .eq('data', 'original data')

      expect(updateError).toBeNull()

      // Verify update
      const { data } = await serviceClient
        .from('rls_test')
        .select('*')
        .eq('data', 'updated by service')

      expect(data!.length).toBeGreaterThanOrEqual(1)

      // Clean up
      await serviceClient.from('rls_test').delete().eq('data', 'updated by service')
    })
  })
})
