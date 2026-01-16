/**
 * DELETE Operation Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/delete
 *
 * Each test corresponds to an example from the documentation.
 */

import { describe, it, expect, beforeAll, beforeEach, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - DELETE Operations', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  // Set up test data before each test
  beforeEach(async () => {
    await supabase.from('countries').insert([
      { id: 9001, name: 'Delete_Test_1', code: 'DT1' },
      { id: 9002, name: 'Delete_Test_2', code: 'DT2' },
      { id: 9003, name: 'Delete_Test_3', code: 'DT3' },
    ])
  })

  // Clean up after each test
  afterEach(async () => {
    await supabase.from('countries').delete().gte('id', 9000)
  })

  /**
   * Example 1: Delete a single record
   * Docs: https://supabase.com/docs/reference/javascript/delete#delete-a-single-record
   *
   * const response = await supabase
   *   .from('countries')
   *   .delete()
   *   .eq('id', 1)
   */
  describe('1. Delete a single record', () => {
    it('should delete a single record matching the filter', async () => {
      const { error } = await supabase
        .from('countries')
        .delete()
        .eq('id', 9001)

      expect(error).toBeNull()

      // Verify the record was deleted
      const { data: verify } = await supabase
        .from('countries')
        .select()
        .eq('id', 9001)

      expect(verify).toEqual([])
    })

    it('should not affect other records', async () => {
      await supabase.from('countries').delete().eq('id', 9001)

      // Verify other records still exist
      const { data: remaining } = await supabase
        .from('countries')
        .select()
        .in('id', [9002, 9003])

      expect(remaining!.length).toBe(2)
    })
  })

  /**
   * Example 2: Delete a record and return it
   * Docs: https://supabase.com/docs/reference/javascript/delete#delete-a-record-and-return-it
   *
   * const { data, error } = await supabase
   *   .from('countries')
   *   .delete()
   *   .eq('id', 1)
   *   .select()
   */
  describe('2. Delete a record and return it', () => {
    it('should delete and return the deleted record', async () => {
      const { data, error } = await supabase
        .from('countries')
        .delete()
        .eq('id', 9001)
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].id).toBe(9001)
      expect(data![0].name).toBe('Delete_Test_1')
    })

    it('should return only selected columns', async () => {
      const { data, error } = await supabase
        .from('countries')
        .delete()
        .eq('id', 9002)
        .select('name')

      expect(error).toBeNull()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
    })
  })

  /**
   * Example 3: Delete multiple records
   * Docs: https://supabase.com/docs/reference/javascript/delete#delete-multiple-records
   *
   * const response = await supabase
   *   .from('countries')
   *   .delete()
   *   .in('id', [1, 2, 3])
   */
  describe('3. Delete multiple records', () => {
    it('should delete multiple records using in() filter', async () => {
      const { error } = await supabase
        .from('countries')
        .delete()
        .in('id', [9001, 9002])

      expect(error).toBeNull()

      // Verify the records were deleted
      const { data: verify } = await supabase
        .from('countries')
        .select()
        .in('id', [9001, 9002])

      expect(verify).toEqual([])

      // Verify remaining record
      const { data: remaining } = await supabase
        .from('countries')
        .select()
        .eq('id', 9003)

      expect(remaining!.length).toBe(1)
    })

    it('should delete and return multiple records', async () => {
      const { data, error } = await supabase
        .from('countries')
        .delete()
        .in('id', [9001, 9002, 9003])
        .select()

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
    })
  })

  // Additional DELETE tests
  describe('Additional DELETE functionality', () => {
    it('should not delete any records if filter matches none', async () => {
      const { data, error } = await supabase
        .from('countries')
        .delete()
        .eq('id', -9999)
        .select()

      expect(error).toBeNull()
      expect(data).toEqual([])
    })

    it('should delete records using other filters', async () => {
      const { data, error } = await supabase
        .from('countries')
        .delete()
        .like('name', 'Delete_Test%')
        .select()

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
    })

    it('should handle delete on non-existent table with error', async () => {
      const { error } = await supabase
        .from('nonexistent_table')
        .delete()
        .eq('id', 1)

      expect(error).not.toBeNull()
    })

    it('should handle delete with combined filters', async () => {
      const { data, error } = await supabase
        .from('countries')
        .delete()
        .gte('id', 9001)
        .lte('id', 9002)
        .select()

      expect(error).toBeNull()
      expect(data!.length).toBe(2)
    })
  })
})

/**
 * Compatibility Summary for DELETE:
 *
 * IMPLEMENTED:
 * - Single delete: .delete().eq('col', val)
 * - Delete with return: .delete().eq(...).select()
 * - Multiple delete: .delete().in('col', [...])
 * - Delete with various filters
 * - Delete with selected return columns
 */
