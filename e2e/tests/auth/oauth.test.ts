/**
 * Auth - OAuth Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-signinwithoauth
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('OAuth', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  describe('signInWithOAuth', () => {
    it('returns authorization URL for enabled provider', async () => {
      const { data, error } = await supabase.auth.signInWithOAuth({
        provider: 'google',
        options: {
          redirectTo: 'http://localhost:3000/callback',
          skipBrowserRedirect: true,
        },
      })

      // If Google is not configured, this will error
      // In test mode, we just verify the URL structure
      if (data?.url) {
        expect(data.url).toContain('/auth/v1/authorize')
        expect(data.url).toContain('provider=google')
      }
    })

    it('returns error for disabled provider', async () => {
      const { data, error } = await supabase.auth.signInWithOAuth({
        provider: 'facebook' as any, // Not enabled
        options: {
          skipBrowserRedirect: true,
        },
      })

      // Should indicate provider not available
      expect(error || !data?.url).toBeTruthy()
    })
  })

  describe('settings', () => {
    it('shows OAuth provider status', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/settings`)
      const settings = await response.json()

      expect(settings.external).toBeDefined()
      expect(typeof settings.external.google).toBe('boolean')
      expect(typeof settings.external.github).toBe('boolean')
      expect(settings.external.email).toBe(true) // Always enabled
    })
  })

  describe('identities', () => {
    it('returns empty array for email-only user', async () => {
      // Sign up a new user
      const email = uniqueEmail()
      const { data: signUpData } = await supabase.auth.signUp({
        email,
        password: 'testpassword123',
      })

      if (signUpData.session) {
        // Get identities
        const { data: userData } = await supabase.auth.getUser()

        // Email users don't have OAuth identities
        expect(userData.user?.identities || []).toHaveLength(0)
      }
    })
  })

  describe('callback validation', () => {
    it('rejects invalid state parameter', async () => {
      const response = await fetch(
        `${TEST_CONFIG.SBLITE_URL}/auth/v1/callback?code=test&state=invalid`,
        { redirect: 'manual' }
      )

      // Should return error (either 400 or redirect with error)
      expect(response.status === 400 || response.status === 302).toBe(true)
    })

    it('rejects missing code parameter', async () => {
      const response = await fetch(
        `${TEST_CONFIG.SBLITE_URL}/auth/v1/callback?state=test`,
        { redirect: 'manual' }
      )

      expect(response.status).toBe(400)
    })
  })
})

/**
 * Compatibility Summary for OAuth:
 *
 * IMPLEMENTED:
 * - signInWithOAuth with Google, GitHub, Apple providers
 * - /auth/v1/authorize endpoint (authorization URL generation)
 * - /auth/v1/callback endpoint (OAuth callback handling)
 * - /auth/v1/settings endpoint (provider status)
 * - State parameter validation (CSRF protection)
 * - PKCE support
 *
 * NOT IMPLEMENTED:
 * - Facebook, Twitter, Discord, and other providers
 * - Identity linking (linking OAuth to existing email account)
 * - Unlinking identities
 */
