/**
 * Basic Filter Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/using-filters
 * https://supabase.com/docs/reference/javascript/eq
 * https://supabase.com/docs/reference/javascript/neq
 * https://supabase.com/docs/reference/javascript/gt
 * https://supabase.com/docs/reference/javascript/gte
 * https://supabase.com/docs/reference/javascript/lt
 * https://supabase.com/docs/reference/javascript/lte
 * https://supabase.com/docs/reference/javascript/like
 * https://supabase.com/docs/reference/javascript/ilike
 * https://supabase.com/docs/reference/javascript/is
 * https://supabase.com/docs/reference/javascript/in
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Filters - Basic Operators', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * Using Filters - General Examples
   * Docs: https://supabase.com/docs/reference/javascript/using-filters
   */
  describe('Using Filters - General', () => {
    /**
     * Example: Applying Filters
     * Filters must be called after .select()
     */
    it('should apply filters after select', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .select('name, section_id')
        .eq('name', 'violin')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('violin')
    })

    /**
     * Example: Chaining Multiple Filters
     */
    it('should chain multiple filters (AND logic)', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select('name, country_id')
        .gte('population', 1000)
        .lt('population', 1000000)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should return cities with population between 1000 and 1000000
      expect(data!.every((city) => city.name !== undefined)).toBe(true)
    })

    /**
     * Example: JSON Column Filtering
     * Uses arrow operator (-> or ->>) to filter on nested JSON fields
     */
    it('should filter on JSON columns using arrow notation', async () => {
      const { data, error } = await supabase
        .from('users')
        .select()
        .eq('address->postcode', 90210)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('Jane Smith')
    })
  })

  /**
   * eq() - Equals
   * Docs: https://supabase.com/docs/reference/javascript/eq
   *
   * Match only rows where column is equal to value
   */
  describe('eq() - Equals', () => {
    it('should match rows where column equals value', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('name', 'Leia')

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('Leia')
    })

    it('should return empty array when no match', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('name', 'NonexistentCharacter')

      expect(error).toBeNull()
      expect(data).toEqual([])
    })

    it('should work with numeric values', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('id', 2)

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(data![0].id).toBe(2)
    })
  })

  /**
   * neq() - Not Equals
   * Docs: https://supabase.com/docs/reference/javascript/neq
   *
   * Match only rows where column is not equal to value
   */
  describe('neq() - Not Equals', () => {
    it('should match rows where column does not equal value', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .neq('name', 'Leia')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((char) => char.name !== 'Leia')).toBe(true)
    })
  })

  /**
   * gt() - Greater Than
   * Docs: https://supabase.com/docs/reference/javascript/gt
   *
   * Match only rows where column is greater than value
   */
  describe('gt() - Greater Than', () => {
    it('should match rows where column is greater than value', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .gt('id', 2)

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((char) => char.id > 2)).toBe(true)
    })
  })

  /**
   * gte() - Greater Than or Equal
   * Docs: https://supabase.com/docs/reference/javascript/gte
   *
   * Match only rows where column is greater than or equal to value
   */
  describe('gte() - Greater Than or Equal', () => {
    it('should match rows where column is greater than or equal to value', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .gte('id', 2)

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((char) => char.id >= 2)).toBe(true)
      // Should include id=2
      expect(data!.some((char) => char.id === 2)).toBe(true)
    })
  })

  /**
   * lt() - Less Than
   * Docs: https://supabase.com/docs/reference/javascript/lt
   *
   * Match only rows where column is less than value
   */
  describe('lt() - Less Than', () => {
    it('should match rows where column is less than value', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .lt('id', 3)

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((char) => char.id < 3)).toBe(true)
    })
  })

  /**
   * lte() - Less Than or Equal
   * Docs: https://supabase.com/docs/reference/javascript/lte
   *
   * Match only rows where column is less than or equal to value
   */
  describe('lte() - Less Than or Equal', () => {
    it('should match rows where column is less than or equal to value', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .lte('id', 2)

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((char) => char.id <= 2)).toBe(true)
      // Should include id=2
      expect(data!.some((char) => char.id === 2)).toBe(true)
    })
  })

  /**
   * like() - Pattern Matching (Case-Sensitive)
   * Docs: https://supabase.com/docs/reference/javascript/like
   *
   * Match only rows where column matches pattern (case-sensitive)
   */
  describe('like() - Pattern Matching', () => {
    it('should match rows using LIKE pattern with wildcards', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .like('name', '%Lu%')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.some((char) => char.name === 'Luke')).toBe(true)
    })

    it('should match with prefix wildcard', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .like('name', '%a')

      expect(error).toBeNull()
      // Should match names ending with 'a' (Leia, Yoda)
      expect(data!.every((char) => char.name.endsWith('a'))).toBe(true)
    })

    it('should match with suffix wildcard', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .like('name', 'L%')

      expect(error).toBeNull()
      // Should match names starting with 'L' (Luke, Leia)
      expect(data!.every((char) => char.name.startsWith('L'))).toBe(true)
    })
  })

  /**
   * ilike() - Pattern Matching (Case-Insensitive)
   * Docs: https://supabase.com/docs/reference/javascript/ilike
   *
   * Match only rows where column matches pattern (case-insensitive)
   */
  describe('ilike() - Case-Insensitive Pattern Matching', () => {
    it('should match rows case-insensitively', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .ilike('name', '%lu%')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      // Should match 'Luke' even though we searched for lowercase 'lu'
      expect(data!.some((char) => char.name === 'Luke')).toBe(true)
    })

    it('should match regardless of case', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .ilike('name', 'LUKE')

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('Luke')
    })
  })

  /**
   * is() - Check for Exact Values (null, true, false)
   * Docs: https://supabase.com/docs/reference/javascript/is
   *
   * Match only rows where column IS value (for null, true, false)
   */
  describe('is() - Null/Boolean Check', () => {
    it('should match rows where column is null', async () => {
      const { data, error } = await supabase
        .from('countries')
        .select()
        .is('code', null)

      expect(error).toBeNull()
      // All matching rows should have null code
      if (data!.length > 0) {
        expect(data!.every((country) => country.code === null)).toBe(true)
      }
    })

    it.skip('should match rows where boolean column is true', async () => {
      // Requires a table with boolean column
    })

    it.skip('should match rows where boolean column is false', async () => {
      // Requires a table with boolean column
    })
  })

  /**
   * in() - Match Any Value in Array
   * Docs: https://supabase.com/docs/reference/javascript/in
   *
   * Match only rows where column is included in values array
   */
  describe('in() - Match Any in Array', () => {
    it('should match rows where column is in array of values', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .in('name', ['Leia', 'Han'])

      expect(error).toBeNull()
      expect(data!.length).toBe(2)
      expect(data!.map((c) => c.name).sort()).toEqual(['Han', 'Leia'])
    })

    it('should work with numeric arrays', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .in('id', [1, 3, 5])

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
      expect(data!.map((c) => c.id).sort()).toEqual([1, 3, 5])
    })

    it('should return empty array when no matches', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .in('name', ['NonExistent1', 'NonExistent2'])

      expect(error).toBeNull()
      expect(data).toEqual([])
    })
  })

  // Combined filter tests
  describe('Combined Filters', () => {
    it('should combine eq and gt filters', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select()
        .eq('country_id', 1)
        .gt('population', 1000000)

      expect(error).toBeNull()
      expect(data!.every((city) => city.country_id === 1)).toBe(true)
      expect(data!.every((city) => city.population > 1000000)).toBe(true)
    })

    it('should combine gte and lte for range query', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .gte('id', 2)
        .lte('id', 4)

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
      expect(data!.every((char) => char.id >= 2 && char.id <= 4)).toBe(true)
    })
  })
})

/**
 * Compatibility Summary for Basic Filters:
 *
 * IMPLEMENTED:
 * - eq(): Equals
 * - neq(): Not equals
 * - gt(): Greater than
 * - gte(): Greater than or equal
 * - lt(): Less than
 * - lte(): Less than or equal
 * - like(): Pattern matching (case-sensitive)
 * - ilike(): Pattern matching (case-insensitive)
 * - is(): Null check
 * - in(): Match any in array
 *
 * NOT IMPLEMENTED:
 * - JSON arrow operator filtering (address->postcode)
 */
