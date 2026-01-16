/**
 * Test helpers and utilities for sblite E2E tests
 */

import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from './global-setup'

// Generate unique test identifiers
export function uniqueId(prefix: string = 'test'): string {
  return `${prefix}_${Date.now()}_${Math.random().toString(36).substring(7)}`
}

// Generate unique email for testing
export function uniqueEmail(): string {
  return `test_${Date.now()}_${Math.random().toString(36).substring(7)}@example.com`
}

// Wait for a condition to be true
export async function waitFor(
  condition: () => Promise<boolean>,
  timeout: number = 5000,
  interval: number = 100
): Promise<void> {
  const start = Date.now()
  while (Date.now() - start < timeout) {
    if (await condition()) return
    await new Promise((resolve) => setTimeout(resolve, interval))
  }
  throw new Error(`Timeout waiting for condition after ${timeout}ms`)
}

// Test data generators
export const testData = {
  // Characters table (from Supabase docs examples)
  characters: [
    { id: 1, name: 'Luke', homeworld: 'Tatooine' },
    { id: 2, name: 'Leia', homeworld: 'Alderaan' },
    { id: 3, name: 'Han', homeworld: 'Corellia' },
    { id: 4, name: 'Yoda', homeworld: 'Dagobah' },
    { id: 5, name: 'Chewbacca', homeworld: 'Kashyyyk' },
  ],

  // Countries table
  countries: [
    { id: 1, name: 'United States', code: 'US' },
    { id: 2, name: 'Canada', code: 'CA' },
    { id: 3, name: 'Mexico', code: 'MX' },
  ],

  // Instruments table
  instruments: [
    { id: 1, name: 'violin', section_id: 1 },
    { id: 2, name: 'viola', section_id: 1 },
    { id: 3, name: 'flute', section_id: 2 },
    { id: 4, name: 'clarinet', section_id: 2 },
    { id: 5, name: 'piano', section_id: 3 },
  ],

  // Orchestral sections table
  orchestral_sections: [
    { id: 1, name: 'strings' },
    { id: 2, name: 'woodwinds' },
    { id: 3, name: 'percussion' },
  ],

  // Cities table (for filter examples)
  cities: [
    { id: 1, name: 'New York', country_id: 1, population: 8336817 },
    { id: 2, name: 'Los Angeles', country_id: 1, population: 3979576 },
    { id: 3, name: 'Toronto', country_id: 2, population: 2731571 },
    { id: 4, name: 'Vancouver', country_id: 2, population: 631486 },
    { id: 5, name: 'Smalltown', country_id: 1, population: 5000 },
  ],

  // Users table (for JSON examples)
  users: [
    {
      id: 1,
      name: 'John Doe',
      address: { street: '123 Main St', city: 'New York', postcode: 10001 },
    },
    {
      id: 2,
      name: 'Jane Smith',
      address: { street: '456 Oak Ave', city: 'Beverly Hills', postcode: 90210 },
    },
  ],

  // Issues table (for array examples)
  issues: [
    { id: 1, title: 'Bug: Login fails', tags: ['is:open', 'priority:high'] },
    { id: 2, title: 'Feature: Dark mode', tags: ['is:open', 'priority:low'] },
    { id: 3, title: 'Bug: Fixed crash', tags: ['is:closed', 'severity:high'] },
  ],

  // Quotes table (for text search)
  quotes: [
    { id: 1, catchphrase: 'The quick brown fox jumps over the lazy dog' },
    { id: 2, catchphrase: 'The fat cat sat on the mat' },
    { id: 3, catchphrase: 'A rolling stone gathers no moss' },
  ],

  // Classes table (for containedBy)
  classes: [
    { id: 1, name: 'Morning Yoga', days: ['monday', 'wednesday', 'friday'] },
    { id: 2, name: 'Evening Spin', days: ['tuesday', 'thursday'] },
    { id: 3, name: 'Weekend Run', days: ['saturday', 'sunday'] },
  ],

  // Reservations table (for range examples)
  reservations: [
    { id: 1, room: 'A', during: '[2000-01-01 09:00, 2000-01-01 10:00)' },
    { id: 2, room: 'B', during: '[2000-01-01 12:00, 2000-01-01 14:00)' },
    { id: 3, room: 'A', during: '[2000-01-02 08:00, 2000-01-02 09:00)' },
  ],

  // Messages table (for self-join examples)
  messages: [
    { id: 1, content: 'Hello!', sender_id: 1, receiver_id: 2 },
    { id: 2, content: 'Hi there!', sender_id: 2, receiver_id: 1 },
  ],

  // Teams table (for many-to-many)
  teams: [
    { id: 1, name: 'Team Alpha' },
    { id: 2, name: 'Team Beta' },
  ],

  // User teams junction
  user_teams: [
    { user_id: 1, team_id: 1 },
    { user_id: 1, team_id: 2 },
    { user_id: 2, team_id: 1 },
  ],
}

// Auth test helpers
export async function signUpTestUser(
  client: SupabaseClient,
  email?: string,
  password: string = 'test-password-123'
) {
  const testEmail = email || uniqueEmail()
  const { data, error } = await client.auth.signUp({
    email: testEmail,
    password,
  })
  return { data, error, email: testEmail, password }
}

export async function signInTestUser(
  client: SupabaseClient,
  email: string,
  password: string
) {
  const { data, error } = await client.auth.signInWithPassword({
    email,
    password,
  })
  return { data, error }
}

// REST API test helpers
export interface TestResult {
  passed: boolean
  expected: any
  actual: any
  error?: string
}

export function assertDeepEqual(expected: any, actual: any): TestResult {
  const passed = JSON.stringify(expected) === JSON.stringify(actual)
  return {
    passed,
    expected,
    actual,
    error: passed ? undefined : `Expected ${JSON.stringify(expected)}, got ${JSON.stringify(actual)}`,
  }
}

// Compatibility status tracking
export type CompatibilityStatus = 'pass' | 'fail' | 'partial' | 'not_implemented'

export interface CompatibilityTest {
  name: string
  category: string
  subcategory: string
  docUrl: string
  status: CompatibilityStatus
  notes?: string
}
