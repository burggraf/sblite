/**
 * SMTP Mode Tests
 *
 * Tests for email sending via SMTP. These tests are skipped by default
 * and require a local SMTP server (like Mailpit) to be running.
 *
 * To run these tests:
 * 1. Start Mailpit: docker run -p 8025:8025 -p 1025:1025 axllent/mailpit
 * 2. Start sblite with SMTP mode:
 *    SBLITE_MAIL_MODE=smtp SBLITE_SMTP_HOST=localhost SBLITE_SMTP_PORT=1025 ./sblite serve
 * 3. Run tests: SBLITE_TEST_SMTP=true npm run test:email:smtp
 */

import { describe, it, expect, beforeAll, beforeEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueEmail } from '../../setup/test-helpers'

// Skip all tests unless SBLITE_TEST_SMTP is set
const runSmtpTests = process.env.SBLITE_TEST_SMTP === 'true'

// Mailpit API configuration (default Mailpit HTTP API port)
const MAILPIT_API = process.env.MAILPIT_API || 'http://localhost:8025/api'

/**
 * Helper to get emails from Mailpit API
 */
async function getMailpitEmails(): Promise<any[]> {
  const response = await fetch(`${MAILPIT_API}/v1/messages`)
  if (!response.ok) {
    throw new Error(`Mailpit API error: ${response.status}`)
  }
  const data = await response.json()
  return data.messages || []
}

/**
 * Helper to clear Mailpit inbox
 */
async function clearMailpitInbox(): Promise<void> {
  await fetch(`${MAILPIT_API}/v1/messages`, { method: 'DELETE' })
}

/**
 * Helper to find email in Mailpit by recipient
 */
async function findMailpitEmail(
  to: string,
  timeout: number = 5000
): Promise<any | null> {
  const startTime = Date.now()

  while (Date.now() - startTime < timeout) {
    const emails = await getMailpitEmails()
    const match = emails.find((email: any) =>
      email.To?.some((recipient: any) => recipient.Address === to)
    )
    if (match) {
      return match
    }
    await new Promise((resolve) => setTimeout(resolve, 200))
  }

  return null
}

describe.skipIf(!runSmtpTests)('SMTP Mode', () => {
  let supabase: SupabaseClient

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Verify Mailpit is accessible
    try {
      await getMailpitEmails()
    } catch (error) {
      console.error('Mailpit API not accessible. Make sure Mailpit is running on port 8025.')
      throw error
    }
  })

  beforeEach(async () => {
    await clearMailpitInbox()
  })

  describe('Password Recovery via SMTP', () => {
    it('should send recovery email through SMTP', async () => {
      const testEmail = uniqueEmail()

      // Create user first
      await supabase.auth.signUp({ email: testEmail, password: 'test-password-123' })
      await clearMailpitInbox()

      // Request password reset
      const { error } = await supabase.auth.resetPasswordForEmail(testEmail)
      expect(error).toBeNull()

      // Check Mailpit for the email
      const email = await findMailpitEmail(testEmail)
      expect(email).not.toBeNull()
      expect(email.Subject).toContain('password')
    })
  })

  describe('Magic Link via SMTP', () => {
    it('should send magic link email through SMTP', async () => {
      const testEmail = uniqueEmail()

      // Request magic link
      const { error } = await supabase.auth.signInWithOtp({ email: testEmail })
      expect(error).toBeNull()

      // Check Mailpit for the email
      const email = await findMailpitEmail(testEmail)
      expect(email).not.toBeNull()
      expect(email.Subject).toContain('login')
    })
  })

  describe('User Invite via SMTP', () => {
    it('should send invite email through SMTP', async () => {
      const inviteEmail = uniqueEmail()

      // Send invite
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

      // Check Mailpit for the email
      const email = await findMailpitEmail(inviteEmail)
      expect(email).not.toBeNull()
      expect(email.Subject).toContain('invite')
    })
  })

  describe('SMTP Configuration', () => {
    it('should include correct sender address', async () => {
      const testEmail = uniqueEmail()

      await supabase.auth.signInWithOtp({ email: testEmail })

      const email = await findMailpitEmail(testEmail)
      expect(email).not.toBeNull()
      // Verify From address is set (exact value depends on configuration)
      expect(email.From).toBeDefined()
    })
  })
})

/**
 * Documentation for running SMTP tests locally
 *
 * ## Using Mailpit (Recommended)
 *
 * Mailpit is a lightweight email testing tool with a web UI.
 *
 * 1. Start Mailpit with Docker:
 *    ```bash
 *    docker run -d \
 *      --name mailpit \
 *      -p 8025:8025 \
 *      -p 1025:1025 \
 *      axllent/mailpit
 *    ```
 *
 * 2. Configure sblite for SMTP:
 *    ```bash
 *    export SBLITE_MAIL_MODE=smtp
 *    export SBLITE_SMTP_HOST=localhost
 *    export SBLITE_SMTP_PORT=1025
 *    # No auth needed for Mailpit
 *    ```
 *
 * 3. Start sblite:
 *    ```bash
 *    ./sblite serve
 *    ```
 *
 * 4. Run SMTP tests:
 *    ```bash
 *    cd e2e
 *    SBLITE_TEST_SMTP=true npm run test:email:smtp
 *    ```
 *
 * 5. View emails in browser: http://localhost:8025
 *
 * ## Using Mailhog (Alternative)
 *
 * Similar to Mailpit but older:
 *
 * ```bash
 * docker run -d -p 8025:8025 -p 1025:1025 mailhog/mailhog
 * ```
 *
 * Note: Mailhog API is slightly different, may need adapter.
 *
 * ## Cleanup
 *
 * Stop and remove the container:
 * ```bash
 * docker stop mailpit && docker rm mailpit
 * ```
 */
