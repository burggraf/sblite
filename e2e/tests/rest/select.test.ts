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
   * Full test coverage in quoted-identifiers.test.ts
   * Note: supabase-js strips spaces in .select() but handles them in filters/order.
   */
  describe('4. Query referenced tables with spaces', () => {
    it('should handle table names with spaces', async () => {
      const { data, error } = await supabase.from('my table').select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(3)
      // Verify columns with spaces are returned
      expect(data![0]).toHaveProperty('my column')
    })

    it('should handle column names with spaces in filters', async () => {
      // Note: .select('col') strips spaces, but filters work correctly
      const { data, error } = await supabase
        .from('my table')
        .select()
        .eq('my column', 'first row')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0]['my column']).toBe('first row')
    })
  })

  /**
   * Example 5: Query referenced tables through a join table
   * Docs: https://supabase.com/docs/reference/javascript/select#query-referenced-tables-through-a-join-table
   *
   * Queries through junction tables like user_teams to get users with their teams.
   */
  describe('5. Query referenced tables through join table', () => {
    it('should query through many-to-many join tables', async () => {
      // Query users with their teams through the user_teams junction table
      const { data, error } = await supabase.from('users').select(`
          id,
          name,
          teams (
            id,
            name
          )
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      // User 1 (John Doe) should have teams
      const john = data!.find((u) => u.name === 'John Doe')
      expect(john).toBeDefined()
      expect(Array.isArray(john!.teams)).toBe(true)
      expect(john!.teams.length).toBe(2) // John is in both teams

      // User 2 (Jane Smith) should have one team
      const jane = data!.find((u) => u.name === 'Jane Smith')
      expect(jane).toBeDefined()
      expect(Array.isArray(jane!.teams)).toBe(true)
      expect(jane!.teams.length).toBe(1) // Jane is in Team Alpha only
    })
  })

  /**
   * Example 6: Query the same referenced table multiple times
   * Docs: https://supabase.com/docs/reference/javascript/select#query-the-same-referenced-table-multiple-times
   *
   * Uses hint syntax (!) to disambiguate when multiple FKs point to the same table.
   */
  describe('6. Query same referenced table multiple times', () => {
    it('should allow aliasing for multiple references to same table', async () => {
      // Query messages with both sender and receiver user info
      // Uses !fk_hint syntax to disambiguate the two FKs to users table
      const { data, error } = await supabase.from('messages').select(`
          id,
          content,
          sender:users!sender_id (
            id,
            name
          ),
          receiver:users!receiver_id (
            id,
            name
          )
        `)

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      // Check first message structure
      const msg = data![0]
      expect(msg).toHaveProperty('id')
      expect(msg).toHaveProperty('content')
      expect(msg).toHaveProperty('sender')
      expect(msg).toHaveProperty('receiver')
      expect(msg.sender).toHaveProperty('name')
      expect(msg.receiver).toHaveProperty('name')

      // Verify the relationships are correct (message 1: John -> Jane)
      const msg1 = data!.find((m) => m.id === 1)
      expect(msg1).toBeDefined()
      expect(msg1!.sender.name).toBe('John Doe')
      expect(msg1!.receiver.name).toBe('Jane Smith')
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
   * Filter main table based on values in referenced tables.
   */
  describe('8. Filtering through referenced tables', () => {
    it('should filter on fields from referenced tables', async () => {
      // Get cities where the country name is 'Canada'
      const { data, error } = await supabase
        .from('cities')
        .select('name, countries(name)')
        .eq('countries.name', 'Canada')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(2) // Toronto and Vancouver

      // All returned cities should be in Canada
      const cityNames = data!.map((c) => c.name).sort()
      expect(cityNames).toEqual(['Toronto', 'Vancouver'])
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
    it('should extract fields from JSON columns using arrow notation', async () => {
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
 * - JSON path extraction (-> and ->>)
 * - HEAD method for count-only queries (head: true)
 * - Many-to-many join table queries (through junction tables)
 * - Aliased joins with FK hints: alias:table!fk_hint(cols)
 * - Filtering through referenced tables: .eq('relation.col', 'value')
 *
 * NOT IMPLEMENTED:
 * - Schema switching (N/A for SQLite)
 * - Table names with spaces (requires special setup)
 */
