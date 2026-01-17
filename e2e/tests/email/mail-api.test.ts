/**
 * Mail API Tests
 *
 * Tests for the mail viewer API endpoints (catch mode only).
 * These tests verify the /mail/api/* endpoints work correctly.
 */

import { describe, it, expect, beforeAll, beforeEach, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'
import {
  getEmails,
  getEmail,
  deleteEmail,
  clearAllEmails,
  CaughtEmail,
} from '../../setup/mail-helpers'

describe('Mail API', () => {
  let supabase: SupabaseClient

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  beforeEach(async () => {
    // Clear all emails before each test
    await clearAllEmails()
  })

  afterAll(async () => {
    // Clean up
    await clearAllEmails()
  })

  describe('GET /mail/api/emails', () => {
    it('should return empty array when no emails', async () => {
      const emails = await getEmails()
      expect(emails).toEqual([])
    })

    it('should return emails after triggering email send', async () => {
      const testEmail = uniqueEmail()

      // Create user first (recovery emails only sent to existing users)
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      // Trigger an email by requesting password reset
      await supabase.auth.resetPasswordForEmail(testEmail)

      // Wait briefly for email processing
      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      expect(emails.length).toBeGreaterThanOrEqual(1)
    })

    it('should support limit parameter', async () => {
      const testEmails = [uniqueEmail(), uniqueEmail(), uniqueEmail()]

      // Create users first
      for (const email of testEmails) {
        await supabase.auth.signUp({ email, password: 'test-password-123' })
      }
      await clearAllEmails()

      // Trigger multiple emails
      for (const email of testEmails) {
        await supabase.auth.resetPasswordForEmail(email)
      }

      await new Promise((resolve) => setTimeout(resolve, 300))

      const emails = await getEmails(2)
      expect(emails.length).toBeLessThanOrEqual(2)
    })

    it('should support offset parameter for pagination', async () => {
      const testEmails = [uniqueEmail(), uniqueEmail(), uniqueEmail()]

      // Create users first
      for (const email of testEmails) {
        await supabase.auth.signUp({ email, password: 'test-password-123' })
      }
      await clearAllEmails()

      for (const email of testEmails) {
        await supabase.auth.resetPasswordForEmail(email)
      }

      await new Promise((resolve) => setTimeout(resolve, 300))

      const allEmails = await getEmails()
      const offsetEmails = await getEmails(10, 1)

      // Offset should return emails after skipping the first one
      if (allEmails.length > 1) {
        expect(offsetEmails.length).toBe(allEmails.length - 1)
      }
    })

    it('should return emails in descending order by default (newest first)', async () => {
      const email1 = uniqueEmail()
      const email2 = uniqueEmail()

      // Create users first
      await supabase.auth.signUp({ email: email1, password: 'test-password-123' })
      await supabase.auth.signUp({ email: email2, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(email1)
      await new Promise((resolve) => setTimeout(resolve, 100))
      await supabase.auth.resetPasswordForEmail(email2)

      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      if (emails.length >= 2) {
        // Newest email should be first
        const date0 = new Date(emails[0].created_at)
        const date1 = new Date(emails[1].created_at)
        expect(date0.getTime()).toBeGreaterThanOrEqual(date1.getTime())
      }
    })
  })

  describe('GET /mail/api/emails/:id', () => {
    it('should return single email by ID', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(testEmail)

      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      expect(emails.length).toBeGreaterThanOrEqual(1)

      const email = await getEmail(emails[0].id)
      expect(email).not.toBeNull()
      expect(email!.id).toBe(emails[0].id)
    })

    it('should return null for non-existent ID', async () => {
      const email = await getEmail('non-existent-id-12345')
      expect(email).toBeNull()
    })
  })

  describe('DELETE /mail/api/emails/:id', () => {
    it('should delete single email by ID', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(testEmail)

      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      expect(emails.length).toBeGreaterThanOrEqual(1)

      const emailId = emails[0].id
      await deleteEmail(emailId)

      // Verify it's deleted
      const deletedEmail = await getEmail(emailId)
      expect(deletedEmail).toBeNull()
    })

    it('should not error when deleting non-existent email', async () => {
      // Should not throw
      await deleteEmail('non-existent-id-12345')
    })
  })

  describe('DELETE /mail/api/emails', () => {
    it('should clear all emails', async () => {
      const testEmails = [uniqueEmail(), uniqueEmail()]

      // Create users first
      for (const email of testEmails) {
        await supabase.auth.signUp({ email, password: 'test-password-123' })
      }
      await clearAllEmails()

      for (const email of testEmails) {
        await supabase.auth.resetPasswordForEmail(email)
      }

      await new Promise((resolve) => setTimeout(resolve, 300))

      const before = await getEmails()
      expect(before.length).toBeGreaterThanOrEqual(2)

      await clearAllEmails()

      const after = await getEmails()
      expect(after.length).toBe(0)
    })
  })

  describe('Email Content Structure', () => {
    it('should include all required fields', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(testEmail)

      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      expect(emails.length).toBeGreaterThanOrEqual(1)

      const email = emails[0]
      expect(email).toHaveProperty('id')
      expect(email).toHaveProperty('to')
      expect(email).toHaveProperty('from')
      expect(email).toHaveProperty('subject')
      expect(email).toHaveProperty('type')
      expect(email).toHaveProperty('body_html')
      expect(email).toHaveProperty('body_text')
      expect(email).toHaveProperty('created_at')
    })

    it('should have correct type for recovery email', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(testEmail)

      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      const recoveryEmail = emails.find((e) => e.to === testEmail)

      expect(recoveryEmail).toBeDefined()
      expect(recoveryEmail!.type).toBe('recovery')
    })

    it('should contain verification URL in body', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearAllEmails()

      await supabase.auth.resetPasswordForEmail(testEmail)

      await new Promise((resolve) => setTimeout(resolve, 200))

      const emails = await getEmails()
      const recoveryEmail = emails.find((e) => e.to === testEmail)

      expect(recoveryEmail).toBeDefined()
      // Check for verification URL pattern
      expect(recoveryEmail!.body_html).toMatch(/\/auth\/v1\/verify/)
      expect(recoveryEmail!.body_html).toMatch(/token=/)
    })
  })
})
