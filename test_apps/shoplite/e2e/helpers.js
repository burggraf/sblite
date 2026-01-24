import { createClient } from '@supabase/supabase-js'

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || 'http://localhost:8080'
const SUPABASE_ANON_KEY = process.env.VITE_SUPABASE_ANON_KEY || 'test-anon-key'
const SUPABASE_SERVICE_KEY = process.env.VITE_SUPABASE_SERVICE_KEY || 'test-service-key'

// Anon client for regular user operations
export const supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY)

// Service role client for test setup/cleanup
export const adminSupabase = createClient(SUPABASE_URL, SUPABASE_SERVICE_KEY)

// Generate unique test email
export function testEmail() {
  return `test-${Date.now()}-${Math.random().toString(36).slice(2)}@example.com`
}

// Test password
export const TEST_PASSWORD = 'TestPassword123!'

// Create a test user and return credentials
export async function createTestUser() {
  const email = testEmail()
  const { data, error } = await supabase.auth.signUp({
    email,
    password: TEST_PASSWORD
  })

  if (error) throw error

  return {
    email,
    password: TEST_PASSWORD,
    user: data.user
  }
}

// Sign in as a test user via the UI
export async function signInViaUI(page, email, password) {
  await page.goto('/login')
  await page.getByTestId('email-input').fill(email)
  await page.getByTestId('password-input').fill(password)
  await page.getByTestId('submit-button').click()
  await page.waitForURL('/')
}

// Register a new user via the UI
// If email confirmation is required, this only submits the form
export async function registerViaUI(page, email, password) {
  await page.goto('/register')
  await page.getByTestId('email-input').fill(email)
  await page.getByTestId('password-input').fill(password)
  await page.getByTestId('confirm-password-input').fill(password)
  await page.getByTestId('submit-button').click()
  // Wait for either redirect to home (auto-confirm) or success message (needs confirmation)
  await Promise.race([
    page.waitForURL('/'),
    page.locator('.alert-success').waitFor({ timeout: 5000 }).catch(() => {})
  ])
}

// Register and confirm email, then sign in
// Use this when you need a fully authenticated user
import { waitForEmail, extractVerificationUrl, confirmEmail, clearAllEmails } from './mail-helpers.js'

export async function registerAndConfirmViaUI(page, email, password) {
  await clearAllEmails()
  await page.goto('/register')
  await page.getByTestId('email-input').fill(email)
  await page.getByTestId('password-input').fill(password)
  await page.getByTestId('confirm-password-input').fill(password)
  await page.getByTestId('submit-button').click()

  // Check if we got redirected (auto-confirm) or need to confirm email
  try {
    await page.waitForURL('/', { timeout: 2000 })
    // Auto-confirmed, we're done
    return
  } catch {
    // Need to confirm email - wait for confirmation message
  }

  // Wait for the confirmation message to appear
  await page.getByTestId('confirmation-message').waitFor({ timeout: 5000 })

  // Wait for confirmation email and confirm it
  const confirmationEmail = await waitForEmail(email, 'confirmation', 10000)
  const verificationUrl = extractVerificationUrl(confirmationEmail)
  await confirmEmail(verificationUrl)

  // Now sign in
  await signInViaUI(page, email, password)
}

// Add product to cart via UI
export async function addProductToCart(page, productIndex = 0) {
  await page.goto('/')
  const addButtons = page.getByTestId('add-to-cart')
  await addButtons.nth(productIndex).click()
  // Wait for "Added!" state
  await page.waitForTimeout(500)
}

// Clear user's cart using service role
export async function clearUserCart(userId) {
  await adminSupabase
    .from('cart_items')
    .delete()
    .eq('user_id', userId)
}

// Clear user's orders using service role
export async function clearUserOrders(userId) {
  // First get order IDs
  const { data: orders } = await adminSupabase
    .from('orders')
    .select('id')
    .eq('user_id', userId)

  if (orders && orders.length > 0) {
    const orderIds = orders.map(o => o.id)

    // Delete order items first
    await adminSupabase
      .from('order_items')
      .delete()
      .in('order_id', orderIds)

    // Then delete orders
    await adminSupabase
      .from('orders')
      .delete()
      .eq('user_id', userId)
  }
}
