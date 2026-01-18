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

async function cleanupTestPolicies(page: any) {
  const policiesResponse = await page.evaluate(async () => {
    const res = await fetch('/_/api/policies');
    return res.ok ? await res.json() : { policies: [] };
  });

  for (const p of policiesResponse.policies || []) {
    if (p.policy_name.startsWith('test_')) {
      await page.evaluate(async (policyId: number) => {
        await fetch(`/_/api/policies/${policyId}`, { method: 'DELETE' });
      }, p.id);
    }
  }
}

async function createTestTable(page: any, name: string) {
  await page.evaluate(async (tableName: string) => {
    await fetch('/_/api/tables', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: tableName,
        columns: [
          { name: 'id', type: 'uuid', primary: true },
          { name: 'user_id', type: 'uuid', nullable: true },
          { name: 'content', type: 'text', nullable: true }
        ]
      })
    });
  }, name);
}

test.describe('RLS Policies View', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('navigates to Policies section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();

    await expect(page.locator('.policies-layout')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.policy-tables-panel')).toBeVisible();
  });

  test('displays tables list in left panel', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_policies_table');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    // Reload to see the new table
    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policy-tables-panel');

    await expect(page.locator('.policy-tables-panel')).toContainText('test_policies_table');
  });

  test('shows RLS toggle for each table', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_rls_toggle');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    const tableItem = page.locator('.table-list-item').filter({ hasText: 'test_rls_toggle' });
    await expect(tableItem.locator('.rls-toggle')).toBeVisible();
    await expect(tableItem.locator('input[type="checkbox"]')).toBeVisible();
  });

  test('clicking table shows policies panel', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_select_table');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_select_table' }).click();

    await expect(page.locator('.policy-content-panel h2')).toContainText('test_select_table');
    await expect(page.getByRole('button', { name: '+ New Policy' })).toBeVisible();
  });

  test('shows warning when RLS is disabled', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_rls_warning');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_rls_warning' }).click();

    await expect(page.locator('.message-warning')).toContainText('RLS is disabled');
    await expect(page.getByRole('button', { name: 'Enable RLS' })).toBeVisible();
  });

  test('can enable RLS for table', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_enable_rls');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_enable_rls' }).click();

    // Click Enable RLS button
    await page.getByRole('button', { name: 'Enable RLS' }).click();

    // Warning should change to show RLS is enabled but no policies
    await expect(page.locator('.message-error')).toContainText('no policies exist');
  });
});

test.describe('Policy CRUD', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('opens create policy modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_create_modal');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_create_modal' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();

    await expect(page.locator('.modal')).toBeVisible();
    await expect(page.locator('.modal-header')).toContainText('Create Policy');
  });

  test('creates policy with form fields', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await cleanupTestPolicies(page);
    await createTestTable(page, 'test_create_policy');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_create_policy' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();
    await page.waitForSelector('.modal');

    // Fill form
    await page.locator('.modal input[type="text"]').first().fill('test_view_own_data');
    await page.locator('.modal select').first().selectOption('SELECT');
    await page.locator('.modal textarea').first().fill('auth.uid() = user_id');

    await page.locator('.modal').getByRole('button', { name: 'Create Policy' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Policy should appear in list
    await expect(page.locator('.policy-card')).toContainText('test_view_own_data');
    await expect(page.locator('.policy-card')).toContainText('SELECT');
  });

  test('shows SQL preview while editing', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_sql_preview');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_sql_preview' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();
    await page.waitForSelector('.modal');

    // SQL preview should show table name
    await expect(page.locator('.sql-preview')).toContainText('test_sql_preview');

    // Fill in policy name
    await page.locator('.modal input[type="text"]').first().fill('my_policy');

    // SQL preview should update
    await expect(page.locator('.sql-preview')).toContainText('my_policy');
  });

  test('applies template to form', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_templates');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_templates' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();
    await page.waitForSelector('.modal');

    // Select template
    await page.locator('.modal select').nth(1).selectOption('own_data');

    // USING expression should be populated
    const usingTextarea = page.locator('.modal textarea').first();
    await expect(usingTextarea).toHaveValue('auth.uid() = user_id');
  });

  test('edits existing policy', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await cleanupTestPolicies(page);
    await createTestTable(page, 'test_edit_policy');

    // Create a policy first
    await page.evaluate(async () => {
      await fetch('/_/api/policies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          table_name: 'test_edit_policy',
          policy_name: 'test_original_name',
          command: 'SELECT',
          using_expr: 'true',
          enabled: true
        })
      });
    });

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_edit_policy' }).click();
    await page.waitForSelector('.policy-card');

    // Click Edit on the policy
    await page.locator('.policy-card').getByRole('button', { name: 'Edit' }).click();
    await page.waitForSelector('.modal');

    // Modal should show Edit Policy
    await expect(page.locator('.modal-header')).toContainText('Edit Policy');

    // Update the policy name
    await page.locator('.modal input[type="text"]').first().fill('test_updated_name');
    await page.locator('.modal').getByRole('button', { name: 'Save Changes' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Updated name should appear
    await expect(page.locator('.policy-card')).toContainText('test_updated_name');
  });

  test('deletes policy with confirmation', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await cleanupTestPolicies(page);
    await createTestTable(page, 'test_delete_policy');

    // Create a policy
    await page.evaluate(async () => {
      await fetch('/_/api/policies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          table_name: 'test_delete_policy',
          policy_name: 'test_to_delete',
          command: 'SELECT',
          using_expr: 'true',
          enabled: true
        })
      });
    });

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_delete_policy' }).click();
    await page.waitForSelector('.policy-card');

    // Set up dialog handler
    page.on('dialog', dialog => dialog.accept());

    // Click Delete
    await page.locator('.policy-card').getByRole('button', { name: 'Delete' }).click();

    // Policy should be removed
    await expect(page.locator('.policy-card')).toHaveCount(0, { timeout: 5000 });
  });

  test('toggles policy enabled state', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await cleanupTestPolicies(page);
    await createTestTable(page, 'test_toggle_enabled');

    // Create a policy (enabled by default)
    await page.evaluate(async () => {
      await fetch('/_/api/policies', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          table_name: 'test_toggle_enabled',
          policy_name: 'test_toggle_policy',
          command: 'SELECT',
          using_expr: 'true',
          enabled: true
        })
      });
    });

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_toggle_enabled' }).click();
    await page.waitForSelector('.policy-card');

    // Toggle should be checked
    const toggle = page.locator('.policy-card .policy-toggle input[type="checkbox"]');
    await expect(toggle).toBeChecked();

    // Uncheck it
    await toggle.uncheck();

    // Should now show Disabled
    await expect(page.locator('.policy-card .toggle-label')).toContainText('Disabled');
  });
});

test.describe('Policy Testing', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows test panel when clicking Test Policy', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_panel');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_panel' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();
    await page.waitForSelector('.modal');

    // Click Test Policy button
    await page.getByRole('button', { name: 'Test Policy' }).click();

    // Test panel should appear
    await expect(page.locator('.policy-test-panel')).toBeVisible();
    await expect(page.locator('.policy-test-panel select')).toBeVisible();
  });

  test('runs policy test and shows result', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_run_test');

    // Insert some data
    await page.evaluate(async () => {
      await fetch('/_/api/data/test_run_test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: crypto.randomUUID(), content: 'test data' })
      });
    });

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_run_test' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();
    await page.waitForSelector('.modal');

    // Fill in a simple policy
    await page.locator('.modal input[type="text"]').first().fill('test_policy');
    await page.locator('.modal textarea').first().fill('true');

    // Open test panel and run test
    await page.getByRole('button', { name: 'Test Policy' }).click();
    await page.getByRole('button', { name: 'Run Test' }).click();

    // Should show success with row count
    await expect(page.locator('.test-result')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.test-result')).toContainText('allow access');
  });

  test('shows error for invalid expression', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestTables(page);
    await createTestTable(page, 'test_invalid_expr');

    await page.locator('.nav-item').filter({ hasText: 'Policies' }).click();
    await page.waitForSelector('.policies-layout');

    await page.locator('.table-list-item').filter({ hasText: 'test_invalid_expr' }).click();
    await page.getByRole('button', { name: '+ New Policy' }).click();
    await page.waitForSelector('.modal');

    // Fill in an invalid expression
    await page.locator('.modal input[type="text"]').first().fill('test_policy');
    await page.locator('.modal textarea').first().fill('invalid_column = 123');

    // Open test panel and run test
    await page.getByRole('button', { name: 'Test Policy' }).click();
    await page.getByRole('button', { name: 'Run Test' }).click();

    // Should show error
    await expect(page.locator('.test-result.test-error')).toBeVisible({ timeout: 5000 });
  });
});
