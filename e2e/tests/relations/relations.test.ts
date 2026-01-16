/**
 * Relationship Query Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/select#query-referenced-tables
 *
 * Tests for embedded resources (foreign key relationships) support.
 */

import { describe, it, expect, beforeAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'

describe('Relationship Queries', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  /**
   * Many-to-One Relationships
   * cities.country_id -> countries.id
   */
  describe('Many-to-One (cities -> countries)', () => {
    it('should embed parent record using FK column reference with alias', async () => {
      // Note: Using alias is recommended to avoid conflict between FK column and embed name
      const { data, error } = await supabase
        .from('cities')
        .select('name, country:country_id(name)')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      // Each city should have an embedded country object
      data!.forEach((city: any) => {
        expect(city.name).toBeDefined()
        expect(city.country).toBeDefined()
        expect(city.country.name).toBeDefined()
      })
    })

    it('should embed parent record with alias', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select('name, country:country_id(name)')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)

      // Alias should work
      data!.forEach((city: any) => {
        expect(city.name).toBeDefined()
        expect(city.country).toBeDefined()
        expect(city.country.name).toBeDefined()
      })
    })

    it('should select specific columns from related table', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select('name, country:country_id(name, code)')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)

      // Should have name and code from countries
      const city = data![0] as any
      expect(city.country.name).toBeDefined()
      expect(city.country.code).toBeDefined()
    })
  })

  /**
   * One-to-Many Relationships
   * countries.id <- cities.country_id
   */
  describe('One-to-Many (countries -> cities)', () => {
    it('should embed child records as array', async () => {
      const { data, error } = await supabase
        .from('countries')
        .select('name, cities(name)')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      // Each country should have an array of cities
      data!.forEach((country) => {
        expect(country.name).toBeDefined()
        expect(Array.isArray(country.cities)).toBe(true)
      })
    })

    it('should embed child records with specific columns', async () => {
      const { data, error } = await supabase
        .from('countries')
        .select('name, cities(name, population)')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)

      // Find a country with cities
      const countryWithCities = data!.find((c) => c.cities.length > 0)
      expect(countryWithCities).toBeDefined()
      expect(countryWithCities!.cities[0].name).toBeDefined()
      expect(countryWithCities!.cities[0].population).toBeDefined()
    })

    it('should return empty array for no children', async () => {
      // Mexico has no cities in our test data
      const { data, error } = await supabase
        .from('countries')
        .select('name, cities(name)')
        .eq('name', 'Mexico')

      expect(error).toBeNull()
      expect(data!.length).toBe(1)
      expect(Array.isArray(data![0].cities)).toBe(true)
      expect(data![0].cities.length).toBe(0)
    })
  })

  /**
   * Inner Join (using !inner)
   */
  describe('Inner Join (!inner)', () => {
    it('should filter out rows without matching relations', async () => {
      // Using !inner should only return instruments that have a section
      const { data, error } = await supabase
        .from('instruments')
        .select('name, section:section_id!inner(name)')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBeGreaterThan(0)

      // All results should have a non-null section
      data!.forEach((row: any) => {
        expect(row.section).not.toBeNull()
        expect(row.section.name).toBeDefined()
      })
    })

    it('should filter parent by child existence in one-to-many', async () => {
      // Countries with at least one city
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
   * Orchestral sections and instruments (from Supabase docs example)
   */
  describe('Sections and Instruments', () => {
    it('should embed instruments in sections (one-to-many)', async () => {
      const { data, error } = await supabase
        .from('orchestral_sections')
        .select('name, instruments(name)')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)

      // Each section should have an array of instruments
      data!.forEach((section) => {
        expect(section.name).toBeDefined()
        expect(Array.isArray(section.instruments)).toBe(true)
      })

      // Strings section should have violin and viola
      const strings = data!.find((s) => s.name === 'strings')
      expect(strings).toBeDefined()
      const instrumentNames = strings!.instruments.map((i: any) => i.name)
      expect(instrumentNames).toContain('violin')
      expect(instrumentNames).toContain('viola')
    })

    it('should embed section in instrument (many-to-one)', async () => {
      const { data, error } = await supabase
        .from('instruments')
        .select('name, section:section_id(name)')

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)

      // Violin should be in strings
      const violin = data!.find((i) => i.name === 'violin') as any
      expect(violin).toBeDefined()
      expect(violin!.section).toBeDefined()
      expect(violin!.section.name).toBe('strings')
    })
  })

  /**
   * Combined queries with filters
   */
  describe('Relations with Filters', () => {
    it('should apply filters to main table with relations', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select('name, country:country_id(name)')
        .eq('country_id', 1) // United States

      expect(error).toBeNull()
      expect(data!.length).toBeGreaterThan(0)

      // All cities should be in the United States
      data!.forEach((city: any) => {
        expect(city.country.name).toBe('United States')
      })
    })

    it('should combine relations with ordering', async () => {
      const { data, error } = await supabase
        .from('cities')
        .select('name, population, country:country_id(name)')
        .order('population', { ascending: false })
        .limit(3)

      expect(error).toBeNull()
      expect(data!.length).toBe(3)

      // Should be ordered by population descending
      expect(data![0].population).toBeGreaterThanOrEqual(data![1].population)
      expect(data![1].population).toBeGreaterThanOrEqual(data![2].population)

      // Should have embedded country
      data!.forEach((city: any) => {
        expect(city.country).toBeDefined()
      })
    })
  })
})

/**
 * Compatibility Summary for Relationships:
 *
 * IMPLEMENTED:
 * - Many-to-one embedding (FK column reference)
 * - One-to-many embedding (child table reference)
 * - Aliased relations (country:country_id)
 * - Inner joins (!inner modifier)
 * - Specific column selection in relations
 * - Combined with filters and modifiers
 *
 * NOT IMPLEMENTED:
 * - Many-to-many through join tables
 * - Filtering on related table columns
 * - Ordering on related tables
 * - Nested relations beyond one level
 * - Self-referential relationships
 */
