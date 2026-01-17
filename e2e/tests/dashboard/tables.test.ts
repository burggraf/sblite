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

async function cleanupTestTables(request: any) {
  const tables = await request.get('/_/api/tables');
  const list = await tables.json();
  for (const t of list) {
    if (t.name.startsWith('test_')) {
      await request.delete(`/_/api/tables/${t.name}`);
    }
  }
}

test.describe('Table Management', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test.beforeEach(async ({ request }) => {
    await cleanupTestTables(request);
  });

  test('displays table list panel', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await expect(page.locator('.table-list-panel')).toBeVisible();
    await expect(page.locator('.panel-header')).toContainText('Tables');
  });

  test('creates new table via modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.getByRole('button', { name: '+ New' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[placeholder="my_table"]').fill('test_products');
    await page.locator('.modal .column-row input[placeholder="column_name"]').first().fill('id');

    await page.getByRole('button', { name: 'Create Table' }).click();

    await expect(page.locator('.table-list-item').filter({ hasText: 'test_products' })).toBeVisible({ timeout: 5000 });
  });

  test('selects table and shows data grid', async ({ page, request, context }) => {
    await ensureSetup(request);
    // Create table via API
    await request.post('/_/api/tables', {
      data: { name: 'test_items', columns: [{ name: 'id', type: 'text', primary: true }] }
    });

    await login(page, context);

    await page.locator('.table-list-item').filter({ hasText: 'test_items' }).click();

    await expect(page.locator('.data-grid')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.table-toolbar h2')).toContainText('test_items');
  });

  test('adds row via modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_add_row', columns: [
        { name: 'id', type: 'text', primary: true },
        { name: 'name', type: 'text', nullable: true }
      ]}
    });

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_add_row' }).click();
    await page.waitForSelector('.data-grid');

    await page.getByRole('button', { name: '+ Add Row' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input').first().fill('row-1');
    await page.locator('.modal input').nth(1).fill('Test Name');
    await page.getByRole('button', { name: 'Add' }).click();

    await expect(page.locator('.data-grid')).toContainText('row-1');
  });

  test('inline edits cell', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_inline', columns: [
        { name: 'id', type: 'text', primary: true },
        { name: 'value', type: 'text', nullable: true }
      ]}
    });
    await request.post('/_/api/data/test_inline', { data: { id: 'edit-1', value: 'original' } });

    await login(page, context);
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
    await request.post('/_/api/tables', {
      data: { name: 'test_delete_me', columns: [{ name: 'id', type: 'text', primary: true }] }
    });

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_delete_me' }).click();
    await page.waitForSelector('.data-grid');

    page.on('dialog', dialog => dialog.accept());
    await page.getByRole('button', { name: 'Delete Table' }).click();

    await expect(page.locator('.table-list-item').filter({ hasText: 'test_delete_me' })).not.toBeVisible({ timeout: 5000 });
  });

  test('pagination works', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_pagination', columns: [{ name: 'id', type: 'integer', primary: true }] }
    });
    // Insert 30 rows
    for (let i = 1; i <= 30; i++) {
      await request.post('/_/api/data/test_pagination', { data: { id: i } });
    }

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_pagination' }).click();
    await page.waitForSelector('.data-grid');

    await expect(page.locator('.pagination-info')).toContainText('30 rows');
    await expect(page.locator('.pagination-info')).toContainText('Page 1');

    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.locator('.pagination-info')).toContainText('Page 2');
  });
});
