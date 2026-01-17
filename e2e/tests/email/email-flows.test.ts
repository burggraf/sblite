/**
 * Email Flows Tests
 *
 * Tests for authentication flows that trigger email sending.
 * Verifies correct emails are sent for each flow.
 */

import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'
import {
  clearAllEmails,
  waitForEmail,
  findEmail,
  assertNoEmailSent,
  countEmailsByType,
  extractToken,
} from '../../setup/mail-helpers'

describe('Email Flows', () => {
  let supabase: SupabaseClient
  let serviceClient: SupabaseClient

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    serviceClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  beforeEach(async () => {
    await clearAllEmails()
  })

  afterAll(async () => {
    await clearAllEmails()
  })

  describe('Password Recovery (resetPasswordForEmail)', () => {
    it('should send recovery email for existing user', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })

      // Clear signup emails
      await clearAllEmails()

      // Request password reset
      const { error } = await supabase.auth.resetPasswordForEmail(testEmail)
      expect(error).toBeNull()

      // Verify email was sent
      const email = await waitForEmail(testEmail, 'recovery')
      expect(email.subject).toContain('password')
      expect(email.to).toBe(testEmail)
    })

    it('should include verification token in recovery email', async () => {
      const testEmail = uniqueEmail()
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(testEmail)

      const email = await waitForEmail(testEmail, 'recovery')
      const token = extractToken(email)
      expect(token).not.toBeNull()
      expect(token!.length).toBeGreaterThan(10)
    })

    it('should send recovery email even for non-existent user (no information leak)', async () => {
      const nonExistentEmail = uniqueEmail()

      // Request password reset for non-existent user
      const { error } = await supabase.auth.resetPasswordForEmail(nonExistentEmail)

      // API should not error (no information leak about user existence)
      expect(error).toBeNull()

      // However, no actual email should be sent (or a dummy email might be sent)
      // The important thing is the API doesn't reveal user existence
    })

    it('should accept redirectTo option', async () => {
      const testEmail = uniqueEmail()
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      const { error } = await supabase.auth.resetPasswordForEmail(testEmail, {
        redirectTo: 'https://myapp.com/reset-password',
      })

      expect(error).toBeNull()
      const email = await waitForEmail(testEmail, 'recovery')
      expect(email).toBeDefined()
    })
  })

  describe('Magic Link (signInWithOtp)', () => {
    it('should send magic link email', async () => {
      const testEmail = uniqueEmail()

      const { error } = await supabase.auth.signInWithOtp({ email: testEmail })
      expect(error).toBeNull()

      const email = await waitForEmail(testEmail, 'magic_link')
      expect(email.type).toBe('magic_link')
      expect(email.to).toBe(testEmail)
    })

    it('should include verification token in magic link email', async () => {
      const testEmail = uniqueEmail()

      await supabase.auth.signInWithOtp({ email: testEmail })

      const email = await waitForEmail(testEmail, 'magic_link')
      const token = extractToken(email)
      expect(token).not.toBeNull()
    })

    it('should work for existing users', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      // Request magic link
      const { error } = await supabase.auth.signInWithOtp({ email: testEmail })
      expect(error).toBeNull()

      const email = await waitForEmail(testEmail, 'magic_link')
      expect(email).toBeDefined()
    })
  })

  describe('User Invite (Admin Only)', () => {
    it('should send invite email via service role', async () => {
      const inviteEmail = uniqueEmail()

      // Use service role to invite user
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/invite`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          apikey: TEST_CONFIG.SBLITE_SERVICE_KEY,
        },
        body: JSON.stringify({ email: inviteEmail }),
      })

      expect(response.ok).toBe(true)

      const email = await waitForEmail(inviteEmail, 'invite')
      expect(email.type).toBe('invite')
      expect(email.to).toBe(inviteEmail)
    })

    it('should include verification token in invite email', async () => {
      const inviteEmail = uniqueEmail()

      await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/invite`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          apikey: TEST_CONFIG.SBLITE_SERVICE_KEY,
        },
        body: JSON.stringify({ email: inviteEmail }),
      })

      const email = await waitForEmail(inviteEmail, 'invite')
      const token = extractToken(email)
      expect(token).not.toBeNull()
    })

    it('should reject invite without service role', async () => {
      const inviteEmail = uniqueEmail()

      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/invite`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${TEST_CONFIG.SBLITE_ANON_KEY}`,
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({ email: inviteEmail }),
      })

      // Should be unauthorized
      expect(response.status).toBeGreaterThanOrEqual(400)
    })
  })

  describe('Resend Email', () => {
    it('should resend confirmation email', async () => {
      const testEmail = uniqueEmail()

      // Sign up (generates confirmation email)
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      // Resend confirmation
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/resend`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({ type: 'signup', email: testEmail }),
      })

      expect(response.ok).toBe(true)

      const email = await waitForEmail(testEmail, 'confirmation')
      expect(email.type).toBe('confirmation')
    })

    it('should resend recovery email', async () => {
      const testEmail = uniqueEmail()

      // Sign up first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      // Request resend of recovery email
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/resend`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({ type: 'recovery', email: testEmail }),
      })

      expect(response.ok).toBe(true)

      const email = await waitForEmail(testEmail, 'recovery')
      expect(email.type).toBe('recovery')
    })
  })

  describe('Security', () => {
    it('should not reveal user existence via API response', async () => {
      const existingEmail = uniqueEmail()
      const nonExistingEmail = uniqueEmail()

      // Create one user
      await supabase.auth.signUp({ email: existingEmail, password: 'test-password-123' })

      // Request password reset for both
      const { error: existingError } = await supabase.auth.resetPasswordForEmail(existingEmail)
      const { error: nonExistingError } =
        await supabase.auth.resetPasswordForEmail(nonExistingEmail)

      // Both should succeed (no information leak)
      expect(existingError).toBeNull()
      expect(nonExistingError).toBeNull()
    })

    it('should not send email to non-existent user for recovery', async () => {
      const nonExistentEmail = uniqueEmail()

      await supabase.auth.resetPasswordForEmail(nonExistentEmail)

      // Wait briefly and verify no email was actually sent
      await new Promise((resolve) => setTimeout(resolve, 500))

      const email = await findEmail(nonExistentEmail, 'recovery', { timeout: 500 })
      // Email should not be sent to non-existent user
      expect(email).toBeNull()
    })
  })
})
