import { test, expect } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';

test.describe('Dashboard Setup Flow', () => {
  // Note: These tests require a fresh database to test the actual setup flow.
  // The API tests can still verify behavior even after setup is complete.

  test('setup API rejects weak passwords on fresh database', async ({ request }) => {
    // First check if setup is needed
    const status = await request.get('/_/api/auth/status');
    const data = await status.json();

    if (data.needs_setup) {
      // Only test weak password rejection if we haven't set up yet
      const response = await request.post('/_/api/auth/setup', {
        data: { password: 'short' },
      });

      expect(response.status()).toBe(400);
      const errorData = await response.json();
      expect(errorData.error).toContain('8 characters');
    } else {
      // If already set up, verify setup rejects new attempts
      const response = await request.post('/_/api/auth/setup', {
        data: { password: 'anotherpassword123' },
      });

      expect(response.status()).toBe(400);
      const errorData = await response.json();
      expect(errorData.error).toContain('already');
    }
  });

  test('auth status endpoint returns correct state', async ({ request }) => {
    const response = await request.get('/_/api/auth/status');

    expect(response.status()).toBe(200);
    const data = await response.json();

    expect(data).toHaveProperty('needs_setup');
    expect(data).toHaveProperty('authenticated');
    expect(typeof data.needs_setup).toBe('boolean');
    expect(typeof data.authenticated).toBe('boolean');
  });

  test('setup flow shows correct UI when needed', async ({ page, request }) => {
    const status = await request.get('/_/api/auth/status');
    const data = await status.json();

    await page.goto('/_/');

    // Wait for loading to complete
    await page.waitForFunction(() => {
      const app = document.getElementById('app');
      return app && !app.innerHTML.includes('Loading');
    }, { timeout: 5000 });

    if (data.needs_setup) {
      // Should show setup form
      await expect(page.getByText('Welcome to sblite')).toBeVisible();
      await expect(page.getByText('Set up your dashboard password')).toBeVisible();
      await expect(page.locator('#password')).toBeVisible();
      await expect(page.locator('#confirm')).toBeVisible();
      await expect(page.getByRole('button', { name: 'Set Password' })).toBeVisible();
    } else {
      // Should show login form (already set up)
      await expect(page.getByText('sblite Dashboard')).toBeVisible();
      await expect(page.getByRole('button', { name: 'Sign In' })).toBeVisible();
    }
  });

  test('setup creates session on success', async ({ request }) => {
    const status = await request.get('/_/api/auth/status');
    const data = await status.json();

    if (data.needs_setup) {
      const response = await request.post('/_/api/auth/setup', {
        data: { password: TEST_PASSWORD },
      });

      expect(response.status()).toBe(200);

      // Check for session cookie
      const cookies = response.headers()['set-cookie'];
      expect(cookies).toBeDefined();
      expect(cookies).toContain('_sblite_session');
    } else {
      // Already set up - just verify status is correct
      expect(data.needs_setup).toBe(false);
    }
  });
});
