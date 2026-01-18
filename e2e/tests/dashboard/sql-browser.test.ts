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

test.describe('SQL Browser View', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('navigates to SQL Browser section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'SQL Browser' })).toBeVisible();
    await expect(page.locator('.sql-browser-view')).toBeVisible();
  });

  test('displays SQL editor and results panel', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    await expect(page.locator('.sql-editor-container')).toBeVisible();
    await expect(page.locator('.sql-results-container')).toBeVisible();
  });

  test('shows table picker dropdown', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Click Tables button
    await page.getByRole('button', { name: 'Tables' }).click();
    await expect(page.locator('.table-picker-dropdown')).toBeVisible();
  });
});

test.describe('SQL Editor', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('can type SQL in editor', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT * FROM test_table');
    await expect(editor).toHaveValue('SELECT * FROM test_table');
  });

  test('syntax highlighting colors keywords', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT * FROM users WHERE id = 1');

    // Check that keywords are highlighted
    const highlight = page.locator('.sql-editor-highlight');
    await expect(highlight.locator('.sql-keyword').first()).toBeVisible();
  });

  test('clicking table in picker inserts name', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Clear editor and focus it
    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT * FROM ');
    await editor.focus();

    // Open table picker and click first table if any
    await page.getByRole('button', { name: 'Tables' }).click();
    const firstTable = page.locator('.picker-item').first();
    if (await firstTable.isVisible()) {
      const tableName = await firstTable.textContent();
      await firstTable.click();
      // Table name should be inserted
      const value = await editor.inputValue();
      expect(value).toContain(tableName?.trim() || '');
    }
  });
});

test.describe('Query Execution', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('SELECT query displays results table', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT name FROM sqlite_master WHERE type = "table" LIMIT 5');

    await page.getByRole('button', { name: 'Run Query' }).click();

    // Wait for results
    await expect(page.locator('.sql-results-table')).toBeVisible({ timeout: 10000 });
  });

  test('shows row count and execution time', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT 1 as test');

    await page.getByRole('button', { name: 'Run Query' }).click();

    // Wait for results header with row count and time
    await expect(page.locator('.sql-results-header')).toBeVisible({ timeout: 10000 });
    const headerText = await page.locator('.sql-results-header').textContent();
    expect(headerText).toMatch(/\d+ row/);
    expect(headerText).toMatch(/\d+ms/);
  });

  test('invalid SQL shows error message', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('INVALID SQL QUERY HERE');

    await page.getByRole('button', { name: 'Run Query' }).click();

    // Should show error
    await expect(page.locator('.sql-error')).toBeVisible({ timeout: 10000 });
  });

  test('empty query shows warning', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('');

    await page.getByRole('button', { name: 'Run Query' }).click();

    // Should show error about empty query
    await expect(page.locator('.sql-error')).toBeVisible({ timeout: 5000 });
  });
});

test.describe('Results Table', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('clicking column header sorts results', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT name, type FROM sqlite_master LIMIT 10');

    await page.getByRole('button', { name: 'Run Query' }).click();
    await expect(page.locator('.sql-results-table')).toBeVisible({ timeout: 10000 });

    // Click column header to sort
    const firstHeader = page.locator('.sql-results-table th').first();
    await firstHeader.click();

    // Should show sort indicator
    const headerText = await firstHeader.textContent();
    expect(headerText).toMatch(/▲|▼/);
  });

  test('export CSV button visible when results exist', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT 1 as test, 2 as value');

    await page.getByRole('button', { name: 'Run Query' }).click();
    await expect(page.locator('.sql-results-table')).toBeVisible({ timeout: 10000 });

    // Export buttons should be visible
    await expect(page.getByRole('button', { name: 'Export CSV' })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Export JSON' })).toBeVisible();
  });
});

test.describe('Destructive Query Protection', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('DELETE shows confirmation dialog', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('DELETE FROM nonexistent_table WHERE id = 1');

    await page.getByRole('button', { name: 'Run Query' }).click();

    // Should show confirmation modal
    await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.modal').getByText('Destructive Query')).toBeVisible();
  });

  test('canceling prevents execution', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    const editor = page.locator('#sql-editor-input');
    await editor.fill('DELETE FROM nonexistent_table WHERE id = 1');

    await page.getByRole('button', { name: 'Run Query' }).click();
    await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

    // Click cancel
    await page.locator('.modal').getByRole('button', { name: 'Cancel' }).click();

    // Modal should close, no error shown (query wasn't executed)
    await expect(page.locator('.modal')).not.toBeVisible();
  });
});

test.describe('History', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('query added to history after execution', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Clear history first
    await page.evaluate(() => localStorage.removeItem('sblite_sql_browser_history'));

    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT 1 as history_test');

    await page.getByRole('button', { name: 'Run Query' }).click();
    await expect(page.locator('.sql-results-table')).toBeVisible({ timeout: 10000 });

    // Open history
    await page.getByRole('button', { name: /History/ }).click();
    await expect(page.locator('.sql-history-dropdown')).toBeVisible();
    await expect(page.locator('.history-item').first()).toBeVisible();
  });

  test('clicking history item restores query', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Run a query first
    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT 42 as answer');
    await page.getByRole('button', { name: 'Run Query' }).click();
    await expect(page.locator('.sql-results-table')).toBeVisible({ timeout: 10000 });

    // Change query
    await editor.fill('SELECT 1');

    // Open history and click item
    await page.getByRole('button', { name: /History/ }).click();
    await page.locator('.history-item').first().click();

    // Query should be restored
    await expect(editor).toHaveValue(/SELECT 42 as answer/);
  });

  test('clear history removes entries', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Run a query to have history
    const editor = page.locator('#sql-editor-input');
    await editor.fill('SELECT 1');
    await page.getByRole('button', { name: 'Run Query' }).click();
    await expect(page.locator('.sql-results-header')).toBeVisible({ timeout: 10000 });

    // Open history and clear
    await page.getByRole('button', { name: /History/ }).click();
    await page.getByRole('button', { name: 'Clear History' }).click();

    // Open history again - should be empty
    await page.getByRole('button', { name: /History/ }).click();
    await expect(page.locator('.history-empty')).toBeVisible();
  });
});
