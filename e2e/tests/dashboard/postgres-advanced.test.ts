import { test, expect } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';

async function ensureSetup(request: any) {
  const status = await request.get('/_/api/auth/status');
  const data = await status.json();
  if (data.needs_setup) {
    await request.post('/_/api/auth/setup', { data: { password: TEST_PASSWORD } });
  }
}

test.describe('PostgreSQL Translation - Array Operations', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('translates ARRAY literal to json_array', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT ARRAY[1, 2, 3] as arr',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('json_array');
    // Result should be a JSON array
    expect(data.rows[0][0]).toBe('[1,2,3]');
  });

  test('translates ARRAY with strings', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT ARRAY['a', 'b', 'c'] as arr",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('json_array');
    expect(data.rows[0][0]).toBe('["a","b","c"]');
  });

  test('translates empty ARRAY', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT ARRAY[] as empty_arr',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.rows[0][0]).toBe('[]');
  });

  test('translates array subscript access', async ({ request }) => {
    // First create a table with array data
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS array_test' }
    });

    await request.post('/_/api/sql', {
      data: {
        query: "CREATE TABLE array_test (id INTEGER, tags TEXT)",
      }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO array_test VALUES (1, '["red", "green", "blue"]')`
      }
    });

    // Test array subscript with PostgreSQL syntax
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT tags[1] as first_tag FROM array_test WHERE id = 1',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('json_extract');
    expect(data.rows[0][0]).toBe('red');

    // Cleanup
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE array_test' }
    });
  });

  test('translates = ANY() operator', async ({ request }) => {
    // Create test table
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS any_test' }
    });

    await request.post('/_/api/sql', {
      data: {
        query: "CREATE TABLE any_test (id INTEGER PRIMARY KEY, name TEXT)"
      }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO any_test VALUES (1, 'Alice'), (2, 'Bob'), (3, 'Charlie')`
      }
    });

    // Test = ANY()
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT name FROM any_test WHERE id = ANY(ARRAY[1, 3]) ORDER BY id",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('EXISTS');
    expect(data.translated_query).toContain('json_each');
    expect(data.rows.length).toBe(2);
    expect(data.rows[0][0]).toBe('Alice');
    expect(data.rows[1][0]).toBe('Charlie');

    // Cleanup
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE any_test' }
    });
  });

  test('translates = ALL() operator', async ({ request }) => {
    // Test = ALL() with a simple expression
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT 5 > ALL(ARRAY[1, 2, 3, 4]) as result",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('NOT EXISTS');
    expect(data.translated_query).toContain('json_each');
    // 5 > ALL([1,2,3,4]) should be true (1)
    expect(data.rows[0][0]).toBe(1);
  });
});

test.describe('PostgreSQL Translation - Window Functions', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);

    // Create test table for window function tests
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS window_test' }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `CREATE TABLE window_test (
          id INTEGER PRIMARY KEY,
          dept TEXT,
          name TEXT,
          salary INTEGER
        )`
      }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO window_test VALUES
          (1, 'Sales', 'Alice', 50000),
          (2, 'Sales', 'Bob', 60000),
          (3, 'Engineering', 'Charlie', 70000),
          (4, 'Engineering', 'Diana', 80000),
          (5, 'Engineering', 'Eve', 75000)`
      }
    });
  });

  test.afterAll(async ({ request }) => {
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS window_test' }
    });
  });

  test('supports ROW_NUMBER() OVER (ORDER BY)', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: 'SELECT name, ROW_NUMBER() OVER (ORDER BY salary DESC) as rank FROM window_test',
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.rows.length).toBe(5);
    // Diana (80000) should be rank 1
    expect(data.rows[0][0]).toBe('Diana');
    expect(data.rows[0][1]).toBe(1);
  });

  test('supports PARTITION BY in window function', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          name,
          dept,
          ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) as dept_rank
        FROM window_test
        ORDER BY dept, dept_rank`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.rows.length).toBe(5);
    // Check Engineering department rankings
    const engineering = data.rows.filter((r: any[]) => r[1] === 'Engineering');
    expect(engineering[0][2]).toBe(1); // Diana is #1 in Engineering
  });

  test('supports SUM() OVER (PARTITION BY)', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          name,
          dept,
          salary,
          SUM(salary) OVER (PARTITION BY dept) as dept_total
        FROM window_test
        ORDER BY dept, name`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    // Engineering total: 70000 + 80000 + 75000 = 225000
    const charlie = data.rows.find((r: any[]) => r[0] === 'Charlie');
    expect(charlie[3]).toBe(225000);
    // Sales total: 50000 + 60000 = 110000
    const alice = data.rows.find((r: any[]) => r[0] === 'Alice');
    expect(alice[3]).toBe(110000);
  });

  test('supports AVG() OVER with frame specification', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          name,
          salary,
          AVG(salary) OVER (ORDER BY salary ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) as moving_avg
        FROM window_test
        ORDER BY salary`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.rows.length).toBe(5);
    // Moving average should be calculated correctly
  });

  test('supports RANK() and DENSE_RANK()', async ({ request }) => {
    // Add duplicate salary for testing RANK
    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO window_test VALUES (6, 'Sales', 'Frank', 60000)`
      }
    });

    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          name,
          salary,
          RANK() OVER (ORDER BY salary DESC) as rank,
          DENSE_RANK() OVER (ORDER BY salary DESC) as dense_rank
        FROM window_test
        ORDER BY salary DESC`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    // Bob and Frank both have 60000, should have same rank
    const bob = data.rows.find((r: any[]) => r[0] === 'Bob');
    const frank = data.rows.find((r: any[]) => r[0] === 'Frank');
    expect(bob[2]).toBe(frank[2]); // Same RANK

    // Remove the test row
    await request.post('/_/api/sql', {
      data: { query: 'DELETE FROM window_test WHERE id = 6' }
    });
  });
});

test.describe('PostgreSQL Translation - Regex Operators', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);

    // Create test table
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS regex_test' }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `CREATE TABLE regex_test (
          id INTEGER PRIMARY KEY,
          email TEXT,
          name TEXT
        )`
      }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO regex_test VALUES
          (1, 'admin@example.com', 'Admin User'),
          (2, 'user@example.com', 'Regular User'),
          (3, 'john.doe@company.org', 'John Doe'),
          (4, 'JANE@COMPANY.ORG', 'Jane Smith')`
      }
    });
  });

  test.afterAll(async ({ request }) => {
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS regex_test' }
    });
  });

  test('translates ~ (regex match) operator', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT name FROM regex_test WHERE email ~ '^admin'",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('REGEXP');
    expect(data.rows.length).toBe(1);
    expect(data.rows[0][0]).toBe('Admin User');
  });

  test('translates ~* (case-insensitive regex) operator', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT name FROM regex_test WHERE email ~* '@company.org$' ORDER BY name",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('LOWER');
    expect(data.translated_query).toContain('REGEXP');
    expect(data.rows.length).toBe(2);
    // Both John Doe and Jane Smith should match (case-insensitive)
  });

  test('translates !~ (regex not match) operator', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT name FROM regex_test WHERE email !~ '@example.com$' ORDER BY name",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('NOT');
    expect(data.translated_query).toContain('REGEXP');
    expect(data.rows.length).toBe(2);
    // John and Jane don't have @example.com emails
  });

  test('translates !~* (case-insensitive regex not match) operator', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT COUNT(*) as cnt FROM regex_test WHERE name !~* '^j'",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.translated_query).toContain('NOT');
    expect(data.translated_query).toContain('LOWER');
    expect(data.translated_query).toContain('REGEXP');
    // Admin User and Regular User don't start with 'j'
    expect(data.rows[0][0]).toBe(2);
  });

  test('supports complex regex patterns', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: "SELECT email FROM regex_test WHERE email ~ '[a-z]+\\.[a-z]+@' ORDER BY email",
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.rows.length).toBe(1);
    expect(data.rows[0][0]).toBe('john.doe@company.org');
  });
});

test.describe('PostgreSQL Translation - Combined Features', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test('supports array with window function', async ({ request }) => {
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          ARRAY[1, 2, 3] as arr,
          ROW_NUMBER() OVER (ORDER BY 1) as row_num`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
  });

  test('supports multiple PostgreSQL features in one query', async ({ request }) => {
    // Create test table
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE IF EXISTS combined_test' }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `CREATE TABLE combined_test (
          id INTEGER PRIMARY KEY,
          name TEXT,
          email TEXT,
          score INTEGER
        )`
      }
    });

    await request.post('/_/api/sql', {
      data: {
        query: `INSERT INTO combined_test VALUES
          (1, 'Alice', 'alice@test.com', 85),
          (2, 'Bob', 'bob@test.com', 90),
          (3, 'Charlie', 'charlie@admin.com', 75)`
      }
    });

    // Query combining multiple features
    const response = await request.post('/_/api/sql', {
      data: {
        query: `SELECT
          name,
          score,
          ROW_NUMBER() OVER (ORDER BY score DESC) as rank,
          score > ALL(ARRAY[70, 80]) as above_80
        FROM combined_test
        WHERE email ~ '@test.com$'
        ORDER BY score DESC`,
        postgres_mode: true
      }
    });

    const data = await response.json();
    expect(data.error).toBeUndefined();
    expect(data.was_translated).toBe(true);
    expect(data.rows.length).toBe(2); // Alice and Bob only

    // Cleanup
    await request.post('/_/api/sql', {
      data: { query: 'DROP TABLE combined_test' }
    });
  });
});
