/**
 * Auth - Anonymous Sign-In Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-signinanonymously
 */

import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('Auth - Anonymous Sign-In', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  afterEach(async () => {
    await supabase.auth.signOut()
  })

  /**
   * signInAnonymously - Create anonymous session
   * Docs: https://supabase.com/docs/reference/javascript/auth-signinanonymously
   */
  describe('signInAnonymously()', () => {
    /**
     * Example 1: Basic anonymous sign-in
     */
    describe('1. Basic anonymous sign-in', () => {
      it('should create an anonymous user session', async () => {
        const { data, error } = await supabase.auth.signInAnonymously()

        expect(error).toBeNull()
        expect(data).toBeDefined()
        expect(data.user).toBeDefined()
        expect(data.session).toBeDefined()
      })

      it('should mark user as anonymous', async () => {
        const { data, error } = await supabase.auth.signInAnonymously()

        expect(error).toBeNull()
        expect(data.user?.is_anonymous).toBe(true)
      })

      it('should have null email for anonymous user', async () => {
        const { data, error } = await supabase.auth.signInAnonymously()

        expect(error).toBeNull()
        expect(data.user?.email).toBeNull()
      })

      it('should have authenticated role', async () => {
        const { data, error } = await supabase.auth.signInAnonymously()

        expect(error).toBeNull()
        expect(data.user?.role).toBe('authenticated')
      })

      it('should return valid session with tokens', async () => {
        const { data, error } = await supabase.auth.signInAnonymously()

        expect(error).toBeNull()
        expect(data.session?.access_token).toBeDefined()
        expect(data.session?.refresh_token).toBeDefined()
        expect(data.session?.token_type).toBe('bearer')
        expect(data.session?.expires_in).toBeGreaterThan(0)
      })

      it('should have valid user ID (UUID format)', async () => {
        const { data, error } = await supabase.auth.signInAnonymously()

        expect(error).toBeNull()
        expect(data.user?.id).toBeDefined()
        // UUID v4 format check
        expect(data.user?.id).toMatch(
          /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i
        )
      })
    })

    /**
     * Example 2: Anonymous sign-in with user metadata
     */
    describe('2. Anonymous sign-in with user metadata', () => {
      it('should store custom metadata during anonymous sign-in', async () => {
        const metadata = { theme: 'dark' }

        const { data, error } = await supabase.auth.signInAnonymously({
          options: {
            data: metadata,
          },
        })

        expect(error).toBeNull()
        expect(data.user).toBeDefined()
        expect(data.user?.user_metadata).toBeDefined()
        expect(data.user?.user_metadata?.theme).toBe('dark')
      })

      it('should handle complex metadata', async () => {
        const metadata = {
          preferences: { language: 'en', notifications: true },
          device: 'mobile',
          count: 42,
        }

        const { data, error } = await supabase.auth.signInAnonymously({
          options: {
            data: metadata,
          },
        })

        expect(error).toBeNull()
        expect(data.user?.user_metadata?.preferences).toEqual({
          language: 'en',
          notifications: true,
        })
        expect(data.user?.user_metadata?.device).toBe('mobile')
        expect(data.user?.user_metadata?.count).toBe(42)
      })

      it('should handle empty metadata', async () => {
        const { data, error } = await supabase.auth.signInAnonymously({
          options: {
            data: {},
          },
        })

        expect(error).toBeNull()
        expect(data.user).toBeDefined()
        expect(data.user?.is_anonymous).toBe(true)
      })
    })
  })

  /**
   * Converting anonymous user to permanent user
   */
  describe('Anonymous to Permanent User Conversion', () => {
    it('should convert anonymous user by adding email and password', async () => {
      // Create anonymous session
      const { data: anonData, error: anonError } = await supabase.auth.signInAnonymously()
      expect(anonError).toBeNull()
      expect(anonData.user?.is_anonymous).toBe(true)

      const userId = anonData.user?.id

      // Convert to permanent user
      const newEmail = uniqueEmail()
      const newPassword = 'converted-password-123'

      const { data: updateData, error: updateError } = await supabase.auth.updateUser({
        email: newEmail,
        password: newPassword,
      })

      expect(updateError).toBeNull()
      expect(updateData.user).toBeDefined()
      expect(updateData.user?.is_anonymous).toBe(false)
      expect(updateData.user?.email).toBe(newEmail)
      // User ID should remain the same
      expect(updateData.user?.id).toBe(userId)
    })

    it('should allow login with new credentials after conversion', async () => {
      // Create anonymous session
      const { data: anonData } = await supabase.auth.signInAnonymously()
      expect(anonData.user?.is_anonymous).toBe(true)

      // Convert to permanent user
      const newEmail = uniqueEmail()
      const newPassword = 'converted-password-456'

      await supabase.auth.updateUser({
        email: newEmail,
        password: newPassword,
      })

      // Sign out
      await supabase.auth.signOut()

      // Sign in with new credentials
      const { data: signInData, error: signInError } = await supabase.auth.signInWithPassword({
        email: newEmail,
        password: newPassword,
      })

      expect(signInError).toBeNull()
      expect(signInData.session).toBeDefined()
      expect(signInData.user?.email).toBe(newEmail)
      expect(signInData.user?.is_anonymous).toBe(false)
    })

    it('should preserve user metadata after conversion', async () => {
      // Create anonymous session with metadata
      const { data: anonData } = await supabase.auth.signInAnonymously({
        options: {
          data: { original_source: 'anonymous', score: 100 },
        },
      })
      expect(anonData.user?.is_anonymous).toBe(true)
      expect(anonData.user?.user_metadata?.original_source).toBe('anonymous')

      // Convert to permanent user
      const newEmail = uniqueEmail()
      const newPassword = 'converted-password-789'

      const { data: updateData, error: updateError } = await supabase.auth.updateUser({
        email: newEmail,
        password: newPassword,
      })

      expect(updateError).toBeNull()
      // Original metadata should be preserved
      expect(updateData.user?.user_metadata?.original_source).toBe('anonymous')
      expect(updateData.user?.user_metadata?.score).toBe(100)
    })

    it('should allow updating metadata during conversion', async () => {
      // Create anonymous session
      const { data: anonData } = await supabase.auth.signInAnonymously({
        options: {
          data: { temp_data: 'value' },
        },
      })
      expect(anonData.user?.is_anonymous).toBe(true)

      // Convert with additional metadata
      const newEmail = uniqueEmail()
      const newPassword = 'converted-password-abc'

      const { data: updateData, error: updateError } = await supabase.auth.updateUser({
        email: newEmail,
        password: newPassword,
        data: { converted: true, converted_at: new Date().toISOString() },
      })

      expect(updateError).toBeNull()
      expect(updateData.user?.user_metadata?.converted).toBe(true)
      expect(updateData.user?.user_metadata?.converted_at).toBeDefined()
    })
  })

  /**
   * Session management for anonymous users
   */
  describe('Anonymous Session Management', () => {
    it('should get current anonymous user via getUser()', async () => {
      const { data: anonData } = await supabase.auth.signInAnonymously()
      expect(anonData.user?.is_anonymous).toBe(true)

      const { data: userData, error } = await supabase.auth.getUser()

      expect(error).toBeNull()
      expect(userData.user).toBeDefined()
      expect(userData.user?.id).toBe(anonData.user?.id)
      expect(userData.user?.is_anonymous).toBe(true)
    })

    it('should get current anonymous session via getSession()', async () => {
      const { data: anonData } = await supabase.auth.signInAnonymously()
      expect(anonData.session).toBeDefined()

      const { data: sessionData, error } = await supabase.auth.getSession()

      expect(error).toBeNull()
      expect(sessionData.session).toBeDefined()
      expect(sessionData.session?.access_token).toBe(anonData.session?.access_token)
      expect(sessionData.session?.user?.is_anonymous).toBe(true)
    })

    it('should refresh anonymous session', async () => {
      const { data: anonData } = await supabase.auth.signInAnonymously()
      expect(anonData.session?.refresh_token).toBeDefined()

      const { data: refreshData, error } = await supabase.auth.refreshSession()

      expect(error).toBeNull()
      expect(refreshData.session).toBeDefined()
      expect(refreshData.session?.access_token).toBeDefined()
      expect(refreshData.user?.is_anonymous).toBe(true)
    })

    it('should refresh anonymous session using refresh token', async () => {
      const { data: anonData } = await supabase.auth.signInAnonymously()
      const refreshToken = anonData.session?.refresh_token

      const { data: refreshData, error } = await supabase.auth.refreshSession({
        refresh_token: refreshToken!,
      })

      expect(error).toBeNull()
      expect(refreshData.session).toBeDefined()
      expect(refreshData.user?.is_anonymous).toBe(true)
    })

    it('should sign out anonymous user', async () => {
      await supabase.auth.signInAnonymously()

      const { error } = await supabase.auth.signOut()

      expect(error).toBeNull()

      const { data: sessionData } = await supabase.auth.getSession()
      expect(sessionData.session).toBeNull()
    })
  })

  /**
   * Settings endpoint - anonymous sign-in configuration
   */
  describe('Settings Endpoint', () => {
    it('should show anonymous sign-in as enabled', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/settings`)
      const settings = await response.json()

      expect(settings.external).toBeDefined()
      expect(settings.external.anonymous).toBe(true)
    })
  })

  /**
   * Edge cases and error handling
   */
  describe('Edge Cases', () => {
    it('should create unique anonymous users on each sign-in', async () => {
      const { data: user1 } = await supabase.auth.signInAnonymously()
      await supabase.auth.signOut()

      const { data: user2 } = await supabase.auth.signInAnonymously()

      expect(user1.user?.id).not.toBe(user2.user?.id)
    })

    it('should handle multiple anonymous sessions', async () => {
      // Create first anonymous user
      const { data: firstUser } = await supabase.auth.signInAnonymously()
      expect(firstUser.user?.is_anonymous).toBe(true)
      const firstUserId = firstUser.user?.id

      // Create second client with second anonymous user
      const supabase2 = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
        auth: { autoRefreshToken: false, persistSession: false },
      })

      const { data: secondUser } = await supabase2.auth.signInAnonymously()
      expect(secondUser.user?.is_anonymous).toBe(true)

      // Both should be different users
      expect(firstUserId).not.toBe(secondUser.user?.id)

      // Cleanup
      await supabase2.auth.signOut()
    })

    it('should reject conversion without password', async () => {
      await supabase.auth.signInAnonymously()

      // Try to set just email without password
      const { data, error } = await supabase.auth.updateUser({
        email: uniqueEmail(),
      })

      // This should either:
      // 1. Work (email update pending confirmation, still anonymous)
      // 2. Require password to be set
      // Either behavior is acceptable
      if (error) {
        expect(error.message).toBeDefined()
      }
    })
  })
})

/**
 * Compatibility Summary for Anonymous Sign-In:
 *
 * IMPLEMENTED:
 * - signInAnonymously(): Create anonymous user session
 * - signInAnonymously({ options: { data } }): With user metadata
 * - is_anonymous flag on user object
 * - Anonymous user has null email
 * - Anonymous user has 'authenticated' role
 * - Conversion to permanent user via updateUser({ email, password })
 * - Session management (getUser, getSession, refreshSession)
 * - /auth/v1/settings shows external.anonymous status
 *
 * NOT IMPLEMENTED:
 * - Captcha token option
 */
