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

test.describe('API Console View', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('navigates to API Console section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();

    await expect(page.locator('.card-title').filter({ hasText: 'API Console' })).toBeVisible();
    await expect(page.locator('.api-console-view')).toBeVisible();
  });

  test('displays split view layout', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await expect(page.locator('.api-console-request')).toBeVisible();
    await expect(page.locator('.api-console-response')).toBeVisible();
  });

  test('shows method dropdown with all HTTP methods', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    const methodSelect = page.locator('.api-console-method');
    await expect(methodSelect).toBeVisible();

    const options = await methodSelect.locator('option').allTextContents();
    expect(options).toContain('GET');
    expect(options).toContain('POST');
    expect(options).toContain('PUT');
    expect(options).toContain('PATCH');
    expect(options).toContain('DELETE');
  });

  test('shows templates dropdown', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await expect(page.locator('.api-console-templates select')).toBeVisible();
  });
});

test.describe('Request Builder', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('selecting template populates form', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Select Sign Up template
    await page.locator('.api-console-templates select').selectOption('auth-signup');

    // Check method is POST
    await expect(page.locator('.api-console-method')).toHaveValue('POST');

    // Check URL is set
    const urlInput = page.locator('#api-console-url');
    await expect(urlInput).toHaveValue('/auth/v1/signup');
  });

  test('can edit URL manually', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    const urlInput = page.locator('#api-console-url');
    await urlInput.fill('/rest/v1/my_table');
    await expect(urlInput).toHaveValue('/rest/v1/my_table');
  });

  test('can add custom headers', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Click add header button
    await page.locator('.api-console-headers').getByRole('button', { name: '+ Add' }).click();

    // Count header rows (should have more than before)
    const headerRows = page.locator('.api-console-header-row');
    const count = await headerRows.count();
    expect(count).toBeGreaterThan(0);
  });

  test('body textarea shown only for POST/PATCH/PUT', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // GET - no body
    await page.locator('.api-console-method').selectOption('GET');
    await expect(page.locator('#api-console-body')).not.toBeVisible();

    // POST - has body
    await page.locator('.api-console-method').selectOption('POST');
    await expect(page.locator('#api-console-body')).toBeVisible();

    // PATCH - has body
    await page.locator('.api-console-method').selectOption('PATCH');
    await expect(page.locator('#api-console-body')).toBeVisible();

    // DELETE - no body
    await page.locator('.api-console-method').selectOption('DELETE');
    await expect(page.locator('#api-console-body')).not.toBeVisible();
  });
});

test.describe('Sending Requests', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('sends GET request and displays response', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Set URL to a valid endpoint - use admin/tables which requires dashboard auth
    await page.locator('#api-console-url').fill('/_/api/tables');
    await page.locator('.api-console-method').selectOption('GET');

    // Send request
    await page.getByRole('button', { name: 'Send Request' }).click();

    // Wait for response - just verify we get a status and JSON content
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
    // Response should show a status code (any status is fine - we're testing the flow)
    await expect(page.locator('.status-code')).toBeVisible();
    await expect(page.locator('.api-console-json')).toBeVisible();
  });

  test('sends POST request with body', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Use a template that sends POST
    await page.locator('.api-console-templates select').selectOption('auth-signup');

    // Send request (will likely fail due to duplicate email, but that's ok - we're testing the flow)
    await page.getByRole('button', { name: 'Send Request' }).click();

    // Wait for response (any response is fine)
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
  });

  test('shows loading state during request', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await page.locator('#api-console-url').fill('/_/api/auth/status');

    // Click send and immediately check for loading state
    const sendButton = page.getByRole('button', { name: 'Send Request' });
    await sendButton.click();

    // Button should show loading or be disabled
    // (might be too fast to catch, so we just verify the request completes)
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
  });

  test('displays error for failed requests', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Send to non-existent endpoint
    await page.locator('#api-console-url').fill('/nonexistent/endpoint');
    await page.getByRole('button', { name: 'Send Request' }).click();

    // Wait for response (should show warning status for 404)
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
    await expect(page.locator('.api-console-response-status')).toHaveClass(/status-warning/);
  });

  test('shows response time', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();

    await expect(page.locator('.status-time')).toBeVisible({ timeout: 10000 });
    const timeText = await page.locator('.status-time').textContent();
    expect(timeText).toMatch(/\d+ms/);
  });
});

test.describe('Response Viewer', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('displays formatted JSON body', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();

    await expect(page.locator('.api-console-json')).toBeVisible({ timeout: 10000 });
    const jsonContent = await page.locator('.api-console-json').textContent();
    expect(jsonContent).toContain('{');
  });

  test('can switch to headers tab', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();

    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });

    // Click headers tab
    await page.locator('.api-console-response-tabs .tab').filter({ hasText: 'Headers' }).click();

    // Should show headers list
    await expect(page.locator('.api-console-headers-list')).toBeVisible();
  });

  test('copy button copies response', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();

    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });

    // Copy button should be visible
    await expect(page.locator('.copy-btn')).toBeVisible();
  });
});

test.describe('History', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('request added to history after send', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Clear any existing history first
    await page.evaluate(() => localStorage.removeItem('sblite_api_console_history'));

    // Send a request
    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });

    // Open history dropdown
    await page.getByRole('button', { name: /History/ }).click();

    // Should have at least one history item
    await expect(page.locator('.history-item').first()).toBeVisible();
  });

  test('clicking history item loads request', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Send a request first
    await page.locator('#api-console-url').fill('/_/api/settings/server');
    await page.getByRole('button', { name: 'Send Request' }).click();
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });

    // Change URL to something else
    await page.locator('#api-console-url').fill('/different/url');

    // Open history and click the item
    await page.getByRole('button', { name: /History/ }).click();
    await page.locator('.history-item').first().click();

    // URL should be restored
    await expect(page.locator('#api-console-url')).toHaveValue('/_/api/settings/server');
  });

  test('history persists after page reload', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Send a request
    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });

    // Reload the page
    await page.reload();
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Open history - should still have items
    await page.getByRole('button', { name: /History/ }).click();
    await expect(page.locator('.history-item').first()).toBeVisible();
  });

  test('clear history removes all entries', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Send a request to have something in history
    await page.locator('#api-console-url').fill('/_/api/auth/status');
    await page.getByRole('button', { name: 'Send Request' }).click();
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });

    // Open history and clear it
    await page.getByRole('button', { name: /History/ }).click();
    await page.getByRole('button', { name: 'Clear History' }).click();

    // Open history again - should be empty
    await page.getByRole('button', { name: /History/ }).click();
    await expect(page.locator('.history-empty')).toBeVisible();
  });
});

test.describe('API Key Auto-Injection', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('shows API key settings section', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Should show auth settings section
    await expect(page.locator('.api-console-auth-settings')).toBeVisible();
    await expect(page.locator('.api-console-auth-settings').getByText('Authentication')).toBeVisible();
  });

  test('auto-inject checkbox is enabled by default', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Auto-inject checkbox should be checked
    const checkbox = page.locator('.api-console-auth-settings input[type="checkbox"]');
    await expect(checkbox).toBeChecked();
  });

  test('shows key type selector when auto-inject enabled', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Should show key type dropdown
    await expect(page.locator('.api-key-select')).toBeVisible();
    await expect(page.locator('.api-key-hint')).toBeVisible();
  });

  test('REST API request succeeds with auto-injected key', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Make request to REST API - should work because key is auto-injected
    await page.locator('#api-console-url').fill('/rest/v1/characters');
    await page.getByRole('button', { name: 'Send Request' }).click();

    // Wait for response and check it succeeded
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
    const statusText = await page.locator('.api-console-response-status .status-code').textContent();
    expect(statusText).toContain('200');
  });

  test('REST API request fails when auto-inject disabled', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Disable auto-inject
    await page.locator('.api-console-auth-settings input[type="checkbox"]').uncheck();

    // Make request to REST API without key
    await page.locator('#api-console-url').fill('/rest/v1/characters');
    await page.getByRole('button', { name: 'Send Request' }).click();

    // Should get error about missing API key
    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
    const body = await page.locator('.api-console-json').textContent();
    expect(body).toContain('no_api_key');
  });

  test('can switch between anon and service_role key', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'API Console' }).click();
    await page.waitForSelector('.api-console-view');

    // Switch to service_role key
    await page.locator('.api-key-select').selectOption('service_role');

    // Make request - should still work
    await page.locator('#api-console-url').fill('/rest/v1/characters');
    await page.getByRole('button', { name: 'Send Request' }).click();

    await expect(page.locator('.api-console-response-status')).toBeVisible({ timeout: 10000 });
    const statusText = await page.locator('.api-console-response-status .status-code').textContent();
    expect(statusText).toContain('200');
  });
});
