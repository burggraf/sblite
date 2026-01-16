/**
 * Global test setup for sblite E2E tests
 *
 * This file runs before all tests and sets up the test environment.
 */

import { beforeAll, afterAll, beforeEach } from 'vitest'
import { createClient } from '@supabase/supabase-js'
import { execSync } from 'child_process'
import * as fs from 'fs'
import * as path from 'path'

// Test configuration
export const TEST_CONFIG = {
  SBLITE_URL: process.env.SBLITE_URL || 'http://localhost:8080',
  SBLITE_ANON_KEY: process.env.SBLITE_ANON_KEY || 'test-anon-key',
  SBLITE_SERVICE_KEY: process.env.SBLITE_SERVICE_KEY || 'test-service-key',
  JWT_SECRET: process.env.SBLITE_JWT_SECRET || 'super-secret-jwt-key-please-change-in-production',
  DB_PATH: process.env.SBLITE_DB_PATH || path.join(__dirname, '../../test.db'),
}

// Create Supabase client for testing
export function createTestClient() {
  return createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
    auth: {
      autoRefreshToken: false,
      persistSession: false,
    },
  })
}

// Create authenticated client
export function createAuthenticatedClient(accessToken: string) {
  return createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
    global: {
      headers: {
        Authorization: `Bearer ${accessToken}`,
      },
    },
    auth: {
      autoRefreshToken: false,
      persistSession: false,
    },
  })
}

// Global setup
beforeAll(async () => {
  console.log('\n📦 Setting up E2E test environment...')
  console.log(`   URL: ${TEST_CONFIG.SBLITE_URL}`)
  console.log(`   DB: ${TEST_CONFIG.DB_PATH}`)
})

afterAll(async () => {
  console.log('\n🧹 Cleaning up E2E test environment...')
})
