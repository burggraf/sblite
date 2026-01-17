/**
 * Mail API helpers for E2E tests
 *
 * Provides utilities for interacting with the catch mode mail API
 * to verify email sending in tests.
 */

import { TEST_CONFIG } from './global-setup'

/**
 * Represents an email caught by the mail API
 */
export interface CaughtEmail {
  id: string
  to: string
  from: string
  subject: string
  type: string
  body_html: string
  body_text: string
  created_at: string
}

/**
 * Get the mail API base URL
 */
function getMailApiUrl(): string {
  return `${TEST_CONFIG.SBLITE_URL}/mail/api`
}

/**
 * List caught emails with optional pagination
 */
export async function getEmails(limit?: number, offset?: number): Promise<CaughtEmail[]> {
  const params = new URLSearchParams()
  if (limit !== undefined) params.set('limit', String(limit))
  if (offset !== undefined) params.set('offset', String(offset))

  const url = `${getMailApiUrl()}/emails${params.toString() ? '?' + params.toString() : ''}`
  const response = await fetch(url)

  if (!response.ok) {
    throw new Error(`Failed to get emails: ${response.status} ${response.statusText}`)
  }

  return response.json()
}

/**
 * Get a single email by ID
 */
export async function getEmail(id: string): Promise<CaughtEmail | null> {
  const response = await fetch(`${getMailApiUrl()}/emails/${id}`)

  if (response.status === 404) {
    return null
  }

  if (!response.ok) {
    throw new Error(`Failed to get email: ${response.status} ${response.statusText}`)
  }

  return response.json()
}

/**
 * Delete a single email by ID
 */
export async function deleteEmail(id: string): Promise<void> {
  const response = await fetch(`${getMailApiUrl()}/emails/${id}`, {
    method: 'DELETE',
  })

  if (!response.ok && response.status !== 404) {
    throw new Error(`Failed to delete email: ${response.status} ${response.statusText}`)
  }
}

/**
 * Clear all caught emails
 */
export async function clearAllEmails(): Promise<void> {
  const response = await fetch(`${getMailApiUrl()}/emails`, {
    method: 'DELETE',
  })

  if (!response.ok) {
    throw new Error(`Failed to clear emails: ${response.status} ${response.statusText}`)
  }
}

/**
 * Options for finding an email
 */
export interface FindEmailOptions {
  /** Maximum time to wait in ms (default: 5000) */
  timeout?: number
  /** Polling interval in ms (default: 100) */
  interval?: number
  /** Only find emails created after this date */
  after?: Date
}

/**
 * Find an email matching criteria with polling
 *
 * @param to - Recipient email address
 * @param type - Email type (confirmation, recovery, magic_link, invite, email_change)
 * @param options - Search options
 * @returns The matching email or null if not found within timeout
 */
export async function findEmail(
  to: string,
  type: string,
  options: FindEmailOptions = {}
): Promise<CaughtEmail | null> {
  const { timeout = 5000, interval = 100, after } = options
  const startTime = Date.now()

  while (Date.now() - startTime < timeout) {
    const emails = await getEmails()
    const match = emails.find((email) => {
      if (email.to !== to) return false
      if (email.type !== type) return false
      if (after && new Date(email.created_at) <= after) return false
      return true
    })

    if (match) {
      return match
    }

    await new Promise((resolve) => setTimeout(resolve, interval))
  }

  return null
}

/**
 * Wait for an email to arrive, throwing if not found
 *
 * @param to - Recipient email address
 * @param type - Email type
 * @param timeout - Maximum wait time in ms (default: 5000)
 * @returns The matching email
 * @throws Error if email not found within timeout
 */
export async function waitForEmail(
  to: string,
  type: string,
  timeout: number = 5000
): Promise<CaughtEmail> {
  const email = await findEmail(to, type, { timeout })
  if (!email) {
    throw new Error(`Email not found: to=${to}, type=${type} (waited ${timeout}ms)`)
  }
  return email
}

/**
 * Extract verification token from email body
 *
 * Looks for token parameter in URLs like:
 * - /auth/v1/verify?token=ABC123&type=signup
 * - token=ABC123
 */
export function extractToken(email: CaughtEmail): string | null {
  // Try HTML body first, then text body
  const body = email.body_html || email.body_text

  // Match token in query string
  const match = body.match(/token=([^&"\s<>]+)/)
  return match ? match[1] : null
}

/**
 * Extract the full verification URL from email body
 */
export function extractVerificationUrl(email: CaughtEmail): string | null {
  const body = email.body_html || email.body_text

  // Match href URLs containing /auth/v1/verify
  const hrefMatch = body.match(/href="([^"]*\/auth\/v1\/verify[^"]*)"/i)
  if (hrefMatch) {
    return hrefMatch[1]
  }

  // Match plain URLs containing /auth/v1/verify
  const urlMatch = body.match(/(https?:\/\/[^\s<>"]*\/auth\/v1\/verify[^\s<>"]*)/i)
  return urlMatch ? urlMatch[1] : null
}

/**
 * Count emails by type
 */
export async function countEmailsByType(type: string): Promise<number> {
  const emails = await getEmails()
  return emails.filter((email) => email.type === type).length
}

/**
 * Get emails for a specific recipient
 */
export async function getEmailsForRecipient(to: string): Promise<CaughtEmail[]> {
  const emails = await getEmails()
  return emails.filter((email) => email.to === to)
}

/**
 * Assert that no email was sent to a recipient
 * Useful for security tests (e.g., verifying non-existent users don't trigger emails)
 */
export async function assertNoEmailSent(
  to: string,
  type?: string,
  options: FindEmailOptions = {}
): Promise<void> {
  const { timeout = 1000, interval = 100 } = options
  const startTime = Date.now()

  // Wait a short time to ensure no email arrives
  await new Promise((resolve) => setTimeout(resolve, Math.min(timeout, 500)))

  const emails = await getEmails()
  const matches = emails.filter((email) => {
    if (email.to !== to) return false
    if (type && email.type !== type) return false
    return true
  })

  if (matches.length > 0) {
    throw new Error(
      `Expected no email to ${to}${type ? ` of type ${type}` : ''}, but found ${matches.length}`
    )
  }
}
