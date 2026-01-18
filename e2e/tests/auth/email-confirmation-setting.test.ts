/**
 * Email Confirmation Setting Tests
 *
 * Tests for the configurable email confirmation requirement feature.
 * By default, email confirmation is required for new signups.
 */

import { describe, it, expect, beforeAll, afterAll, beforeEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'
import { clearAllEmails, waitForEmail, extractToken } from '../../setup/mail-helpers'

// Dashboard API helper
async function setEmailConfirmationRequired(required: boolean): Promise<void> {
  // First login to dashboard (if needed - skip auth for this test)
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/settings/auth-config`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ require_email_confirmation: required }),
  })
  if (!response.ok) {
    throw new Error(`Failed to set email confirmation setting: ${response.status}`)
  }
}

async function getEmailConfirmationRequired(): Promise<boolean> {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/settings/auth-config`)
  if (!response.ok) {
    throw new Error(`Failed to get email confirmation setting: ${response.status}`)
  }
  const data = await response.json()
  return data.require_email_confirmation
}

describe('Email Confirmation Setting', () => {
  let supabase: SupabaseClient
  let originalSetting: boolean

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Save original setting
    originalSetting = await getEmailConfirmationRequired()
  })

  afterAll(async () => {
    // Restore original setting
    await setEmailConfirmationRequired(originalSetting)
  })

  beforeEach(async () => {
    await clearAllEmails()
  })

  describe('Public Settings Endpoint', () => {
    it('should return mailer_autoconfirm=false when confirmation required', async () => {
      await setEmailConfirmationRequired(true)

      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/settings`)
      expect(response.ok).toBe(true)

      const settings = await response.json()
      expect(settings.mailer_autoconfirm).toBe(false)
    })

    it('should return mailer_autoconfirm=true when confirmation not required', async () => {
      await setEmailConfirmationRequired(false)

      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/settings`)
      expect(response.ok).toBe(true)

      const settings = await response.json()
      expect(settings.mailer_autoconfirm).toBe(true)
    })
  })

  describe('Signup with Confirmation Required', () => {
    beforeAll(async () => {
      await setEmailConfirmationRequired(true)
    })

    it('should not return session on signup when confirmation required', async () => {
      const email = uniqueEmail()

      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      expect(error).toBeNull()
      expect(data.user).toBeDefined()
      expect(data.user?.id).toBeDefined()
      // Session should NOT be returned when confirmation is required
      expect(data.session).toBeNull()
    })

    it('should send confirmation email on signup', async () => {
      const email = uniqueEmail()

      await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      // Should receive confirmation email
      const confirmEmail = await waitForEmail(email, 'confirmation', 5000)
      expect(confirmEmail).toBeDefined()
      expect(confirmEmail.subject).toContain('Confirm')
    })

    it('should reject login before email confirmation', async () => {
      const email = uniqueEmail()

      // Sign up
      await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      // Try to sign in without confirming
      const { data, error } = await supabase.auth.signInWithPassword({
        email,
        password: 'test-password-123',
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('Email not confirmed')
      expect(data.session).toBeNull()
    })

    it('should allow login after email confirmation', async () => {
      const email = uniqueEmail()

      // Sign up
      await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      // Get confirmation email and extract token
      const confirmEmail = await waitForEmail(email, 'confirmation', 5000)
      const token = extractToken(confirmEmail)
      expect(token).not.toBeNull()

      // Verify email
      const verifyResponse = await fetch(`${TEST_CONFIG.SBLITE_URL}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({ type: 'signup', token }),
      })
      expect(verifyResponse.ok).toBe(true)

      // Now login should work
      const { data, error } = await supabase.auth.signInWithPassword({
        email,
        password: 'test-password-123',
      })

      expect(error).toBeNull()
      expect(data.session).not.toBeNull()
      expect(data.session?.access_token).toBeDefined()
    })
  })

  describe('Signup without Confirmation Required', () => {
    beforeAll(async () => {
      await setEmailConfirmationRequired(false)
    })

    afterAll(async () => {
      // Reset to required
      await setEmailConfirmationRequired(true)
    })

    it('should return session immediately on signup', async () => {
      const email = uniqueEmail()

      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      expect(error).toBeNull()
      expect(data.user).toBeDefined()
      // Session SHOULD be returned when confirmation not required
      expect(data.session).not.toBeNull()
      expect(data.session?.access_token).toBeDefined()
    })

    it('should allow login immediately after signup', async () => {
      const email = uniqueEmail()

      // Sign up
      await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      // Sign out
      await supabase.auth.signOut()

      // Should be able to sign in immediately
      const { data, error } = await supabase.auth.signInWithPassword({
        email,
        password: 'test-password-123',
      })

      expect(error).toBeNull()
      expect(data.session).not.toBeNull()
    })
  })

  describe('Dashboard API', () => {
    it('should get current auth config', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/settings/auth-config`)
      expect(response.ok).toBe(true)

      const data = await response.json()
      expect(data).toHaveProperty('require_email_confirmation')
      expect(typeof data.require_email_confirmation).toBe('boolean')
    })

    it('should update auth config', async () => {
      // Set to false
      await setEmailConfirmationRequired(false)
      let setting = await getEmailConfirmationRequired()
      expect(setting).toBe(false)

      // Set to true
      await setEmailConfirmationRequired(true)
      setting = await getEmailConfirmationRequired()
      expect(setting).toBe(true)
    })
  })
})
