import { test, expect } from '@playwright/test'
import { createClient } from '@supabase/supabase-js'

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || 'http://localhost:8080'
const SUPABASE_ANON_KEY = process.env.VITE_SUPABASE_ANON_KEY || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE3Njg3MTY2NDUsImlzcyI6InNibGl0ZSIsInJvbGUiOiJhbm9uIn0.HgBiWMyahpSwGurQpvKmfqrCY70GWkhvzO8eHTnPzg8'
const SUPABASE_SERVICE_KEY = process.env.VITE_SUPABASE_SERVICE_KEY || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpYXQiOjE3Njg3MTYyNzksImlzcyI6InNibGl0ZSIsInJvbGUiOiJzZXJ2aWNlX3JvbGUifQ.aVNuDYg5y5uRMOEkvJU01F4Uxix3CihRXeq9l_v8_hg'

// Service role client for test setup/cleanup
const adminSupabase = createClient(SUPABASE_URL, SUPABASE_SERVICE_KEY)

// Generate unique test email
function testEmail(prefix = 'admin-test') {
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}@example.com`
}

const TEST_PASSWORD = 'TestPassword123!'

// Clean up all users and roles before tests
async function cleanupDatabase() {
  // Delete all user_roles
  await adminSupabase
    .from('user_roles')
    .delete()
    .neq('user_id', 'none')

  // Delete all users
  await adminSupabase
    .from('auth_users')
    .delete()
    .neq('id', 'none')
}

// Get confirmation token for a user by email
async function getConfirmationToken(email) {
  const { data } = await adminSupabase
    .from('auth_users')
    .select('confirmation_token')
    .eq('email', email)
    .single()

  return data?.confirmation_token
}

// Confirm user email by verifying the token
async function confirmUserEmail(token) {
  const response = await fetch(`${SUPABASE_URL}/auth/v1/verify`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'apikey': SUPABASE_ANON_KEY
    },
    body: JSON.stringify({ token, type: 'signup' })
  })

  if (!response.ok) {
    throw new Error(`Failed to confirm email: ${response.statusText}`)
  }

  return response.json()
}

// Get role for a user
async function getUserRole(userId) {
  const { data } = await adminSupabase
    .from('user_roles')
    .select('role')
    .eq('user_id', userId)
    .maybeSingle()

  return data?.role || null
}

// Get user count
async function getUserCount() {
  const { count } = await adminSupabase
    .from('auth_users')
    .select('*', { count: 'exact', head: true })

  return count || 0
}

// Tests run serially to control user creation order
test.describe.serial('Admin Roles', () => {
  let firstUserEmail
  let firstUserId
  let secondUserEmail
  let secondUserId

  test.beforeAll(async () => {
    // Clean database before all tests
    await cleanupDatabase()

    // Verify clean state
    const count = await getUserCount()
    expect(count).toBe(0)
  })

  test.afterAll(async () => {
    // Clean up after tests
    await cleanupDatabase()
  })

  test('first user is promoted to admin after registration', async ({ page }) => {
    firstUserEmail = testEmail('first')

    // Navigate to register page
    await page.goto('/register')

    // Fill form
    await page.getByTestId('email-input').fill(firstUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('confirm-password-input').fill(TEST_PASSWORD)

    // Submit
    await page.getByTestId('submit-button').click()

    // Wait for the signup to complete (user created but not confirmed)
    // The page may redirect to home or show a confirmation message
    await page.waitForTimeout(500)

    // Get the confirmation token from the database
    const token = await getConfirmationToken(firstUserEmail)
    expect(token).toBeTruthy()

    // Confirm the email
    await confirmUserEmail(token)

    // Now sign in via the login page
    await page.goto('/login')
    await page.getByTestId('email-input').fill(firstUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('submit-button').click()

    // Should redirect to home
    await expect(page).toHaveURL('/')

    // Should show sign out button (logged in)
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Wait for role to be fetched and displayed
    await expect(page.locator('.user-role')).toBeVisible({ timeout: 5000 })

    // First user should be Admin
    await expect(page.locator('.user-role')).toHaveText('Admin')

    // Get the user ID for later verification
    const { data: users } = await adminSupabase
      .from('auth_users')
      .select('id')
      .eq('email', firstUserEmail)
      .single()

    firstUserId = users?.id

    // Verify in database
    const role = await getUserRole(firstUserId)
    expect(role).toBe('admin')
  })

  test('second user is NOT promoted to admin (stays customer)', async ({ page }) => {
    secondUserEmail = testEmail('second')

    // Navigate to register page
    await page.goto('/register')

    // Fill form
    await page.getByTestId('email-input').fill(secondUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('confirm-password-input').fill(TEST_PASSWORD)

    // Submit
    await page.getByTestId('submit-button').click()

    // Wait for the signup to complete
    await page.waitForTimeout(500)

    // Get the confirmation token and confirm
    const token = await getConfirmationToken(secondUserEmail)
    expect(token).toBeTruthy()
    await confirmUserEmail(token)

    // Now sign in via the login page
    await page.goto('/login')
    await page.getByTestId('email-input').fill(secondUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('submit-button').click()

    // Should redirect to home
    await expect(page).toHaveURL('/')

    // Should show sign out button (logged in)
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Wait for role to be displayed
    await expect(page.locator('.user-role')).toBeVisible({ timeout: 5000 })

    // Second user should be Customer (not Admin)
    await expect(page.locator('.user-role')).toHaveText('Customer')

    // Get the user ID for verification
    const { data: users } = await adminSupabase
      .from('auth_users')
      .select('id')
      .eq('email', secondUserEmail)
      .single()

    secondUserId = users?.id

    // Verify in database - should have no role entry
    const role = await getUserRole(secondUserId)
    expect(role).toBeNull()
  })

  test('user info displays correctly in navbar', async ({ page }) => {
    // Sign in as first user (admin)
    await page.goto('/login')
    await page.getByTestId('email-input').fill(firstUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('submit-button').click()

    // Should be on home page
    await expect(page).toHaveURL('/')

    // Should show user info
    await expect(page.locator('.user-info')).toBeVisible({ timeout: 5000 })

    // Should display email (since no name set)
    await expect(page.locator('.user-info')).toContainText(firstUserEmail)

    // Should show Admin role
    await expect(page.locator('.user-role')).toHaveText('Admin')
  })

  test('sign out clears session and redirects correctly', async ({ page }) => {
    // Sign in
    await page.goto('/login')
    await page.getByTestId('email-input').fill(firstUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('submit-button').click()

    // Should be logged in
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Click sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Should show sign in link (logged out)
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible()

    // User info should not be visible
    await expect(page.locator('.user-info')).not.toBeVisible()

    // Cart link should not be visible (protected)
    await expect(page.getByRole('link', { name: 'Cart' })).not.toBeVisible()
  })

  test('session is cleared after sign out and page reload', async ({ page }) => {
    // Sign in
    await page.goto('/login')
    await page.getByTestId('email-input').fill(firstUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('submit-button').click()

    // Should be logged in
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Should be logged out
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible()

    // Reload the page
    await page.reload()

    // Should still be logged out after reload
    await expect(page.getByRole('link', { name: 'Sign In' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Sign Out' })).not.toBeVisible()
  })

  test('admin role persists across page reloads', async ({ page }) => {
    // Verify the admin role still exists in database
    const roleBeforeTest = await getUserRole(firstUserId)
    expect(roleBeforeTest).toBe('admin')

    // Sign in as first user (admin)
    await page.goto('/login')
    await page.getByTestId('email-input').fill(firstUserEmail)
    await page.getByTestId('password-input').fill(TEST_PASSWORD)
    await page.getByTestId('submit-button').click()

    // Should be logged in
    await expect(page).toHaveURL('/')
    await expect(page.getByRole('button', { name: 'Sign Out' })).toBeVisible()

    // Wait for role to load - should be Admin
    await expect(page.locator('.user-role')).toHaveText('Admin', { timeout: 10000 })

    // Reload the page
    await page.reload()

    // Should still show Admin role after reload
    await expect(page.locator('.user-role')).toBeVisible({ timeout: 5000 })
    await expect(page.locator('.user-role')).toHaveText('Admin', { timeout: 10000 })
  })
})
