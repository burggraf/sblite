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

test.describe('API Docs - Navigation', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows API Docs in sidebar navigation', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Check Documentation section and API Docs link
    await expect(page.getByText('Documentation')).toBeVisible();
    await expect(page.locator('.nav-item').filter({ hasText: 'API Docs' })).toBeVisible();
  });

  test('navigates to API Docs view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();

    // Should show API Docs layout with sidebar
    await expect(page.locator('.api-docs-layout')).toBeVisible();
    await expect(page.locator('.api-docs-sidebar')).toBeVisible();
    await expect(page.locator('.api-docs-content')).toBeVisible();
  });

  test('shows API Docs sidebar with sections', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    // Check sidebar sections
    await expect(page.locator('.api-docs-nav-item').filter({ hasText: 'Introduction' })).toBeVisible();
    await expect(page.locator('.api-docs-nav-item').filter({ hasText: 'Authentication' })).toBeVisible();
    await expect(page.locator('.api-docs-nav-item').filter({ hasText: 'User Management' })).toBeVisible();
    await expect(page.locator('.api-docs-sidebar-title').filter({ hasText: 'Tables and Views' })).toBeVisible();
    await expect(page.locator('.api-docs-sidebar-title').filter({ hasText: 'Stored Procedures' })).toBeVisible();
  });
});

test.describe('API Docs - Static Pages', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('displays Introduction page by default', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-page');

    await expect(page.locator('.api-docs-page h1')).toContainText('Connect To Your Project');
    await expect(page.locator('.api-docs-lang-tabs')).toBeVisible();
  });

  test('navigates to Authentication page', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    await page.locator('.api-docs-nav-item').filter({ hasText: 'Authentication' }).click();

    await expect(page.locator('.api-docs-page h1')).toContainText('Authentication');
    await expect(page.locator('.api-docs-warning')).toBeVisible(); // Service key warning
  });

  test('navigates to User Management page', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    await page.locator('.api-docs-nav-item').filter({ hasText: 'User Management' }).click();

    await expect(page.locator('.api-docs-page h1')).toContainText('User Management');
    // Check some user management operations are shown
    await expect(page.locator('.api-docs-page h3').filter({ hasText: 'Sign Up' })).toBeVisible();
    await expect(page.locator('.api-docs-page h3').filter({ hasText: 'Sign In with Password' })).toBeVisible();
  });

  test('navigates to Tables Introduction page', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    // Click on Tables Introduction
    await page.locator('.api-docs-nav-item.api-docs-nav-sub').filter({ hasText: 'Introduction' }).first().click();

    await expect(page.locator('.api-docs-page h1')).toContainText('Tables and Views');
    await expect(page.locator('.api-docs-page h2').filter({ hasText: 'REST Conventions' })).toBeVisible();
  });

  test('navigates to Stored Procedures Introduction page', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    // Find the RPC intro under Stored Procedures section
    const rpcIntro = page.locator('.api-docs-nav-item.api-docs-nav-sub').filter({ hasText: 'Introduction' }).last();
    await rpcIntro.click();

    await expect(page.locator('.api-docs-page h1')).toContainText('Stored Procedures');
    await expect(page.locator('.api-docs-page h2').filter({ hasText: 'Calling Functions' })).toBeVisible();
  });
});

test.describe('API Docs - Language Toggle', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('switches between JavaScript and Bash examples', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-lang-tabs');

    // JavaScript should be active by default
    await expect(page.locator('.api-docs-lang-tab.active')).toContainText('JavaScript');

    // Click Bash tab
    await page.locator('.api-docs-lang-tab').filter({ hasText: 'Bash' }).click();
    await expect(page.locator('.api-docs-lang-tab.active')).toContainText('Bash');

    // Code block should show curl command
    await expect(page.locator('.api-docs-code-block code').first()).toContainText('curl');

    // Click JavaScript tab
    await page.locator('.api-docs-lang-tab').filter({ hasText: 'JavaScript' }).click();
    await expect(page.locator('.api-docs-lang-tab.active')).toContainText('JavaScript');

    // Code block should show createClient
    await expect(page.locator('.api-docs-code-block code').first()).toContainText('createClient');
  });
});

test.describe('API Docs - Table Documentation', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows user tables in sidebar', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    // Create a test table
    await createTableViaPage(page, 'test_products', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'name', type: 'text', is_nullable: false },
      { name: 'price', type: 'numeric', is_nullable: true }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    // Table should appear in sidebar
    await expect(page.locator('.api-docs-nav-item').filter({ hasText: 'test_products' })).toBeVisible();
  });

  test('displays table documentation with columns', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    // Create a test table
    await createTableViaPage(page, 'test_orders', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'customer', type: 'text', is_nullable: false },
      { name: 'total', type: 'numeric', is_nullable: false }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');

    // Click on the test table
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_orders' }).click();

    // Should show table name as heading
    await expect(page.locator('.api-docs-page h1')).toContainText('test_orders');

    // Should show columns
    await expect(page.locator('.api-docs-column-section')).toHaveCount(3);
    await expect(page.locator('.api-docs-page h3').filter({ hasText: 'Column: id' })).toBeVisible();
    await expect(page.locator('.api-docs-page h3').filter({ hasText: 'Column: customer' })).toBeVisible();
    await expect(page.locator('.api-docs-page h3').filter({ hasText: 'Column: total' })).toBeVisible();

    // Should show CRUD operations
    await expect(page.locator('.api-docs-page h2').filter({ hasText: 'Read Rows' })).toBeVisible();
    await expect(page.locator('.api-docs-page h2').filter({ hasText: 'Insert Rows' })).toBeVisible();
    await expect(page.locator('.api-docs-page h2').filter({ hasText: 'Update Rows' })).toBeVisible();
    await expect(page.locator('.api-docs-page h2').filter({ hasText: 'Delete Rows' })).toBeVisible();
  });

  test('shows column metadata badges', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_items', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'required_field', type: 'text', is_nullable: false }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_items' }).click();

    // Check badges are shown
    await expect(page.locator('.api-docs-badge.required')).toBeVisible();
    await expect(page.locator('.api-docs-badge.type')).toBeVisible();
    await expect(page.locator('.api-docs-badge.format')).toBeVisible();
  });
});

test.describe('API Docs - Backend API', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('GET /_/api/apidocs/tables returns table list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_api_table', [
      { name: 'id', type: 'uuid', is_primary: true }
    ]);

    const response = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/tables');
      return { ok: res.ok, data: await res.json() };
    });

    expect(response.ok).toBe(true);
    expect(Array.isArray(response.data)).toBe(true);
    const testTable = response.data.find((t: any) => t.name === 'test_api_table');
    expect(testTable).toBeDefined();
    expect(testTable.columns).toBeDefined();
  });

  test('GET /_/api/apidocs/tables/{name} returns table details', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_detail_table', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'name', type: 'text', is_nullable: false }
    ]);

    const response = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/tables/test_detail_table');
      return { ok: res.ok, data: await res.json() };
    });

    expect(response.ok).toBe(true);
    expect(response.data.name).toBe('test_detail_table');
    expect(response.data.columns).toHaveLength(2);
    expect(response.data.columns[0]).toHaveProperty('name');
    expect(response.data.columns[0]).toHaveProperty('type');
    expect(response.data.columns[0]).toHaveProperty('format');
    expect(response.data.columns[0]).toHaveProperty('required');
  });

  test('PATCH /_/api/apidocs/tables/{name}/description updates table description', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_desc_table', [
      { name: 'id', type: 'uuid', is_primary: true }
    ]);

    const updateResponse = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/tables/test_desc_table/description', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: 'Test table description' })
      });
      return { ok: res.ok };
    });

    expect(updateResponse.ok).toBe(true);

    // Verify the description was saved
    const getResponse = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/tables/test_desc_table');
      return { ok: res.ok, data: await res.json() };
    });

    expect(getResponse.data.description).toBe('Test table description');
  });

  test('PATCH /_/api/apidocs/tables/{name}/columns/{column}/description updates column description', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_col_desc', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'email', type: 'text', is_nullable: false }
    ]);

    const updateResponse = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/tables/test_col_desc/columns/email/description', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ description: 'User email address' })
      });
      return { ok: res.ok };
    });

    expect(updateResponse.ok).toBe(true);

    // Verify the description was saved
    const getResponse = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/tables/test_col_desc');
      return { ok: res.ok, data: await res.json() };
    });

    const emailCol = getResponse.data.columns.find((c: any) => c.name === 'email');
    expect(emailCol.description).toBe('User email address');
  });

  test('GET /_/api/apidocs/functions returns function list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    const response = await page.evaluate(async () => {
      const res = await fetch('/_/api/apidocs/functions');
      return { ok: res.ok, data: await res.json() };
    });

    expect(response.ok).toBe(true);
    expect(Array.isArray(response.data)).toBe(true);
  });
});

test.describe('API Docs - Description Editing UI', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('allows editing table description', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_edit_desc', [
      { name: 'id', type: 'uuid', is_primary: true }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_edit_desc' }).click();

    // Find and edit the table description
    const descTextarea = page.locator('#table-desc');
    await expect(descTextarea).toBeVisible();

    await descTextarea.fill('This is my test table');
    await page.locator('.api-docs-editable button').first().click();

    // Reload and verify
    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_edit_desc' }).click();

    await expect(page.locator('#table-desc')).toHaveValue('This is my test table');
  });

  test('allows editing column description', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_col_edit', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'status', type: 'text', is_nullable: false }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_col_edit' }).click();

    // Find and edit the status column description
    const colDescInput = page.locator('#col-desc-status');
    await expect(colDescInput).toBeVisible();

    await colDescInput.fill('Current status of the record');

    // Find the Save button next to this input
    const saveButton = colDescInput.locator('..').locator('button');
    await saveButton.click();

    // Reload and verify
    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_col_edit' }).click();

    await expect(page.locator('#col-desc-status')).toHaveValue('Current status of the record');
  });
});

test.describe('API Docs - Code Examples', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows correct JavaScript code examples for tables', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_js_examples', [
      { name: 'id', type: 'uuid', is_primary: true },
      { name: 'title', type: 'text', is_nullable: false }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_js_examples' }).click();

    // Ensure JavaScript is selected
    await page.locator('.api-docs-lang-tab').filter({ hasText: 'JavaScript' }).click();

    // Check code examples contain table name
    const codeBlocks = page.locator('.api-docs-code-block code');
    const firstCode = await codeBlocks.first().textContent();
    expect(firstCode).toContain('test_js_examples');
    expect(firstCode).toContain('supabase');
  });

  test('shows correct Bash code examples for tables', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);

    await createTableViaPage(page, 'test_bash_examples', [
      { name: 'id', type: 'uuid', is_primary: true }
    ]);

    await page.locator('.nav-item').filter({ hasText: 'API Docs' }).click();
    await page.waitForSelector('.api-docs-sidebar');
    await page.locator('.api-docs-nav-item').filter({ hasText: 'test_bash_examples' }).click();

    // Switch to Bash
    await page.locator('.api-docs-lang-tab').filter({ hasText: 'Bash' }).click();

    // Check code examples contain curl and table name
    const codeBlocks = page.locator('.api-docs-code-block code');
    const firstCode = await codeBlocks.first().textContent();
    expect(firstCode).toContain('curl');
    expect(firstCode).toContain('test_bash_examples');
    expect(firstCode).toContain('apikey');
  });
});
