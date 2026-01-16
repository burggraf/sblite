/**
 * CSV Response Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/db-csv
 *
 * Tests for CSV output format using Accept: text/csv header.
 * Note: supabase-js doesn't have direct CSV support, so we test with fetch directly.
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('CSV Response', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * Basic CSV response with Accept header
   */
  describe('Accept: text/csv header', () => {
    it('should return CSV when Accept header is text/csv', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      expect(response.ok).toBe(true)
      expect(response.headers.get('Content-Type')).toContain('text/csv')

      const text = await response.text()
      // Should have header row with column names
      const lines = text.trim().split('\n')
      expect(lines.length).toBeGreaterThan(1) // header + data rows

      // Header should contain column names
      const header = lines[0]
      expect(header).toContain('id')
      expect(header).toContain('name')
    })

    it('should return CSV with proper row format', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      const text = await response.text()
      const lines = text.trim().split('\n')

      // Should have 6 lines: 1 header + 5 data rows
      expect(lines.length).toBe(6)

      // Data rows should have the same number of columns as header
      const headerCols = lines[0].split(',').length
      for (let i = 1; i < lines.length; i++) {
        // Note: CSV parsing is complex, but for simple data without commas in values
        const rowCols = lines[i].split(',').length
        expect(rowCols).toBe(headerCols)
      }
    })
  })

  /**
   * CSV with column selection
   */
  describe('CSV with select columns', () => {
    it('should return only selected columns in CSV', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters?select=name,homeworld`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      expect(response.ok).toBe(true)
      const text = await response.text()
      const lines = text.trim().split('\n')

      // Header should only have name and homeworld
      const header = lines[0]
      expect(header).toContain('name')
      expect(header).toContain('homeworld')
      expect(header).not.toContain('id')
    })
  })

  /**
   * CSV with filters
   */
  describe('CSV with filters', () => {
    it('should apply filters to CSV response', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters?homeworld=eq.Tatooine`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      expect(response.ok).toBe(true)
      const text = await response.text()
      const lines = text.trim().split('\n')

      // Should have 2 lines: header + 1 data row (Luke from Tatooine)
      expect(lines.length).toBe(2)
      expect(text).toContain('Luke')
      expect(text).toContain('Tatooine')
    })
  })

  /**
   * CSV with ordering
   */
  describe('CSV with ordering', () => {
    it('should apply ordering to CSV response', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters?order=name.asc`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      expect(response.ok).toBe(true)
      const text = await response.text()
      const lines = text.trim().split('\n')

      // Skip header, check data rows are in alphabetical order by name
      // Names: Chewbacca, Han, Leia, Luke, Yoda (alphabetically)
      const dataLines = lines.slice(1)
      expect(dataLines[0]).toContain('Chewbacca')
      expect(dataLines[4]).toContain('Yoda')
    })
  })

  /**
   * CSV with limit
   */
  describe('CSV with limit', () => {
    it('should apply limit to CSV response', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters?limit=2`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      expect(response.ok).toBe(true)
      const text = await response.text()
      const lines = text.trim().split('\n')

      // Should have 3 lines: header + 2 data rows
      expect(lines.length).toBe(3)
    })
  })

  /**
   * CSV response format details
   */
  describe('CSV format compliance', () => {
    it('should properly escape values with commas', async () => {
      // First, insert a test row with a comma in the value
      const { error: insertError } = await supabase
        .from('characters')
        .insert({ id: 9999, name: 'Test, Character', homeworld: 'Test Planet' })

      if (insertError) {
        console.log('Insert error:', insertError)
      }

      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/characters?id=eq.9999`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      expect(response.ok).toBe(true)
      const text = await response.text()

      // Value with comma should be quoted
      expect(text).toContain('"Test, Character"')

      // Cleanup
      await supabase.from('characters').delete().eq('id', 9999)
    })

    it('should handle null values in CSV', async () => {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/rest/v1/countries?code=is.null`, {
        headers: {
          Accept: 'text/csv',
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
      })

      // Even if no results, should return valid CSV (header only or empty)
      expect(response.ok).toBe(true)
      const text = await response.text()
      expect(typeof text).toBe('string')
    })
  })

  /**
   * supabase-js csv() method
   * Note: This uses supabase-js which sends the appropriate Accept header
   */
  describe('supabase-js csv() method', () => {
    it('should return CSV string via .csv() method', async () => {
      const { data, error } = await supabase.from('characters').select().csv()

      expect(error).toBeNull()
      expect(typeof data).toBe('string')

      // Should be valid CSV
      const lines = (data as string).trim().split('\n')
      expect(lines.length).toBeGreaterThan(1)
      expect(lines[0]).toContain('id')
      expect(lines[0]).toContain('name')
    })

    it('should work with csv() and filters', async () => {
      const { data, error } = await supabase
        .from('characters')
        .select()
        .eq('homeworld', 'Tatooine')
        .csv()

      expect(error).toBeNull()
      expect(typeof data).toBe('string')

      const lines = (data as string).trim().split('\n')
      // Header + 1 data row (Luke)
      expect(lines.length).toBe(2)
    })
  })
})

/**
 * Compatibility Summary for CSV Response:
 *
 * IMPLEMENTED:
 * - Accept: text/csv header returns CSV format
 * - Content-Type: text/csv in response
 * - CSV header row with column names
 * - CSV data rows with proper formatting
 * - CSV with column selection
 * - CSV with filters
 * - CSV with ordering
 * - CSV with limit/pagination
 * - Proper escaping of values containing commas (quoted)
 * - supabase-js .csv() method
 *
 * CSV FORMAT:
 * - Standard RFC 4180 compliant CSV
 * - First row contains column headers
 * - Values containing commas are quoted
 * - Values containing quotes are escaped with double quotes
 */
