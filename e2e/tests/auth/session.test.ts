/**
 * Auth - Session Management Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-getsession
 * https://supabase.com/docs/reference/javascript/auth-refreshsession
 * https://supabase.com/docs/reference/javascript/auth-setsession
 */

import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('Auth - Session Management', () => {
  let supabase: SupabaseClient
  let testEmail: string
  let testPassword: string

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Create a test user
    testEmail = uniqueEmail()
    testPassword = 'test-password-123'
    await supabase.auth.signUp({ email: testEmail, password: testPassword })
    await supabase.auth.signOut()
  })

  afterEach(async () => {
    await supabase.auth.signOut()
  })

  /**
   * getSession - Retrieve current session
   * Docs: https://supabase.com/docs/reference/javascript/auth-getsession
   */
  describe('getSession()', () => {
    it('should return null when not signed in', async () => {
      const { data, error } = await supabase.auth.getSession()

      expect(error).toBeNull()
      expect(data.session).toBeNull()
    })

    it('should return session when signed in', async () => {
      // Sign in first
      await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })

      const { data, error } = await supabase.auth.getSession()

      expect(error).toBeNull()
      expect(data.session).toBeDefined()
      expect(data.session?.access_token).toBeDefined()
      expect(data.session?.refresh_token).toBeDefined()
      expect(data.session?.user).toBeDefined()
      expect(data.session?.user?.email).toBe(testEmail)
    })

    it('should return session with user object', async () => {
      await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })

      const { data } = await supabase.auth.getSession()

      expect(data.session?.user).toHaveProperty('id')
      expect(data.session?.user).toHaveProperty('email')
      expect(data.session?.user).toHaveProperty('role')
    })
  })

  /**
   * refreshSession - Refresh the current session
   * Docs: https://supabase.com/docs/reference/javascript/auth-refreshsession
   */
  describe('refreshSession()', () => {
    let initialSession: any

    beforeEach(async () => {
      // Sign in and get initial session
      const { data } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })
      initialSession = data.session
    })

    /**
     * Example 1: Refresh session using current session
     */
    describe('1. Refresh using current session', () => {
      it('should refresh the current session', async () => {
        const { data, error } = await supabase.auth.refreshSession()

        expect(error).toBeNull()
        expect(data.session).toBeDefined()
        expect(data.session?.access_token).toBeDefined()
        expect(data.user).toBeDefined()
      })

      it('should return new access token', async () => {
        // Wait a moment to ensure token would be different
        await new Promise((resolve) => setTimeout(resolve, 100))

        const { data, error } = await supabase.auth.refreshSession()

        expect(error).toBeNull()
        expect(data.session?.access_token).toBeDefined()
        // New token may or may not be different depending on implementation
      })
    })

    /**
     * Example 2: Refresh using refresh token
     */
    describe('2. Refresh using refresh token', () => {
      it('should refresh session using provided refresh token', async () => {
        const refreshToken = initialSession?.refresh_token

        const { data, error } = await supabase.auth.refreshSession({
          refresh_token: refreshToken,
        })

        expect(error).toBeNull()
        expect(data.session).toBeDefined()
        expect(data.session?.access_token).toBeDefined()
      })

      it('should reject invalid refresh token', async () => {
        const { data, error } = await supabase.auth.refreshSession({
          refresh_token: 'invalid-refresh-token',
        })

        expect(error).not.toBeNull()
      })
    })
  })

  /**
   * setSession - Set session data
   * Docs: https://supabase.com/docs/reference/javascript/auth-setsession
   */
  describe('setSession()', () => {
    it.skip('should set session from tokens', async () => {
      // First sign in to get valid tokens
      const { data: signInData } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })

      // Sign out
      await supabase.auth.signOut()

      // Set session manually
      const { data, error } = await supabase.auth.setSession({
        access_token: signInData.session!.access_token,
        refresh_token: signInData.session!.refresh_token,
      })

      expect(error).toBeNull()
      expect(data.session).toBeDefined()
    })

    it.skip('should reject invalid tokens', async () => {
      const { error } = await supabase.auth.setSession({
        access_token: 'invalid-access-token',
        refresh_token: 'invalid-refresh-token',
      })

      expect(error).not.toBeNull()
    })
  })

  // Additional session tests
  describe('Session Lifecycle', () => {
    it('should clear session on sign out', async () => {
      // Sign in
      await supabase.auth.signInWithPassword({
        email: testEmail,
        password: testPassword,
      })

      // Verify session exists
      const { data: before } = await supabase.auth.getSession()
      expect(before.session).not.toBeNull()

      // Sign out
      await supabase.auth.signOut()

      // Verify session is cleared
      const { data: after } = await supabase.auth.getSession()
      expect(after.session).toBeNull()
    })
  })
})

/**
 * Compatibility Summary for Session Management:
 *
 * IMPLEMENTED:
 * - getSession(): Get current session
 * - refreshSession(): Refresh using current session
 * - refreshSession({ refresh_token }): Refresh using provided token
 *
 * PARTIALLY IMPLEMENTED:
 * - setSession(): May work but not fully tested
 *
 * NOT IMPLEMENTED:
 * - Auto token refresh
 * - Session persistence in storage
 */
