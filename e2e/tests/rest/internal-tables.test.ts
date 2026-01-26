/**
 * Internal Tables Security Tests
 *
 * Tests that internal/system tables are not accessible via the public REST API.
 * This prevents unauthorized access to sensitive data like:
 * - auth_* tables (user credentials, sessions, tokens)
 * - storage_* tables (storage metadata)
 * - _* tables (internal sblite metadata)
 * - sqlite_* tables (SQLite internal tables)
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - Internal Tables Security', () => {
  let supabase: SupabaseClient
  let serviceRoleClient: SupabaseClient

  beforeAll(() => {
    // Client with anon key (should be blocked from internal tables)
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Client with service role key (should also be blocked from internal tables via REST API)
    serviceRoleClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  describe('Auth tables blocked', () => {
    const authTables = [
      'auth_users',
      'auth_sessions',
      'auth_refresh_tokens',
      'auth_identities',
    ]

    for (const table of authTables) {
      it(`SELECT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).select()

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })

      it(`INSERT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).insert({ test: 'value' })

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })

      it(`UPDATE on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).update({ test: 'value' }).eq('id', 1)

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })

      it(`DELETE on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).delete().eq('id', 1)

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })
    }
  })

  describe('Storage tables blocked', () => {
    const storageTables = [
      'storage_buckets',
      'storage_objects',
    ]

    for (const table of storageTables) {
      it(`SELECT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).select()

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })

      it(`INSERT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).insert({ test: 'value' })

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })
    }
  })

  describe('Internal metadata tables blocked (underscore prefix)', () => {
    const internalTables = [
      '_columns',
      '_dashboard',
      '_rls_policies',
      '_schema_migrations',
    ]

    for (const table of internalTables) {
      it(`SELECT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).select()

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })

      it(`INSERT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).insert({ test: 'value' })

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })
    }
  })

  describe('SQLite internal tables blocked', () => {
    const sqliteTables = [
      'sqlite_sequence',
      'sqlite_master',
    ]

    for (const table of sqliteTables) {
      it(`SELECT on ${table} should return 404`, async () => {
        const { data, error } = await supabase.from(table).select()

        expect(error).not.toBeNull()
        expect(error?.message).toContain('not found')
        expect(data).toBeNull()
      })
    }
  })

  describe('Service role key also blocked', () => {
    it('service_role should not access auth_users via REST API', async () => {
      const { data, error } = await serviceRoleClient.from('auth_users').select()

      expect(error).not.toBeNull()
      expect(error?.message).toContain('not found')
      expect(data).toBeNull()
    })

    it('service_role should not access _columns via REST API', async () => {
      const { data, error } = await serviceRoleClient.from('_columns').select()

      expect(error).not.toBeNull()
      expect(error?.message).toContain('not found')
      expect(data).toBeNull()
    })
  })

  describe('Regular tables still accessible', () => {
    it('should be able to access regular user tables (not blocked by security)', async () => {
      // Try to access a table that doesn't start with auth_, storage_, _ or sqlite_
      // The test_regular_table won't exist, but we should get a different error than "not found"
      // (we'd get a SQLite error about the table not existing, not our security error)
      const { data, error } = await supabase.from('test_regular_table').select()

      // If there's an error, it should be because the table doesn't exist in SQLite,
      // not because it was blocked by our security check
      // Our security error message contains "Table 'x' not found" while SQLite error is different
      if (error) {
        expect(error.message).not.toMatch(/Table 'test_regular_table' not found/)
      }
    })

    it('tables with similar names should still work', async () => {
      // A table named "authentication_logs" or "user_storage" should not be blocked
      // (they don't start with auth_ or storage_, they just contain those words)
      // This test just verifies the logic doesn't over-block
      // Note: These tables may not exist in test setup, so we check the error type
      const { error: authLogsError } = await supabase.from('authentication_logs').select()
      const { error: userStorageError } = await supabase.from('user_storage').select()

      // These should either work (if table exists) or fail with a different error (table doesn't exist)
      // but NOT fail with "table_not_found" security error
      if (authLogsError) {
        // If there's an error, it should be because the table doesn't exist in the DB,
        // not because it was blocked by our security check
        // The error message from SQLite would be different from our security error
        expect(authLogsError.message).not.toContain("Table 'authentication_logs' not found")
      }
      if (userStorageError) {
        expect(userStorageError.message).not.toContain("Table 'user_storage' not found")
      }
    })
  })
})
