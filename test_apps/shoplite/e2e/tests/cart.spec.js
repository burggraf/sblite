import { test, expect } from '@playwright/test'
import { testEmail, TEST_PASSWORD, registerViaUI, signInViaUI, addProductToCart } from '../helpers.js'

test.describe('Shopping Cart', () => {
  test('add product to cart requires auth', async ({ page }) => {
    await page.goto('/')

    // Click add to cart without being logged in
    const addButton = page.getByTestId('add-to-cart').first()
    await addButton.click()

    // Should redirect to login
    await expect(page).toHaveURL('/login')
  })

  test('add product to cart', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Go to home and add product
    await page.goto('/')
    const addButton = page.getByTestId('add-to-cart').first()
    await addButton.click()

    // Wait for added state
    await expect(addButton).toHaveText('Added!')

    // Check cart badge shows 1
    await expect(page.getByTestId('cart-badge')).toHaveText('1')
  })

  test('update cart item quantity', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add product to cart
    await addProductToCart(page, 0)

    // Go to cart
    await page.goto('/cart')

    // Get initial total
    const initialTotal = await page.getByTestId('cart-total').textContent()

    // Increase quantity
    await page.locator('.cart-item-actions button').nth(1).click() // + button

    // Wait for update
    await page.waitForTimeout(500)

    // Total should have changed
    const newTotal = await page.getByTestId('cart-total').textContent()
    expect(newTotal).not.toBe(initialTotal)

    // Quantity should be 2
    const quantityInput = page.getByTestId('quantity-input')
    await expect(quantityInput).toHaveValue('2')
  })

  test('remove item from cart', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add product to cart
    await addProductToCart(page, 0)

    // Go to cart
    await page.goto('/cart')

    // Should have one item
    await expect(page.getByTestId('cart-item')).toHaveCount(1)

    // Remove item
    await page.getByTestId('remove-item').click()

    // Should show empty state
    await expect(page.locator('.empty-state')).toBeVisible()
  })

  test('cart persists across sessions', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add product to cart
    await addProductToCart(page, 0)

    // Check cart badge
    await expect(page.getByTestId('cart-badge')).toHaveText('1')

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // Sign back in
    await signInViaUI(page, email, TEST_PASSWORD)

    // Cart should still have item
    await expect(page.getByTestId('cart-badge')).toHaveText('1')
  })

  test('cart is user-isolated (RLS)', async ({ page, context }) => {
    // User 1 adds item to cart
    const email1 = testEmail()
    await registerViaUI(page, email1, TEST_PASSWORD)
    await addProductToCart(page, 0)
    await expect(page.getByTestId('cart-badge')).toHaveText('1')

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // User 2 registers (should have empty cart)
    const email2 = testEmail()
    await registerViaUI(page, email2, TEST_PASSWORD)

    // User 2's cart should be empty (no badge)
    await expect(page.getByTestId('cart-badge')).not.toBeVisible()

    // Go to cart page
    await page.goto('/cart')
    await expect(page.locator('.empty-state')).toBeVisible()
  })
})
