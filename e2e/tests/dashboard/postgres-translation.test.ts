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

test.describe('PostgreSQL Translation - UI', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('displays PostgreSQL mode toggle in SQL Browser', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Check that PostgreSQL mode toggle exists
    await expect(page.locator('.postgres-mode-toggle')).toBeVisible();
    await expect(page.locator('.postgres-mode-toggle .toggle-text')).toHaveText('PostgreSQL Mode');

    // Toggle should have a checkbox
    const checkbox = page.locator('.postgres-mode-toggle input[type="checkbox"]');
    await expect(checkbox).toBeVisible();
  });

  test('PostgreSQL mode toggle persists across page reloads', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Enable PostgreSQL mode
    const checkbox = page.locator('.postgres-mode-toggle input[type="checkbox"]');
    const wasChecked = await checkbox.isChecked();

    if (!wasChecked) {
      await checkbox.click();
      await expect(checkbox).toBeChecked();
    }

    // Reload the page
    await page.reload();
    await page.waitForSelector('.sql-browser-view');

    // PostgreSQL mode should still be enabled
    const checkboxAfterReload = page.locator('.postgres-mode-toggle input[type="checkbox"]');
    await expect(checkboxAfterReload).toBeChecked();

    // Clean up - disable it
    await checkboxAfterReload.click();
  });
});

test.describe('PostgreSQL Translation - Date/Time Functions', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('translates NOW() function', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT NOW() as current_time',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain("datetime('now')");
    expect(data.columns).toContain('current_time');
  });

  test('translates CURRENT_TIMESTAMP', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT CURRENT_TIMESTAMP as ts',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain("datetime('now')");
  });

  test('translates CURRENT_DATE', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT CURRENT_DATE as today',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain("date('now')");
  });

  test('translates INTERVAL expressions', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT NOW() - INTERVAL '7 days' as week_ago",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain("'+7 day'");
  });
});

test.describe('PostgreSQL Translation - String Functions', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('translates LEFT() function', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT LEFT('hello world', 5) as result",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('SUBSTR');
    expect(data.rows[0][0]).toBe('hello');
  });

  test('translates RIGHT() function', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT RIGHT('hello world', 5) as result",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('SUBSTR');
    expect(data.rows[0][0]).toBe('world');
  });

  test('translates POSITION() function', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT POSITION('world' IN 'hello world') as pos",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('INSTR');
    expect(data.rows[0][0]).toBe(7);
  });

  test('translates ILIKE to LIKE', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT 'Hello' ILIKE 'hello' as result",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('LIKE');
  });
});

test.describe('PostgreSQL Translation - Type Casts', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('removes ::uuid type casts', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT '550e8400-e29b-41d4-a716-446655440000'::uuid as id",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).not.toContain('::uuid');
    expect(data.rows[0][0]).toBe('550e8400-e29b-41d4-a716-446655440000');
  });

  test('removes ::timestamptz type casts', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT '2024-01-01 12:00:00'::timestamptz as ts",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).not.toContain('::timestamptz');
  });

  test('removes ::text type casts', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT 123::text as str",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).not.toContain('::text');
  });
});

test.describe('PostgreSQL Translation - Boolean Literals', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('translates TRUE to 1', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT TRUE as val',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('1');
    expect(data.rows[0][0]).toBe(1);
  });

  test('translates FALSE to 0', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT FALSE as val',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('0');
    expect(data.rows[0][0]).toBe(0);
  });
});

test.describe('PostgreSQL Translation - CREATE TABLE', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);

    // Clean up any existing test table
    await request.post('/_/api/sql', {
      data: {
        query: 'DROP TABLE IF EXISTS pg_test_users'
      }
    });
  });

  test.afterAll(async ({ request }) => {
    // Clean up test table
    await request.post('/_/api/sql', {
      data: {
        query: 'DROP TABLE IF EXISTS pg_test_users'
      }
    });
  });

  test('translates PostgreSQL data types in CREATE TABLE', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `CREATE TABLE pg_test_users (
          id UUID PRIMARY KEY,
          email TEXT NOT NULL,
          active BOOLEAN DEFAULT TRUE,
          metadata JSONB,
          created_at TIMESTAMPTZ DEFAULT NOW()
        )`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);

    // Check that types were translated
    expect(data.translated_query).toContain('TEXT'); // UUID -> TEXT
    expect(data.translated_query).toContain('INTEGER'); // BOOLEAN -> INTEGER
    expect(data.translated_query).toContain("datetime('now')"); // NOW() -> datetime('now')
  });

  test('translates gen_random_uuid() in DEFAULT', async ({ request }) => {
    // Drop table if exists
    await request.post('/_/api/sql', {
      data: {
        query: 'DROP TABLE IF EXISTS pg_uuid_test'
      }
    });

    const response = await request.post('/_/api/sql', {
      data: {
        query: `CREATE TABLE pg_uuid_test (
          id UUID PRIMARY KEY DEFAULT gen_random_uuid()
        )`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('TEXT'); // UUID -> TEXT
    expect(data.translated_query).toContain('randomblob'); // gen_random_uuid translation

    // Insert a row to verify UUID generation works
    const insertResponse = await request.post('/_/api/sql', {
      data: {
        query: 'INSERT INTO pg_uuid_test DEFAULT VALUES'
      }
    });
    const insertData = await insertResponse.json();
    expect(insertData.error).toBeUndefined();

    // Select the generated UUID
    const selectResponse = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT id FROM pg_uuid_test'
      }
    });
    const selectData = await selectResponse.json();
    expect(selectData.rows.length).toBe(1);

    const uuid = selectData.rows[0][0];
    // Verify it's a valid UUID format (RFC 4122)
    expect(uuid).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i);

    // Clean up
    await request.post('/_/api/sql', {
      data: {
        query: 'DROP TABLE pg_uuid_test'
      }
    });
  });
});

test.describe('PostgreSQL Translation - JSON Operators', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('translates -> JSON operator', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT json_object('name', 'John', 'age', 30) ->'name' as result`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('json_extract');
  });

  test('translates ->> JSON text operator', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT json_object('name', 'John') ->>'name' as result`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('json_extract');
  });
});

test.describe('PostgreSQL Translation - Aggregate Functions', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('translates GREATEST to MAX', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT GREATEST(1, 5, 3) as result',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('MAX');
    expect(data.rows[0][0]).toBe(5);
  });

  test('translates LEAST to MIN', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT LEAST(1, 5, 3) as result',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('MIN');
    expect(data.rows[0][0]).toBe(1);
  });
});

test.describe('PostgreSQL Translation - Disabled Mode', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('does not translate when postgres_mode is false', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT NOW() as current_time',
        postgres_mode: false
      }
    });

    const data = await response.json();
    // Query should fail because NOW() doesn't exist in SQLite
    expect(data.error).toBeDefined();
    expect(data.error).toContain('no such function');
  });

  test('does not translate when postgres_mode is omitted', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT NOW() as current_time'
      }
    });

    const data = await response.json();
    // Query should fail because NOW() doesn't exist in SQLite
    expect(data.error).toBeDefined();
  });
});

test.describe('PostgreSQL Translation - Complex Queries', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);

    // Create a test table
    await request.post('/_/api/sql', {
      data: {
        query: 'DROP TABLE IF EXISTS pg_complex_test'
      }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `CREATE TABLE pg_complex_test (
          id TEXT PRIMARY KEY,
          name TEXT,
          active INTEGER,
          created_at TEXT
        )`
      }
    });

    // Insert test data
    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO pg_complex_test VALUES
          ('1', 'Alice', 1, datetime('now', '-10 days')),
          ('2', 'Bob', 1, datetime('now', '-5 days')),
          ('3', 'Charlie', 0, datetime('now', '-2 days'))`
      }
    });
  });

  test.afterAll(async ({ request }) => {
    await request.post('/_/api/sql', {
      data: {
        query: 'DROP TABLE IF EXISTS pg_complex_test'
      }
    });
  });

  test('translates complex WHERE clause with multiple functions', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          LEFT(name, 3) as short_name,
          active = TRUE as is_active
        FROM pg_complex_test
        WHERE active = TRUE
        AND created_at > NOW() - INTERVAL '7 days'`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('SUBSTR');
    expect(data.translated_query).toContain('active = 1');
    expect(data.translated_query).toContain("datetime('now')");
    expect(data.rows.length).toBe(1); // Only Bob is within 7 days and active
  });
});

test.describe('PostgreSQL Translation - UI Display', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('displays translation info in UI when query is translated', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Enable PostgreSQL mode
    const checkbox = page.locator('.postgres-mode-toggle input[type="checkbox"]');
    const isChecked = await checkbox.isChecked();
    if (!isChecked) {
      await checkbox.click();
    }

    // Type and run a PostgreSQL query
    const editor = page.locator('#sql-editor-input');
    await editor.clear();
    await editor.fill('SELECT NOW() as current_time');

    await page.getByRole('button', { name: /Run Query/ }).click();

    // Wait for results
    await page.waitForSelector('.sql-results-header', { timeout: 5000 });

    // Check that translation info is displayed
    await expect(page.locator('.sql-translation-info')).toBeVisible();
    await expect(page.locator('.sql-translation-info')).toContainText('Translated from PostgreSQL syntax');

    // Check that details can be expanded
    const details = page.locator('.sql-translation-info details');
    await expect(details).toBeVisible();
    await details.locator('summary').click();

    // Check that translated query is shown
    await expect(page.locator('.sql-translated-query')).toBeVisible();
    const translatedQuery = await page.locator('.sql-translated-query').textContent();
    expect(translatedQuery).toContain("datetime('now')");

    // Clean up - disable PostgreSQL mode
    await checkbox.click();
  });

  test('does not display translation info when query is not translated', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.locator('.nav-item').filter({ hasText: 'SQL Browser' }).click();
    await page.waitForSelector('.sql-browser-view');

    // Ensure PostgreSQL mode is disabled
    const checkbox = page.locator('.postgres-mode-toggle input[type="checkbox"]');
    const isChecked = await checkbox.isChecked();
    if (isChecked) {
      await checkbox.click();
    }

    // Type and run a SQLite query
    const editor = page.locator('#sql-editor-input');
    await editor.clear();
    await editor.fill("SELECT datetime('now') as current_time");

    await page.getByRole('button', { name: /Run Query/ }).click();

    // Wait for results
    await page.waitForSelector('.sql-results-header', { timeout: 5000 });

    // Check that translation info is NOT displayed
    await expect(page.locator('.sql-translation-info')).not.toBeVisible();
  });
});
