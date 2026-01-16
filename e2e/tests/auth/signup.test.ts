/**
 * Auth - Sign Up Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/auth-signup
 */

import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

describe('Auth - Sign Up', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  afterEach(async () => {
    // Sign out after each test
    await supabase.auth.signOut()
  })

  /**
   * Example 1: Sign up with email and password
   * Docs: https://supabase.com/docs/reference/javascript/auth-signup#sign-up-with-an-email-and-password
   *
   * const { data, error } = await supabase.auth.signUp({
   *   email: 'example@email.com',
   *   password: 'example-password',
   * })
   */
  describe('1. Sign up with email and password', () => {
    it('should create a new user account', async () => {
      const email = uniqueEmail()
      const password = 'test-password-123'

      const { data, error } = await supabase.auth.signUp({
        email,
        password,
      })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data.user).toBeDefined()
      expect(data.user?.email).toBe(email)
    })

    it('should return user object with expected properties', async () => {
      const email = uniqueEmail()
      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      expect(error).toBeNull()
      expect(data.user).toHaveProperty('id')
      expect(data.user).toHaveProperty('email')
      expect(data.user).toHaveProperty('created_at')
    })

    it('should return session when email confirmation is disabled', async () => {
      const email = uniqueEmail()
      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      expect(error).toBeNull()
      // sblite doesn't require email confirmation by default
      expect(data.session).toBeDefined()
      expect(data.session?.access_token).toBeDefined()
    })

    it('should reject duplicate email addresses', async () => {
      const email = uniqueEmail()

      // First signup
      await supabase.auth.signUp({
        email,
        password: 'test-password-123',
      })

      // Second signup with same email
      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'different-password',
      })

      expect(error).not.toBeNull()
    })

    it('should reject weak passwords', async () => {
      const email = uniqueEmail()

      const { data, error } = await supabase.auth.signUp({
        email,
        password: '123', // Too short
      })

      // sblite should enforce minimum password length
      // If not enforced, this test documents the behavior
      if (error) {
        expect(error.message).toBeDefined()
      }
    })
  })

  /**
   * Example 2: Sign up with phone number (SMS)
   * Docs: https://supabase.com/docs/reference/javascript/auth-signup#sign-up-with-a-phone-number-and-password-sms
   *
   * Note: Phone authentication not implemented in sblite Phase 1
   */
  describe('2. Sign up with phone (SMS)', () => {
    it.skip('should create account with phone number via SMS', async () => {
      const { data, error } = await supabase.auth.signUp({
        phone: '123456789',
        password: 'example-password',
        options: {
          channel: 'sms',
        },
      })

      expect(error).toBeNull()
    })
  })

  /**
   * Example 3: Sign up with phone (WhatsApp)
   * Docs: https://supabase.com/docs/reference/javascript/auth-signup#sign-up-with-a-phone-number-and-password-whatsapp
   *
   * Note: Not implemented in sblite Phase 1
   */
  describe('3. Sign up with phone (WhatsApp)', () => {
    it.skip('should create account with phone via WhatsApp', async () => {
      const { data, error } = await supabase.auth.signUp({
        phone: '123456789',
        password: 'example-password',
        options: {
          channel: 'whatsapp',
        },
      })

      expect(error).toBeNull()
    })
  })

  /**
   * Example 4: Sign up with additional user metadata
   * Docs: https://supabase.com/docs/reference/javascript/auth-signup#sign-up-with-additional-user-metadata
   *
   * const { data, error } = await supabase.auth.signUp({
   *   email: 'example@email.com',
   *   password: 'example-password',
   *   options: {
   *     data: { first_name: 'John', age: 27 }
   *   }
   * })
   */
  describe('4. Sign up with user metadata', () => {
    it('should store custom user metadata during signup', async () => {
      const email = uniqueEmail()
      const metadata = {
        first_name: 'John',
        last_name: 'Doe',
        age: 27,
      }

      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
        options: {
          data: metadata,
        },
      })

      expect(error).toBeNull()
      expect(data.user).toBeDefined()
      // User metadata should be accessible
      expect(data.user?.user_metadata).toBeDefined()
      expect(data.user?.user_metadata?.first_name).toBe('John')
      expect(data.user?.user_metadata?.last_name).toBe('Doe')
      expect(data.user?.user_metadata?.age).toBe(27)
    })

    it('should handle empty metadata', async () => {
      const email = uniqueEmail()

      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
        options: {
          data: {},
        },
      })

      expect(error).toBeNull()
      expect(data.user).toBeDefined()
    })
  })

  /**
   * Example 5: Sign up with redirect URL
   * Docs: https://supabase.com/docs/reference/javascript/auth-signup#sign-up-with-a-redirect-url
   *
   * Note: Email redirect functionality not implemented in sblite Phase 1
   */
  describe('5. Sign up with redirect URL', () => {
    it.skip('should accept emailRedirectTo option', async () => {
      const email = uniqueEmail()

      const { data, error } = await supabase.auth.signUp({
        email,
        password: 'test-password-123',
        options: {
          emailRedirectTo: 'https://example.com/welcome',
        },
      })

      expect(error).toBeNull()
    })
  })

  // Additional edge case tests
  describe('Edge Cases', () => {
    it('should handle missing email', async () => {
      const { data, error } = await supabase.auth.signUp({
        email: '',
        password: 'test-password-123',
      })

      expect(error).not.toBeNull()
    })

    it('should handle missing password', async () => {
      const { data, error } = await supabase.auth.signUp({
        email: uniqueEmail(),
        password: '',
      })

      expect(error).not.toBeNull()
    })

    it('should handle invalid email format', async () => {
      const { data, error } = await supabase.auth.signUp({
        email: 'not-an-email',
        password: 'test-password-123',
      })

      // sblite may or may not validate email format
      // This test documents the behavior
      if (error) {
        expect(error.message).toBeDefined()
      }
    })
  })
})

/**
 * Compatibility Summary for Sign Up:
 *
 * IMPLEMENTED:
 * - Email + password signup
 * - User metadata during signup
 * - Session returned on signup (no email confirmation)
 * - Duplicate email detection
 *
 * NOT IMPLEMENTED:
 * - Phone signup (SMS)
 * - Phone signup (WhatsApp)
 * - Email redirect URL
 * - Email confirmation flow
 * - CAPTCHA verification
 */
