import { test, expect } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';

// Ensure setup is complete before auth tests
async function ensureSetup(request: any) {
  const status = await request.get('/_/api/auth/status');
  const data = await status.json();

  if (data.needs_setup) {
    await request.post('/_/api/auth/setup', {
      data: { password: TEST_PASSWORD },
    });
  }
}

test.describe('Dashboard Authentication', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows login form when not authenticated', async ({ page, request, context }) => {
    await ensureSetup(request);
    // Clear cookies to ensure we're logged out
    await context.clearCookies();
    await page.goto('/_/');

    // Wait for loading to complete
    await page.waitForSelector('.auth-container', { timeout: 5000 });

    // Should show login form
    await expect(page.getByText('sblite Dashboard')).toBeVisible();
    await expect(page.getByText('Enter your password to continue')).toBeVisible();
    await expect(page.locator('#password')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
  });

  test('successful login redirects to dashboard', async ({ page, request, context }) => {
    await ensureSetup(request);
    await context.clearCookies();
    await page.goto('/_/');
    await page.waitForSelector('.auth-container', { timeout: 5000 });

    // Fill in credentials
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();

    // Should redirect to dashboard
    await page.waitForSelector('.sidebar', { timeout: 5000 });
    await expect(page.locator('.logo')).toHaveText('sblite');
  });

  test('shows error for invalid password', async ({ page, request, context }) => {
    await ensureSetup(request);
    await context.clearCookies();
    await page.goto('/_/');
    await page.waitForSelector('.auth-container', { timeout: 5000 });

    // Try wrong password
    await page.locator('#password').fill('wrongpassword');
    await page.getByRole('button', { name: 'Sign In' }).click();

    // Should show error
    await expect(page.getByText('Invalid password')).toBeVisible({ timeout: 5000 });

    // Should stay on login page
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
  });

  test('logout returns to login page', async ({ page, request, context }) => {
    await ensureSetup(request);
    await context.clearCookies();

    // Login first
    await page.goto('/_/');
    await page.waitForSelector('.auth-container', { timeout: 5000 });
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();

    // Wait for dashboard
    await page.waitForSelector('.sidebar', { timeout: 5000 });

    // Click logout
    await page.getByText('Logout').click();

    // Should return to login page
    await page.waitForSelector('.auth-container', { timeout: 5000 });
    await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
  });

  test('session persists across page reloads', async ({ page, request, context }) => {
    await ensureSetup(request);
    await context.clearCookies();

    // Login
    await page.goto('/_/');
    await page.waitForSelector('.auth-container', { timeout: 5000 });
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();

    // Wait for dashboard
    await page.waitForSelector('.sidebar', { timeout: 5000 });

    // Reload page
    await page.reload();

    // Should still be on dashboard
    await page.waitForSelector('.sidebar', { timeout: 5000 });
    await expect(page.locator('.logo')).toHaveText('sblite');
  });

  test('login API returns session cookie', async ({ request }) => {
    await ensureSetup(request);

    const response = await request.post('/_/api/auth/login', {
      data: { password: TEST_PASSWORD },
    });

    expect(response.status()).toBe(200);

    // Check for session cookie
    const cookies = response.headers()['set-cookie'];
    expect(cookies).toBeDefined();
    expect(cookies).toContain('_sblite_session');
  });

  test('login API rejects wrong password', async ({ request }) => {
    await ensureSetup(request);

    const response = await request.post('/_/api/auth/login', {
      data: { password: 'wrongpassword' },
    });

    expect(response.status()).toBe(401);
    const data = await response.json();
    expect(data.error).toBe('Invalid password');
  });
});
