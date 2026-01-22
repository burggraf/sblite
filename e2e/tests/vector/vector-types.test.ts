/**
 * Vector Types Tests
 *
 * Tests for vector type validation and storage in sblite.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

const TEST_PASSWORD = 'testpassword123'
let sessionCookie = ''
let serviceClient: SupabaseClient

/**
 * Setup dashboard auth
 */
async function setupDashboardAuth(): Promise<void> {
  const statusRes = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/auth/status`)
  const status = await statusRes.json()

  if (status.needs_setup) {
    await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/auth/setup`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password: TEST_PASSWORD })
    })
  }

  const loginRes = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password: TEST_PASSWORD })
  })

  const setCookie = loginRes.headers.get('set-cookie')
  if (setCookie) {
    sessionCookie = setCookie.split(';')[0]
  }
}

/**
 * Execute SQL via dashboard API
 */
async function executeSQL(query: string): Promise<void> {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Cookie': sessionCookie
    },
    body: JSON.stringify({ query, postgres_mode: false })
  })
  const result = await response.json()
  if (result.error) {
    throw new Error(`SQL execution failed: ${result.error}`)
  }
}

/**
 * Create table via Admin API
 */
async function createTable(name: string, columns: Array<{ name: string; type: string; primary?: boolean }>): Promise<void> {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'apikey': TEST_CONFIG.SBLITE_SERVICE_KEY
    },
    body: JSON.stringify({ name, columns })
  })
  if (!response.ok) {
    const error = await response.json()
    throw new Error(`Failed to create table: ${error.message}`)
  }
}

describe('Vector Types', () => {
  beforeAll(async () => {
    await setupDashboardAuth()

    serviceClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Create test table with vector column
    try {
      await createTable('vector_test', [
        { name: 'id', type: 'uuid', primary: true },
        { name: 'content', type: 'text' },
        { name: 'embedding', type: 'vector(3)' }
      ])
    } catch (e) {
      // Table may already exist
    }
  })

  afterAll(async () => {
    // Cleanup
    try {
      await executeSQL('DROP TABLE IF EXISTS vector_test')
    } catch (e) {
      // Ignore cleanup errors
    }
  })

  describe('Vector insertion', () => {
    it('should insert valid vector as JSON array', async () => {
      const { data, error } = await serviceClient
        .from('vector_test')
        .insert({
          id: '00000000-0000-0000-0000-000000000001',
          content: 'test document',
          embedding: [0.1, 0.2, 0.3]
        })
        .select()
        .single()

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      expect(data.content).toBe('test document')
    })

    it('should insert valid vector as string', async () => {
      const { data, error } = await serviceClient
        .from('vector_test')
        .insert({
          id: '00000000-0000-0000-0000-000000000002',
          content: 'another document',
          embedding: '[0.4, 0.5, 0.6]'
        })
        .select()
        .single()

      expect(error).toBeNull()
      expect(data).not.toBeNull()
    })

    it('should reject vector with wrong dimension', async () => {
      const { data, error } = await serviceClient
        .from('vector_test')
        .insert({
          id: '00000000-0000-0000-0000-000000000003',
          content: 'wrong dimension',
          embedding: [0.1, 0.2, 0.3, 0.4] // 4 elements, expected 3
        })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('dimension')
    })

    it('should reject non-numeric vector elements', async () => {
      const { data, error } = await serviceClient
        .from('vector_test')
        .insert({
          id: '00000000-0000-0000-0000-000000000004',
          content: 'invalid vector',
          embedding: '[0.1, "text", 0.3]'
        })

      expect(error).not.toBeNull()
    })

    it('should allow null embedding', async () => {
      const { data, error } = await serviceClient
        .from('vector_test')
        .insert({
          id: '00000000-0000-0000-0000-000000000005',
          content: 'no embedding',
          embedding: null
        })
        .select()
        .single()

      expect(error).toBeNull()
      expect(data.embedding).toBeNull()
    })
  })

  describe('Vector retrieval', () => {
    it('should retrieve vector as JSON array', async () => {
      const { data, error } = await serviceClient
        .from('vector_test')
        .select('*')
        .eq('id', '00000000-0000-0000-0000-000000000001')
        .single()

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      // Vector is stored as JSON array string
      expect(typeof data.embedding).toBe('string')
      const parsed = JSON.parse(data.embedding)
      expect(Array.isArray(parsed)).toBe(true)
      expect(parsed.length).toBe(3)
    })
  })

  describe('Vector update', () => {
    it('should update vector value', async () => {
      const { error: updateError } = await serviceClient
        .from('vector_test')
        .update({ embedding: [0.7, 0.8, 0.9] })
        .eq('id', '00000000-0000-0000-0000-000000000001')

      expect(updateError).toBeNull()

      const { data, error } = await serviceClient
        .from('vector_test')
        .select('embedding')
        .eq('id', '00000000-0000-0000-0000-000000000001')
        .single()

      expect(error).toBeNull()
      const parsed = JSON.parse(data.embedding)
      expect(parsed[0]).toBeCloseTo(0.7)
    })
  })
})
