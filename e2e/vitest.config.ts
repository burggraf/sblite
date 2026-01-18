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
    // Note: Run with --no-file-parallelism --max-workers=1 for storage tests
    fileParallelism: false,
    pool: 'threads',
    poolOptions: {
      threads: {
        singleThread: true,
      },
    },
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
