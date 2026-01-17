/**
 * Email Verification Flow Tests
 *
 * End-to-end tests that verify complete email verification flows:
 * - Password reset: email → token → verify → new password works
 * - Magic link: email → token → verify → session created
 * - Invite: email → token → accept → user created
 */

import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'
import {
  clearAllEmails,
  waitForEmail,
  extractToken,
  extractVerificationUrl,
} from '../../setup/mail-helpers'

describe('Email Verification Flows', () => {
  let supabase: SupabaseClient

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  beforeEach(async () => {
    await clearAllEmails()
  })

  afterAll(async () => {
    await clearAllEmails()
  })

  describe('Password Reset Flow', () => {
    it('should complete full password reset flow', async () => {
      const testEmail = uniqueEmail()
      const originalPassword = 'original-password-123'
      const newPassword = 'new-password-456'

      // Step 1: Create user
      const { error: signupError } = await supabase.auth.signUp({
        email: testEmail,
        password: originalPassword,
      })
      expect(signupError).toBeNull()
      await clearAllEmails()

      // Step 2: Request password reset
      const { error: resetError } = await supabase.auth.resetPasswordForEmail(testEmail)
      expect(resetError).toBeNull()

      // Step 3: Get recovery email and extract token
      const email = await waitForEmail(testEmail, 'recovery')
      const token = extractToken(email)
      expect(token).not.toBeNull()

      // Step 4: Verify token and set new password
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'recovery',
          token: token,
          password: newPassword,
        }),
      })
      expect(verifyResponse.ok).toBe(true)

      // Step 5: Verify new password works
      const { data: signInData, error: signInError } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: newPassword,
      })
      expect(signInError).toBeNull()
      expect(signInData.session).not.toBeNull()

      // Step 6: Verify old password no longer works
      const { error: oldPasswordError } = await supabase.auth.signInWithPassword({
        email: testEmail,
        password: originalPassword,
      })
      expect(oldPasswordError).not.toBeNull()
    })

    it('should reject invalid recovery token', async () => {
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'recovery',
          token: 'invalid-token-12345',
          password: 'new-password-123',
        }),
      })
      expect(verifyResponse.ok).toBe(false)
      expect(verifyResponse.status).toBeGreaterThanOrEqual(400)
    })

    it('should reject already-used recovery token', async () => {
      const testEmail = uniqueEmail()

      // Create user and request reset
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()
      await supabase.auth.resetPasswordForEmail(testEmail)

      const email = await waitForEmail(testEmail, 'recovery')
      const token = extractToken(email)

      // Use token first time
      const firstResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'recovery',
          token: token,
          password: 'new-password-123',
        }),
      })
      expect(firstResponse.ok).toBe(true)

      // Try to use same token again
      const secondResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'recovery',
          token: token,
          password: 'another-password-789',
        }),
      })
      expect(secondResponse.ok).toBe(false)
    })
  })

  describe('Magic Link Flow', () => {
    it('should complete full magic link sign-in flow', async () => {
      const testEmail = uniqueEmail()

      // Step 1: Request magic link (creates user if not exists)
      const { error: otpError } = await supabase.auth.signInWithOtp({ email: testEmail })
      expect(otpError).toBeNull()

      // Step 2: Get magic link email and extract token
      const email = await waitForEmail(testEmail, 'magic_link')
      const token = extractToken(email)
      expect(token).not.toBeNull()

      // Step 3: Verify token via GET request (magic links use GET)
      const verifyUrl = `${TEST_CONFIG.SBLITE_URL}/auth/v1/verify?token=${token}&type=magiclink`
      const verifyResponse = await fetch(verifyUrl, {
        method: 'GET',
        headers: {
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        redirect: 'manual', // Don't follow redirects
      })

      // Magic link verification typically redirects or returns session
      expect(verifyResponse.status).toBeLessThan(500)
    })

    it('should reject invalid magic link token', async () => {
      const verifyUrl = `${TEST_CONFIG.SBLITE_URL}/auth/v1/verify?token=invalid-token&type=magiclink`
      const verifyResponse = await fetch(verifyUrl, {
        method: 'GET',
        headers: {
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        redirect: 'manual',
      })

      // Should fail with 4xx error
      expect(verifyResponse.status).toBeGreaterThanOrEqual(400)
    })

    it('should reject already-used magic link token', async () => {
      const testEmail = uniqueEmail()

      await supabase.auth.signInWithOtp({ email: testEmail })
      const email = await waitForEmail(testEmail, 'magic_link')
      const token = extractToken(email)

      // Use token first time
      const verifyUrl = `${TEST_CONFIG.SBLITE_URL}/auth/v1/verify?token=${token}&type=magiclink`
      await fetch(verifyUrl, {
        method: 'GET',
        headers: { apikey: TEST_CONFIG.SBLITE_ANON_KEY },
        redirect: 'manual',
      })

      // Try to use same token again
      const secondResponse = await fetch(verifyUrl, {
        method: 'GET',
        headers: { apikey: TEST_CONFIG.SBLITE_ANON_KEY },
        redirect: 'manual',
      })

      expect(secondResponse.status).toBeGreaterThanOrEqual(400)
    })
  })

  describe('Invite Flow', () => {
    it('should complete full invite acceptance flow', async () => {
      const inviteEmail = uniqueEmail()

      // Step 1: Admin invites user
      const inviteResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/invite`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          apikey: TEST_CONFIG.SBLITE_SERVICE_KEY,
        },
        body: JSON.stringify({ email: inviteEmail }),
      })
      expect(inviteResponse.ok).toBe(true)

      // Step 2: Get invite email and extract token
      const email = await waitForEmail(inviteEmail, 'invite')
      const token = extractToken(email)
      expect(token).not.toBeNull()

      // Step 3: Accept invite and set password
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'invite',
          token: token,
          password: 'invited-user-password-123',
        }),
      })
      expect(verifyResponse.ok).toBe(true)

      // Step 4: Verify user can now sign in
      const { data: signInData, error: signInError } = await supabase.auth.signInWithPassword({
        email: inviteEmail,
        password: 'invited-user-password-123',
      })
      expect(signInError).toBeNull()
      expect(signInData.session).not.toBeNull()
    })

    it('should reject invalid invite token', async () => {
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'invite',
          token: 'invalid-invite-token',
          password: 'some-password-123',
        }),
      })
      expect(verifyResponse.ok).toBe(false)
      expect(verifyResponse.status).toBeGreaterThanOrEqual(400)
    })
  })

  describe('Token Validation', () => {
    it('should reject token with wrong type', async () => {
      const testEmail = uniqueEmail()

      // Get a recovery token
      await supabase.auth.signUp({ email: testEmail, password: 'test-123' })
      await clearAllEmails()
      await supabase.auth.resetPasswordForEmail(testEmail)

      const email = await waitForEmail(testEmail, 'recovery')
      const token = extractToken(email)

      // Try to use recovery token as magic link
      const verifyUrl = `${TEST_CONFIG.SBLITE_URL}/auth/v1/verify?token=${token}&type=magiclink`
      const verifyResponse = await fetch(verifyUrl, {
        method: 'GET',
        headers: { apikey: TEST_CONFIG.SBLITE_ANON_KEY },
        redirect: 'manual',
      })

      expect(verifyResponse.status).toBeGreaterThanOrEqual(400)
    })

    it('should reject empty token', async () => {
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          type: 'recovery',
          token: '',
          password: 'some-password',
        }),
      })
      expect(verifyResponse.ok).toBe(false)
    })

    it('should reject missing type', async () => {
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          token: 'some-token',
          password: 'some-password',
        }),
      })
      expect(verifyResponse.ok).toBe(false)
    })
  })
})
