import { execSync } from 'child_process';
import { request } from '@playwright/test';

const TEST_PASSWORD = 'testpassword123';
const BASE_URL = process.env.SBLITE_URL || 'http://localhost:8080';

async function globalSetup() {
  console.log('Dashboard test global setup starting...');

  // Reset the dashboard table to clear any existing password
  const dbPath = process.env.SBLITE_DB_PATH || '../test.db';
  try {
    execSync(`sqlite3 "${dbPath}" "DELETE FROM _dashboard WHERE key = 'password_hash';"`, {
      cwd: process.cwd(),
      stdio: 'inherit',
    });
    console.log('Dashboard password reset successfully');
  } catch (error) {
    console.log('Note: Could not reset dashboard (table may not exist yet)');
  }

  // Verify the server is running and check auth status
  const context = await request.newContext({ baseURL: BASE_URL });
  try {
    const response = await context.get('/_/api/auth/status');
    const data = await response.json();
    console.log('Auth status:', data);

    // If setup is needed, do it now so all tests have a consistent starting point
    if (data.needs_setup) {
      const setupResponse = await context.post('/_/api/auth/setup', {
        data: { password: TEST_PASSWORD },
      });
      if (setupResponse.ok()) {
        console.log('Dashboard setup completed with test password');
      } else {
        console.error('Dashboard setup failed:', await setupResponse.text());
      }
    }
  } catch (error) {
    console.error('Could not connect to server. Make sure sblite is running on', BASE_URL);
    throw error;
  } finally {
    await context.dispose();
  }

  console.log('Dashboard test global setup complete');
}

export default globalSetup;
