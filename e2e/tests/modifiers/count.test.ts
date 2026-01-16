/**
 * Count Query Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/select#querying-with-count-option
 *
 * Tests for count option in select queries (exact, planned, estimated).
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Count Queries', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * count: 'exact' - Returns exact row count
   */
  describe('count: exact', () => {
    it('should return count with exact option', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact' })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(count).toBeDefined()
      expect(count).toBe(5) // 5 characters in test data
      expect(data!.length).toBe(5)
    })

    it('should return count matching data length', async () => {
      const { data, count, error } = await supabase
        .from('countries')
        .select('*', { count: 'exact' })

      expect(error).toBeNull()
      expect(count).toBe(data!.length)
    })

    it('should return count with filters applied', async () => {
      const { data, count, error } = await supabase
        .from('cities')
        .select('*', { count: 'exact' })
        .eq('country_id', 1) // US cities

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)
      expect(count).toBe(data!.length)
    })
  })

  /**
   * head: true - Returns only count without data
   * Uses HTTP HEAD method to get count without returning data.
   */
  describe('head: true (count only)', () => {
    it('should return only count when head is true', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact', head: true })

      expect(error).toBeNull()
      expect(data).toBeNull()
      expect(count).toBe(5)
    })

    it('should return count with filters when head is true', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact', head: true })
        .eq('homeworld', 'Tatooine')

      expect(error).toBeNull()
      expect(data).toBeNull()
      expect(count).toBe(1) // Only Luke is from Tatooine
    })

    it('should return zero count for no matches', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact', head: true })
        .eq('name', 'NonExistent')

      expect(error).toBeNull()
      expect(data).toBeNull()
      expect(count).toBe(0)
    })
  })

  /**
   * count with limit - Count should reflect total, not limited
   */
  describe('count with limit', () => {
    it('should return total count even with limit applied', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact' })
        .limit(2)

      expect(error).toBeNull()
      expect(data!.length).toBe(2) // Limited to 2
      expect(count).toBe(5) // But count should be total
    })

    it('should return total count with range applied', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact' })
        .range(0, 1) // First 2 rows

      expect(error).toBeNull()
      expect(data!.length).toBe(2)
      expect(count).toBe(5) // Total count
    })
  })

  /**
   * count with ordering - Count should work with order
   */
  describe('count with ordering', () => {
    it('should return count with order applied', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'exact' })
        .order('name', { ascending: true })

      expect(error).toBeNull()
      expect(count).toBe(5)
      // Data should still be ordered
      expect(data![0].name.localeCompare(data![1].name)).toBeLessThanOrEqual(0)
    })
  })

  /**
   * count: 'planned' - PostgreSQL's planned row count estimate
   * Note: For SQLite, this may behave like 'exact'
   */
  describe('count: planned', () => {
    it('should return planned count', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'planned' })

      expect(error).toBeNull()
      expect(count).toBeDefined()
      // Planned count should be a number
      expect(typeof count).toBe('number')
    })
  })

  /**
   * count: 'estimated' - PostgreSQL's estimated row count
   * Note: For SQLite, this may behave like 'exact'
   */
  describe('count: estimated', () => {
    it('should return estimated count', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('*', { count: 'estimated' })

      expect(error).toBeNull()
      expect(count).toBeDefined()
      expect(typeof count).toBe('number')
    })
  })

  /**
   * count with specific columns
   */
  describe('count with column selection', () => {
    it('should return count when selecting specific columns', async () => {
      const { data, count, error } = await supabase
        .from('characters')
        .select('name', { count: 'exact' })

      expect(error).toBeNull()
      expect(count).toBe(5)
      expect(data!.length).toBe(5)
      // Should only have name column
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
    })
  })
})

/**
 * Compatibility Summary for Count Queries:
 *
 * IMPLEMENTED:
 * - count: 'exact' - Returns exact row count
 * - count: 'planned' - Returns planned count (same as exact for SQLite)
 * - count: 'estimated' - Returns estimated count (same as exact for SQLite)
 * - Count with filters
 * - Count with limit/range (count reflects total, not limited result)
 * - Count with ordering
 * - Count with column selection
 * - head: true - Returns only count without data (HTTP HEAD method)
 *
 * NOTES:
 * - SQLite doesn't have EXPLAIN ANALYZE like PostgreSQL, so planned/estimated
 *   counts may return the same as exact count
 */
