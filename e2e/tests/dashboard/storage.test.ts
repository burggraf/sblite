import { test, expect, Page, BrowserContext } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';

async function ensureSetup(request: any) {
  const status = await request.get('/_/api/auth/status');
  const data = await status.json();

  if (data.needs_setup) {
    await request.post('/_/api/auth/setup', {
      data: { password: TEST_PASSWORD },
    });
  }
}

async function login(page: Page, context: BrowserContext) {
  await context.clearCookies();
  await page.goto('/_/');

  // Wait for loading to complete
  await page.waitForFunction(() => {
    const app = document.getElementById('app');
    return app && !app.innerHTML.includes('Loading');
  }, { timeout: 5000 });

  // Check if we need to login
  const needsLogin = await page.locator('.auth-container').isVisible();
  if (needsLogin) {
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();
  }

  await page.waitForSelector('.sidebar', { timeout: 5000 });
}

async function cleanupTestBuckets(page: Page) {
  // Fetch buckets and delete test buckets using page context (which has auth cookies)
  const bucketsResponse = await page.evaluate(async () => {
    const res = await fetch('/_/api/storage/buckets');
    return res.ok ? await res.json() : [];
  });

  for (const bucket of bucketsResponse) {
    if (bucket.name.startsWith('test-')) {
      // Empty the bucket first
      await page.evaluate(async (bucketId: string) => {
        await fetch(`/_/api/storage/buckets/${bucketId}/empty`, { method: 'POST' });
      }, bucket.id);
      // Then delete it
      await page.evaluate(async (bucketId: string) => {
        await fetch(`/_/api/storage/buckets/${bucketId}`, { method: 'DELETE' });
      }, bucket.id);
    }
  }
}

async function createBucketViaApi(page: Page, name: string, isPublic: boolean = false) {
  return await page.evaluate(async ({ name, isPublic }: { name: string, isPublic: boolean }) => {
    const res = await fetch('/_/api/storage/buckets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, public: isPublic })
    });
    return res.ok ? await res.json() : null;
  }, { name, isPublic });
}

async function uploadFileViaApi(page: Page, bucketName: string, fileName: string, content: string) {
  return await page.evaluate(async ({ bucketName, fileName, content }: { bucketName: string, fileName: string, content: string }) => {
    const formData = new FormData();
    formData.append('bucket', bucketName);
    formData.append('path', '');
    formData.append('file', new Blob([content], { type: 'text/plain' }), fileName);

    const res = await fetch('/_/api/storage/objects/upload', {
      method: 'POST',
      body: formData
    });
    return res.ok;
  }, { bucketName, fileName, content });
}

async function navigateToStorage(page: Page) {
  await page.locator('.nav-item').filter({ hasText: 'Storage' }).click();
  await page.waitForSelector('.storage-layout', { timeout: 5000 });
}

test.describe('Dashboard Storage', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('should display storage navigation', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    // Storage should be visible in the sidebar
    await expect(page.locator('.nav-item').filter({ hasText: 'Storage' })).toBeVisible();

    // Navigate to storage view
    await page.locator('.nav-item').filter({ hasText: 'Storage' }).click();

    // Should show storage layout
    await expect(page.locator('.storage-layout')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.storage-sidebar')).toBeVisible();
  });

  test('should create a bucket', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);
    await navigateToStorage(page);

    // Click the + New button
    await page.getByRole('button', { name: '+ New' }).click();
    await page.waitForSelector('.modal', { timeout: 5000 });

    // Fill in bucket name
    await page.locator('#bucket-name').fill('test-create-bucket');

    // Click Create Bucket button
    await page.getByRole('button', { name: 'Create Bucket' }).click();

    // Wait for modal to close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Verify bucket appears in the list
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-create-bucket' })).toBeVisible({ timeout: 5000 });
  });

  test('should list buckets', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket via API
    await createBucketViaApi(page, 'test-list-bucket');

    await navigateToStorage(page);

    // Verify bucket is listed
    await expect(page.locator('.bucket-list')).toBeVisible();
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-list-bucket' })).toBeVisible({ timeout: 5000 });
  });

  test('should select a bucket and show file browser', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket via API
    await createBucketViaApi(page, 'test-select-bucket');

    await navigateToStorage(page);

    // Click on the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-select-bucket' }).click();

    // Should show file browser
    await expect(page.locator('.file-browser')).toBeVisible({ timeout: 5000 });

    // Should show the bucket name in breadcrumb
    await expect(page.locator('.breadcrumb')).toContainText('test-select-bucket');
  });

  test('should upload a file', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket via API
    await createBucketViaApi(page, 'test-upload-bucket');

    await navigateToStorage(page);

    // Select the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-upload-bucket' }).click();
    await page.waitForSelector('.file-browser', { timeout: 5000 });

    // Create a test file and upload it via the hidden file input
    const fileInput = page.locator('#file-upload-input');

    // Create a buffer with test content
    const buffer = Buffer.from('Test file content for upload');

    // Set the file input
    await fileInput.setInputFiles({
      name: 'test-upload.txt',
      mimeType: 'text/plain',
      buffer: buffer
    });

    // Wait for upload to complete - file should appear in the browser
    await expect(page.locator('.file-card, .file-row').filter({ hasText: 'test-upload.txt' })).toBeVisible({ timeout: 10000 });
  });

  test('should toggle between grid and list view', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket via API
    await createBucketViaApi(page, 'test-view-toggle');

    // Upload a file via API
    await uploadFileViaApi(page, 'test-view-toggle', 'test-file.txt', 'test content');

    await navigateToStorage(page);

    // Select the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-view-toggle' }).click();
    await page.waitForSelector('.file-browser', { timeout: 5000 });

    // Wait for file to appear
    await expect(page.locator('.file-card, .file-row').filter({ hasText: 'test-file.txt' })).toBeVisible({ timeout: 5000 });

    // Grid view should be default
    await expect(page.locator('.file-browser-content.grid')).toBeVisible();

    // Click list view button
    await page.locator('.view-toggle button[title="List view"]').click();

    // Should switch to list view
    await expect(page.locator('.file-browser-content.list')).toBeVisible();
    await expect(page.locator('.file-list-table')).toBeVisible();

    // Click grid view button
    await page.locator('.view-toggle button[title="Grid view"]').click();

    // Should switch back to grid view
    await expect(page.locator('.file-browser-content.grid')).toBeVisible();
  });

  test('should delete files', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket via API
    await createBucketViaApi(page, 'test-delete-file');

    // Upload a file via API
    await uploadFileViaApi(page, 'test-delete-file', 'to-delete.txt', 'content to delete');

    await navigateToStorage(page);

    // Select the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-delete-file' }).click();
    await page.waitForSelector('.file-browser', { timeout: 5000 });

    // Wait for file to appear
    await expect(page.locator('.file-card').filter({ hasText: 'to-delete.txt' })).toBeVisible({ timeout: 5000 });

    // Click on the file to select it
    await page.locator('.file-card').filter({ hasText: 'to-delete.txt' }).click();

    // Should show delete button with count
    await expect(page.getByRole('button', { name: /Delete \(1\)/ })).toBeVisible();

    // Register dialog handler for confirmation
    page.once('dialog', dialog => dialog.accept());

    // Click delete button
    await page.getByRole('button', { name: /Delete \(1\)/ }).click();

    // File should be removed
    await expect(page.locator('.file-card').filter({ hasText: 'to-delete.txt' })).not.toBeVisible({ timeout: 5000 });
  });

  test('should update bucket settings', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a private bucket via API
    await createBucketViaApi(page, 'test-update-settings', false);

    await navigateToStorage(page);

    // Verify bucket shows as private
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-update-settings' })).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-update-settings' }).locator('.bucket-badge')).toContainText('Private');

    // Click settings button on the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-update-settings' }).locator('button[title="Settings"]').click();

    // Should show settings modal
    await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.modal-header')).toContainText('Bucket Settings');

    // Toggle public checkbox
    await page.locator('#bucket-public').check();

    // Click Save Changes
    await page.getByRole('button', { name: 'Save Changes' }).click();

    // Modal should close
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });

    // Bucket should now show as public
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-update-settings' }).locator('.bucket-badge')).toContainText('Public', { timeout: 5000 });
  });

  test('should delete a bucket', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create an empty bucket via API
    await createBucketViaApi(page, 'test-delete-bucket');

    await navigateToStorage(page);

    // Verify bucket exists
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-delete-bucket' })).toBeVisible({ timeout: 5000 });

    // Click settings button on the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-delete-bucket' }).locator('button[title="Settings"]').click();

    // Should show settings modal
    await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

    // Register dialog handler for confirmation
    page.once('dialog', dialog => dialog.accept());

    // Click Delete Bucket button
    await page.getByRole('button', { name: 'Delete Bucket' }).click();

    // Modal should close and bucket should be removed
    await expect(page.locator('.modal')).not.toBeVisible({ timeout: 5000 });
    await expect(page.locator('.bucket-item').filter({ hasText: 'test-delete-bucket' })).not.toBeVisible({ timeout: 5000 });
  });

  test('should show empty state for bucket with no files', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create an empty bucket
    await createBucketViaApi(page, 'test-empty-bucket');

    await navigateToStorage(page);

    // Select the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-empty-bucket' }).click();
    await page.waitForSelector('.file-browser', { timeout: 5000 });

    // Should show empty state message
    await expect(page.locator('.file-browser-empty')).toBeVisible();
    await expect(page.locator('.file-browser-empty')).toContainText('No files');
  });

  test('should empty a bucket', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket via API
    await createBucketViaApi(page, 'test-empty-operation');

    // Upload files via API
    await uploadFileViaApi(page, 'test-empty-operation', 'file1.txt', 'content 1');
    await uploadFileViaApi(page, 'test-empty-operation', 'file2.txt', 'content 2');

    await navigateToStorage(page);

    // Select the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-empty-operation' }).click();
    await page.waitForSelector('.file-browser', { timeout: 5000 });

    // Verify files exist
    await expect(page.locator('.file-card, .file-row').filter({ hasText: 'file1.txt' })).toBeVisible({ timeout: 5000 });

    // Click settings button on the bucket
    await page.locator('.bucket-item').filter({ hasText: 'test-empty-operation' }).locator('button[title="Settings"]').click();

    // Should show settings modal
    await expect(page.locator('.modal')).toBeVisible({ timeout: 5000 });

    // Register dialog handler for confirmation
    page.once('dialog', dialog => dialog.accept());

    // Click Empty Bucket button
    await page.getByRole('button', { name: 'Empty Bucket' }).click();

    // Wait for empty operation - files should be removed
    await page.waitForTimeout(500); // Small wait for API call
    await expect(page.locator('.file-browser-empty')).toBeVisible({ timeout: 5000 });
  });
});

test.describe('Dashboard Storage API', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('GET /_/api/storage/buckets returns bucket list', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket first
    await createBucketViaApi(page, 'test-api-list');

    const data = await page.evaluate(async () => {
      const res = await fetch('/_/api/storage/buckets');
      return res.ok ? await res.json() : null;
    });

    expect(data).not.toBeNull();
    expect(Array.isArray(data)).toBeTruthy();

    const testBucket = data.find((b: any) => b.name === 'test-api-list');
    expect(testBucket).toBeDefined();
    expect(testBucket).toHaveProperty('id');
    expect(testBucket).toHaveProperty('name');
    expect(testBucket).toHaveProperty('public');
  });

  test('POST /_/api/storage/buckets creates a bucket', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    const result = await page.evaluate(async () => {
      const res = await fetch('/_/api/storage/buckets', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: 'test-api-create', public: true })
      });
      return {
        ok: res.ok,
        status: res.status,
        data: res.ok ? await res.json() : null
      };
    });

    expect(result.ok).toBeTruthy();
    expect(result.status).toBe(201);
    expect(result.data).toHaveProperty('id');
    expect(result.data.name).toBe('test-api-create');
    expect(result.data.public).toBe(true);
  });

  test('PUT /_/api/storage/buckets/:id updates a bucket', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket first
    const bucket = await createBucketViaApi(page, 'test-api-update', false);

    const result = await page.evaluate(async (bucketId: string) => {
      const res = await fetch(`/_/api/storage/buckets/${bucketId}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ public: true })
      });
      return {
        ok: res.ok,
        data: res.ok ? await res.json() : null
      };
    }, bucket.id);

    expect(result.ok).toBeTruthy();
    expect(result.data.public).toBe(true);
  });

  test('DELETE /_/api/storage/buckets/:id deletes a bucket', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket first
    const bucket = await createBucketViaApi(page, 'test-api-delete');

    const result = await page.evaluate(async (bucketId: string) => {
      const res = await fetch(`/_/api/storage/buckets/${bucketId}`, {
        method: 'DELETE'
      });
      return { ok: res.ok, status: res.status };
    }, bucket.id);

    expect(result.ok).toBeTruthy();
    expect(result.status).toBe(204);
  });

  test('POST /_/api/storage/objects/list returns objects', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);
    await cleanupTestBuckets(page);

    // Create a bucket and upload a file
    await createBucketViaApi(page, 'test-api-objects');
    await uploadFileViaApi(page, 'test-api-objects', 'api-test.txt', 'api test content');

    const result = await page.evaluate(async () => {
      const res = await fetch('/_/api/storage/objects/list', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ bucket: 'test-api-objects', prefix: '', limit: 100 })
      });
      return {
        ok: res.ok,
        data: res.ok ? await res.json() : null
      };
    });

    expect(result.ok).toBeTruthy();
    expect(Array.isArray(result.data)).toBeTruthy();

    const testFile = result.data.find((obj: any) => obj.name === 'api-test.txt');
    expect(testFile).toBeDefined();
  });
});
