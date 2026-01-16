/**
 * UPSERT Operation Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/upsert
 *
 * Each test corresponds to an example from the documentation.
 */

import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - UPSERT Operations', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  // Clean up after each test
  afterEach(async () => {
    await supabase.from('instruments').delete().gte('id', 9000)
    await supabase.from('users').delete().gte('id', 9000)
  })

  /**
   * Example 1: Upsert your data
   * Docs: https://supabase.com/docs/reference/javascript/upsert#upsert-your-data
   *
   * const { data, error } = await supabase
   *   .from('instruments')
   *   .upsert({ id: 1, name: 'piano' })
   *   .select()
   */
  describe('1. Upsert your data', () => {
    it('should insert a new record if it does not exist', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .upsert({ id: 9001, name: 'upsert_new_piano' })
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('upsert_new_piano')
    })

    it('should update an existing record if it exists', async () => {
      // First insert
      await supabase.from('instruments').insert({ id: 9002, name: 'original_name' })

      // Upsert should update
      const { data, error } = await supabase
        .from('instruments')
        .upsert({ id: 9002, name: 'upserted_name' })
        .select()

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('upserted_name')
    })
  })

  /**
   * Example 2: Bulk Upsert your data
   * Docs: https://supabase.com/docs/reference/javascript/upsert#bulk-upsert-your-data
   *
   * const { data, error } = await supabase
   *   .from('instruments')
   *   .upsert([
   *     { id: 1, name: 'piano' },
   *     { id: 2, name: 'harp' },
   *   ])
   *   .select()
   */
  describe('2. Bulk Upsert your data', () => {
    it('should upsert multiple records', async () => {
      // Insert one record first
      await supabase.from('instruments').insert({ id: 9003, name: 'existing' })

      // Bulk upsert with mix of new and existing
      const { data, error } = await supabase
        .from('instruments')
        .upsert([
          { id: 9003, name: 'bulk_updated' }, // This should update
          { id: 9004, name: 'bulk_new_harp' }, // This should insert
        ])
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(2)

      const updated = data!.find((r) => r.id === 9003)
      const inserted = data!.find((r) => r.id === 9004)

      expect(updated!.name).toBe('bulk_updated')
      expect(inserted!.name).toBe('bulk_new_harp')
    })
  })

  /**
   * Example 3: Upserting into tables with constraints
   * Docs: https://supabase.com/docs/reference/javascript/upsert#upserting-into-tables-with-constraints
   *
   * const { data, error } = await supabase
   *   .from('users')
   *   .upsert(
   *     { id: 42, handle: 'saoirse', display_name: 'Saoirse' },
   *     { onConflict: 'handle' }
   *   )
   *   .select()
   */
  describe('3. Upserting into tables with constraints', () => {
    it.skip('should use specified column for conflict resolution', async () => {
      // Requires onConflict option support
      // This is not standard PostgREST - it's a Supabase client feature

      const { data, error } = await supabase
        .from('users')
        .upsert(
          { id: 9005, handle: 'testuser', display_name: 'Test User' },
          { onConflict: 'handle' }
        )
        .select()

      expect(error).toBeNull()
    })
  })

  // Additional UPSERT tests
  describe('Additional UPSERT functionality', () => {
    it('should handle upsert without select (no return)', async () => {
      const { error } = await supabase
        .from('instruments')
        .upsert({ id: 9006, name: 'no_return_upsert' })

      expect(error).toBeNull()

      // Verify it was inserted
      const { data: verify } = await supabase
        .from('instruments')
        .select()
        .eq('id', 9006)

      expect(verify!.length).toBe(1)
    })

    it('should return selected columns only', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .upsert({ id: 9007, name: 'partial_select', section_id: 1 })
        .select('name')

      expect(error).toBeNull()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
    })

    it('should handle upsert to non-existent table with error', async () => {
      const { error } = await supabase
        .from('nonexistent_table')
        .upsert({ id: 1, name: 'test' })

      expect(error).not.toBeNull()
    })
  })
})

/**
 * Compatibility Summary for UPSERT:
 *
 * IMPLEMENTED:
 * - Single upsert: .upsert({...})
 * - Upsert with return: .upsert({...}).select()
 * - Bulk upsert: .upsert([{...}, {...}])
 *
 * NOT IMPLEMENTED:
 * - onConflict option for custom conflict columns
 * - ignoreDuplicates option
 */
