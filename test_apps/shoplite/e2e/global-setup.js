/**
 * Global setup for e2e tests
 *
 * Updates email templates to point to the frontend instead of backend
 */

import { execSync } from 'child_process'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const DB_PATH = process.env.SBLITE_DB_PATH || path.join(__dirname, '..', 'shoplite.db')

export default async function globalSetup() {
  console.log('Running global setup...')

  // Update the recovery email template to point to frontend
  const recoveryTemplate = `<h2>Reset your password</h2><p>Click the link below to reset your password:</p><p><a href="http://localhost:3000/reset-password?token={{.Token}}&type=recovery">Reset Password</a></p><p>This link expires in {{.ExpiresIn}}.</p>`

  try {
    execSync(`sqlite3 "${DB_PATH}" "UPDATE auth_email_templates SET body_html = '${recoveryTemplate}' WHERE type = 'recovery'"`, {
      encoding: 'utf-8'
    })
    console.log('Updated recovery email template')
  } catch (err) {
    console.warn('Failed to update recovery email template:', err.message)
  }

  console.log('Global setup complete')
}
