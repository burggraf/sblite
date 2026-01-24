import { test, expect } from '@playwright/test'
import { testEmail, TEST_PASSWORD, registerAndConfirmViaUI } from '../helpers.js'
import { waitForEmail, extractVerificationUrl, extractToken, extractResetPasswordUrl, clearAllEmails } from '../mail-helpers.js'

test.describe('Forgot Password', () => {
  test.beforeEach(async () => {
    // Clear any existing emails before each test
    await clearAllEmails()
  })

  test('forgot password link is visible on login page', async ({ page }) => {
    await page.goto('/login')

    // Check that forgot password link exists
    await expect(page.getByTestId('forgot-password-link')).toBeVisible()
    await expect(page.getByTestId('forgot-password-link')).toHaveText('Forgot password?')
  })

  test('clicking forgot password shows reset form', async ({ page }) => {
    await page.goto('/login')

    // Click forgot password link
    await page.getByTestId('forgot-password-link').click()

    // Should show reset password form
    await expect(page.locator('.auth-title')).toHaveText('Reset Password')
    await expect(page.getByTestId('forgot-email-input')).toBeVisible()
    await expect(page.getByTestId('forgot-submit-button')).toBeVisible()
  })

  test('can go back to login from forgot password', async ({ page }) => {
    await page.goto('/login')

    // Go to forgot password view
    await page.getByTestId('forgot-password-link').click()
    await expect(page.locator('.auth-title')).toHaveText('Reset Password')

    // Click back to sign in
    await page.getByRole('button', { name: 'Back to Sign In' }).click()

    // Should be back at login
    await expect(page.locator('.auth-title')).toHaveText('Sign In')
    await expect(page.getByTestId('password-input')).toBeVisible()
  })

  test('submitting forgot password shows success message', async ({ page }) => {
    const email = testEmail()

    await page.goto('/login')

    // Go to forgot password view
    await page.getByTestId('forgot-password-link').click()

    // Fill email and submit
    await page.getByTestId('forgot-email-input').fill(email)
    await page.getByTestId('forgot-submit-button').click()

    // Should show success message (regardless of whether email exists)
    await expect(page.locator('.alert-success')).toBeVisible()
    await expect(page.locator('.alert-success')).toContainText('password reset link')
  })

  test('email is pre-filled when switching to forgot password', async ({ page }) => {
    const email = testEmail()

    await page.goto('/login')

    // Enter email in login form
    await page.getByTestId('email-input').fill(email)

    // Switch to forgot password
    await page.getByTestId('forgot-password-link').click()

    // Email should be pre-filled
    await expect(page.getByTestId('forgot-email-input')).toHaveValue(email)
  })

  test('full password reset flow with catch mode email', async ({ page }) => {
    const email = testEmail()
    const newPassword = 'NewSecurePassword456!'

    // First register a user
    await registerAndConfirmViaUI(page, email, TEST_PASSWORD)

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()
    await page.waitForURL('/')

    // Clear emails before requesting reset
    await clearAllEmails()

    // Request password reset
    await page.goto('/login')
    await page.getByTestId('forgot-password-link').click()
    await page.getByTestId('forgot-email-input').fill(email)
    await page.getByTestId('forgot-submit-button').click()

    // Wait for success message
    await expect(page.locator('.alert-success')).toBeVisible()

    // Wait for recovery email to arrive
    const recoveryEmail = await waitForEmail(email, 'recovery', 10000)
    expect(recoveryEmail).toBeTruthy()
    expect(recoveryEmail.to).toBe(email)

    // Extract the token from the email
    const token = extractToken(recoveryEmail)
    expect(token).toBeTruthy()

    // Call the verify endpoint directly with the new password
    // (sblite requires password with the recovery token)
    const verifyResponse = await fetch(`http://localhost:8080/auth/v1/verify`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'recovery',
        token: token,
        password: newPassword
      })
    })
    expect(verifyResponse.ok).toBeTruthy()

    // Sign in with new password
    await page.goto('/login')
    await page.getByTestId('email-input').fill(email)
    await page.getByTestId('password-input').fill(newPassword)
    await page.getByTestId('submit-button').click()

    // Should successfully sign in
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })

  test('full password reset flow via frontend UI', async ({ page }) => {
    const email = testEmail()
    const newPassword = 'NewSecurePassword789!'

    // First register a user
    await registerAndConfirmViaUI(page, email, TEST_PASSWORD)

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()
    await page.waitForURL('/')

    // Clear emails before requesting reset
    await clearAllEmails()

    // Request password reset
    await page.goto('/login')
    await page.getByTestId('forgot-password-link').click()
    await page.getByTestId('forgot-email-input').fill(email)
    await page.getByTestId('forgot-submit-button').click()

    // Wait for success message
    await expect(page.locator('.alert-success')).toBeVisible()

    // Wait for recovery email to arrive
    const recoveryEmail = await waitForEmail(email, 'recovery', 10000)
    expect(recoveryEmail).toBeTruthy()

    // Extract the reset password URL (frontend URL)
    const resetUrl = extractResetPasswordUrl(recoveryEmail)
    expect(resetUrl).toBeTruthy()
    expect(resetUrl).toContain('/reset-password')
    expect(resetUrl).toContain('token=')
    expect(resetUrl).toContain('type=recovery')

    // Navigate to the reset password page
    await page.goto(resetUrl)

    // Should show the reset password form
    await expect(page.locator('.auth-title')).toHaveText('Reset Password')
    await expect(page.getByTestId('new-password-input')).toBeVisible()
    await expect(page.getByTestId('confirm-password-input')).toBeVisible()

    // Fill in new password
    await page.getByTestId('new-password-input').fill(newPassword)
    await page.getByTestId('confirm-password-input').fill(newPassword)
    await page.getByTestId('reset-submit-button').click()

    // Should show success message
    await expect(page.locator('.auth-title')).toHaveText('Password Updated')
    await expect(page.locator('.alert-success')).toContainText('successfully updated')

    // Sign in with new password
    await page.goto('/login')
    await page.getByTestId('email-input').fill(email)
    await page.getByTestId('password-input').fill(newPassword)
    await page.getByTestId('submit-button').click()

    // Should successfully sign in
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })

  test('password reset validates password length via API', async ({ page }) => {
    const email = testEmail()

    // Register a user
    await registerAndConfirmViaUI(page, email, TEST_PASSWORD)
    await page.getByRole('button', { name: 'Sign Out' }).click()
    await clearAllEmails()

    // Request password reset
    await page.goto('/login')
    await page.getByTestId('forgot-password-link').click()
    await page.getByTestId('forgot-email-input').fill(email)
    await page.getByTestId('forgot-submit-button').click()

    // Wait for email
    const recoveryEmail = await waitForEmail(email, 'recovery', 10000)
    const token = extractToken(recoveryEmail)

    // Try to reset with short password
    const response = await fetch('http://localhost:8080/auth/v1/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'recovery',
        token: token,
        password: 'short'
      })
    })

    // Should fail with validation error
    expect(response.status).toBe(400)
    const data = await response.json()
    expect(data.message).toContain('6 characters')
  })

  test('old password stops working after reset', async ({ page }) => {
    const email = testEmail()
    const newPassword = 'NewSecurePassword456!'

    // Register a user
    await registerAndConfirmViaUI(page, email, TEST_PASSWORD)
    await page.getByRole('button', { name: 'Sign Out' }).click()
    await clearAllEmails()

    // Request password reset
    await page.goto('/login')
    await page.getByTestId('forgot-password-link').click()
    await page.getByTestId('forgot-email-input').fill(email)
    await page.getByTestId('forgot-submit-button').click()

    const recoveryEmail = await waitForEmail(email, 'recovery', 10000)
    const token = extractToken(recoveryEmail)

    // Reset password via API
    const verifyResponse = await fetch('http://localhost:8080/auth/v1/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        type: 'recovery',
        token: token,
        password: newPassword
      })
    })
    expect(verifyResponse.ok).toBeTruthy()

    // Try to sign in with OLD password
    await page.goto('/login')
    await page.getByTestId('email-input').fill(email)
    await page.getByTestId('password-input').fill(TEST_PASSWORD) // Old password
    await page.getByTestId('submit-button').click()

    // Should show error (invalid credentials)
    await expect(page.locator('.alert-error')).toBeVisible()
    await expect(page).toHaveURL('/login')

    // Now sign in with new password to verify it works
    await page.getByTestId('password-input').fill(newPassword)
    await page.getByTestId('submit-button').click()
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })
})
