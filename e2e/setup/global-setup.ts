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
import * as jwt from 'jsonwebtoken'

// JWT secret for test environment
const JWT_SECRET = process.env.SBLITE_JWT_SECRET || 'super-secret-jwt-key-please-change-in-production'

// Generate API keys using the same JWT secret as the server
function generateTestAPIKey(role: string): string {
  return jwt.sign(
    { role, iss: 'sblite', iat: Math.floor(Date.now() / 1000) },
    JWT_SECRET
  )
}

// Test configuration
export const TEST_CONFIG = {
  SBLITE_URL: process.env.SBLITE_URL || 'http://localhost:8080',
  SBLITE_ANON_KEY: process.env.SBLITE_ANON_KEY || generateTestAPIKey('anon'),
  SBLITE_SERVICE_KEY: process.env.SBLITE_SERVICE_KEY || generateTestAPIKey('service_role'),
  JWT_SECRET: JWT_SECRET,
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
