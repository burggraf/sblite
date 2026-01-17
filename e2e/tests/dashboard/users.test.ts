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

async function createTestUser(request: any, email: string) {
  const response = await request.post('/auth/v1/signup', {
    data: { email, password: 'testpass123' }
  });
  return response.json();
}

async function cleanupTestUsers(page: any) {
  // Navigate to Users view if not already there
  await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
  await page.waitForSelector('.data-grid', { timeout: 5000 }).catch(() => {});

  // Fetch users and delete test users using page context (which has auth cookies)
  const usersResponse = await page.evaluate(async () => {
    const res = await fetch('/_/api/users?limit=200');
    return res.ok ? await res.json() : { users: [] };
  });

  for (const u of usersResponse.users || []) {
    if (u.email.startsWith('test_')) {
      await page.evaluate(async (userId: string) => {
        await fetch(`/_/api/users/${userId}`, { method: 'DELETE' });
      }, u.id);
    }
  }
}

test.describe('User Management', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('navigates to Users section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();

    await expect(page.locator('.users-view')).toBeVisible();
    await expect(page.locator('.table-toolbar h2')).toContainText('Users');
  });

  test('displays users list with pagination info', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    // Create a test user
    await createTestUser(request, 'test_list_user@example.com');

    // Reload to see the new user
    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    await expect(page.locator('.data-grid')).toBeVisible();
    await expect(page.locator('.pagination-info')).toBeVisible();
    // Should show at least our test user
    await expect(page.locator('.data-grid')).toContainText('test_list_user@example.com');
  });

  test('shows user in list with correct columns', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);
    await createTestUser(request, 'test_columns@example.com');

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Check that expected columns exist
    await expect(page.locator('.data-grid th')).toContainText(['Email', 'Created']);
    // Check that test user email is visible
    await expect(page.locator('.data-grid')).toContainText('test_columns@example.com');
  });

  test('opens user detail modal when clicking View', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);
    await createTestUser(request, 'test_view_modal@example.com');

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Find the row with test user and click View
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_view_modal@example.com' });
    await row.getByRole('button', { name: 'View' }).click();

    await expect(page.locator('.modal')).toBeVisible();
    await expect(page.locator('.modal-header')).toContainText('User Details');
    // Email is in an input value, check the input has the correct value
    await expect(page.locator('.modal input[type="text"]').nth(1)).toHaveValue('test_view_modal@example.com');
  });

  test('updates user email_confirmed status', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);
    await createTestUser(request, 'test_confirm@example.com');

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Open user modal
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_confirm@example.com' });
    await row.getByRole('button', { name: 'View' }).click();
    await page.waitForSelector('.modal');

    // Toggle email confirmed checkbox
    const checkbox = page.locator('.modal input[type="checkbox"]');
    const wasChecked = await checkbox.isChecked();
    await checkbox.click();

    // Save changes
    await page.getByRole('button', { name: 'Save Changes' }).click();

    // Modal should close and user should be updated
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Re-open modal to verify change persisted
    await row.getByRole('button', { name: 'View' }).click();
    await page.waitForSelector('.modal');
    const newChecked = await page.locator('.modal input[type="checkbox"]').isChecked();
    expect(newChecked).toBe(!wasChecked);
  });

  test('updates user metadata', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);
    await createTestUser(request, 'test_metadata@example.com');

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Open user modal
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_metadata@example.com' });
    await row.getByRole('button', { name: 'View' }).click();
    await page.waitForSelector('.modal');

    // Update user_metadata - the first textarea is for user metadata
    const metadataInput = page.locator('.modal textarea').first();
    await metadataInput.fill('{"name": "Test User", "role": "tester"}');
    // Trigger onchange by blurring the textarea
    await metadataInput.blur();

    // Save changes
    await page.getByRole('button', { name: 'Save Changes' }).click();
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Verify via API - use page context which has auth cookies
    const list = await page.evaluate(async () => {
      const res = await fetch('/_/api/users');
      return res.ok ? await res.json() : { users: [] };
    });
    const user = list.users.find((u: any) => u.email === 'test_metadata@example.com');
    // The metadata is stored as a JSON string
    const metadata = typeof user.raw_user_meta_data === 'string'
      ? JSON.parse(user.raw_user_meta_data)
      : user.raw_user_meta_data;
    expect(metadata?.name).toBe('Test User');
  });

  test('deletes user from list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);
    await createTestUser(request, 'test_delete_user@example.com');

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Verify user exists
    await expect(page.locator('.data-grid')).toContainText('test_delete_user@example.com');

    // Set up dialog handler for confirmation
    page.on('dialog', dialog => dialog.accept());

    // Find the row with test user and click Delete (in the row, not modal)
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_delete_user@example.com' });
    await row.getByRole('button', { name: 'Delete' }).click();

    // User should be removed from list
    await expect(page.locator('.data-grid')).not.toContainText('test_delete_user@example.com', { timeout: 5000 });
  });

  test('pagination works for users list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    // Create 30 test users to trigger pagination
    for (let i = 1; i <= 30; i++) {
      await createTestUser(request, `test_page_${i.toString().padStart(2, '0')}@example.com`);
    }

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Check pagination info shows at least 30 users and Page 1
    await expect(page.locator('.pagination-info')).toContainText('users');
    await expect(page.locator('.pagination-info')).toContainText('Page 1');

    // Navigate to next page
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.locator('.pagination-info')).toContainText('Page 2');
  });

  test('cancel in user modal closes without saving', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);
    await createTestUser(request, 'test_cancel@example.com');

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.data-grid');

    // Open user modal
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_cancel@example.com' });
    await row.getByRole('button', { name: 'View' }).click();
    await page.waitForSelector('.modal');

    // Make a change
    const checkbox = page.locator('.modal input[type="checkbox"]');
    const wasChecked = await checkbox.isChecked();
    await checkbox.click();

    // Cancel
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Re-open and verify change was NOT saved
    await row.getByRole('button', { name: 'View' }).click();
    await page.waitForSelector('.modal');
    const stillSame = await page.locator('.modal input[type="checkbox"]').isChecked();
    expect(stillSame).toBe(wasChecked);
  });
});
