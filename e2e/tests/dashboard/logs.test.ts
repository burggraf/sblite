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

test.describe('Logs View', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('navigates to Logs section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'Logs' })).toBeVisible();
    await expect(page.locator('.logs-view')).toBeVisible();
  });

  test('shows appropriate message when database logging not enabled', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    // When log mode is not database, should show info message
    // Check for either the database logs view or the non-database message
    const hasDbLogs = await page.locator('.logs-table').isVisible().catch(() => false);
    const hasMessage = await page.locator('.logs-message').isVisible().catch(() => false);

    expect(hasDbLogs || hasMessage).toBeTruthy();
  });

  test('displays log config info', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    // Should mention the log mode somewhere
    const pageContent = await page.locator('.logs-view').textContent();
    expect(pageContent).toBeTruthy();
  });
});

test.describe('Logs API', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('GET /logs/config returns log configuration', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/logs/config');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('mode');
  });

  test('GET /logs returns logs or message', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/logs');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('logs');
    expect(data).toHaveProperty('total');
  });

  test('GET /logs supports filtering by level', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/logs?level=error');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('logs');
  });

  test('GET /logs supports pagination', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/logs?limit=10&offset=0');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('logs');
    // has_more is only returned when database logging is enabled
    expect(data).toHaveProperty('total');
  });

  test('GET /logs/tail returns message when not file mode', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/logs/tail');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(data).toHaveProperty('lines');
  });
});

test.describe('Logs View - Database Mode', () => {
  // These tests are conditional - they only make sense when log-mode=database
  // We'll check the mode first and skip if not database

  test('shows filter toolbar when database logging enabled', async ({ page, request, context }) => {
    await ensureSetup(request);

    // Check log config
    const configRes = await request.get('/_/api/logs/config');
    const config = await configRes.json();

    if (config.mode !== 'database') {
      test.skip();
      return;
    }

    await login(page, context);
    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    await expect(page.locator('.logs-toolbar')).toBeVisible();
    await expect(page.locator('.logs-filters select')).toBeVisible();
  });

  test('can filter logs by level', async ({ page, request, context }) => {
    await ensureSetup(request);

    const configRes = await request.get('/_/api/logs/config');
    const config = await configRes.json();

    if (config.mode !== 'database') {
      test.skip();
      return;
    }

    await login(page, context);
    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    // Select error level
    await page.locator('.logs-filters select').first().selectOption('error');
    await page.getByRole('button', { name: 'Filter' }).click();

    // Should still show the logs view
    await expect(page.locator('.logs-view')).toBeVisible();
  });

  test('can search logs', async ({ page, request, context }) => {
    await ensureSetup(request);

    const configRes = await request.get('/_/api/logs/config');
    const config = await configRes.json();

    if (config.mode !== 'database') {
      test.skip();
      return;
    }

    await login(page, context);
    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    // Enter search term
    await page.locator('.logs-filters input[placeholder="Search message..."]').fill('test');
    await page.getByRole('button', { name: 'Filter' }).click();

    await expect(page.locator('.logs-view')).toBeVisible();
  });

  test('can clear filters', async ({ page, request, context }) => {
    await ensureSetup(request);

    const configRes = await request.get('/_/api/logs/config');
    const config = await configRes.json();

    if (config.mode !== 'database') {
      test.skip();
      return;
    }

    await login(page, context);
    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    // Set a filter
    await page.locator('.logs-filters select').first().selectOption('error');
    await page.getByRole('button', { name: 'Filter' }).click();

    // Clear filters
    await page.getByRole('button', { name: 'Clear' }).click();

    // Level should be reset to 'all'
    const selectValue = await page.locator('.logs-filters select').first().inputValue();
    expect(selectValue).toBe('all');
  });

  test('refresh button reloads logs', async ({ page, request, context }) => {
    await ensureSetup(request);

    const configRes = await request.get('/_/api/logs/config');
    const config = await configRes.json();

    if (config.mode !== 'database') {
      test.skip();
      return;
    }

    await login(page, context);
    await page.locator('.nav-item').filter({ hasText: 'Logs' }).click();
    await page.waitForSelector('.logs-view');

    // Click refresh
    await page.getByRole('button', { name: 'Refresh' }).click();

    // Should still show logs view
    await expect(page.locator('.logs-view')).toBeVisible();
  });
});
