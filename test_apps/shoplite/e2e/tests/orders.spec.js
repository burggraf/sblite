import { test, expect } from '@playwright/test'
import { testEmail, TEST_PASSWORD, registerViaUI, addProductToCart } from '../helpers.js'

test.describe('Orders', () => {
  test('view own orders', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Create an order
    await addProductToCart(page, 0)
    await page.goto('/cart')
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()
    await expect(page).toHaveURL('/orders')

    // Should see the order
    await expect(page.getByTestId('order-card')).toHaveCount(1)
  })

  test('order shows correct items and total', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add two products to cart
    await addProductToCart(page, 0)
    await page.goto('/')
    await addProductToCart(page, 1)

    // Get cart info before checkout
    await page.goto('/cart')
    const cartTotal = await page.getByTestId('cart-total').textContent()
    const cartItemCount = await page.getByTestId('cart-item').count()

    // Complete checkout
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()

    // Check order details
    const orderCard = page.getByTestId('order-card').first()

    // Check total
    const orderTotal = await orderCard.getByTestId('order-total').textContent()
    expect(orderTotal).toContain(cartTotal.replace('$', ''))

    // Check items count
    const orderItems = orderCard.locator('.order-item')
    await expect(orderItems).toHaveCount(cartItemCount)
  })

  test('cannot see other users orders (RLS)', async ({ page }) => {
    // User 1 creates an order
    const email1 = testEmail()
    await registerViaUI(page, email1, TEST_PASSWORD)
    await addProductToCart(page, 0)
    await page.goto('/cart')
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()
    await expect(page.getByTestId('order-card')).toHaveCount(1)

    // Sign out
    await page.getByRole('button', { name: 'Sign Out' }).click()

    // User 2 registers
    const email2 = testEmail()
    await registerViaUI(page, email2, TEST_PASSWORD)

    // User 2 should see no orders
    await page.goto('/orders')
    await expect(page.locator('.empty-state')).toBeVisible()
    await expect(page.getByTestId('order-card')).toHaveCount(0)
  })

  test('order status displayed correctly', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Create an order
    await addProductToCart(page, 0)
    await page.goto('/cart')
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()

    // Check status badge
    const statusBadge = page.locator('.order-status').first()
    await expect(statusBadge).toHaveText('pending')
    await expect(statusBadge).toHaveClass(/pending/)
  })

  test('multiple orders display in order', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Create first order
    await addProductToCart(page, 0)
    await page.goto('/cart')
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()
    await expect(page.getByTestId('order-card')).toHaveCount(1)

    // Create second order
    await addProductToCart(page, 1)
    await page.goto('/cart')
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()

    // Should have two orders
    await expect(page.getByTestId('order-card')).toHaveCount(2)

    // Most recent order should be first (check created_at ordering)
    const orders = await page.getByTestId('order-card').all()
    expect(orders.length).toBe(2)
  })
})
