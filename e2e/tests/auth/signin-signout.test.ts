/**
 * Auth - Sign In / Sign Out Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-signinwithpassword
 * https://supabase.com/docs/reference/javascript/auth-signout
 */

import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('Auth - Sign In / Sign Out', () => {
  let supabase: SupabaseClient
  let testEmail: string
  let testPassword: string

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Create a test user for sign in tests
    testEmail = uniqueEmail()
    testPassword = 'test-password-123'
    await supabase.auth.signUp({ email: testEmail, password: testPassword })
    await supabase.auth.signOut()
  })

  afterEach(async () => {
    await supabase.auth.signOut()
  })

  /**
   * signInWithPassword - Sign in with email
   * Docs: https://supabase.com/docs/reference/javascript/auth-signinwithpassword
   */
  describe('signInWithPassword()', () => {
    /**
     * Example 1: Sign in with email and password
     */
    describe('1. Sign in with email and password', () => {
      it('should authenticate user with valid credentials', async () => {
        const { data, error } = await supabase.auth.signInWithPassword({
          email: testEmail,
          password: testPassword,
        })

        expect(error).toBeNull()
        expect(data).toBeDefined()
        expect(data.user).toBeDefined()
        expect(data.user?.email).toBe(testEmail)
        expect(data.session).toBeDefined()
        expect(data.session?.access_token).toBeDefined()
      })

      it('should return session with access token and refresh token', async () => {
        const { data, error } = await supabase.auth.signInWithPassword({
          email: testEmail,
          password: testPassword,
        })

        expect(error).toBeNull()
        expect(data.session?.access_token).toBeDefined()
        expect(data.session?.refresh_token).toBeDefined()
        expect(data.session?.token_type).toBe('bearer')
        expect(data.session?.expires_in).toBeGreaterThan(0)
      })

      it('should reject invalid password', async () => {
        const { data, error } = await supabase.auth.signInWithPassword({
          email: testEmail,
          password: 'wrong-password',
        })

        expect(error).not.toBeNull()
        expect(data.user).toBeNull()
        expect(data.session).toBeNull()
      })

      it('should reject non-existent user', async () => {
        const { data, error } = await supabase.auth.signInWithPassword({
          email: 'nonexistent@example.com',
          password: testPassword,
        })

        expect(error).not.toBeNull()
      })
    })

    /**
     * Example 2: Sign in with phone and password
     * Not implemented in sblite Phase 1
     */
    describe('2. Sign in with phone and password', () => {
      it.skip('should authenticate with phone number', async () => {
        const { data, error } = await supabase.auth.signInWithPassword({
          phone: '+13334445555',
          password: 'some-password',
        })

        expect(error).toBeNull()
      })
    })
  })

  /**
   * signOut - Sign out
   * Docs: https://supabase.com/docs/reference/javascript/auth-signout
   */
  describe('signOut()', () => {
    beforeEach(async () => {
      // Sign in before each signout test
      await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })
    })

    /**
     * Example 1: Sign out (all sessions)
     */
    describe('1. Sign out all sessions', () => {
      it('should sign out the current user', async () => {
        const { error } = await supabase.auth.signOut()

        expect(error).toBeNull()

        // Verify signed out
        const { data: sessionData } = await supabase.auth.getSession()
        expect(sessionData.session).toBeNull()
      })
    })

    /**
     * Example 2: Sign out (current session only)
     */
    describe('2. Sign out current session', () => {
      it.skip('should sign out only the current session', async () => {
        // Requires scope: 'local' support
        const { error } = await supabase.auth.signOut({ scope: 'local' })

        expect(error).toBeNull()
      })
    })

    /**
     * Example 3: Sign out (other sessions)
     */
    describe('3. Sign out other sessions', () => {
      it.skip('should sign out all other sessions except current', async () => {
        // Requires scope: 'others' support
        const { error } = await supabase.auth.signOut({ scope: 'others' })

        expect(error).toBeNull()
      })
    })
  })

  // Additional tests
  describe('Additional Sign In Tests', () => {
    it('should handle empty email', async () => {
      const { error } = await supabase.auth.signInWithPassword({
        email: '',
        password: testPassword,
      })

      expect(error).not.toBeNull()
    })

    it('should handle empty password', async () => {
      const { error } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: '',
      })

      expect(error).not.toBeNull()
    })

    it('should allow multiple sign ins (refresh session)', async () => {
      // First sign in
      const { data: first } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })

      // Second sign in should work
      const { data: second, error } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })

      expect(error).toBeNull()
      expect(second.session?.access_token).toBeDefined()
    })
  })
})

/**
 * Compatibility Summary for Sign In / Sign Out:
 *
 * IMPLEMENTED:
 * - signInWithPassword with email
 * - signOut (all sessions)
 * - Session returned with access_token, refresh_token, expires_in
 *
 * NOT IMPLEMENTED:
 * - signInWithPassword with phone
 * - signOut with scope: 'local'
 * - signOut with scope: 'others'
 */
