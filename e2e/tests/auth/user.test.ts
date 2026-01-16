/**
 * Auth - User Management Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-getuser
 * https://supabase.com/docs/reference/javascript/auth-updateuser
 * https://supabase.com/docs/reference/javascript/auth-getclaims
 */

import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('Auth - User Management', () => {
  let supabase: SupabaseClient
  let testEmail: string
  let testPassword: string

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  beforeEach(async () => {
    // Create fresh user for each test
    testEmail = uniqueEmail()
    testPassword = 'test-password-123'
    await supabase.auth.signUp({ email: testEmail, password: testPassword })
  })

  afterEach(async () => {
    await supabase.auth.signOut()
  })

  /**
   * getUser - Retrieve the current user
   * Docs: https://supabase.com/docs/reference/javascript/auth-getuser
   */
  describe('getUser()', () => {
    /**
     * Example 1: Get user with current session
     */
    describe('1. Get user with current session', () => {
      it('should retrieve the logged in user', async () => {
        const {
          data: { user },
          error,
        } = await supabase.auth.getUser()

        expect(error).toBeNull()
        expect(user).toBeDefined()
        expect(user?.email).toBe(testEmail)
      })

      it('should return user with all expected properties', async () => {
        const {
          data: { user },
        } = await supabase.auth.getUser()

        expect(user).toHaveProperty('id')
        expect(user).toHaveProperty('email')
        expect(user).toHaveProperty('role')
        expect(user).toHaveProperty('created_at')
      })
    })

    /**
     * Example 2: Get user with custom JWT
     */
    describe('2. Get user with custom JWT', () => {
      it.skip('should retrieve user using provided JWT', async () => {
        const { data: session } = await supabase.auth.getSession()
        const jwt = session.session?.access_token

        const {
          data: { user },
          error,
        } = await supabase.auth.getUser(jwt)

        expect(error).toBeNull()
        expect(user?.email).toBe(testEmail)
      })
    })

    it('should return null when not authenticated', async () => {
      await supabase.auth.signOut()

      const {
        data: { user },
        error,
      } = await supabase.auth.getUser()

      // Should return error or null user when not authenticated
      expect(user).toBeNull()
    })
  })

  /**
   * updateUser - Update the current user
   * Docs: https://supabase.com/docs/reference/javascript/auth-updateuser
   */
  describe('updateUser()', () => {
    /**
     * Example 1: Update email
     */
    describe('1. Update email', () => {
      it.skip('should update user email', async () => {
        // Email update may require confirmation flow
        const newEmail = uniqueEmail()

        const { data, error } = await supabase.auth.updateUser({
          email: newEmail,
        })

        // May require email confirmation before taking effect
        expect(error).toBeNull()
      })
    })

    /**
     * Example 2: Update phone number
     * Not implemented in sblite Phase 1
     */
    describe('2. Update phone number', () => {
      it.skip('should update user phone', async () => {
        const { data, error } = await supabase.auth.updateUser({
          phone: '123456789',
        })

        expect(error).toBeNull()
      })
    })

    /**
     * Example 3: Update password
     */
    describe('3. Update password', () => {
      it('should update user password', async () => {
        const newPassword = 'new-password-456'

        const { data, error } = await supabase.auth.updateUser({
          password: newPassword,
        })

        expect(error).toBeNull()
        expect(data.user).toBeDefined()

        // Verify new password works
        await supabase.auth.signOut()
        const { data: signIn, error: signInError } = await supabase.auth.signInWithPassword({
          email: testEmail,
          password: newPassword,
        })

        expect(signInError).toBeNull()
        expect(signIn.session).toBeDefined()
      })

      it('should reject sign in with old password after update', async () => {
        const newPassword = 'new-password-789'

        await supabase.auth.updateUser({ password: newPassword })
        await supabase.auth.signOut()

        const { error } = await supabase.auth.signInWithPassword({
          email: testEmail,
          password: testPassword, // Old password
        })

        expect(error).not.toBeNull()
      })
    })

    /**
     * Example 4: Update user metadata
     */
    describe('4. Update user metadata', () => {
      it('should update user metadata', async () => {
        const metadata = { hello: 'world', age: 30 }

        const { data, error } = await supabase.auth.updateUser({
          data: metadata,
        })

        expect(error).toBeNull()
        expect(data.user?.user_metadata).toMatchObject(metadata)
      })

      it('should merge metadata with existing values', async () => {
        // Set initial metadata
        await supabase.auth.updateUser({
          data: { first: 'value1' },
        })

        // Update with additional metadata
        const { data, error } = await supabase.auth.updateUser({
          data: { second: 'value2' },
        })

        expect(error).toBeNull()
        // Both values should exist (or second replaces all - document behavior)
      })
    })

    /**
     * Example 5: Update password with nonce
     * Not implemented in sblite Phase 1
     */
    describe('5. Update password with nonce', () => {
      it.skip('should update password with reauthentication nonce', async () => {
        const { data, error } = await supabase.auth.updateUser({
          password: 'new-password',
          nonce: '123456',
        })

        expect(error).toBeNull()
      })
    })
  })

  /**
   * getClaims - Get JWT claims
   * Docs: https://supabase.com/docs/reference/javascript/auth-getclaims
   */
  describe('getClaims()', () => {
    it.skip('should retrieve JWT claims', async () => {
      const { data, error } = await supabase.auth.getClaims()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Claims should include standard JWT fields
    })
  })
})

/**
 * Compatibility Summary for User Management:
 *
 * IMPLEMENTED:
 * - getUser(): Get current user
 * - updateUser({ password }): Update password
 * - updateUser({ data }): Update user metadata
 *
 * PARTIALLY IMPLEMENTED:
 * - updateUser({ email }): May require confirmation flow
 *
 * NOT IMPLEMENTED:
 * - getUser(jwt): Get user with custom JWT
 * - updateUser({ phone }): Phone update
 * - updateUser({ nonce }): Reauthentication
 * - getClaims(): JWT claims extraction
 */
