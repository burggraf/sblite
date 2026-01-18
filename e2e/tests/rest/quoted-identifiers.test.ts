/**
 * Quoted Identifiers Tests
 *
 * Tests for tables and columns with spaces or special characters in their names.
 * These require proper SQL identifier quoting to work correctly.
 *
 * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables-with-spaces
 *
 * Note: supabase-js has a client-side limitation where .select('column name') strips spaces
 * from column names. The raw REST API works correctly. Filters (.eq(), .order(), etc.)
 * handle spaces properly. Tests use select('*') and verify via filters/order.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - Quoted Identifiers', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * SELECT operations with quoted identifiers
   */
  describe('SELECT', () => {
    it('should select all columns from table with spaces in name', async () => {
      const { data, error } = await supabase.from('my table').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(true)
      expect(data!.length).toBe(3)
      // Verify columns with spaces are returned correctly
      expect(data![0]).toHaveProperty('my column')
      expect(data![0]).toHaveProperty('another column')
    })

    it('should select specific column with spaces via raw API', async () => {
      // Note: supabase-js strips spaces in .select(), so we use raw fetch
      const response = await fetch(
        `${TEST_CONFIG.SBLITE_URL}/rest/v1/my%20table?select=my%20column`,
        { headers: { apikey: TEST_CONFIG.SBLITE_ANON_KEY } }
      )
      const data = await response.json()

      expect(response.ok).toBe(true)
      expect(data.length).toBe(3)
      expect(data[0]).toHaveProperty('my column')
      expect(data[0]).not.toHaveProperty('id')
    })

    it('should select multiple columns with spaces via raw API', async () => {
      const response = await fetch(
        `${TEST_CONFIG.SBLITE_URL}/rest/v1/my%20table?select=my%20column,another%20column`,
        { headers: { apikey: TEST_CONFIG.SBLITE_ANON_KEY } }
      )
      const data = await response.json()

      expect(response.ok).toBe(true)
      expect(data.length).toBe(3)
      expect(data[0]).toHaveProperty('my column')
      expect(data[0]).toHaveProperty('another column')
      expect(data[0]).not.toHaveProperty('id')
    })

    it('should filter on column with spaces in name', async () => {
      const { data, error } = await supabase
        .from('my table')
        .select()
        .eq('my column', 'first row')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0]['my column']).toBe('first row')
    })

    it('should order by column with spaces in name', async () => {
      const { data, error } = await supabase
        .from('my table')
        .select()
        .order('another column', { ascending: false })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(3)
      expect(data![0]['another column']).toBe(300)
      expect(data![2]['another column']).toBe(100)
    })
  })

  /**
   * INSERT operations with quoted identifiers
   */
  describe('INSERT', () => {
    it('should insert row into table with spaces in name', async () => {
      const { data, error } = await supabase
        .from('my table')
        .insert({ id: 100, 'my column': 'inserted row', 'another column': 999 })
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0]['my column']).toBe('inserted row')
      expect(data![0]['another column']).toBe(999)
    })

    afterAll(async () => {
      // Clean up inserted row
      await supabase.from('my table').delete().eq('id', 100)
    })
  })

  /**
   * UPDATE operations with quoted identifiers
   */
  describe('UPDATE', () => {
    it('should update column with spaces in name', async () => {
      // First insert a test row
      await supabase
        .from('my table')
        .insert({ id: 101, 'my column': 'to update', 'another column': 0 })

      // Update it
      const { data, error } = await supabase
        .from('my table')
        .update({ 'my column': 'updated value', 'another column': 42 })
        .eq('id', 101)
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0]['my column']).toBe('updated value')
      expect(data![0]['another column']).toBe(42)

      // Clean up
      await supabase.from('my table').delete().eq('id', 101)
    })
  })

  /**
   * DELETE operations with quoted identifiers
   */
  describe('DELETE', () => {
    it('should delete from table with spaces filtering on column with spaces', async () => {
      // First insert a test row
      await supabase
        .from('my table')
        .insert({ id: 102, 'my column': 'to delete', 'another column': 0 })

      // Verify it exists
      const { data: before } = await supabase
        .from('my table')
        .select()
        .eq('id', 102)
      expect(before!.length).toBe(1)

      // Delete it
      const { error } = await supabase
        .from('my table')
        .delete()
        .eq('my column', 'to delete')

      expect(error).toBeNull()

      // Verify it's gone
      const { data: after } = await supabase
        .from('my table')
        .select()
        .eq('id', 102)
      expect(after!.length).toBe(0)
    })
  })

  /**
   * Edge cases
   */
  describe('Edge Cases', () => {
    it('should handle comparison operators on columns with spaces', async () => {
      const { data, error } = await supabase
        .from('my table')
        .select()
        .gt('another column', 150)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(2) // rows with 200 and 300
    })

    it('should handle IN filter on column with spaces', async () => {
      const { data, error } = await supabase
        .from('my table')
        .select()
        .in('my column', ['first row', 'third row'])

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(2)
    })

    it('should handle LIKE filter on column with spaces', async () => {
      const { data, error } = await supabase
        .from('my table')
        .select()
        .like('my column', '%row')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(3)
    })
  })
})
