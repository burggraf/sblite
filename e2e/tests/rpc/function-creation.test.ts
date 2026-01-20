/**
 * RPC - Function Creation Tests
 *
 * Tests for CREATE/DROP FUNCTION via SQL browser
 */

import { describe, it, expect, afterAll } from 'vitest'
import { TEST_CONFIG } from '../../setup/global-setup'

/**
 * Helper function to execute SQL via the dashboard API.
 * Returns the JSON response (does not throw on SQL errors).
 */
async function executeSql(query: string, postgresMode = true) {
  const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/_/api/sql`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query, postgres_mode: postgresMode })
  })
  return response.json()
}

describe('RPC - Function Creation', () => {
  afterAll(async () => {
    // Cleanup all test functions
    await executeSql(`
      DROP FUNCTION IF EXISTS test_create_func;
      DROP FUNCTION IF EXISTS test_replace_func;
      DROP FUNCTION IF EXISTS test_drop_func;
      DROP FUNCTION IF EXISTS get_current_time;
      DROP FUNCTION IF EXISTS get_multi_col;
      DROP FUNCTION IF EXISTS get_ids;
      DROP FUNCTION IF EXISTS test_quote_tags;
      DROP FUNCTION IF EXISTS test_quotes;
      DROP FUNCTION IF EXISTS plpgsql_func;
    `)
  })

  describe('CREATE FUNCTION', () => {
    it('should create function via SQL browser', async () => {
      const result = await executeSql(`
        CREATE FUNCTION test_create_func() RETURNS integer LANGUAGE sql AS $$ SELECT 42 $$;
      `)

      expect(result.error).toBeUndefined()
      expect(result.rows[0][0]).toContain('CREATE FUNCTION')
    })

    it('should fail if function already exists', async () => {
      // First ensure the function exists
      await executeSql(`CREATE OR REPLACE FUNCTION test_create_func() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`)

      // Second create without OR REPLACE should fail
      const result = await executeSql(`CREATE FUNCTION test_create_func() RETURNS integer LANGUAGE sql AS $$ SELECT 2 $$;`)

      expect(result.error).toBeDefined()
      expect(result.error).toContain('already exists')
    })
  })

  describe('CREATE OR REPLACE FUNCTION', () => {
    it('should update existing function', async () => {
      // Create initial
      await executeSql(`CREATE OR REPLACE FUNCTION test_replace_func() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`)

      // Replace
      const result = await executeSql(`CREATE OR REPLACE FUNCTION test_replace_func() RETURNS integer LANGUAGE sql AS $$ SELECT 2 $$;`)

      expect(result.error).toBeUndefined()
    })
  })

  describe('DROP FUNCTION', () => {
    it('should drop existing function', async () => {
      // Create first
      await executeSql(`CREATE OR REPLACE FUNCTION test_drop_func() RETURNS integer LANGUAGE sql AS $$ SELECT 1 $$;`)

      // Drop
      const result = await executeSql(`DROP FUNCTION test_drop_func;`)

      expect(result.error).toBeUndefined()
      expect(result.rows[0][0]).toContain('DROP FUNCTION')
    })

    it('should fail if function does not exist', async () => {
      const result = await executeSql(`DROP FUNCTION nonexistent_func;`)

      expect(result.error).toBeDefined()
    })

    it('should succeed with IF EXISTS when function does not exist', async () => {
      const result = await executeSql(`DROP FUNCTION IF EXISTS nonexistent_func;`)

      expect(result.error).toBeUndefined()
    })
  })

  describe('Language validation', () => {
    it('should reject LANGUAGE plpgsql', async () => {
      const result = await executeSql(`
        CREATE FUNCTION plpgsql_func() RETURNS void LANGUAGE plpgsql AS $$ BEGIN END; $$;
      `)

      expect(result.error).toBeDefined()
      expect(result.error).toContain('only LANGUAGE sql is supported')
    })
  })

  describe('PostgreSQL translation', () => {
    it('should translate NOW() in function body', async () => {
      const result = await executeSql(`
        CREATE OR REPLACE FUNCTION get_current_time() RETURNS text LANGUAGE sql AS $$ SELECT NOW() $$;
      `)

      expect(result.error).toBeUndefined()
    })
  })

  describe('RETURNS TABLE and SETOF', () => {
    it('should parse complex RETURNS TABLE definitions', async () => {
      const result = await executeSql(`
        CREATE OR REPLACE FUNCTION get_multi_col()
        RETURNS TABLE(id text, name text, score integer)
        LANGUAGE sql AS $$ SELECT 'a', 'test', 100 $$;
      `)
      expect(result.error).toBeUndefined()
      expect(result.rows[0][0]).toContain('CREATE FUNCTION')
    })

    it('should parse RETURNS SETOF type', async () => {
      const result = await executeSql(`
        CREATE OR REPLACE FUNCTION get_ids()
        RETURNS SETOF text LANGUAGE sql AS $$ SELECT 'a' UNION SELECT 'b' $$;
      `)
      expect(result.error).toBeUndefined()
      expect(result.rows[0][0]).toContain('CREATE FUNCTION')
    })
  })

  describe('Dollar-quoted bodies', () => {
    it('should handle different dollar-quote tags', async () => {
      const result = await executeSql(`
        CREATE OR REPLACE FUNCTION test_quote_tags()
        RETURNS text LANGUAGE sql AS $body$ SELECT 'test' $body$;
      `)
      expect(result.error).toBeUndefined()
    })

    it('should handle bodies with single quotes', async () => {
      const result = await executeSql(`
        CREATE OR REPLACE FUNCTION test_quotes()
        RETURNS text LANGUAGE sql AS $$ SELECT 'hello''world' $$;
      `)
      expect(result.error).toBeUndefined()
    })
  })
})
