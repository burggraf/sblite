/**
 * Mail helpers for shoplite E2E tests
 *
 * Queries the auth_emails table directly via sqlite3 CLI.
 * Requires sblite to be running with --mail-mode=catch
 */

import { execSync } from 'child_process'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const DB_PATH = process.env.SBLITE_DB_PATH || path.join(__dirname, '..', 'shoplite.db')

/**
 * Run a sqlite3 query and return JSON results
 */
function query(sql) {
  try {
    const result = execSync(`sqlite3 -json "${DB_PATH}" "${sql}"`, {
      encoding: 'utf-8',
      timeout: 5000
    })
    return result.trim() ? JSON.parse(result) : []
  } catch (err) {
    // Empty result returns non-zero exit code
    if (err.stdout) {
      const out = err.stdout.trim()
      return out ? JSON.parse(out) : []
    }
    return []
  }
}

/**
 * Get all caught emails
 */
export function getEmails() {
  return query(`SELECT id, to_email as 'to', from_email as 'from', subject, email_type as type, body_html, body_text, created_at FROM auth_emails ORDER BY created_at DESC`)
}

/**
 * Clear all caught emails
 */
export function clearAllEmails() {
  execSync(`sqlite3 "${DB_PATH}" "DELETE FROM auth_emails"`, { encoding: 'utf-8' })
}

/**
 * Wait for an email to arrive
 * @param {string} to - Recipient email address
 * @param {string} type - Email type (confirmation, recovery, magic_link)
 * @param {number} timeout - Maximum wait time in ms (default: 5000)
 */
export async function waitForEmail(to, type, timeout = 5000) {
  const startTime = Date.now()
  const interval = 200

  while (Date.now() - startTime < timeout) {
    const emails = getEmails()
    const match = emails.find((email) => {
      return email.to === to && email.type === type
    })

    if (match) {
      return match
    }

    await new Promise((resolve) => setTimeout(resolve, interval))
  }

  throw new Error(`Email not found: to=${to}, type=${type} (waited ${timeout}ms)`)
}

/**
 * Extract verification token from email body
 * @param {object} email - Email object
 */
export function extractToken(email) {
  const body = email.body_html || email.body_text

  // Match token in query string
  const match = body.match(/token=([^&"\s<>]+)/)
  if (!match) return null

  // URL-decode the token
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}

/**
 * Extract the full verification URL from email body
 * @param {object} email - Email object
 */
export function extractVerificationUrl(email) {
  const body = email.body_html || email.body_text

  // Match href URLs containing /auth/v1/verify
  const hrefMatch = body.match(/href="([^"]*\/auth\/v1\/verify[^"]*)"/)
  if (hrefMatch) {
    // Decode HTML entities (e.g., &amp; -> &)
    return hrefMatch[1].replace(/&amp;/g, '&')
  }

  // Match plain URLs containing /auth/v1/verify
  const urlMatch = body.match(/(https?:\/\/[^\s<>"]*\/auth\/v1\/verify[^\s<>"]*)/)
  return urlMatch ? urlMatch[1].replace(/&amp;/g, '&') : null
}

/**
 * Extract the reset password URL from recovery email body (frontend URL)
 * @param {object} email - Email object
 */
export function extractResetPasswordUrl(email) {
  const body = email.body_html || email.body_text

  // Match href URLs containing /reset-password
  const hrefMatch = body.match(/href="([^"]*\/reset-password[^"]*)"/)
  if (hrefMatch) {
    // Decode HTML entities (e.g., &amp; -> &)
    return hrefMatch[1].replace(/&amp;/g, '&')
  }

  // Match plain URLs containing /reset-password
  const urlMatch = body.match(/(https?:\/\/[^\s<>"]*\/reset-password[^\s<>"]*)/)
  return urlMatch ? urlMatch[1].replace(/&amp;/g, '&') : null
}

/**
 * Confirm a user's email by calling the verification endpoint
 * @param {string} verificationUrl - The full verification URL from the email
 */
export async function confirmEmail(verificationUrl) {
  const response = await fetch(verificationUrl)
  if (!response.ok) {
    throw new Error(`Failed to confirm email: ${response.status}`)
  }
  return response
}
