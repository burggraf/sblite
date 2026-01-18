import { test, expect } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';

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

test.describe('Settings View', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('navigates to Settings section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'Settings' })).toBeVisible();
    await expect(page.locator('.settings-view')).toBeVisible();
  });

  test('displays server information section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Server section should be expanded by default
    await expect(page.locator('.section-header').filter({ hasText: 'Server Information' })).toBeVisible();
    await expect(page.locator('.info-item').filter({ hasText: 'Version' }).first()).toBeVisible();
    await expect(page.locator('.info-item').filter({ hasText: 'Host' })).toBeVisible();
    await expect(page.locator('.info-item').filter({ hasText: 'Database' })).toBeVisible();
  });

  test('displays uptime and memory stats', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    await expect(page.locator('.info-item').filter({ hasText: 'Uptime' })).toBeVisible();
    await expect(page.locator('.info-item').filter({ hasText: 'Memory' })).toBeVisible();
    await expect(page.locator('.info-item').filter({ hasText: 'Goroutines' })).toBeVisible();
  });

  test('can expand and collapse sections', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Click on Authentication section to expand
    await page.locator('.section-header').filter({ hasText: 'Authentication' }).click();

    // Should show JWT secret info
    await expect(page.locator('.info-item').filter({ hasText: 'JWT Secret' })).toBeVisible();

    // Click again to collapse
    await page.locator('.section-header').filter({ hasText: 'Authentication' }).click();

    // JWT info should be hidden
    await expect(page.locator('.info-item').filter({ hasText: 'JWT Secret' })).not.toBeVisible();
  });

  test('displays masked JWT secret', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Expand auth section
    await page.locator('.section-header').filter({ hasText: 'Authentication' }).click();

    // Should show masked secret
    await expect(page.locator('.info-item').filter({ hasText: 'JWT Secret' })).toBeVisible();
    const secretText = await page.locator('.info-item').filter({ hasText: 'JWT Secret' }).locator('span.mono').textContent();
    expect(secretText).toMatch(/^\*\*\*|using default/);
  });

  test('lists email templates', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Expand templates section
    await page.locator('.section-header').filter({ hasText: 'Email Templates' }).click();

    // Should show template types
    await expect(page.locator('.template-item').filter({ hasText: 'confirmation' })).toBeVisible();
    await expect(page.locator('.template-item').filter({ hasText: 'recovery' })).toBeVisible();
    await expect(page.locator('.template-item').filter({ hasText: 'invite' })).toBeVisible();
  });

  test('can edit email template', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Expand templates section
    await page.locator('.section-header').filter({ hasText: 'Email Templates' }).click();

    // Click edit on confirmation template
    await page.locator('.template-item').filter({ hasText: 'confirmation' }).getByRole('button', { name: 'Edit' }).click();

    // Should show edit form
    await expect(page.locator('.template-item.editing')).toBeVisible();
    await expect(page.locator('.template-item.editing input[type="text"]')).toBeVisible();
    await expect(page.locator('.template-item.editing textarea').first()).toBeVisible();
  });

  test('can cancel template editing', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Expand templates section
    await page.locator('.section-header').filter({ hasText: 'Email Templates' }).click();

    // Click edit
    await page.locator('.template-item').filter({ hasText: 'confirmation' }).getByRole('button', { name: 'Edit' }).click();
    await expect(page.locator('.template-item.editing')).toBeVisible();

    // Click cancel
    await page.getByRole('button', { name: 'Cancel' }).click();

    // Should hide edit form
    await expect(page.locator('.template-item.editing')).not.toBeVisible();
  });

  test('displays export section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Expand export section
    await page.locator('.section-header').filter({ hasText: 'Export & Backup' }).click();

    // Should show export buttons
    await expect(page.getByRole('button', { name: 'Export PostgreSQL Schema' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Export Data (JSON)' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Download Database Backup' })).toBeVisible();
  });

  test('shows regenerate secret modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Settings' }).click();
    await page.waitForSelector('.settings-view');

    // Expand auth section
    await page.locator('.section-header').filter({ hasText: 'Authentication' }).click();

    // Click regenerate button (if visible - may not be visible if secret is from env)
    const regenerateBtn = page.getByRole('button', { name: 'Regenerate JWT Secret' });
    if (await regenerateBtn.isVisible()) {
      await regenerateBtn.click();

      // Should show modal with warning
      await expect(page.locator('.modal')).toBeVisible();
      await expect(page.locator('.modal').getByText('Warning')).toBeVisible();
      await expect(page.locator('.modal input[type="text"]')).toBeVisible();

      // Cancel
      await page.locator('.modal').getByRole('button', { name: 'Cancel' }).click();
      await expect(page.locator('.modal')).not.toBeVisible();
    }
  });
});

test.describe('Settings API', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('GET /settings/server returns server info', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/settings/server');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('version');
    expect(data).toHaveProperty('host');
    expect(data).toHaveProperty('port');
    expect(data).toHaveProperty('uptime_seconds');
    expect(data).toHaveProperty('memory_mb');
  });

  test('GET /settings/auth returns auth settings', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/settings/auth');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('jwt_secret_masked');
    expect(data).toHaveProperty('jwt_secret_source');
    expect(data).toHaveProperty('access_token_expiry');
  });

  test('GET /settings/templates returns templates list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/settings/templates');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(Array.isArray(data)).toBeTruthy();
    expect(data.length).toBeGreaterThan(0);

    const types = data.map((t: any) => t.type);
    expect(types).toContain('confirmation');
    expect(types).toContain('recovery');
  });

  test('PATCH /settings/templates/:type updates template', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Update and verify
    const result = await page.evaluate(async () => {
      // Get current template
      const listRes = await fetch('/_/api/settings/templates');
      const templates = await listRes.json();
      const original = templates.find((t: any) => t.type === 'confirmation');

      // Update template
      const updateRes = await fetch('/_/api/settings/templates/confirmation', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          subject: 'Test Subject',
          body_html: original.body_html,
          body_text: original.body_text
        })
      });

      if (!updateRes.ok) return { success: false };

      // Verify change
      const verifyRes = await fetch('/_/api/settings/templates');
      const updated = (await verifyRes.json()).find((t: any) => t.type === 'confirmation');

      // Reset to original
      await fetch('/_/api/settings/templates/confirmation/reset', { method: 'POST' });

      return { success: true, subject: updated.subject };
    });

    expect(result.success).toBeTruthy();
    expect(result.subject).toBe('Test Subject');
  });

  test('POST /settings/templates/:type/reset resets template', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/settings/templates/confirmation/reset', { method: 'POST' });
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data.success).toBeTruthy();
    expect(data.subject).toBe('Confirm your email');
  });

  test('GET /export/schema returns SQL', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const result = await page.evaluate(async () => {
      const res = await fetch('/_/api/export/schema');
      return {
        ok: res.ok,
        contentType: res.headers.get('content-type')
      };
    });

    expect(result.ok).toBeTruthy();
    expect(result.contentType).toContain('application/sql');
  });
});
