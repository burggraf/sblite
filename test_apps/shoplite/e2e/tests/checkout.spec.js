import { test, expect } from '@playwright/test'
import { testEmail, TEST_PASSWORD, registerViaUI, addProductToCart } from '../helpers.js'

test.describe('Checkout', () => {
  test('checkout creates order from cart', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add product to cart
    await addProductToCart(page, 0)

    // Go to cart
    await page.goto('/cart')

    // Click checkout
    await page.getByTestId('checkout-button').click()
    await expect(page).toHaveURL('/checkout')

    // Place order
    await page.getByTestId('place-order-button').click()

    // Should redirect to orders with success message
    await expect(page).toHaveURL('/orders')
    await expect(page.locator('.alert-success')).toBeVisible()
  })

  test('cart is cleared after checkout', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add product to cart
    await addProductToCart(page, 0)
    await expect(page.getByTestId('cart-badge')).toHaveText('1')

    // Complete checkout
    await page.goto('/cart')
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()
    await expect(page).toHaveURL('/orders')

    // Cart badge should be gone
    await expect(page.getByTestId('cart-badge')).not.toBeVisible()

    // Cart page should be empty
    await page.goto('/cart')
    await expect(page.locator('.empty-state')).toBeVisible()
  })

  test('order total matches cart total', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add products to cart
    await addProductToCart(page, 0)
    await page.goto('/')
    await addProductToCart(page, 1)

    // Go to cart and get total
    await page.goto('/cart')
    const cartTotal = await page.getByTestId('cart-total').textContent()

    // Go to checkout and verify total matches
    await page.getByTestId('checkout-button').click()
    const checkoutTotal = await page.getByTestId('checkout-total').textContent()

    expect(checkoutTotal).toBe(cartTotal)

    // Place order
    await page.getByTestId('place-order-button').click()
    await expect(page).toHaveURL('/orders')

    // Verify order total matches
    const orderTotal = await page.getByTestId('order-total').first().textContent()
    expect(orderTotal).toContain(cartTotal.replace('$', ''))
  })

  test('order items match cart items', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Add two products to cart
    await addProductToCart(page, 0)
    await page.goto('/')
    await addProductToCart(page, 1)

    // Go to cart and wait for items to load
    await page.goto('/cart')
    await page.waitForSelector('[data-testid="cart-item"]')
    const cartItemCount = await page.getByTestId('cart-item').count()

    // Complete checkout
    await page.getByTestId('checkout-button').click()
    await page.getByTestId('place-order-button').click()
    await expect(page).toHaveURL('/orders')

    // Order should have same number of items
    const orderCard = page.getByTestId('order-card').first()
    const orderItems = orderCard.locator('.order-item')
    await expect(orderItems).toHaveCount(cartItemCount)
  })

  test('empty cart redirects from checkout', async ({ page }) => {
    const email = testEmail()
    await registerViaUI(page, email, TEST_PASSWORD)

    // Try to go to checkout with empty cart
    await page.goto('/checkout')

    // Should redirect to cart
    await expect(page).toHaveURL('/cart')
  })
})
