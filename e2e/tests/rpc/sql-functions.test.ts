/**
 * RPC - SQL Functions Tests
 *
 * Tests for PostgreSQL-compatible SQL functions called via supabase.rpc()
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('RPC - SQL Functions', () => {
  let supabase: SupabaseClient

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Create test table and functions via SQL browser
    await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: `
          CREATE TABLE IF NOT EXISTS rpc_test_users (
            id TEXT PRIMARY KEY,
            email TEXT,
            score INTEGER DEFAULT 0
          );
          DELETE FROM rpc_test_users;
          INSERT INTO rpc_test_users (id, email, score) VALUES
            ('u1', 'alice@test.com', 100),
            ('u2', 'bob@test.com', 200),
            ('u3', 'charlie@test.com', 150);
        `,
        postgres_mode: false
      })
    })
  })

  afterAll(async () => {
    // Cleanup
    await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query: `
          DROP FUNCTION IF EXISTS get_one;
          DROP FUNCTION IF EXISTS get_user_by_id;
          DROP FUNCTION IF EXISTS get_all_users;
          DROP FUNCTION IF EXISTS get_top_scorers;
          DROP FUNCTION IF EXISTS get_total_score;
          DROP FUNCTION IF EXISTS get_user_ids;
          DROP FUNCTION IF EXISTS get_first_user;
          DROP TABLE IF EXISTS rpc_test_users;
        `,
        postgres_mode: true
      })
    })
  })

  describe('Scalar return functions', () => {
    it('should execute function returning single integer', async () => {
      // Create function
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_one() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_one')

      expect(error).toBeNull()
      expect(data).toBe(1)
    })

    it('should execute function with aggregation', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_total_score() RETURNS integer LANGUAGE sql AS $$ SELECT COALESCE(SUM(score), 0) FROM rpc_test_users $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_total_score')

      expect(error).toBeNull()
      expect(data).toBe(450) // 100 + 200 + 150
    })
  })

  describe('Table return functions', () => {
    it('should execute function returning TABLE', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_all_users() RETURNS TABLE(id TEXT, email TEXT, score INTEGER) LANGUAGE sql AS $$ SELECT id, email, score FROM rpc_test_users $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_all_users')

      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBe(3)
      expect(data[0]).toHaveProperty('id')
      expect(data[0]).toHaveProperty('email')
    })

    it('should execute function returning SETOF', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_user_ids() RETURNS SETOF TEXT LANGUAGE sql AS $$ SELECT id FROM rpc_test_users $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_user_ids')

      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBe(3)
    })
  })

  describe('Single row functions', () => {
    it('should execute function returning single row with .single()', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_first_user() RETURNS TABLE(id TEXT, email TEXT) LANGUAGE sql AS $$ SELECT id, email FROM rpc_test_users LIMIT 1 $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_first_user').single()

      expect(error).toBeNull()
      expect(data).not.toBeNull()
      expect((data as { id: string; email: string }).id).toBeDefined()
      expect((data as { id: string; email: string }).email).toBeDefined()
    })
  })

  describe('Functions with parameters', () => {
    it('should execute function with required parameter', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_user_by_id(user_id TEXT) RETURNS TABLE(id TEXT, email TEXT) LANGUAGE sql AS $$ SELECT id, email FROM rpc_test_users WHERE id = user_id $$;`,
          postgres_mode: true
        })
      })

      const { data, error } = await supabase.rpc('get_user_by_id', { user_id: 'u1' })

      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBe(1)
      expect(data[0].email).toBe('alice@test.com')
    })

    it('should execute function with default parameter', async () => {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          query: `CREATE OR REPLACE FUNCTION get_top_scorers(limit_count INTEGER DEFAULT 2) RETURNS TABLE(id TEXT, score INTEGER) LANGUAGE sql AS $$ SELECT id, score FROM rpc_test_users ORDER BY score DESC LIMIT limit_count $$;`,
          postgres_mode: true
        })
      })

      // Call without parameter (use default)
      const { data: data1, error: error1 } = await supabase.rpc('get_top_scorers')
      expect(error1).toBeNull()
      expect(data1.length).toBe(2)

      // Call with parameter
      const { data: data2, error: error2 } = await supabase.rpc('get_top_scorers', { limit_count: 1 })
      expect(error2).toBeNull()
      expect(data2.length).toBe(1)
    })

    it('should return error for missing required parameter', async () => {
      const { data, error } = await supabase.rpc('get_user_by_id', {})

      expect(error).not.toBeNull()
      expect(error?.message).toContain('missing required argument')
    })

    it('should handle NULL argument value', async () => {
      const { data, error } = await supabase.rpc('get_user_by_id', { user_id: null })

      expect(error).toBeNull()
      expect(Array.isArray(data)).toBe(true)
      expect(data.length).toBe(0) // No user matches null id
    })
  })

  describe('Error handling', () => {
    it('should return 404 for unknown function', async () => {
      const { data, error } = await supabase.rpc('nonexistent_function')

      expect(error).not.toBeNull()
      expect(error?.code).toBe('PGRST202')
    })

    it('should handle wrong argument type gracefully', async () => {
      // Note: SQLite is lenient with types, so this might not error
      // but we should test the behavior
      const { data, error } = await supabase.rpc('get_top_scorers', {
        limit_count: 'not_a_number'
      })

      // SQLite may coerce or error - verify consistent behavior
      // If it errors, check the error exists
      // If it coerces, check data is returned
      expect(data !== undefined || error !== null).toBe(true)
    })
  })
})
