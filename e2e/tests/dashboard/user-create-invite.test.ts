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

test.describe('User Creation', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows Create User button in toolbar', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await expect(page.getByRole('button', { name: '+ Create User' })).toBeVisible();
  });

  test('opens Create User modal when clicking button', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();

    await expect(page.locator('.modal')).toBeVisible();
    await expect(page.locator('.modal-header')).toContainText('Create User');
    await expect(page.locator('.modal input[type="email"]')).toBeVisible();
    await expect(page.locator('.modal input[type="password"]')).toBeVisible();
    await expect(page.locator('.modal input[type="checkbox"]')).toBeVisible();
  });

  test('creates user with auto-confirm enabled', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_created@example.com');
    await page.locator('.modal input[type="password"]').fill('securepass123');
    // Auto-confirm is checked by default

    await page.locator('.modal').getByRole('button', { name: 'Create User' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // User should appear in list
    await expect(page.locator('.data-grid')).toContainText('test_created@example.com');

    // User should be confirmed (check mark visible)
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_created@example.com' });
    await expect(row.locator('.text-success')).toContainText('✓');
  });

  test('creates user without auto-confirm', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_unconfirmed@example.com');
    await page.locator('.modal input[type="password"]').fill('securepass123');
    // Uncheck auto-confirm
    await page.locator('.modal input[type="checkbox"]').uncheck();

    await page.locator('.modal').getByRole('button', { name: 'Create User' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // User should appear in list
    await expect(page.locator('.data-grid')).toContainText('test_unconfirmed@example.com');

    // User should NOT be confirmed (dash visible, not check mark)
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_unconfirmed@example.com' });
    await expect(row.locator('.text-muted')).toContainText('—');
  });

  test('shows error for invalid email', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('invalid-email');
    await page.locator('.modal input[type="password"]').fill('securepass123');

    await page.locator('.modal').getByRole('button', { name: 'Create User' }).click();

    await expect(page.locator('.modal .message-error')).toContainText('valid email');
  });

  test('shows error for short password', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test@example.com');
    await page.locator('.modal input[type="password"]').fill('short');

    await page.locator('.modal').getByRole('button', { name: 'Create User' }).click();

    await expect(page.locator('.modal .message-error')).toContainText('at least 6 characters');
  });

  test('shows error for duplicate email', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    // Create first user
    await page.evaluate(async () => {
      await fetch('/_/api/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'test_duplicate@example.com', password: 'pass123456', auto_confirm: true })
      });
    });

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_duplicate@example.com');
    await page.locator('.modal input[type="password"]').fill('securepass123');

    await page.locator('.modal').getByRole('button', { name: 'Create User' }).click();

    await expect(page.locator('.modal .message-error')).toContainText('already exists');
  });

  test('cancel closes modal without creating user', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: '+ Create User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_cancel_create@example.com');
    await page.locator('.modal input[type="password"]').fill('securepass123');

    await page.getByRole('button', { name: 'Cancel' }).click();

    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });
    await expect(page.locator('.data-grid')).not.toContainText('test_cancel_create@example.com');
  });
});

test.describe('User Invitation', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows Invite User button in toolbar', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await expect(page.getByRole('button', { name: 'Invite User' })).toBeVisible();
  });

  test('opens Invite User modal when clicking button', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();

    await expect(page.locator('.modal')).toBeVisible();
    await expect(page.locator('.modal-header')).toContainText('Invite User');
    await expect(page.locator('.modal input[type="email"]')).toBeVisible();
  });

  test('sends invite and shows invite link', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_invited@example.com');
    await page.getByRole('button', { name: 'Send Invite' }).click();

    // Should show success state
    await expect(page.locator('.modal .message-success')).toContainText('test_invited@example.com');
    await expect(page.locator('.modal-header')).toContainText('Invite Sent');

    // Should show invite link
    const inviteLinkInput = page.locator('.modal .invite-link-input');
    await expect(inviteLinkInput).toBeVisible();
    const inviteLink = await inviteLinkInput.inputValue();
    expect(inviteLink).toContain('/auth/v1/verify');
    expect(inviteLink).toContain('type=invite');

    // Should show Copy Link button
    await expect(page.getByRole('button', { name: 'Copy Link' })).toBeVisible();
  });

  test('shows error for invalid email in invite', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('invalid-email');
    await page.getByRole('button', { name: 'Send Invite' }).click();

    await expect(page.locator('.modal .message-error')).toContainText('valid email');
  });

  test('shows error when inviting existing confirmed user', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    // Create existing confirmed user
    await page.evaluate(async () => {
      await fetch('/_/api/users', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email: 'test_existing@example.com', password: 'pass123456', auto_confirm: true })
      });
    });

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_existing@example.com');
    await page.getByRole('button', { name: 'Send Invite' }).click();

    await expect(page.locator('.modal .message-error')).toContainText('already exists');
  });

  test('cancel closes invite modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test@example.com');
    await page.getByRole('button', { name: 'Cancel' }).click();

    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });
  });

  test('Done button closes modal and refreshes user list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_invite_done@example.com');
    await page.getByRole('button', { name: 'Send Invite' }).click();

    // Wait for success state
    await expect(page.locator('.modal .message-success')).toBeVisible();

    // Click Done
    await page.getByRole('button', { name: 'Done' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // User should appear in list (invited users show up in the list)
    await expect(page.locator('.data-grid')).toContainText('test_invite_done@example.com');
  });

  test('invited user appears as unconfirmed', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestUsers(page);

    await page.locator('.nav-item').filter({ hasText: 'Users' }).click();
    await page.waitForSelector('.users-view');

    await page.getByRole('button', { name: 'Invite User' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[type="email"]').fill('test_invite_unconfirmed@example.com');
    await page.getByRole('button', { name: 'Send Invite' }).click();

    await expect(page.locator('.modal .message-success')).toBeVisible();
    await page.getByRole('button', { name: 'Done' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // User should appear in list as unconfirmed
    const row = page.locator('.data-grid tbody tr').filter({ hasText: 'test_invite_unconfirmed@example.com' });
    await expect(row).toBeVisible();
    await expect(row.locator('.text-muted')).toContainText('—');
  });
});
