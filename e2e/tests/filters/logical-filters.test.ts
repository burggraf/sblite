/**
 * Logical Filter Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/match
 * https://supabase.com/docs/reference/javascript/not
 * https://supabase.com/docs/reference/javascript/or
 * https://supabase.com/docs/reference/javascript/filter
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Filters - Logical Operators', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * match() - Match Multiple Column Values
   * Docs: https://supabase.com/docs/reference/javascript/match
   *
   * Match only rows where each column matches the corresponding value
   */
  describe('match() - Match Multiple Columns', () => {
    it('should match rows where all columns match their values', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .match({ id: 2, name: 'Leia' })

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('Leia')
    })

    it('should work with multiple eq() calls (equivalent)', async () => {
      // Alternative approach using chained eq()
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .eq('id', 2)
        .eq('name', 'Leia')

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(data![0].name).toBe('Leia')
    })
  })

  /**
   * not() - Negate Filter
   * Docs: https://supabase.com/docs/reference/javascript/not
   *
   * Match only rows which don't satisfy the filter
   */
  describe('not() - Negate Filter', () => {
    it('should negate is null filter', async () => {
      const { data, error } = await supabase
        .from('countries')
        .select()
        .not('name', 'is', null)

      expect(error).toBeNull()
      expect(data!.every((c) => c.name !== null)).toBe(true)
    })

    it('should negate eq filter', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .not('homeworld', 'eq', 'Tatooine')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((c) => c.name !== 'Luke')).toBe(true) // Luke is from Tatooine
    })

    it('should negate in filter', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .not('name', 'in', '(Luke,Leia)')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(data!.every((c) => c.name !== 'Luke' && c.name !== 'Leia')).toBe(true)
    })
  })

  /**
   * or() - OR Logic
   * Docs: https://supabase.com/docs/reference/javascript/or
   *
   * Match only rows which satisfy at least one of the filters
   */
  describe('or() - OR Logic', () => {
    /**
     * Example 1: Basic OR
     */
    it('should match rows satisfying any condition', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name')
        .or('id.eq.2,name.eq.Han')

      expect(error).toBeNull()
      expect(data!.length).toBe(2) // Leia (id=2) and Han
      const names = data!.map((c) => c.name).sort()
      expect(names).toEqual(['Han', 'Leia'])
    })

    /**
     * Example 2: OR with multiple conditions
     */
    it('should match multiple OR conditions on different columns', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select('name, homeworld')
        .or('homeworld.eq.Tatooine,homeworld.eq.Alderaan')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      data!.forEach((row) => {
        expect(['Tatooine', 'Alderaan']).toContain(row.homeworld)
      })
    })

    /**
     * Example 3: OR on referenced tables
     */
    it.skip('should apply or filter to referenced tables', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select(`
          name,
          instruments!inner (
            name
          )
        `)
        .or('section_id.eq.1,name.eq.guzheng', { referencedTable: 'instruments' })

      expect(error).toBeNull()
    })
  })

  /**
   * filter() - Generic Filter
   * Docs: https://supabase.com/docs/reference/javascript/filter
   *
   * Match only rows which satisfy the filter using raw PostgREST syntax
   */
  describe('filter() - Generic Filter', () => {
    /**
     * Example 1: Basic filter
     */
    it.skip('should apply raw PostgREST filter', async () => {
      // Requires filter() support
      const { data, error } = await supabase
        .from('characters')
        .select()
        .filter('name', 'in', '("Han","Yoda")')

      expect(error).toBeNull()
      expect(data!.length).toBe(2)
    })

    /**
     * Example 2: Filter on referenced table
     */
    it.skip('should apply filter on referenced tables', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select(`
          name,
          instruments!inner (
            name
          )
        `)
        .filter('instruments.name', 'eq', 'flute')

      expect(error).toBeNull()
    })
  })

  // Workaround tests using available filters
  describe('Workarounds for OR logic', () => {
    it('should use multiple queries for OR logic', async () => {
      // When or() is not available, run separate queries
      const [query1, query2] = await Promise.all([
        supabase.from('characters').select().eq('id', 2),
        supabase.from('characters').select().eq('name', 'Han'),
      ])

      expect(query1.error).toBeNull()
      expect(query2.error).toBeNull()

      // Combine results (deduplicate if needed)
      const combined = [...(query1.data || []), ...(query2.data || [])]
      const unique = Array.from(new Map(combined.map((item) => [item.id, item])).values())

      expect(unique.length).toBe(2)
    })

    it('should use in() for simple OR on same column', async () => {
      // in() is essentially OR on the same column
      const { data, error } = await supabase
        .from('characters')
        .select()
        .in('name', ['Leia', 'Han', 'Yoda'])

      expect(error).toBeNull()
      expect(data!.length).toBe(3)
    })
  })
})

/**
 * Compatibility Summary for Logical Filters:
 *
 * IMPLEMENTED:
 * - match(): Multi-column matching
 * - not(): Negation operator (eq, neq, gt, gte, lt, lte, is, in, like, ilike)
 * - or(): OR logic with PostgREST syntax
 *
 * NOT IMPLEMENTED:
 * - filter(): Generic PostgREST filter (use direct operator methods)
 * - or() on referenced tables (referencedTable option)
 *
 * WORKAROUNDS:
 * - or() on same column -> Use in() (still valid alternative)
 */
