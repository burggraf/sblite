import { test, expect } from '@playwright/test';

test.describe('Dashboard Loading', () => {
  test('serves index.html at dashboard root', async ({ page }) => {
    await page.goto('/_/');

    // Page should have the correct title
    await expect(page).toHaveTitle('sblite Dashboard');

    // App container should exist
    await expect(page.locator('#app')).toBeVisible();
  });

  test('loads CSS with correct MIME type', async ({ request }) => {
    const response = await request.get('/_/static/style.css');

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('text/css');
  });

  test('loads JavaScript with correct MIME type', async ({ request }) => {
    const response = await request.get('/_/static/app.js');

    expect(response.status()).toBe(200);
    expect(response.headers()['content-type']).toContain('application/javascript');
  });

  test('serves auth status API', async ({ request }) => {
    const response = await request.get('/_/api/auth/status');

    expect(response.status()).toBe(200);
    const data = await response.json();
    expect(data).toHaveProperty('needs_setup');
    expect(data).toHaveProperty('authenticated');
  });

  test('SPA routing serves index.html for unknown routes', async ({ page }) => {
    await page.goto('/_/some/unknown/route');

    // Should still serve index.html
    await expect(page).toHaveTitle('sblite Dashboard');
    await expect(page.locator('#app')).toBeVisible();
  });

  test('applies CSS styling', async ({ page }) => {
    await page.goto('/_/');

    // Wait for app to load
    await page.waitForFunction(() => {
      const app = document.getElementById('app');
      return app && app.innerHTML.length > 0;
    });

    // Body should have dark theme by default (background color from CSS)
    const bodyBg = await page.evaluate(() => {
      return window.getComputedStyle(document.body).backgroundColor;
    });

    // Dark theme background should not be white/default
    expect(bodyBg).not.toBe('rgba(0, 0, 0, 0)');
  });
});
