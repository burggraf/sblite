import { test, expect, Page, BrowserContext } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';

async function ensureSetup(request: any) {
  const status = await request.get('/_/api/auth/status');
  const data = await status.json();

  if (data.needs_setup) {
    await request.post('/_/api/auth/setup', {
      data: { password: TEST_PASSWORD },
    });
  }
}

async function login(page: Page, context: BrowserContext) {
  await context.clearCookies();
  await page.goto('/_/');

  // Wait for loading to complete
  await page.waitForFunction(() => {
    const app = document.getElementById('app');
    return app && !app.innerHTML.includes('Loading');
  }, { timeout: 5000 });

  // Check if we need to login
  const needsLogin = await page.locator('.auth-container').isVisible();
  if (needsLogin) {
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();
  }

  await page.waitForSelector('.sidebar', { timeout: 5000 });
}

test.describe('Dashboard Navigation', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('sidebar shows all navigation sections', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Check navigation sections
    await expect(page.getByText('Database')).toBeVisible();
    await expect(page.getByText('Auth')).toBeVisible();
    await expect(page.getByText('Security')).toBeVisible();
    await expect(page.getByText('System')).toBeVisible();

    // Check navigation items
    await expect(page.locator('.nav-item').filter({ hasText: 'Tables' })).toBeVisible();
    await expect(page.locator('.nav-item').filter({ hasText: 'Users' })).toBeVisible();
    await expect(page.locator('.nav-item').filter({ hasText: 'Policies' })).toBeVisible();
    await expect(page.locator('.nav-item').filter({ hasText: 'Settings' })).toBeVisible();
    await expect(page.locator('.nav-item').filter({ hasText: 'Logs' })).toBeVisible();
  });

  test('navigates to Tables view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Tables' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'Tables' })).toBeVisible();
  });

  test('navigates to Users view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'Users' })).toBeVisible();
  });

  test('navigates to Policies view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'RLS Policies' })).toBeVisible();
  });

  test('navigates to Settings view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'Settings' })).toBeVisible();
  });

  test('navigates to Logs view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'Logs' })).toBeVisible();
  });

  test('highlights active navigation item', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Tables should be active by default
    await expect(page.locator('.nav-item.active').filter({ hasText: 'Tables' })).toBeVisible();

    // Click Users
    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();

    // Users should now be active
    await expect(page.locator('.nav-item.active').filter({ hasText: 'Users' })).toBeVisible();

    // Tables should no longer be active
    await expect(page.locator('.nav-item.active').filter({ hasText: 'Tables' })).not.toBeVisible();
  });

  test('theme toggle switches themes', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Get initial theme
    const initialTheme = await page.evaluate(() => {
      return document.documentElement.getAttribute('data-theme');
    });

    // Click theme toggle
    await page.locator('.theme-toggle').click();

    // Theme should change
    const newTheme = await page.evaluate(() => {
      return document.documentElement.getAttribute('data-theme');
    });

    expect(newTheme).not.toBe(initialTheme);

    // Toggle back
    await page.locator('.theme-toggle').click();

    const restoredTheme = await page.evaluate(() => {
      return document.documentElement.getAttribute('data-theme');
    });

    expect(restoredTheme).toBe(initialTheme);
  });
});
