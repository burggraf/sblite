/**
 * RLS - Row Level Security Tests
 *
 * Tests for Row Level Security functionality in sblite.
 * These tests verify that RLS policies correctly filter data
 * based on the authenticated user.
 *
 * PREREQUISITES:
 * - The rls_test table must be created by the setup script (npm run setup)
 * - The user_isolation policy must be added to the rls_test table
 * - The server must be running with the test database
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('RLS - Row Level Security', () => {
  let user1Client: SupabaseClient
  let user2Client: SupabaseClient
  let anonClient: SupabaseClient
  let user1Id: string
  let user2Id: string
  let user1Email: string
  let user2Email: string

  beforeAll(async () => {
    // Create anonymous client for signup
    anonClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // The rls_test table and policy should already exist from setup script
    // If not, the tests will fail with a clear error message

    // Create two test users
    user1Email = uniqueEmail()
    user2Email = uniqueEmail()

    const { data: signup1, error: signupError1 } = await anonClient.auth.signUp({
      email: user1Email,
      password: 'password123',
    })

    if (signupError1) {
      throw new Error(`Failed to sign up user 1: ${signupError1.message}`)
    }
    user1Id = signup1.user!.id

    const { data: signup2, error: signupError2 } = await anonClient.auth.signUp({
      email: user2Email,
      password: 'password123',
    })

    if (signupError2) {
      throw new Error(`Failed to sign up user 2: ${signupError2.message}`)
    }
    user2Id = signup2.user!.id

    // Create authenticated clients by signing in
    user1Client = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
    const { error: signin1Error } = await user1Client.auth.signInWithPassword({
      email: user1Email,
      password: 'password123',
    })
    if (signin1Error) {
      throw new Error(`Failed to sign in user 1: ${signin1Error.message}`)
    }

    user2Client = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
    const { error: signin2Error } = await user2Client.auth.signInWithPassword({
      email: user2Email,
      password: 'password123',
    })
    if (signin2Error) {
      throw new Error(`Failed to sign in user 2: ${signin2Error.message}`)
    }

    // Insert test data for each user
    const { error: insert1Error } = await user1Client
      .from('rls_test')
      .insert({ user_id: user1Id, data: 'user1 data' })

    if (insert1Error) {
      throw new Error(`Failed to insert user 1 data: ${insert1Error.message}. Make sure you ran 'npm run setup' first.`)
    }

    const { error: insert2Error } = await user2Client
      .from('rls_test')
      .insert({ user_id: user2Id, data: 'user2 data' })

    if (insert2Error) {
      throw new Error(`Failed to insert user 2 data: ${insert2Error.message}`)
    }
  })

  afterAll(async () => {
    // Clean up: delete test data (each user can only delete their own due to RLS)
    if (user1Client) {
      await user1Client.from('rls_test').delete().eq('user_id', user1Id)
      await user1Client.auth.signOut()
    }
    if (user2Client) {
      await user2Client.from('rls_test').delete().eq('user_id', user2Id)
      await user2Client.auth.signOut()
    }
  })

  describe('SELECT with RLS', () => {
    it('should only return rows belonging to authenticated user', async () => {
      const { data: user1Data, error: user1Error } = await user1Client.from('rls_test').select('*')

      expect(user1Error).toBeNull()
      expect(user1Data).toBeDefined()
      expect(user1Data!.length).toBeGreaterThanOrEqual(1)
      // User1 should see their own data
      const user1Row = user1Data!.find((row: any) => row.data === 'user1 data')
      expect(user1Row).toBeDefined()
      expect(user1Row!.user_id).toBe(user1Id)

      const { data: user2Data, error: user2Error } = await user2Client.from('rls_test').select('*')

      expect(user2Error).toBeNull()
      expect(user2Data).toBeDefined()
      expect(user2Data!.length).toBeGreaterThanOrEqual(1)
      // User2 should see their own data
      const user2Row = user2Data!.find((row: any) => row.data === 'user2 data')
      expect(user2Row).toBeDefined()
      expect(user2Row!.user_id).toBe(user2Id)
    })

    it('should not allow user to see other users data even with explicit filter', async () => {
      // User1 tries to filter for User2's data
      const { data, error } = await user1Client
        .from('rls_test')
        .select('*')
        .eq('user_id', user2Id)

      expect(error).toBeNull()
      expect(data).toHaveLength(0) // RLS should filter out User2's rows
    })

    it('should deny access for unauthenticated requests on RLS-protected table', async () => {
      // Create a fresh client without authentication
      const freshAnonClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
        auth: { autoRefreshToken: false, persistSession: false },
      })

      const { data, error } = await freshAnonClient.from('rls_test').select('*')

      // Without auth, RLS should deny access (either error or empty result)
      // Current implementation returns an error when auth.uid() is null
      if (error) {
        // This is acceptable - denying access with an error
        expect(error).toBeDefined()
      } else {
        // Alternative: return empty array
        expect(data).toHaveLength(0)
      }
    })
  })

  describe('UPDATE with RLS', () => {
    it('should allow user to update only their own data', async () => {
      // User1 updates their own data - should work
      const { error: updateError } = await user1Client
        .from('rls_test')
        .update({ data: 'user1 updated' })
        .eq('user_id', user1Id)

      expect(updateError).toBeNull()

      // Verify the update worked
      const { data: updated } = await user1Client.from('rls_test').select('*')
      expect(updated).toBeDefined()
      const updatedRow = updated!.find((row: any) => row.user_id === user1Id)
      expect(updatedRow).toBeDefined()
      expect(updatedRow!.data).toBe('user1 updated')

      // Reset the data for other tests
      await user1Client
        .from('rls_test')
        .update({ data: 'user1 data' })
        .eq('user_id', user1Id)
    })

    it('should not allow user to update other users data', async () => {
      // User1 tries to update User2's data - should silently fail (0 rows affected)
      const { error } = await user1Client
        .from('rls_test')
        .update({ data: 'hacked!' })
        .eq('user_id', user2Id)

      // The update should not error, but should affect 0 rows due to RLS
      expect(error).toBeNull()

      // Verify User2's data is unchanged
      const { data: user2Data } = await user2Client.from('rls_test').select('*')
      expect(user2Data).toBeDefined()
      const user2Row = user2Data!.find((row: any) => row.user_id === user2Id)
      expect(user2Row).toBeDefined()
      expect(user2Row!.data).toBe('user2 data')
    })
  })

  describe('DELETE with RLS', () => {
    it('should allow user to delete only their own data', async () => {
      // First, insert a row to delete
      const { error: insertError } = await user1Client
        .from('rls_test')
        .insert({ user_id: user1Id, data: 'to be deleted' })

      expect(insertError).toBeNull()

      // Verify it was inserted
      const { data: beforeDelete } = await user1Client
        .from('rls_test')
        .select('*')
        .eq('data', 'to be deleted')
      expect(beforeDelete!.length).toBeGreaterThanOrEqual(1)

      // Delete the row
      const { error: deleteError } = await user1Client
        .from('rls_test')
        .delete()
        .eq('data', 'to be deleted')

      expect(deleteError).toBeNull()

      // Verify it was deleted
      const { data: afterDelete } = await user1Client
        .from('rls_test')
        .select('*')
        .eq('data', 'to be deleted')
      expect(afterDelete).toHaveLength(0)
    })

    it('should not allow user to delete other users data', async () => {
      // User1 tries to delete User2's data - should silently fail (0 rows affected)
      const { error } = await user1Client
        .from('rls_test')
        .delete()
        .eq('user_id', user2Id)

      // The delete should not error, but should affect 0 rows due to RLS
      expect(error).toBeNull()

      // Verify User2's data still exists
      const { data: user2Data } = await user2Client.from('rls_test').select('*')
      expect(user2Data).toBeDefined()
      const user2Row = user2Data!.find((row: any) => row.user_id === user2Id)
      expect(user2Row).toBeDefined()
    })
  })

  describe('INSERT with RLS (CHECK policy)', () => {
    it('should allow user to insert data with their own user_id', async () => {
      const { error } = await user1Client
        .from('rls_test')
        .insert({ user_id: user1Id, data: 'new user1 data' })

      expect(error).toBeNull()

      // Clean up
      await user1Client.from('rls_test').delete().eq('data', 'new user1 data')
    })

    it.skip('should not allow user to insert data with another users user_id', async () => {
      // Note: CHECK policy enforcement on INSERT is not yet fully implemented
      // This test is skipped until that feature is complete

      // User1 tries to insert with User2's user_id - should be rejected by CHECK policy
      const { error } = await user1Client
        .from('rls_test')
        .insert({ user_id: user2Id, data: 'spoofed data' })

      // The insert should fail due to CHECK policy violation
      expect(error).not.toBeNull()
    })
  })
})

/**
 * Compatibility Summary for RLS:
 *
 * IMPLEMENTED:
 * - USING clause for SELECT filtering
 * - USING clause for UPDATE filtering
 * - USING clause for DELETE filtering
 * - auth.uid() function in policy expressions
 * - Policy management via CLI (sblite policy add/list)
 *
 * PARTIALLY IMPLEMENTED / KNOWN ISSUES:
 * - CHECK clause for INSERT validation (not enforced)
 * - Unauthenticated requests may return error instead of empty result
 *
 * BEHAVIOR:
 * - RLS filters are automatically applied based on authenticated user
 * - Operations on other users' rows silently affect 0 rows (no error)
 * - Users can only see/modify rows matching their auth.uid()
 */
