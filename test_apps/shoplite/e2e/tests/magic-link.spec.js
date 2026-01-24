import { test, expect } from '@playwright/test'
import { testEmail, TEST_PASSWORD, registerAndConfirmViaUI } from '../helpers.js'
import { waitForEmail, extractVerificationUrl, clearAllEmails } from '../mail-helpers.js'

test.describe('Magic Link Login', () => {
  test.beforeEach(async () => {
    // Clear any existing emails before each test
    await clearAllEmails()
  })

  test('magic link button is visible on login page', async ({ page }) => {
    await page.goto('/login')

    // Check that magic link button exists
    await expect(page.getByTestId('magic-link-button')).toBeVisible()
    await expect(page.getByTestId('magic-link-button')).toHaveText('Sign in with Magic Link instead')
  })

  test('clicking magic link shows magic link form', async ({ page }) => {
    await page.goto('/login')

    // Click magic link button
    await page.getByTestId('magic-link-button').click()

    // Should show magic link form
    await expect(page.locator('.auth-title')).toHaveText('Sign In with Magic Link')
    await expect(page.getByTestId('magic-email-input')).toBeVisible()
    await expect(page.getByTestId('magic-submit-button')).toBeVisible()
  })

  test('can go back to login from magic link form', async ({ page }) => {
    await page.goto('/login')

    // Go to magic link view
    await page.getByTestId('magic-link-button').click()
    await expect(page.locator('.auth-title')).toHaveText('Sign In with Magic Link')

    // Click back to sign in
    await page.getByRole('button', { name: 'Back to Sign In' }).click()

    // Should be back at login
    await expect(page.locator('.auth-title')).toHaveText('Sign In')
    await expect(page.getByTestId('password-input')).toBeVisible()
  })

  test('submitting magic link shows success message', async ({ page }) => {
    const email = testEmail()

    await page.goto('/login')

    // Go to magic link view
    await page.getByTestId('magic-link-button').click()

    // Fill email and submit
    await page.getByTestId('magic-email-input').fill(email)
    await page.getByTestId('magic-submit-button').click()

    // Should show success message
    await expect(page.locator('.alert-success')).toBeVisible()
    await expect(page.locator('.alert-success')).toContainText('magic link')
    await expect(page.locator('.alert-success')).toContainText(email)
  })

  test('email is pre-filled when switching to magic link', async ({ page }) => {
    const email = testEmail()

    await page.goto('/login')

    // Enter email in login form
    await page.getByTestId('email-input').fill(email)

    // Switch to magic link
    await page.getByTestId('magic-link-button').click()

    // Email should be pre-filled
    await expect(page.getByTestId('magic-email-input')).toHaveValue(email)
  })

  test('magic link login for existing user', async ({ page }) => {
    const email = testEmail()

    // First register a user with password
    await registerAndConfirmViaUI(page, email, TEST_PASSWORD)

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()
    await page.waitForURL('/')

    // Clear any registration-related emails
    await clearAllEmails()

    // Request magic link
    await page.goto('/login')
    await page.getByTestId('magic-link-button').click()
    await page.getByTestId('magic-email-input').fill(email)
    await page.getByTestId('magic-submit-button').click()

    // Wait for success message
    await expect(page.locator('.alert-success')).toBeVisible()

    // Wait for magic link email to arrive
    const magicEmail = await waitForEmail(email, 'magic_link', 10000)
    expect(magicEmail).toBeTruthy()
    expect(magicEmail.to).toBe(email)

    // Extract the magic link URL
    const magicUrl = extractVerificationUrl(magicEmail)
    expect(magicUrl).toBeTruthy()

    // Navigate to the magic link URL
    // This should authenticate the user and redirect to home
    await page.goto(magicUrl)

    // Wait for auth to complete and Sign Out button to appear
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible({ timeout: 10000 })
  })

  test('magic link creates new user when email does not exist', async ({ page }) => {
    const email = testEmail()

    // Request magic link for non-existent user
    await page.goto('/login')
    await page.getByTestId('magic-link-button').click()
    await page.getByTestId('magic-email-input').fill(email)
    await page.getByTestId('magic-submit-button').click()

    // Wait for success message
    await expect(page.locator('.alert-success')).toBeVisible()

    // Wait for magic link email to arrive (user will be created)
    const magicEmail = await waitForEmail(email, 'magic_link', 10000)
    expect(magicEmail).toBeTruthy()

    // Extract and navigate to magic link
    const magicUrl = extractVerificationUrl(magicEmail)
    await page.goto(magicUrl)

    // Wait for auth to complete and Sign Out button to appear
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible({ timeout: 10000 })
  })

  test('magic link token can only be used once', async ({ page, context }) => {
    const email = testEmail()

    // Register a user
    await registerAndConfirmViaUI(page, email, TEST_PASSWORD)
    await page.getByRole('button', { name: 'Sign Out' }).click()
    await clearAllEmails()

    // Request magic link
    await page.goto('/login')
    await page.getByTestId('magic-link-button').click()
    await page.getByTestId('magic-email-input').fill(email)
    await page.getByTestId('magic-submit-button').click()

    // Get the magic link
    const magicEmail = await waitForEmail(email, 'magic_link', 10000)
    const magicUrl = extractVerificationUrl(magicEmail)

    // Use the magic link in first page
    await page.goto(magicUrl)
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible({ timeout: 10000 })

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Try to use the same magic link again
    await page.goto(magicUrl)

    // Should not work - either error page or redirect to login
    // The token is consumed after first use
    await page.waitForTimeout(1000)

    // Should not be signed in
    const signOutButton = page.getByRole('button', { name: 'Sign Out' })
    const isSignedIn = await signOutButton.isVisible().catch(() => false)

    // If signed in, it means the token worked again (which shouldn't happen for security)
    // For now, just verify the flow completes without crashing
    // The token reuse prevention depends on sblite implementation
    expect(isSignedIn).toBe(false)
  })

  test('magic link from password login retains email', async ({ page }) => {
    const email = testEmail()

    await page.goto('/login')

    // Fill in email on password login form
    await page.getByTestId('email-input').fill(email)
    await page.getByTestId('password-input').fill('somepassword')

    // Switch to magic link
    await page.getByTestId('magic-link-button').click()

    // Email should be preserved
    await expect(page.getByTestId('magic-email-input')).toHaveValue(email)

    // Switch back to login
    await page.getByRole('button', { name: 'Back to Sign In' }).click()

    // Email should still be preserved
    await expect(page.getByTestId('email-input')).toHaveValue(email)
  })

  test('magic link handles empty email validation', async ({ page }) => {
    await page.goto('/login')

    // Go to magic link view
    await page.getByTestId('magic-link-button').click()

    // Try to submit without email (HTML5 validation should prevent)
    const emailInput = page.getByTestId('magic-email-input')
    await expect(emailInput).toHaveAttribute('required', '')

    // Clear any value and try to submit
    await emailInput.fill('')
    await page.getByTestId('magic-submit-button').click()

    // Form should not submit, should still be on magic link view
    await expect(page.locator('.auth-title')).toHaveText('Sign In with Magic Link')
  })

  test('magic link with invalid email format shows error', async ({ page }) => {
    await page.goto('/login')

    // Go to magic link view
    await page.getByTestId('magic-link-button').click()

    // Try invalid email (HTML5 validation)
    const emailInput = page.getByTestId('magic-email-input')
    await emailInput.fill('invalid-email')
    await page.getByTestId('magic-submit-button').click()

    // HTML5 validation should prevent submission
    // Check that we're still on the form
    await expect(page.locator('.auth-title')).toHaveText('Sign In with Magic Link')
  })
})
