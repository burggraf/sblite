import { defineConfig } from 'vitest/config'

export default defineConfig({
  test: {
    globals: true,
    environment: 'node',
    testTimeout: 30000,
    hookTimeout: 30000,
    setupFiles: ['./setup/global-setup.ts'],
    include: ['**/*.test.ts'],
    // Run test files sequentially to avoid database state conflicts
    fileParallelism: false,
    sequence: {
      shuffle: false,
    },
    reporters: ['verbose'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html'],
    },
  },
})
