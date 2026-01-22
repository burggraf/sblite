/**
 * Vector Search Tests
 *
 * Tests for vector similarity search via the vector_search RPC function.
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

describe('Vector Search', () => {
  beforeAll(async () => {
    await setupDashboardAuth()

    serviceClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Cleanup any existing test data
    try {
      await executeSQL('DROP TABLE IF EXISTS search_docs')
    } catch (e) {}

    // Create test table with vector column
    await createTable('search_docs', [
      { name: 'id', type: 'uuid', primary: true },
      { name: 'title', type: 'text' },
      { name: 'category', type: 'text' },
      { name: 'embedding', type: 'vector(3)' }
    ])

    // Insert test documents with embeddings
    // Using simple vectors for predictable similarity testing:
    // - doc1: [1, 0, 0] - points along x-axis
    // - doc2: [0, 1, 0] - points along y-axis (orthogonal to doc1)
    // - doc3: [0.707, 0.707, 0] - 45 degrees between x and y
    // - doc4: [0.5, 0.5, 0.707] - has z component
    await serviceClient.from('search_docs').insert([
      { id: '00000000-0000-0000-0000-000000000001', title: 'Document 1', category: 'A', embedding: [1, 0, 0] },
      { id: '00000000-0000-0000-0000-000000000002', title: 'Document 2', category: 'B', embedding: [0, 1, 0] },
      { id: '00000000-0000-0000-0000-000000000003', title: 'Document 3', category: 'A', embedding: [0.707, 0.707, 0] },
      { id: '00000000-0000-0000-0000-000000000004', title: 'Document 4', category: 'B', embedding: [0.5, 0.5, 0.707] },
      { id: '00000000-0000-0000-0000-000000000005', title: 'Document 5', category: 'A', embedding: null }, // No embedding
    ])
  })

  afterAll(async () => {
    try {
      await executeSQL('DROP TABLE IF EXISTS search_docs')
    } catch (e) {}
  })

  describe('Basic search', () => {
    it('should find similar documents using cosine similarity', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0], // Same as doc1
        match_count: 3
      })

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBeLessThanOrEqual(3)

      // First result should be doc1 (identical vector, similarity ≈ 1)
      expect(data[0].title).toBe('Document 1')
      expect(data[0].similarity).toBeCloseTo(1, 5)
    })

    it('should return results sorted by similarity', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_count: 10
      })

      expect(error).toBeNull()
      expect(data.length).toBeGreaterThan(1)

      // Verify descending order by similarity
      for (let i = 1; i < data.length; i++) {
        expect(data[i - 1].similarity).toBeGreaterThanOrEqual(data[i].similarity)
      }
    })

    it('should respect match_count limit', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_count: 2
      })

      expect(error).toBeNull()
      expect(data.length).toBe(2)
    })

    it('should apply match_threshold filter', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_threshold: 0.9, // High threshold
        match_count: 10
      })

      expect(error).toBeNull()
      // Only doc1 should match with similarity ≈ 1
      expect(data.length).toBeLessThanOrEqual(2)
      data.forEach((row: any) => {
        expect(row.similarity).toBeGreaterThanOrEqual(0.9)
      })
    })

    it('should exclude rows with null embeddings', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_count: 10
      })

      expect(error).toBeNull()
      // Document 5 has null embedding and should be excluded
      expect(data.every((row: any) => row.title !== 'Document 5')).toBe(true)
    })
  })

  describe('Different metrics', () => {
    it('should search using L2 distance', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        metric: 'l2',
        match_count: 3
      })

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      // First result should still be doc1 (zero distance)
      expect(data[0].title).toBe('Document 1')
      // For L2, similarity is negative distance (so 0 is best)
      expect(data[0].similarity).toBeCloseTo(0, 5)
    })

    it('should search using dot product', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        metric: 'dot',
        match_count: 3
      })

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      // First result should be doc1
      expect(data[0].title).toBe('Document 1')
    })

    it('should reject invalid metric', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        metric: 'invalid_metric'
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('metric')
    })
  })

  describe('Filter parameter', () => {
    it('should apply additional filter conditions', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        filter: { category: 'A' },
        match_count: 10
      })

      expect(error).toBeNull()
      expect(data.every((row: any) => row.category === 'A')).toBe(true)
    })
  })

  describe('Column selection', () => {
    it('should select specific columns', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        select_columns: ['id', 'title'],
        match_count: 1
      })

      expect(error).toBeNull()
      expect(data[0]).toHaveProperty('id')
      expect(data[0]).toHaveProperty('title')
      expect(data[0]).toHaveProperty('similarity')
      // Category was not selected, but embedding column handling varies
    })
  })

  describe('Error handling', () => {
    it('should return error for missing table_name', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0]
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('table_name')
    })

    it('should return error for missing embedding_column', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        query_embedding: [1, 0, 0]
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('embedding_column')
    })

    it('should return error for missing query_embedding', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding'
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('query_embedding')
    })

    it('should return error for non-existent table', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'nonexistent_table',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0]
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('not found')
    })

    it('should return error for non-existent column', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'nonexistent_column',
        query_embedding: [1, 0, 0]
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('not found')
    })

    it('should return error for non-vector column', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'title', // text column, not vector
        query_embedding: [1, 0, 0]
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('not a vector')
    })

    it('should return error for dimension mismatch', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'search_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0, 0, 0] // 5 dimensions, column has 3
      })

      expect(error).not.toBeNull()
      expect(error?.message).toContain('dimension')
    })
  })
})
