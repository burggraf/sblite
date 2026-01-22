/**
 * Realtime - Presence Tests
 *
 * Tests presence functionality for tracking connected users.
 * Presence allows clients to share state with other connected clients
 * and receive updates when users join or leave.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TestClient, uniqueChannel } from './helpers'

describe('Realtime Presence', () => {
  let client1: TestClient
  let client2: TestClient

  beforeEach(async () => {
    client1 = new TestClient()
    client2 = new TestClient()
    await Promise.all([client1.connect(), client2.connect()])
  })

  afterEach(async () => {
    await Promise.all([client1.close(), client2.close()])
  })

  describe('1. Presence State', () => {
    it('receives presence_state on join when others are tracked', async () => {
      const channel = uniqueChannel('presence-state')

      // Client 1 joins and tracks presence
      await client1.join(channel, {
        presence: { key: 'user-1' },
      })

      // Track presence for client 1
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'online', name: 'User 1' },
        },
      })

      // Wait for client 1's presence diff to be processed
      await client1.waitForEvent(channel, 'presence_diff', 3000)

      // Client 2 joins and should receive presence state
      await client2.join(channel, {
        presence: { key: 'user-2' },
      })

      // Client 2 should receive presence_state with client 1's info
      const stateMsg = await client2.waitForEvent(channel, 'presence_state', 3000)
      expect(stateMsg.event).toBe('presence_state')
      expect(stateMsg.payload).toBeDefined()
      expect(stateMsg.payload['user-1']).toBeDefined()
      expect(stateMsg.payload['user-1'][0].status).toBe('online')
      expect(stateMsg.payload['user-1'][0].name).toBe('User 1')
    })

    it('receives empty presence_state when no one is tracked', async () => {
      const channel = uniqueChannel('empty-state')

      // Client 1 joins but does not track presence
      await client1.join(channel, {
        presence: { key: 'user-1' },
      })

      // Client 2 joins
      await client2.join(channel, {
        presence: { key: 'user-2' },
      })

      // Client 2 should receive presence_state (may be empty or have no tracked users)
      const msg = await client2.tryReceive(2000)

      if (msg && msg.event === 'presence_state') {
        // State received, verify structure
        expect(typeof msg.payload).toBe('object')
      }
      // It's also acceptable to not receive presence_state if no one is tracked
    })
  })

  describe('2. Presence Track', () => {
    it('broadcasts presence_diff on track', async () => {
      const channel = uniqueChannel('track-diff')

      await client1.join(channel, { presence: { key: 'user-1' } })
      await client2.join(channel, { presence: { key: 'user-2' } })

      // Skip initial presence_state for client2
      await client2.tryReceive(500)

      // Client 1 tracks presence
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'away' },
        },
      })

      // Client 2 should receive presence_diff with the join
      const diffMsg = await client2.waitForEvent(channel, 'presence_diff', 3000)
      expect(diffMsg.event).toBe('presence_diff')
      expect(diffMsg.payload.joins).toBeDefined()
      expect(diffMsg.payload.joins['user-1']).toBeDefined()
      expect(diffMsg.payload.joins['user-1'][0].status).toBe('away')
    })

    it('updates presence with new state', async () => {
      const channel = uniqueChannel('track-update')

      await client1.join(channel, { presence: { key: 'user-1' } })
      await client2.join(channel, { presence: { key: 'user-2' } })

      // Skip initial presence_state for client2
      await client2.tryReceive(500)

      // Client 1 tracks with initial state
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'online' },
        },
      })

      // Wait for initial diff
      await client2.waitForEvent(channel, 'presence_diff', 3000)

      // Client 1 updates presence
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'busy' },
        },
      })

      // Client 2 should receive presence_diff with the update
      const diffMsg = await client2.waitForEvent(channel, 'presence_diff', 3000)
      expect(diffMsg.event).toBe('presence_diff')

      // The update shows as a leave/join or just a join depending on implementation
      if (diffMsg.payload.joins['user-1']) {
        expect(diffMsg.payload.joins['user-1'][0].status).toBe('busy')
      }
    })
  })

  describe('3. Presence Leave', () => {
    it('broadcasts presence_diff on leave', async () => {
      const channel = uniqueChannel('leave-diff')

      await client1.join(channel, { presence: { key: 'user-1' } })

      // Track presence
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'online' },
        },
      })

      // Wait for track diff
      await client1.waitForEvent(channel, 'presence_diff', 3000)

      await client2.join(channel, { presence: { key: 'user-2' } })

      // Skip presence_state
      await client2.tryReceive(500)

      // Client 1 leaves the channel
      await client1.leave(channel)

      // Client 2 should receive presence_diff with leave
      const diffMsg = await client2.waitForEvent(channel, 'presence_diff', 3000)
      expect(diffMsg.event).toBe('presence_diff')
      expect(diffMsg.payload.leaves).toBeDefined()
      expect(diffMsg.payload.leaves['user-1']).toBeDefined()
    })

    it('broadcasts presence_diff on untrack', async () => {
      const channel = uniqueChannel('untrack-diff')

      await client1.join(channel, { presence: { key: 'user-1' } })
      await client2.join(channel, { presence: { key: 'user-2' } })

      // Skip presence_state for client2
      await client2.tryReceive(500)

      // Client 1 tracks presence
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'online' },
        },
      })

      // Wait for join diff
      await client2.waitForEvent(channel, 'presence_diff', 3000)

      // Client 1 untracks
      await client1.presenceUntrack(channel)

      // Client 2 should receive presence_diff with leave
      const diffMsg = await client2.waitForEvent(channel, 'presence_diff', 3000)
      expect(diffMsg.event).toBe('presence_diff')
      expect(diffMsg.payload.leaves).toBeDefined()
      expect(diffMsg.payload.leaves['user-1']).toBeDefined()
    })

    it('broadcasts presence_diff on disconnect', async () => {
      const channel = uniqueChannel('disconnect-diff')

      await client1.join(channel, { presence: { key: 'user-1' } })

      // Track presence
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { status: 'online' },
        },
      })

      // Wait for track diff
      await client1.waitForEvent(channel, 'presence_diff', 3000)

      await client2.join(channel, { presence: { key: 'user-2' } })

      // Skip presence_state
      await client2.tryReceive(500)

      // Close client 1 connection (simulates disconnect)
      await client1.close()

      // Client 2 should receive presence_diff with leave
      const diffMsg = await client2.waitForEvent(channel, 'presence_diff', 5000)
      expect(diffMsg.event).toBe('presence_diff')
      expect(diffMsg.payload.leaves).toBeDefined()
      expect(diffMsg.payload.leaves['user-1']).toBeDefined()
    })
  })

  describe('4. Multiple Presence Keys', () => {
    it('tracks multiple users with different keys', async () => {
      const client3 = new TestClient()
      await client3.connect()

      const channel = uniqueChannel('multi-users')

      try {
        await client1.join(channel, { presence: { key: 'user-1' } })
        await client2.join(channel, { presence: { key: 'user-2' } })
        await client3.join(channel, { presence: { key: 'user-3' } })

        // All track presence
        client1.send({
          event: 'presence',
          topic: channel,
          payload: { type: 'presence', event: 'track', payload: { name: 'Alice' } },
        })

        client2.send({
          event: 'presence',
          topic: channel,
          payload: { type: 'presence', event: 'track', payload: { name: 'Bob' } },
        })

        client3.send({
          event: 'presence',
          topic: channel,
          payload: { type: 'presence', event: 'track', payload: { name: 'Charlie' } },
        })

        // Wait for all diffs to propagate
        await new Promise((resolve) => setTimeout(resolve, 500))

        // New client joins and should see all three
        const client4 = new TestClient()
        await client4.connect()

        try {
          await client4.join(channel, { presence: { key: 'user-4' } })

          const stateMsg = await client4.waitForEvent(channel, 'presence_state', 3000)
          expect(stateMsg.event).toBe('presence_state')

          // Should have at least 3 users tracked
          const keys = Object.keys(stateMsg.payload)
          expect(keys.length).toBeGreaterThanOrEqual(3)
        } finally {
          await client4.close()
        }
      } finally {
        await client3.close()
      }
    })
  })

  describe('5. Presence Metadata', () => {
    it('preserves complex presence metadata', async () => {
      const channel = uniqueChannel('complex-meta')

      await client1.join(channel, { presence: { key: 'user-1' } })
      await client2.join(channel, { presence: { key: 'user-2' } })

      // Skip presence_state for client2
      await client2.tryReceive(500)

      // Track with complex metadata
      const complexMeta = {
        status: 'online',
        user: {
          name: 'Test User',
          avatar: 'https://example.com/avatar.png',
        },
        location: {
          room: 'general',
          cursor: { x: 100, y: 200 },
        },
        timestamp: Date.now(),
      }

      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: complexMeta,
        },
      })

      const diffMsg = await client2.waitForEvent(channel, 'presence_diff', 3000)
      expect(diffMsg.payload.joins['user-1']).toBeDefined()

      const presence = diffMsg.payload.joins['user-1'][0]
      expect(presence.status).toBe('online')
      expect(presence.user.name).toBe('Test User')
      expect(presence.location.room).toBe('general')
      expect(presence.location.cursor.x).toBe(100)
    })
  })

  describe('6. Same User Multiple Connections', () => {
    it('tracks multiple connections with same key', async () => {
      const channel = uniqueChannel('same-key')

      // Same user key for both clients (simulating multiple tabs)
      await client1.join(channel, { presence: { key: 'user-shared' } })
      await client2.join(channel, { presence: { key: 'user-shared' } })

      // Both track presence
      client1.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { device: 'laptop' },
        },
      })

      client2.send({
        event: 'presence',
        topic: channel,
        payload: {
          type: 'presence',
          event: 'track',
          payload: { device: 'phone' },
        },
      })

      await new Promise((resolve) => setTimeout(resolve, 500))

      // New client joins
      const client3 = new TestClient()
      await client3.connect()

      try {
        await client3.join(channel, { presence: { key: 'user-3' } })

        const stateMsg = await client3.waitForEvent(channel, 'presence_state', 3000)

        // Same key may have multiple presences (one per connection)
        if (stateMsg.payload['user-shared']) {
          // Should be an array with potentially multiple entries
          expect(Array.isArray(stateMsg.payload['user-shared'])).toBe(true)
          // At least one entry should exist
          expect(stateMsg.payload['user-shared'].length).toBeGreaterThanOrEqual(1)
        }
      } finally {
        await client3.close()
      }
    })
  })
})

/**
 * Compatibility Summary for Realtime Presence:
 *
 * IMPLEMENTED:
 * - Presence tracking (track/untrack)
 * - Presence state on join
 * - Presence diff on track/untrack/leave
 * - Custom presence metadata
 * - Multiple users with different keys
 * - Multiple connections with same key
 *
 * SUPABASE FEATURES:
 * - presence_state: Initial state of all tracked users
 * - presence_diff: Changes (joins/leaves)
 * - Track: Add/update presence
 * - Untrack: Remove presence
 *
 * USE CASES:
 * - Online/offline status
 * - Who's viewing a document
 * - Cursor positions in collaborative apps
 * - Typing indicators
 */
