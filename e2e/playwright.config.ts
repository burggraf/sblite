import { defineConfig, devices } from '@playwright/test';

// Use a separate database for dashboard tests
const DASHBOARD_DB = process.env.SBLITE_DASHBOARD_DB || 'test-dashboard.db';

export default defineConfig({
  testDir: './tests/dashboard',
  fullyParallel: false, // Run tests serially to avoid race conditions
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1, // Single worker for sequential execution
  reporter: 'html',
  use: {
    baseURL: process.env.SBLITE_URL || 'http://localhost:8080',
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
  // Global setup to initialize test database
  globalSetup: './tests/dashboard/global-setup.ts',
});
