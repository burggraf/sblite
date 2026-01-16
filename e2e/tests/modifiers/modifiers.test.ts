/**
 * Modifier Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/using-modifiers
 * https://supabase.com/docs/reference/javascript/db-modifiers-select
 * https://supabase.com/docs/reference/javascript/order
 * https://supabase.com/docs/reference/javascript/limit
 * https://supabase.com/docs/reference/javascript/range
 * https://supabase.com/docs/reference/javascript/single
 * https://supabase.com/docs/reference/javascript/maybesingle
 * https://supabase.com/docs/reference/javascript/db-csv
 * https://supabase.com/docs/reference/javascript/explain
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Modifiers', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * select() - Return Data After Modifying
   * Docs: https://supabase.com/docs/reference/javascript/db-modifiers-select
   *
   * Used with insert/update/upsert/delete to return modified rows
   */
  describe('select() - Return Data After Insert/Update/Delete', () => {
    it('should return data after upsert', async () => {
      const { data, error } = await supabase
        .from('characters')
        .upsert({ id: 9001, name: 'Test Han Solo' })
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('Test Han Solo')

      // Cleanup
      await supabase.from('characters').delete().eq('id', 9001)
    })

    it('should return specific columns after insert', async () => {
      const { data, error } = await supabase
        .from('characters')
        .insert({ id: 9002, name: 'Test Chewie', homeworld: 'Kashyyyk' })
        .select('name, homeworld')

      expect(error).toBeNull()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).toHaveProperty('homeworld')
      expect(data![0]).not.toHaveProperty('id')

      // Cleanup
      await supabase.from('characters').delete().eq('id', 9002)
    })
  })

  /**
   * order() - Sort Results
   * Docs: https://supabase.com/docs/reference/javascript/order
   */
  describe('order() - Sort Results', () => {
    /**
     * Example 1: Order with select
     */
    it('should order results in descending order', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('id, name')
        .order('id', { ascending: false })

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      // Verify descending order
      for (let i = 1; i < data!.length; i++) {
        expect(data![i - 1].id).toBeGreaterThanOrEqual(data![i].id)
      }
    })

    it('should order results in ascending order', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('id, name')
        .order('id', { ascending: true })

      expect(error).toBeNull()
      // Verify ascending order
      for (let i = 1; i < data!.length; i++) {
        expect(data![i - 1].id).toBeLessThanOrEqual(data![i].id)
      }
    })

    it('should order by string column', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .order('name', { ascending: true })

      expect(error).toBeNull()
      // Verify alphabetical order
      for (let i = 1; i < data!.length; i++) {
        expect(data![i - 1].name.localeCompare(data![i].name)).toBeLessThanOrEqual(0)
      }
    })

    /**
     * Example 2: Order on referenced table
     * Not implemented in Phase 1
     */
    it.skip('should order on referenced table', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select(`
          name,
          instruments (
            name
          )
        `)
        .order('name', { referencedTable: 'instruments', ascending: false })

      expect(error).toBeNull()
    })

    /**
     * Example 3: Order parent by referenced table
     * Not implemented in Phase 1
     */
    it.skip('should order parent table by referenced table column', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .select(`
          name,
          section:orchestral_sections (
            name
          )
        `)
        .order('section(name)', { ascending: true })

      expect(error).toBeNull()
    })
  })

  /**
   * limit() - Limit Number of Results
   * Docs: https://supabase.com/docs/reference/javascript/limit
   */
  describe('limit() - Limit Results', () => {
    /**
     * Example 1: Basic limit
     */
    it('should limit the number of returned rows', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .limit(1)

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
    })

    it('should return all rows when limit exceeds total', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .limit(1000)

      expect(error).toBeNull()
      expect(data!.length).toBeLessThanOrEqual(1000)
    })

    /**
     * Example 2: Limit on referenced table
     * Not implemented in Phase 1
     */
    it.skip('should limit results from referenced table', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select(`
          name,
          instruments (
            name
          )
        `)
        .limit(1, { referencedTable: 'instruments' })

      expect(error).toBeNull()
    })
  })

  /**
   * range() - Pagination
   * Docs: https://supabase.com/docs/reference/javascript/range
   */
  describe('range() - Pagination', () => {
    it('should return rows in the specified range', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .order('id', { ascending: true })
        .range(0, 1)

      expect(error).toBeNull()
      expect(data!.length).toBe(2) // 0-indexed, inclusive: rows 0 and 1
    })

    it('should work for pagination', async () => {
      // Get first page
      const page1 = await supabase
        .from('characters')
        .select('id')
        .order('id', { ascending: true })
        .range(0, 1)

      // Get second page
      const page2 = await supabase
        .from('characters')
        .select('id')
        .order('id', { ascending: true })
        .range(2, 3)

      expect(page1.error).toBeNull()
      expect(page2.error).toBeNull()
      expect(page1.data!.length).toBe(2)
      expect(page2.data!.length).toBe(2)

      // Pages should be different
      const page1Ids = page1.data!.map((c) => c.id)
      const page2Ids = page2.data!.map((c) => c.id)
      expect(page1Ids).not.toEqual(page2Ids)
    })
  })

  /**
   * single() - Return Single Object
   * Docs: https://supabase.com/docs/reference/javascript/single
   */
  describe('single() - Return Single Object', () => {
    it('should return data as single object instead of array', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .eq('id', 1)
        .single()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should be an object, not an array
      expect(Array.isArray(data)).toBe(false)
      expect(data).toHaveProperty('name')
    })

    it('should error when multiple rows returned', async () => {
      const { data, error } = await supabase.from('characters').select().single()

      // Should error because multiple rows exist
      expect(error).not.toBeNull()
    })

    it('should error when no rows returned', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('id', -9999)
        .single()

      // Should error because no rows match
      expect(error).not.toBeNull()
    })
  })

  /**
   * maybeSingle() - Return Zero or One Row
   * Docs: https://supabase.com/docs/reference/javascript/maybesingle
   */
  describe('maybeSingle() - Return Zero or One Row', () => {
    it('should return single object when one row matches', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('name', 'Leia')
        .maybeSingle()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(false)
    })

    it('should return null when no rows match', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('name', 'NonexistentCharacter')
        .maybeSingle()

      expect(error).toBeNull()
      expect(data).toBeNull()
    })

    it('should error when multiple rows returned', async () => {
      const { data, error } = await supabase.from('characters').select().maybeSingle()

      // Should error because multiple rows exist
      expect(error).not.toBeNull()
    })
  })

  /**
   * csv() - Return as CSV
   * Docs: https://supabase.com/docs/reference/javascript/db-csv
   */
  describe('csv() - Return as CSV', () => {
    it.skip('should return data as CSV string', async () => {
      // Requires Accept: text/csv header support
      const { data, error } = await supabase.from('characters').select().csv()

      expect(error).toBeNull()
      expect(typeof data).toBe('string')
      expect(data).toContain(',') // CSV has commas
    })
  })

  /**
   * explain() - Query Execution Plan
   * Docs: https://supabase.com/docs/reference/javascript/explain
   */
  describe('explain() - Query Execution Plan', () => {
    /**
     * Example 1: Basic explain
     */
    it.skip('should return query execution plan', async () => {
      // Requires EXPLAIN support
      const { data, error } = await supabase.from('characters').select().explain()

      expect(error).toBeNull()
      expect(data).toBeDefined()
    })

    /**
     * Example 2: Explain with analyze and verbose
     */
    it.skip('should return detailed execution plan', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .explain({ analyze: true, verbose: true })

      expect(error).toBeNull()
    })
  })

  // Combined modifier tests
  describe('Combined Modifiers', () => {
    it('should combine order and limit', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('id, name')
        .order('id', { ascending: false })
        .limit(3)

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
      // Should be in descending order
      expect(data![0].id).toBeGreaterThan(data![1].id)
    })

    it('should combine order, limit, and range for pagination', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('id')
        .order('id', { ascending: true })
        .range(1, 3) // Skip first, get next 3

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
    })
  })
})

/**
 * Compatibility Summary for Modifiers:
 *
 * IMPLEMENTED:
 * - select() with insert/update/upsert/delete
 * - order(): Basic ordering (ascending/descending)
 * - limit(): Limit number of rows
 * - range(): Pagination (offset + limit)
 * - single(): Return single object (errors on 0 or >1 results)
 * - maybeSingle(): Return object or null (errors on >1 results)
 *
 * NOT IMPLEMENTED:
 * - order() on referenced tables
 * - limit() on referenced tables
 * - csv(): Return as CSV format
 * - explain(): Query execution plan
 */
