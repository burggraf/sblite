/**
 * Anonymous Sign-In Settings E2E Tests
 *
 * Tests for the configurable anonymous sign-in feature via dashboard API.
 * By default, anonymous sign-in is enabled for ease of use.
 */

import { test, expect } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';
const BASE_URL = process.env.SBLITE_URL || 'http://localhost:8080';

// Generate API key for auth API calls
function generateAnonKey(): string {
  const jwt = require('jsonwebtoken');
  const secret = process.env.SBLITE_JWT_SECRET || 'super-secret-jwt-key-please-change-in-production';
  return jwt.sign(
    { role: 'anon', iss: 'sblite', iat: Math.floor(Date.now() / 1000) },
    secret
  );
}

async function ensureSetup(request: any) {
  const status = await request.get('/_/api/auth/status');
  const data = await status.json();
  if (data.needs_setup) {
    await request.post('/_/api/auth/setup', { data: { password: TEST_PASSWORD } });
  }
}

async function login(page: any, context: any) {
  await context.clearCookies();
  await page.goto('/_/');
  await page.waitForFunction(() => {
    const app = document.getElementById('app');
    return app && !app.innerHTML.includes('Loading');
  }, { timeout: 5000 });

  const needsLogin = await page.locator('.auth-container').isVisible();
  if (needsLogin) {
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();
  }
  await page.waitForSelector('.sidebar', { timeout: 5000 });
}

// API helpers using page context (which has auth cookies)
async function setAllowAnonymous(page: any, enabled: boolean): Promise<void> {
  const result = await page.evaluate(async (params: { enabled: boolean }) => {
    const res = await fetch('/_/api/settings/auth-config', {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ allow_anonymous: params.enabled }),
    });
    return { ok: res.ok, status: res.status };
  }, { enabled });

  if (!result.ok) {
    throw new Error(`Failed to set allow_anonymous: ${result.status}`);
  }
}

async function getAllowAnonymous(page: any): Promise<boolean> {
  return page.evaluate(async () => {
    const res = await fetch('/_/api/settings/auth-config');
    if (!res.ok) throw new Error('Failed to get auth config');
    const data = await res.json();
    return data.allow_anonymous;
  });
}

async function getAnonymousUserCount(page: any): Promise<number> {
  return page.evaluate(async () => {
    const res = await fetch('/_/api/settings/auth-config');
    if (!res.ok) throw new Error('Failed to get auth config');
    const data = await res.json();
    return data.anonymous_user_count || 0;
  });
}

async function listUsers(page: any, filter?: string): Promise<any> {
  return page.evaluate(async (params: { filter?: string }) => {
    const queryParam = params.filter ? `?filter=${params.filter}` : '';
    const res = await fetch(`/_/api/users${queryParam}`);
    if (!res.ok) throw new Error('Failed to list users');
    return res.json();
  }, { filter });
}

async function deleteUser(page: any, userId: string): Promise<void> {
  const result = await page.evaluate(async (params: { userId: string }) => {
    const res = await fetch(`/_/api/users/${params.userId}`, { method: 'DELETE' });
    return { ok: res.ok, status: res.status };
  }, { userId });

  if (!result.ok) {
    throw new Error(`Failed to delete user: ${result.status}`);
  }
}

// Auth API helper (uses fetch with apikey header, not cookies)
async function signInAnonymously(anonKey: string): Promise<{ data: any; error: any }> {
  const response = await fetch(`${BASE_URL}/auth/v1/signup`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'apikey': anonKey,
    },
    body: JSON.stringify({}),
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: 'Unknown error' }));
    return {
      data: { user: null, session: null },
      error: { status: response.status, message: error.message || error.error_description || 'Unknown error' },
    };
  }

  const data = await response.json();
  return { data, error: null };
}

test.describe('Anonymous Sign-In Settings', () => {
  let originalSetting: boolean;
  const createdUserIds: string[] = [];
  const anonKey = generateAnonKey();

  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test.afterAll(async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Restore original setting if we saved it
    if (originalSetting !== undefined) {
      await setAllowAnonymous(page, originalSetting);
    }

    // Clean up created users
    for (const userId of createdUserIds) {
      try {
        await deleteUser(page, userId);
      } catch {
        // Ignore errors
      }
    }
  });

  test.describe('Dashboard API - Settings Toggle', () => {
    test('GET /settings/auth-config returns allow_anonymous field', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      const data = await page.evaluate(async () => {
        const res = await fetch('/_/api/settings/auth-config');
        return res.ok ? await res.json() : null;
      });

      expect(data).not.toBeNull();
      expect(data).toHaveProperty('allow_anonymous');
      expect(typeof data.allow_anonymous).toBe('boolean');

      // Save original setting for restoration
      originalSetting = data.allow_anonymous;
    });

    test('GET /settings/auth-config returns anonymous_user_count', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      const data = await page.evaluate(async () => {
        const res = await fetch('/_/api/settings/auth-config');
        return res.ok ? await res.json() : null;
      });

      expect(data).not.toBeNull();
      expect(data).toHaveProperty('anonymous_user_count');
      expect(typeof data.anonymous_user_count).toBe('number');
    });

    test('toggle anonymous sign-in setting off', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      // Enable first
      await setAllowAnonymous(page, true);
      let setting = await getAllowAnonymous(page);
      expect(setting).toBe(true);

      // Disable
      await setAllowAnonymous(page, false);
      setting = await getAllowAnonymous(page);
      expect(setting).toBe(false);
    });

    test('toggle anonymous sign-in setting on', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      // Disable first
      await setAllowAnonymous(page, false);
      let setting = await getAllowAnonymous(page);
      expect(setting).toBe(false);

      // Enable
      await setAllowAnonymous(page, true);
      setting = await getAllowAnonymous(page);
      expect(setting).toBe(true);
    });
  });

  test.describe('Public Settings Endpoint', () => {
    test('shows external.anonymous=true when enabled', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, true);

      const response = await request.get('/auth/v1/settings');
      expect(response.ok()).toBe(true);

      const settings = await response.json();
      expect(settings.external).toBeDefined();
      expect(settings.external.anonymous).toBe(true);
    });

    test('shows external.anonymous=false when disabled', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, false);

      const response = await request.get('/auth/v1/settings');
      expect(response.ok()).toBe(true);

      const settings = await response.json();
      expect(settings.external).toBeDefined();
      expect(settings.external.anonymous).toBe(false);

      // Re-enable for other tests
      await setAllowAnonymous(page, true);
    });
  });

  test.describe('Anonymous Signup - Disabled', () => {
    test('returns 403 error when anonymous sign-in is disabled', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, false);

      const { data, error } = await signInAnonymously(anonKey);

      expect(error).not.toBeNull();
      expect(error.status).toBe(403);
      expect(data.user).toBeNull();
      expect(data.session).toBeNull();

      // Re-enable
      await setAllowAnonymous(page, true);
    });

    test('returns error message mentioning anonymous is disabled', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, false);

      const { error } = await signInAnonymously(anonKey);

      expect(error).not.toBeNull();
      expect(error.message.toLowerCase()).toContain('anonymous');

      // Re-enable
      await setAllowAnonymous(page, true);
    });
  });

  test.describe('Anonymous Signup - Enabled', () => {
    test('creates anonymous user when enabled', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, true);

      const { data, error } = await signInAnonymously(anonKey);

      expect(error).toBeNull();
      expect(data.user).toBeDefined();
      expect(data.user.is_anonymous).toBe(true);
      expect(data.session).toBeDefined();
      expect(data.session.access_token).toBeDefined();

      // Track for cleanup
      if (data.user?.id) {
        createdUserIds.push(data.user.id);
      }
    });

    test('increments anonymous user count after signup', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, true);

      const countBefore = await getAnonymousUserCount(page);

      const { data, error } = await signInAnonymously(anonKey);
      expect(error).toBeNull();

      // Track for cleanup
      if (data.user?.id) {
        createdUserIds.push(data.user.id);
      }

      const countAfter = await getAnonymousUserCount(page);
      expect(countAfter).toBe(countBefore + 1);
    });
  });

  test.describe('User List Filtering', () => {
    test('filter users list by anonymous', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      // Ensure we have at least one anonymous user
      await setAllowAnonymous(page, true);
      const { data } = await signInAnonymously(anonKey);
      if (data.user?.id) {
        createdUserIds.push(data.user.id);
      }

      const result = await listUsers(page, 'anonymous');

      expect(result.users).toBeDefined();
      expect(Array.isArray(result.users)).toBe(true);

      // All returned users should be anonymous
      for (const user of result.users) {
        expect(user.is_anonymous).toBe(true);
      }
    });

    test('filter users list by regular', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      const result = await listUsers(page, 'regular');

      expect(result.users).toBeDefined();
      expect(Array.isArray(result.users)).toBe(true);

      // All returned users should NOT be anonymous
      for (const user of result.users) {
        expect(user.is_anonymous).toBe(false);
      }
    });

    test('returns all users when no filter specified', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      const result = await listUsers(page);

      expect(result.users).toBeDefined();
      expect(Array.isArray(result.users)).toBe(true);
    });

    test('returns all users when filter=all', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      const result = await listUsers(page, 'all');

      expect(result.users).toBeDefined();
      expect(Array.isArray(result.users)).toBe(true);
    });

    test('includes is_anonymous field in user objects', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      const result = await listUsers(page);

      expect(result.users).toBeDefined();
      expect(result.users.length).toBeGreaterThan(0);

      for (const user of result.users) {
        expect(user).toHaveProperty('is_anonymous');
        expect(typeof user.is_anonymous).toBe('boolean');
      }
    });
  });

  test.describe('Delete Anonymous User', () => {
    test('delete anonymous user via dashboard API', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, true);

      // Create an anonymous user
      const { data, error } = await signInAnonymously(anonKey);
      expect(error).toBeNull();
      expect(data.user?.id).toBeDefined();

      const userId = data.user.id;

      // Delete the user
      await deleteUser(page, userId);

      // Verify user is deleted
      const result = await listUsers(page, 'anonymous');
      const userExists = result.users.some((u: any) => u.id === userId);
      expect(userExists).toBe(false);
    });

    test('decrements anonymous user count after deletion', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      await setAllowAnonymous(page, true);

      // Create an anonymous user
      const { data, error } = await signInAnonymously(anonKey);
      expect(error).toBeNull();
      const userId = data.user?.id;
      expect(userId).toBeDefined();

      const countBefore = await getAnonymousUserCount(page);

      // Delete the user
      await deleteUser(page, userId!);

      const countAfter = await getAnonymousUserCount(page);
      expect(countAfter).toBe(countBefore - 1);
    });
  });

  test.describe('Edge Cases', () => {
    test('allows creating anonymous user immediately after enabling', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      // Disable first
      await setAllowAnonymous(page, false);

      // Enable and immediately create
      await setAllowAnonymous(page, true);
      const { data, error } = await signInAnonymously(anonKey);

      expect(error).toBeNull();
      expect(data.user).toBeDefined();
      expect(data.user.is_anonymous).toBe(true);

      // Cleanup
      if (data.user?.id) {
        await deleteUser(page, data.user.id);
      }
    });

    test('blocks anonymous user immediately after disabling', async ({ page, request, context }) => {
      await ensureSetup(request);
      await login(page, context);

      // Enable first
      await setAllowAnonymous(page, true);

      // Disable and immediately try to create
      await setAllowAnonymous(page, false);
      const { data, error } = await signInAnonymously(anonKey);

      expect(error).not.toBeNull();
      expect(data.user).toBeNull();

      // Re-enable for other tests
      await setAllowAnonymous(page, true);
    });
  });
});
