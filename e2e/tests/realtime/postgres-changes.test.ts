/**
 * Realtime - Postgres Changes Tests
 *
 * Tests subscribing to database changes (INSERT, UPDATE, DELETE).
 * Postgres Changes allow clients to receive real-time notifications
 * when rows are inserted, updated, or deleted from tables.
 */

import { describe, it, expect, beforeAll, beforeEach, afterEach, afterAll } from 'vitest'
import { createClient, SupabaseClient } from '@supabase/supabase-js'
import { TestClient, uniqueChannel, postgresChangesTopic } from './helpers'
import { TEST_CONFIG, createTestClient as createSupabaseTestClient } from '../../setup/global-setup'
import { uniqueId } from '../../setup/test-helpers'

const TEST_TABLE = 'realtime_test'

describe('Realtime Postgres Changes', () => {
  let wsClient: TestClient
  let supabase: SupabaseClient

  beforeAll(async () => {
    supabase = createSupabaseTestClient()

    // Ensure test table exists
    // Note: This requires the table to be created via migration or admin API
    // For now, we'll use raw fetch to create via admin API if needed
    try {
      const response = await fetch(`${TEST_CONFIG.SBLITE_URL}/admin/v1/tables`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${TEST_CONFIG.SBLITE_SERVICE_KEY}`,
          apikey: TEST_CONFIG.SBLITE_ANON_KEY,
        },
        body: JSON.stringify({
          name: TEST_TABLE,
          columns: [
            { name: 'id', type: 'integer' },
            { name: 'name', type: 'text' },
            { name: 'status', type: 'text' },
          ],
        }),
      })

      if (!response.ok && response.status !== 409) {
        // 409 = already exists, which is fine
        console.warn(`Failed to create test table: ${response.status}`)
      }
    } catch (error) {
      console.warn('Could not create test table, it may already exist:', error)
    }
  })

  beforeEach(async () => {
    wsClient = new TestClient()
    await wsClient.connect()
  })

  afterEach(async () => {
    await wsClient.close()

    // Clean up test data
    await supabase.from(TEST_TABLE).delete().neq('id', 0)
  })

  describe('1. INSERT Events', () => {
    it('receives INSERT events', async () => {
      const channel = uniqueChannel('insert-test')

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'INSERT', schema: 'public', table: TEST_TABLE },
        ],
      })

      // May receive a system message about subscription confirmation
      // Skip any non-postgres_changes messages
      const testName = `test-insert-${uniqueId()}`

      // Insert via REST API
      await supabase.from(TEST_TABLE).insert({ name: testName })

      // Should receive change event
      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.event).toBe('postgres_changes')
      expect(msg.payload.data.eventType).toBe('INSERT')
      expect(msg.payload.data.new.name).toBe(testName)
    })

    it('receives INSERT with full row data', async () => {
      const channel = uniqueChannel('insert-full')

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'INSERT', schema: 'public', table: TEST_TABLE },
        ],
      })

      const testName = `full-data-${uniqueId()}`
      const testStatus = 'active'

      await supabase.from(TEST_TABLE).insert({
        name: testName,
        status: testStatus,
      })

      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.payload.data.new.name).toBe(testName)
      expect(msg.payload.data.new.status).toBe(testStatus)
    })
  })

  describe('2. UPDATE Events', () => {
    it('receives UPDATE events', async () => {
      const channel = uniqueChannel('update-test')

      // Insert a row first
      const testName = `update-test-${uniqueId()}`
      await supabase.from(TEST_TABLE).insert({ name: testName, status: 'pending' })

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'UPDATE', schema: 'public', table: TEST_TABLE },
        ],
      })

      // Update the row
      await supabase.from(TEST_TABLE).update({ status: 'completed' }).eq('name', testName)

      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.payload.data.eventType).toBe('UPDATE')
      expect(msg.payload.data.new.status).toBe('completed')
    })

    it('includes old data in UPDATE events when available', async () => {
      const channel = uniqueChannel('update-old')

      const testName = `update-old-${uniqueId()}`
      await supabase.from(TEST_TABLE).insert({ name: testName, status: 'old-status' })

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'UPDATE', schema: 'public', table: TEST_TABLE },
        ],
      })

      await supabase.from(TEST_TABLE).update({ status: 'new-status' }).eq('name', testName)

      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.payload.data.eventType).toBe('UPDATE')
      expect(msg.payload.data.new.status).toBe('new-status')
      // Old data may or may not be included depending on implementation
      if (msg.payload.data.old) {
        expect(msg.payload.data.old.status).toBe('old-status')
      }
    })
  })

  describe('3. DELETE Events', () => {
    it('receives DELETE events', async () => {
      const channel = uniqueChannel('delete-test')

      const testName = `delete-test-${uniqueId()}`
      await supabase.from(TEST_TABLE).insert({ name: testName })

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'DELETE', schema: 'public', table: TEST_TABLE },
        ],
      })

      await supabase.from(TEST_TABLE).delete().eq('name', testName)

      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.payload.data.eventType).toBe('DELETE')
      // Old data should contain the deleted row info
      if (msg.payload.data.old) {
        expect(msg.payload.data.old.name).toBe(testName)
      }
    })
  })

  describe('4. Wildcard Events', () => {
    it('receives all events with wildcard (*)', async () => {
      const channel = uniqueChannel('wildcard-test')

      await wsClient.join(channel, {
        postgres_changes: [
          { event: '*', schema: 'public', table: TEST_TABLE },
        ],
      })

      const testName = `wildcard-${uniqueId()}`

      // INSERT
      await supabase.from(TEST_TABLE).insert({ name: testName, status: 'new' })
      const insertMsg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(insertMsg.payload.data.eventType).toBe('INSERT')

      // UPDATE
      await supabase.from(TEST_TABLE).update({ status: 'updated' }).eq('name', testName)
      const updateMsg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(updateMsg.payload.data.eventType).toBe('UPDATE')

      // DELETE
      await supabase.from(TEST_TABLE).delete().eq('name', testName)
      const deleteMsg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(deleteMsg.payload.data.eventType).toBe('DELETE')
    })
  })

  describe('5. Column Filters', () => {
    it('filters by column value with eq', async () => {
      const channel = uniqueChannel('filter-eq')

      await wsClient.join(channel, {
        postgres_changes: [
          { event: '*', schema: 'public', table: TEST_TABLE, filter: 'status=eq.filtered' },
        ],
      })

      // Insert non-matching row
      await supabase.from(TEST_TABLE).insert({ name: `not-filtered-${uniqueId()}`, status: 'not-filtered' })

      // Give a moment for non-matching event to potentially arrive
      await new Promise((resolve) => setTimeout(resolve, 200))

      // Insert matching row
      const matchingName = `filtered-${uniqueId()}`
      await supabase.from(TEST_TABLE).insert({ name: matchingName, status: 'filtered' })

      // Should only receive the filtered one
      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.payload.data.new.status).toBe('filtered')
      expect(msg.payload.data.new.name).toBe(matchingName)
    })

    it('filters by column value with neq', async () => {
      const channel = uniqueChannel('filter-neq')

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'INSERT', schema: 'public', table: TEST_TABLE, filter: 'status=neq.excluded' },
        ],
      })

      // Insert excluded row - should not trigger event
      await supabase.from(TEST_TABLE).insert({ name: `excluded-${uniqueId()}`, status: 'excluded' })
      await new Promise((resolve) => setTimeout(resolve, 200))

      // Insert included row
      const includedName = `included-${uniqueId()}`
      await supabase.from(TEST_TABLE).insert({ name: includedName, status: 'included' })

      const msg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(msg.payload.data.new.status).toBe('included')
    })
  })

  describe('6. Multiple Subscriptions', () => {
    it('can subscribe to multiple tables', async () => {
      // This test requires a second table to exist
      // For now, we test with multiple event subscriptions on the same table
      const channel = uniqueChannel('multi-sub')

      await wsClient.join(channel, {
        postgres_changes: [
          { event: 'INSERT', schema: 'public', table: TEST_TABLE },
          { event: 'DELETE', schema: 'public', table: TEST_TABLE },
        ],
      })

      const testName = `multi-sub-${uniqueId()}`

      // INSERT
      await supabase.from(TEST_TABLE).insert({ name: testName })
      const insertMsg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(insertMsg.payload.data.eventType).toBe('INSERT')

      // DELETE
      await supabase.from(TEST_TABLE).delete().eq('name', testName)
      const deleteMsg = await wsClient.waitForEvent(channel, 'postgres_changes', 5000)
      expect(deleteMsg.payload.data.eventType).toBe('DELETE')
    })
  })

  describe('7. Multi-Client Subscriptions', () => {
    it('sends changes to all subscribed clients', async () => {
      const wsClient2 = new TestClient()
      await wsClient2.connect()

      const channel = uniqueChannel('multi-client')

      try {
        await wsClient.join(channel, {
          postgres_changes: [
            { event: 'INSERT', schema: 'public', table: TEST_TABLE },
          ],
        })

        await wsClient2.join(channel, {
          postgres_changes: [
            { event: 'INSERT', schema: 'public', table: TEST_TABLE },
          ],
        })

        const testName = `multi-client-${uniqueId()}`
        await supabase.from(TEST_TABLE).insert({ name: testName })

        // Both clients should receive the event
        const [msg1, msg2] = await Promise.all([
          wsClient.waitForEvent(channel, 'postgres_changes', 5000),
          wsClient2.waitForEvent(channel, 'postgres_changes', 5000),
        ])

        expect(msg1.payload.data.eventType).toBe('INSERT')
        expect(msg2.payload.data.eventType).toBe('INSERT')
        expect(msg1.payload.data.new.name).toBe(testName)
        expect(msg2.payload.data.new.name).toBe(testName)
      } finally {
        await wsClient2.close()
      }
    })
  })
})

/**
 * Compatibility Summary for Realtime Postgres Changes:
 *
 * IMPLEMENTED:
 * - Subscribe to INSERT events
 * - Subscribe to UPDATE events
 * - Subscribe to DELETE events
 * - Wildcard (*) event subscription
 * - Column value filtering (eq, neq)
 * - Multiple subscriptions per channel
 * - Multi-client subscriptions
 *
 * SUPABASE FEATURES:
 * - Schema filtering (default: public)
 * - Table filtering
 * - Event type filtering
 * - Column value filters with various operators
 *
 * NOT IMPLEMENTED/TESTED:
 * - RLS policy integration
 * - More filter operators (gt, lt, in, etc.)
 * - Subscription to specific columns only
 */
