/**
 * SELECT Operation Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/select
 *
 * Each test corresponds to an example from the documentation.
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('REST API - SELECT Operations', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * Example 1: Getting your data
   * Docs: https://supabase.com/docs/reference/javascript/select#getting-your-data
   *
   * const { data, error } = await supabase.from('characters').select()
   */
  describe('1. Getting your data', () => {
    it('should retrieve all rows and columns from a table', async () => {
      const { data, error } = await supabase.from('characters').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(Array.isArray(data)).toBe(true)
      expect(data!.length).toBeGreaterThan(0)
      // Should have all columns
      expect(data![0]).toHaveProperty('id')
      expect(data![0]).toHaveProperty('name')
    })
  })

  /**
   * Example 2: Selecting specific columns
   * Docs: https://supabase.com/docs/reference/javascript/select#selecting-specific-columns
   *
   * const { data, error } = await supabase.from('characters').select('name')
   */
  describe('2. Selecting specific columns', () => {
    it('should return only the specified columns', async () => {
      const { data, error } = await supabase.from('characters').select('name')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      // Should only have name column
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
      expect(data![0]).not.toHaveProperty('homeworld')
    })

    it('should return multiple specific columns', async () => {
      const { data, error } = await supabase.from('characters').select('id, name')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('id')
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('homeworld')
    })
  })

  /**
   * Example 3: Query referenced tables
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables
   *
   * const { data, error } = await supabase
   *   .from('orchestral_sections')
   *   .select(`name, instruments (name)`)
   *
   * Foreign key relationships are auto-detected from SQLite foreign key pragma.
   */
  describe('3. Query referenced tables', () => {
    it('should fetch related data from referenced tables', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select(`
          name,
          instruments (
            name
          )
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).toHaveProperty('instruments')
      expect(Array.isArray(data![0].instruments)).toBe(true)
    })
  })

  /**
   * Example 4: Query referenced tables with spaces in their names
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables-with-spaces
   *
   * Note: Not implemented in Phase 1 - requires embedded resources
   */
  describe('4. Query referenced tables with spaces', () => {
    it.skip('should handle table names with spaces using quotes', async () => {
      // Not implemented in Phase 1
    })
  })

  /**
   * Example 5: Query referenced tables through a join table
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables-through-a-join-table
   *
   * Note: Not implemented in Phase 1 - requires embedded resources
   */
  describe('5. Query referenced tables through join table', () => {
    it.skip('should query through many-to-many join tables', async () => {
      // Not implemented in Phase 1
    })
  })

  /**
   * Example 6: Query the same referenced table multiple times
   * Docs: https://supabase.com/docs/reference/javascript/select#query-the-same-referenced-table-multiple-times
   *
   * Note: Not implemented in Phase 1 - requires embedded resources with aliases
   */
  describe('6. Query same referenced table multiple times', () => {
    it.skip('should allow aliasing for multiple references to same table', async () => {
      // Not implemented in Phase 1
    })
  })

  /**
   * Example 7: Query nested foreign tables through a join table
   * Docs: https://supabase.com/docs/reference/javascript/select#query-nested-foreign-tables-through-a-join-table
   *
   * Note: Not implemented in Phase 1 - requires embedded resources
   */
  describe('7. Query nested foreign tables through join', () => {
    it.skip('should handle deeply nested relationships', async () => {
      // Not implemented in Phase 1
    })
  })

  /**
   * Example 8: Filtering through referenced tables
   * Docs: https://supabase.com/docs/reference/javascript/select#filtering-through-referenced-tables
   *
   * Note: Not implemented in Phase 1 - requires embedded resources
   */
  describe('8. Filtering through referenced tables', () => {
    it.skip('should filter on fields from referenced tables', async () => {
      // Not implemented in Phase 1
    })
  })

  /**
   * Example 9: Querying referenced table with count
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-referenced-table-with-count
   *
   * Note: Not implemented in Phase 1 - requires aggregate functions on relations
   */
  describe('9. Querying referenced table with count', () => {
    it.skip('should return count of related records', async () => {
      // Not implemented in Phase 1
    })
  })

  /**
   * Example 10: Querying with count option
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-with-count-option
   *
   * const { count, error } = await supabase
   *   .from('characters')
   *   .select('*', { count: 'exact', head: true })
   */
  describe('10. Querying with count option', () => {
    it('should return only count without data when head: true', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact', head: true })

      expect(error).toBeNull()
      expect(data).toBeNull()
      expect(count).toBe(5) // 5 characters in test data
    })

    it('should return both count and data', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(count).toBe(data!.length)
    })
  })

  /**
   * Example 11: Querying JSON data
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-json-data
   *
   * const { data, error } = await supabase
   *   .from('users')
   *   .select('id, name, address->city')
   */
  describe('11. Querying JSON data', () => {
    it.skip('should extract fields from JSON columns using arrow notation', async () => {
      // Requires JSON path extraction support
      const { data, error } = await supabase
        .from('users')
        .select(`
          id, name,
          address->city
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('city')
    })
  })

  /**
   * Example 12: Querying referenced table with inner join
   * Docs: https://supabase.com/docs/reference/javascript/select#querying-referenced-table-with-inner-join
   *
   * Inner join filters out parent rows without matching children.
   */
  describe('12. Querying with inner join', () => {
    it('should perform inner join on referenced tables', async () => {
      // Using !inner should only return countries that have cities
      const { data, error } = await supabase
        .from('countries')
        .select('name, cities!inner(name)')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // All returned countries should have at least one city
      data!.forEach((country) => {
        expect(Array.isArray(country.cities)).toBe(true)
        expect(country.cities.length).toBeGreaterThan(0)
      })
    })
  })

  /**
   * Example 13: Switching schemas per query
   * Docs: https://supabase.com/docs/reference/javascript/select#switching-schemas-per-query
   *
   * Note: Not applicable - sblite uses SQLite which doesn't have schemas
   */
  describe('13. Switching schemas per query', () => {
    it.skip('should switch to specified schema', async () => {
      // Not applicable for SQLite
    })
  })

  // Additional basic SELECT tests
  describe('Additional SELECT functionality', () => {
    it('should handle empty table gracefully', async () => {
      // Create a temp empty result with impossible filter
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('id', -9999)

      expect(error).toBeNull()
      expect(data).toEqual([])
    })

    it('should handle non-existent table with error', async () => {
      const { data, error } = await supabase
        .from('nonexistent_table')
        .select()

      // Should return an error for non-existent table
      expect(error).not.toBeNull()
    })

    it('should handle selecting all columns with *', async () => {
      const { data, error } = await supabase.from('characters').select('*')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('id')
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).toHaveProperty('homeworld')
    })
  })
})

/**
 * Compatibility Summary for SELECT:
 *
 * IMPLEMENTED:
 * - Basic select all: .select()
 * - Select specific columns: .select('col1, col2')
 * - Select with wildcard: .select('*')
 * - Referenced tables (embedded resources)
 * - Count options (exact, planned, estimated)
 * - Inner join syntax (!inner)
 *
 * NOT IMPLEMENTED:
 * - Many-to-many join table queries
 * - JSON path extraction
 * - Schema switching (N/A for SQLite)
 */
