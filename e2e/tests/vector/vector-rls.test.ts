/**
 * Vector Search RLS Tests
 *
 * Tests for Row Level Security enforcement with vector search.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

const TEST_PASSWORD = 'testpassword123'
let sessionCookie = ''
let serviceClient: SupabaseClient
let anonClient: SupabaseClient

interface AuthUser {
  id: string
  access_token: string
}

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

/**
 * Enable RLS for a table
 */
async function enableRLS(tableName: string): Promise<void> {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/rls/${tableName}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      'Cookie': sessionCookie
    },
    body: JSON.stringify({ enabled: true })
  })
  if (!response.ok) {
    throw new Error(`Failed to enable RLS`)
  }
}

/**
 * Create RLS policy
 */
async function createPolicy(tableName: string, policyName: string, command: string, usingExpr: string): Promise<void> {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/policies`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'Cookie': sessionCookie
    },
    body: JSON.stringify({
      table_name: tableName,
      policy_name: policyName,
      command: command,
      using_expr: usingExpr,
      check_expr: ''
    })
  })
  if (!response.ok) {
    const error = await response.json()
    throw new Error(`Failed to create policy: ${error.message}`)
  }
}

/**
 * Create a test user and return their credentials
 */
async function createTestUser(email: string, password: string): Promise<AuthUser> {
  const { data, error } = await serviceClient.auth.admin.createUser({
    email,
    password,
    email_confirm: true
  })
  if (error) throw error

  // Sign in to get access token
  const { data: signInData, error: signInError } = await anonClient.auth.signInWithPassword({
    email,
    password
  })
  if (signInError) throw signInError

  return {
    id: data.user.id,
    access_token: signInData.session!.access_token
  }
}

/**
 * Create authenticated client for a user
 */
function createUserClient(accessToken: string): SupabaseClient {
  return createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
    global: {
      headers: {
        Authorization: `Bearer ${accessToken}`
      }
    },
    auth: { autoRefreshToken: false, persistSession: false }
  })
}

describe('Vector Search RLS', () => {
  let user1: AuthUser
  let user2: AuthUser

  beforeAll(async () => {
    await setupDashboardAuth()

    serviceClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    anonClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Cleanup any existing test data
    try {
      await executeSQL('DROP TABLE IF EXISTS rls_vector_docs')
    } catch (e) {}

    // Create test table with user_id for RLS
    await createTable('rls_vector_docs', [
      { name: 'id', type: 'uuid', primary: true },
      { name: 'user_id', type: 'text' },
      { name: 'content', type: 'text' },
      { name: 'embedding', type: 'vector(3)' }
    ])

    // Create test users
    const timestamp = Date.now()
    user1 = await createTestUser(`vectoruser1_${timestamp}@test.com`, 'password123')
    user2 = await createTestUser(`vectoruser2_${timestamp}@test.com`, 'password123')

    // Insert documents owned by different users
    await serviceClient.from('rls_vector_docs').insert([
      { id: '00000000-0000-0000-0000-000000000001', user_id: user1.id, content: 'User1 Doc1', embedding: [1, 0, 0] },
      { id: '00000000-0000-0000-0000-000000000002', user_id: user1.id, content: 'User1 Doc2', embedding: [0.8, 0.6, 0] },
      { id: '00000000-0000-0000-0000-000000000003', user_id: user2.id, content: 'User2 Doc1', embedding: [0, 1, 0] },
      { id: '00000000-0000-0000-0000-000000000004', user_id: user2.id, content: 'User2 Doc2', embedding: [0, 0.8, 0.6] },
    ])

    // Enable RLS and create policy
    await enableRLS('rls_vector_docs')
    await createPolicy('rls_vector_docs', 'user_owns', 'SELECT', "user_id = auth.uid()")
  })

  afterAll(async () => {
    try {
      await executeSQL('DROP TABLE IF EXISTS rls_vector_docs')
    } catch (e) {}
  })

  describe('RLS enforcement', () => {
    it('user1 should only find their own documents', async () => {
      const user1Client = createUserClient(user1.access_token)

      const { data, error } = await user1Client.rpc('vector_search', {
        table_name: 'rls_vector_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_count: 10
      })

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      // Should only see user1's documents
      expect(data.every((row: any) => row.user_id === user1.id)).toBe(true)
      expect(data.length).toBe(2)
    })

    it('user2 should only find their own documents', async () => {
      const user2Client = createUserClient(user2.access_token)

      const { data, error } = await user2Client.rpc('vector_search', {
        table_name: 'rls_vector_docs',
        embedding_column: 'embedding',
        query_embedding: [0, 1, 0],
        match_count: 10
      })

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      // Should only see user2's documents
      expect(data.every((row: any) => row.user_id === user2.id)).toBe(true)
      expect(data.length).toBe(2)
    })

    it('service_role should bypass RLS and find all documents', async () => {
      const { data, error } = await serviceClient.rpc('vector_search', {
        table_name: 'rls_vector_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_count: 10
      })

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      // Service role should see all 4 documents
      expect(data.length).toBe(4)
    })

    it('unauthenticated request should return empty results', async () => {
      const { data, error } = await anonClient.rpc('vector_search', {
        table_name: 'rls_vector_docs',
        embedding_column: 'embedding',
        query_embedding: [1, 0, 0],
        match_count: 10
      })

      // Either returns empty array or error depending on policy
      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      // No user_id matches, so no results
      expect(data.length).toBe(0)
    })

    it('search results should respect RLS even with similarity ordering', async () => {
      const user1Client = createUserClient(user1.access_token)

      // Search with a query that would normally match user2's documents better
      // but RLS should filter them out
      const { data, error } = await user1Client.rpc('vector_search', {
        table_name: 'rls_vector_docs',
        embedding_column: 'embedding',
        query_embedding: [0, 1, 0], // More similar to user2's doc
        match_count: 10
      })

      expect(error).toBeNull()
      // Should still only see user1's documents, even though user2's docs are more similar
      expect(data.every((row: any) => row.user_id === user1.id)).toBe(true)
    })
  })
})
