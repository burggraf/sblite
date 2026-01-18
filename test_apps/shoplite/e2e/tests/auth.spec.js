import { test, expect } from '@playwright/test'
import { testEmail, TEST_PASSWORD, registerViaUI, signInViaUI } from '../helpers.js'

test.describe('Authentication', () => {
  test('register new user', async ({ page }) => {
    const email = testEmail()

    await page.goto('/register')

    // Fill form
    await page.getByTestId('email-input').fill(email)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('confirm-password-input').fill(TEST_PASSWORD)

    // Submit
    await page.getByTestId('submit-button').click()

    // Should redirect to home
    await expect(page).toHaveURL('/')

    // Should show sign out button (logged in)
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })

  test('login with valid credentials', async ({ page }) => {
    const email = testEmail()

    // First register
    await registerViaUI(page, email, TEST_PASSWORD)

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Sign back in
    await signInViaUI(page, email, TEST_PASSWORD)

    // Should be on home page, signed in
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })

  test('login with invalid credentials shows error', async ({ page }) => {
    await page.goto('/login')

    await page.getByTestId('email-input').fill('nonexistent@example.com')
    await page.getByTestId('password-input').fill('wrongpassword')
    await page.getByTestId('submit-button').click()

    // Should show error
    await expect(page.locator('.alert-error')).toBeVisible()

    // Should stay on login page
    await expect(page).toHaveURL('/login')
  })

  test('logout clears session', async ({ page }) => {
    const email = testEmail()

    // Register and verify logged in
    await registerViaUI(page, email, TEST_PASSWORD)
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Should show sign in link (logged out)
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible()

    // Should not show cart or orders links
    await expect(page.getByRole('link', { name: 'Cart' })).not.toBeVisible()
  })

  test('session persists on page reload', async ({ page }) => {
    const email = testEmail()

    // Register
    await registerViaUI(page, email, TEST_PASSWORD)
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Reload page
    await page.reload()

    // Should still be logged in
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()
  })

  test('protected routes redirect to login', async ({ page }) => {
    // Try to access cart without logging in
    await page.goto('/cart')

    // Should redirect to login
    await expect(page).toHaveURL('/login')
  })
})
