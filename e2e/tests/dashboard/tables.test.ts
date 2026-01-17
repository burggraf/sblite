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

async function cleanupTestTables(page: any) {
  // Fetch tables and delete test tables using page context (which has auth cookies)
  const tablesResponse = await page.evaluate(async () => {
    const res = await fetch('/_/api/tables');
    return res.ok ? await res.json() : [];
  });

  for (const t of tablesResponse) {
    if (t.name.startsWith('test_')) {
      await page.evaluate(async (tableName: string) => {
        await fetch(`/_/api/tables/${tableName}`, { method: 'DELETE' });
      }, t.name);
    }
  }
}

async function createTableViaPage(page: any, name: string, columns: any[]) {
  await page.evaluate(async ({ name, columns }: { name: string, columns: any[] }) => {
    await fetch('/_/api/tables', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, columns })
    });
  }, { name, columns });
}

async function insertDataViaPage(page: any, table: string, data: any) {
  await page.evaluate(async ({ table, data }: { table: string, data: any }) => {
    await fetch(`/_/api/data/${table}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });
  }, { table, data });
}

test.describe('Table Management', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('displays table list panel', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await expect(page.locator('.table-list-panel')).toBeVisible();
    await expect(page.locator('.panel-header')).toContainText('Tables');
  });

  test('creates new table via modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await page.getByRole('button', { name: '+ New' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[placeholder="my_table"]').fill('test_products');
    await page.locator('.modal .column-row input[placeholder="column_name"]').first().fill('id');

    await page.getByRole('button', { name: 'Create Table' }).click();

    // Wait for modal to close first
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });
    // Then check the table is in the list
    await expect(page.locator('.table-list-item').filter({ hasText: 'test_products' })).toBeVisible({ timeout: 5000 });
  });

  test('selects table and shows data grid', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    // Create table via page context (has auth cookies)
    await createTableViaPage(page, 'test_items', [{ name: 'id', type: 'text', primary: true }]);

    // Reload page to refresh tables list
    await page.reload();
    await page.waitForSelector('.table-list-panel');

    await page.locator('.table-list-item').filter({ hasText: 'test_items' }).click();

    await expect(page.locator('.data-grid')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.table-toolbar h2')).toContainText('test_items');
  });

  test('adds row via modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_add_row', [
      { name: 'id', type: 'text', primary: true },
      { name: 'name', type: 'text', nullable: true }
    ]);

    // Reload page to refresh tables list
    await page.reload();
    await page.waitForSelector('.table-list-panel');
    await page.locator('.table-list-item').filter({ hasText: 'test_add_row' }).click();
    await page.waitForSelector('.data-grid');

    await page.getByRole('button', { name: '+ Add Row' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input').first().fill('row-1');
    await page.locator('.modal input').nth(1).fill('Test Name');
    await page.locator('.modal').getByRole('button', { name: 'Add', exact: true }).click();

    await expect(page.locator('.data-grid')).toContainText('row-1');
  });

  test('inline edits cell', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_inline', [
      { name: 'id', type: 'text', primary: true },
      { name: 'value', type: 'text', nullable: true }
    ]);
    await insertDataViaPage(page, 'test_inline', { id: 'edit-1', value: 'original' });

    // Reload page to refresh tables list
    await page.reload();
    await page.waitForSelector('.table-list-panel');
    await page.locator('.table-list-item').filter({ hasText: 'test_inline' }).click();
    await page.waitForSelector('.data-grid');

    // Click cell to edit
    await page.locator('.data-cell').filter({ hasText: 'original' }).click();
    await page.locator('.cell-input').fill('updated');
    await page.locator('.cell-input').press('Enter');

    await expect(page.locator('.data-grid')).toContainText('updated');
  });

  test('deletes table', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_delete_me', [{ name: 'id', type: 'text', primary: true }]);

    // Reload page to refresh tables list
    await page.reload();
    await page.waitForSelector('.table-list-panel');
    await page.locator('.table-list-item').filter({ hasText: 'test_delete_me' }).click();
    await page.waitForSelector('.data-grid');

    // Register dialog handler before triggering delete
    page.once('dialog', dialog => dialog.accept());
    await page.getByRole('button', { name: 'Delete Table' }).click();

    // Wait for the table to be removed from the list
    await expect(page.locator('.table-list-item').filter({ hasText: 'test_delete_me' })).toHaveCount(0, { timeout: 5000 });
  });

  test('pagination works', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_pagination', [{ name: 'id', type: 'integer', primary: true }]);
    // Insert 30 rows
    for (let i = 1; i <= 30; i++) {
      await insertDataViaPage(page, 'test_pagination', { id: i });
    }

    // Reload page to refresh tables list
    await page.reload();
    await page.waitForSelector('.table-list-panel');
    await page.locator('.table-list-item').filter({ hasText: 'test_pagination' }).click();
    await page.waitForSelector('.data-grid');

    await expect(page.locator('.pagination-info')).toContainText('30 rows');
    await expect(page.locator('.pagination-info')).toContainText('Page 1');

    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.locator('.pagination-info')).toContainText('Page 2');
  });
});
