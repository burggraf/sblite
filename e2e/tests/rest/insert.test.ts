/**
 * INSERT Operation Tests
 *
 * Tests based on Supabase JavaScript documentation:
 * https://supabase.com/docs/reference/javascript/insert
 *
 * Each test corresponds to an example from the documentation.
 */

import { describe, it, expect, beforeAll, afterEach } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TEST_CONFIG } from '../../setup/global-setup'
import { uniqueId } from '../../setup/test-helpers'

describe('REST API - INSERT Operations', () => {
  let supabase: SupabaseClient

  beforeAll(() => {
    supabase = createClient(TEST_CONFIG.SBLITE_URL, TEST_CONFIG.SBLITE_ANON_KEY, {
      auth: { autoRefreshToken: false, persistSession: false },
    })
  })

  // Clean up test data after each test
  afterEach(async () => {
    // Delete any test records created during tests
    await supabase.from('countries').delete().like('name', 'Test%')
    await supabase.from('countries').delete().eq('name', 'Mordor')
    await supabase.from('countries').delete().eq('name', 'The Shire')
  })

  /**
   * Example 1: Create a record
   * Docs: https://supabase.com/docs/reference/javascript/insert#create-a-record
   *
   * const { error } = await supabase
   *   .from('countries')
   *   .insert({ id: 1, name: 'Mordor' })
   */
  describe('1. Create a record', () => {
    it('should insert a single record into the table', async () => {
      const testId = 9001
      const { error } = await supabase
        .from('countries')
        .insert({ id: testId, name: 'Mordor' })

      expect(error).toBeNull()

      // Verify the record was inserted
      const { data: verify } = await supabase
        .from('countries')
        .select()
        .eq('id', testId)

      expect(verify).toBeDefined()
      expect(verify!.length).toBe(1)
      expect(verify![0].name).toBe('Mordor')
    })

    it('should handle insert with only required fields', async () => {
      const testName = `Test_${uniqueId()}`
      const { error } = await supabase
        .from('countries')
        .insert({ name: testName })

      expect(error).toBeNull()
    })
  })

  /**
   * Example 2: Create a record and return it
   * Docs: https://supabase.com/docs/reference/javascript/insert#create-a-record-and-return-it
   *
   * const { data, error } = await supabase
   *   .from('countries')
   *   .insert({ id: 1, name: 'Mordor' })
   *   .select()
   */
  describe('2. Create a record and return it', () => {
    it('should insert and return the inserted record', async () => {
      const testId = 9002
      const { data, error } = await supabase
        .from('countries')
        .insert({ id: testId, name: 'Test_ReturnedCountry' })
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(1)
      expect(data![0].id).toBe(testId)
      expect(data![0].name).toBe('Test_ReturnedCountry')
    })

    it('should return only selected columns when specified', async () => {
      const testId = 9003
      const { data, error } = await supabase
        .from('countries')
        .insert({ id: testId, name: 'Test_PartialReturn', code: 'TR' })
        .select('name')

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data![0]).toHaveProperty('name')
      expect(data![0]).not.toHaveProperty('id')
    })
  })

  /**
   * Example 3: Bulk create
   * Docs: https://supabase.com/docs/reference/javascript/insert#bulk-create
   *
   * const { error } = await supabase
   *   .from('countries')
   *   .insert([
   *     { id: 1, name: 'Mordor' },
   *     { id: 2, name: 'The Shire' },
   *   ])
   */
  describe('3. Bulk create', () => {
    it('should insert multiple records in a single operation', async () => {
      const testRecords = [
        { id: 9004, name: 'Test_Mordor' },
        { id: 9005, name: 'Test_The Shire' },
        { id: 9006, name: 'Test_Gondor' },
      ]

      const { error } = await supabase.from('countries').insert(testRecords)

      expect(error).toBeNull()

      // Verify all records were inserted
      const { data: verify } = await supabase
        .from('countries')
        .select()
        .in('id', [9004, 9005, 9006])

      expect(verify).toBeDefined()
      expect(verify!.length).toBe(3)
    })

    it('should insert multiple records and return them', async () => {
      const testRecords = [
        { id: 9007, name: 'Test_Rivendell' },
        { id: 9008, name: 'Test_Lothlorien' },
      ]

      const { data, error } = await supabase
        .from('countries')
        .insert(testRecords)
        .select()

      expect(error).toBeNull()
      expect(data).toBeDefined()
      expect(data!.length).toBe(2)
    })
  })

  // Additional INSERT tests
  describe('Additional INSERT functionality', () => {
    it('should handle duplicate primary key with error', async () => {
      // First insert
      await supabase.from('countries').insert({ id: 9009, name: 'Test_First' })

      // Duplicate insert should fail
      const { error } = await supabase
        .from('countries')
        .insert({ id: 9009, name: 'Test_Duplicate' })

      expect(error).not.toBeNull()
    })

    it('should handle insert to non-existent table with error', async () => {
      const { error } = await supabase
        .from('nonexistent_table')
        .insert({ name: 'test' })

      expect(error).not.toBeNull()
    })

    it('should handle insert with null values where allowed', async () => {
      const { data, error } = await supabase
        .from('countries')
        .insert({ id: 9010, name: 'Test_NullCode', code: null })
        .select()

      expect(error).toBeNull()
      expect(data![0].code).toBeNull()
    })
  })
})

/**
 * Compatibility Summary for INSERT:
 *
 * IMPLEMENTED:
 * - Single record insert: .insert({...})
 * - Insert with return: .insert({...}).select()
 * - Bulk insert: .insert([{...}, {...}])
 * - Insert with selected return columns
 *
 * WORKING:
 * - Duplicate key detection
 * - Non-existent table errors
 * - Null value handling
 */
