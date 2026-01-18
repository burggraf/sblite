/**
 * Advanced Filter Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/contains
 * https://supabase.com/docs/reference/javascript/containedby
 * https://supabase.com/docs/reference/javascript/rangegt
 * https://supabase.com/docs/reference/javascript/rangegte
 * https://supabase.com/docs/reference/javascript/rangelt
 * https://supabase.com/docs/reference/javascript/rangelte
 * https://supabase.com/docs/reference/javascript/rangeadjacent
 * https://supabase.com/docs/reference/javascript/overlaps
 * https://supabase.com/docs/reference/javascript/textsearch
 */

import { describe, it, expect, beforeAll, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Filters - Advanced Operators', () => {
  let supabase: SupabaseClient
  let serviceClient: SupabaseClient

  beforeAll(async () => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    serviceClient = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_SERVICE_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })

    // Set up tables and FTS indexes for text search tests
    await setupFTSTables()
  })

  afterAll(async () => {
    // Clean up FTS tables
    await cleanupFTSTables()
  })

  // Helper function to set up FTS tables for text search tests
  async function setupFTSTables() {
    // Create texts table if it doesn't exist
    try {
      // First try to drop existing FTS indexes
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/texts/fts/content_search`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}` },
      }).catch(() => {})

      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/texts`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}` },
      }).catch(() => {})

      // Create texts table
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'texts',
          columns: [
            { name: 'id', type: 'integer', primary: true },
            { name: 'content', type: 'text' },
          ],
        }),
      })

      // Insert test data for texts table
      await serviceClient.from('texts').insert([
        { id: 1, content: 'I like eggs and ham' },
        { id: 2, content: 'Green eggs and ham is a book by Dr. Seuss' },
        { id: 3, content: 'Sam I am, I do not like green eggs and ham' },
      ])

      // Create FTS index on texts.content
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/texts/fts`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'content_search',
          columns: ['content'],
        }),
      })
    } catch (e) {
      // Ignore setup errors (table might already exist)
    }

    // Create FTS index on quotes.catchphrase (quotes table already exists from seed data)
    try {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/quotes/fts/catchphrase_search`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}` },
      }).catch(() => {})

      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/quotes/fts`, {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          name: 'catchphrase_search',
          columns: ['catchphrase'],
        }),
      })
    } catch (e) {
      // Ignore setup errors
    }
  }

  // Helper function to clean up FTS tables
  async function cleanupFTSTables() {
    try {
      await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables/texts`, {
        method: 'DELETE',
        headers: { 'Authorization': `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}` },
      })
    } catch (e) {
      // Ignore cleanup errors
    }
  }

  /**
   * contains() - Array/Range/JSONB Contains
   * Docs: https://supabase.com/docs/reference/javascript/contains
   *
   * Match only rows where column contains every element in value
   */
  describe('contains() - Contains All Elements', () => {
    /**
     * Example 1: On array columns
     */
    it.skip('should match rows where array column contains all elements', async () => {
      // Requires PostgreSQL array support
      const { data, error } = await supabase
        .from('issues')
        .select()
        .contains('tags', ['is:open', 'priority:low'])

      expect(error).toBeNull()
    })

    /**
     * Example 2: On range columns
     */
    it.skip('should match rows where range column contains value', async () => {
      // Requires PostgreSQL range support
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .contains('during', '[2000-01-01 13:00, 2000-01-01 13:30)')

      expect(error).toBeNull()
    })

    /**
     * Example 3: On JSONB columns
     */
    it.skip('should match rows where JSONB column contains object', async () => {
      // Requires PostgreSQL JSONB support
      const { data, error } = await supabase
        .from('users')
        .select('name')
        .contains('address', { postcode: 90210 })

      expect(error).toBeNull()
    })
  })

  /**
   * containedBy() - Contained By Value
   * Docs: https://supabase.com/docs/reference/javascript/containedby
   *
   * Match only rows where column is contained by value
   */
  describe('containedBy() - Contained By', () => {
    /**
     * Example 1: On array columns
     */
    it.skip('should match rows where array is contained by given array', async () => {
      // Requires PostgreSQL array support
      const { data, error } = await supabase
        .from('classes')
        .select('name')
        .containedBy('days', ['monday', 'tuesday', 'wednesday', 'friday'])

      expect(error).toBeNull()
    })

    /**
     * Example 2: On range columns
     */
    it.skip('should match rows where range is contained by given range', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .containedBy('during', '[2000-01-01 00:00, 2000-01-01 23:59)')

      expect(error).toBeNull()
    })

    /**
     * Example 3: On JSONB columns
     */
    it.skip('should match rows where JSONB is contained by given object', async () => {
      const { data, error } = await supabase
        .from('users')
        .select('name')
        .containedBy('address', {})

      expect(error).toBeNull()
    })
  })

  /**
   * rangeGt() - Greater Than Range
   * Docs: https://supabase.com/docs/reference/javascript/rangegt
   */
  describe('rangeGt() - Greater Than Range', () => {
    it.skip('should match rows where range is greater than given range', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .rangeGt('during', '[2000-01-02 08:00, 2000-01-02 09:00)')

      expect(error).toBeNull()
    })
  })

  /**
   * rangeGte() - Greater Than or Equal Range
   * Docs: https://supabase.com/docs/reference/javascript/rangegte
   */
  describe('rangeGte() - Greater Than or Equal Range', () => {
    it.skip('should match rows where range is >= given range', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .rangeGte('during', '[2000-01-02 08:30, 2000-01-02 09:30)')

      expect(error).toBeNull()
    })
  })

  /**
   * rangeLt() - Less Than Range
   * Docs: https://supabase.com/docs/reference/javascript/rangelt
   */
  describe('rangeLt() - Less Than Range', () => {
    it.skip('should match rows where range is less than given range', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .rangeLt('during', '[2000-01-01 15:00, 2000-01-01 16:00)')

      expect(error).toBeNull()
    })
  })

  /**
   * rangeLte() - Less Than or Equal Range
   * Docs: https://supabase.com/docs/reference/javascript/rangelte
   */
  describe('rangeLte() - Less Than or Equal Range', () => {
    it.skip('should match rows where range is <= given range', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .rangeLte('during', '[2000-01-01 14:00, 2000-01-01 16:00)')

      expect(error).toBeNull()
    })
  })

  /**
   * rangeAdjacent() - Mutually Exclusive to Range
   * Docs: https://supabase.com/docs/reference/javascript/rangeadjacent
   */
  describe('rangeAdjacent() - Adjacent Range', () => {
    it.skip('should match rows where range is adjacent to given range', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .rangeAdjacent('during', '[2000-01-01 12:00, 2000-01-01 13:00)')

      expect(error).toBeNull()
    })
  })

  /**
   * overlaps() - Has Common Elements
   * Docs: https://supabase.com/docs/reference/javascript/overlaps
   */
  describe('overlaps() - Has Common Element', () => {
    /**
     * Example 1: On array columns
     */
    it.skip('should match rows where array has common elements', async () => {
      const { data, error } = await supabase
        .from('issues')
        .select('title')
        .overlaps('tags', ['is:closed', 'severity:high'])

      expect(error).toBeNull()
    })

    /**
     * Example 2: On range columns
     */
    it.skip('should match rows where range overlaps', async () => {
      const { data, error } = await supabase
        .from('reservations')
        .select()
        .overlaps('during', '[2000-01-01 12:45, 2000-01-01 13:15)')

      expect(error).toBeNull()
    })
  })

  /**
   * textSearch() - Full Text Search
   * Docs: https://supabase.com/docs/reference/javascript/textsearch
   *
   * Note: The config parameter (e.g., 'english') is accepted for compatibility
   * but ignored by SQLite FTS5. Language configuration is handled via tokenizers
   * at index creation time instead.
   */
  describe('textSearch() - Full Text Search', () => {
    /**
     * Example 1: Text search with config parameter
     * Tests that the config parameter is accepted (even though ignored for SQLite)
     */
    it('should perform text search with config parameter', async () => {
      const { data, error } = await supabase
        .from('texts')
        .select('content')
        .textSearch('content', 'eggs ham', {
          config: 'english',
        })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      // Should match rows containing both eggs and ham
      expect(data![0].content.toLowerCase()).toContain('eggs')
    })

    /**
     * Example 2: Plain text search with config
     * Tests type: 'plain' with config parameter
     */
    it('should perform text search with plain type and config', async () => {
      const { data, error } = await supabase
        .from('quotes')
        .select('catchphrase')
        .textSearch('catchphrase', 'fat cat', {
          type: 'plain',
          config: 'english',
        })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)
      // Should match "The fat cat sat on the mat"
      expect(data![0].catchphrase.toLowerCase()).toContain('fat')
      expect(data![0].catchphrase.toLowerCase()).toContain('cat')
    })

    /**
     * Example 3: Phrase search with config
     * Tests type: 'phrase' with config parameter
     */
    it('should perform text search with phrase type and config', async () => {
      const { data, error } = await supabase
        .from('quotes')
        .select('catchphrase')
        .textSearch('catchphrase', 'fat cat', {
          type: 'phrase',
          config: 'english',
        })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match "The fat cat sat on the mat" (exact phrase)
      expect(data!.length).toBeGreaterThan(0)
    })

    /**
     * Example 4: Websearch with config
     * Tests type: 'websearch' with config parameter
     */
    it('should perform websearch-style text search with config', async () => {
      const { data, error } = await supabase
        .from('quotes')
        .select('catchphrase')
        .textSearch('catchphrase', 'fat or dog', {
          type: 'websearch',
          config: 'english',
        })

      expect(error).toBeNull()
      expect(data).toBeDefined()
      // Should match rows containing "fat" OR "dog"
      expect(data!.length).toBeGreaterThan(0)
    })
  })
})

/**
 * Compatibility Summary for Advanced Filters:
 *
 * IMPLEMENTED:
 * - textSearch(): Full text search using SQLite FTS5
 *   - Supports all query types: fts, plfts (plain), phfts (phrase), wfts (websearch)
 *   - The 'config' parameter is accepted for API compatibility but ignored
 *     (SQLite FTS5 uses tokenizers configured at index creation time instead)
 *
 * NOT IMPLEMENTED (require PostgreSQL-specific features):
 * - contains(): Array/Range/JSONB containment
 * - containedBy(): Contained by check
 * - rangeGt(): Greater than range
 * - rangeGte(): Greater than or equal range
 * - rangeLt(): Less than range
 * - rangeLte(): Less than or equal range
 * - rangeAdjacent(): Adjacent range check
 * - overlaps(): Array/Range overlap
 *
 * These filters require PostgreSQL-specific data types (arrays, ranges)
 * that are not available in SQLite. JSON containment could potentially
 * be implemented using SQLite's JSON functions in the future.
 */
