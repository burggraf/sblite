/**
 * Realtime - Connection Tests
 *
 * Tests WebSocket connection, heartbeat, and basic channel operations
 * using the Phoenix protocol.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TestClient, uniqueChannel } from './helpers'

describe('Realtime Connection', () => {
  let client: TestClient

  beforeEach(async () => {
    client = new TestClient()
    await client.connect()
  })

  afterEach(async () => {
    await client.close()
  })

  describe('1. Basic Connection', () => {
    it('connects with valid API key', async () => {
      // Connection is established in beforeEach
      // Verify by sending a heartbeat
      const reply = await client.heartbeat()
      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })

    it('is connected after connect()', () => {
      expect(client.isConnected()).toBe(true)
    })

    it('is disconnected after close()', async () => {
      await client.close()
      expect(client.isConnected()).toBe(false)
    })
  })

  describe('2. Heartbeat', () => {
    it('responds to heartbeat', async () => {
      const reply = await client.heartbeat()
      expect(reply.event).toBe('phx_reply')
      expect(reply.topic).toBe('phoenix')
      expect(reply.payload.status).toBe('ok')
    })

    it('responds to multiple heartbeats', async () => {
      const reply1 = await client.heartbeat()
      const reply2 = await client.heartbeat()
      const reply3 = await client.heartbeat()

      expect(reply1.payload.status).toBe('ok')
      expect(reply2.payload.status).toBe('ok')
      expect(reply3.payload.status).toBe('ok')

      // Each reply should have a unique ref
      expect(reply1.ref).not.toBe(reply2.ref)
      expect(reply2.ref).not.toBe(reply3.ref)
    })
  })

  describe('3. Channel Join', () => {
    it('joins a channel', async () => {
      const channel = uniqueChannel('join-test')
      const reply = await client.join(channel)

      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })

    it('joins multiple channels', async () => {
      const channel1 = uniqueChannel('multi-1')
      const channel2 = uniqueChannel('multi-2')

      const reply1 = await client.join(channel1)
      const reply2 = await client.join(channel2)

      expect(reply1.payload.status).toBe('ok')
      expect(reply2.payload.status).toBe('ok')
    })

    it('joins channel with broadcast config', async () => {
      const channel = uniqueChannel('broadcast-config')
      const reply = await client.join(channel, {
        broadcast: { self: true, ack: true },
      })

      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })

    it('joins channel with presence config', async () => {
      const channel = uniqueChannel('presence-config')
      const reply = await client.join(channel, {
        presence: { key: 'test-user-123' },
      })

      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })
  })

  describe('4. Channel Leave', () => {
    it('leaves a channel', async () => {
      const channel = uniqueChannel('leave-test')

      await client.join(channel)
      const reply = await client.leave(channel)

      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })

    it('can rejoin a channel after leaving', async () => {
      const channel = uniqueChannel('rejoin-test')

      await client.join(channel)
      await client.leave(channel)
      const reply = await client.join(channel)

      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })
  })

  describe('5. Error Handling', () => {
    it('handles joining with empty topic', async () => {
      // The server should reject or handle gracefully
      // Implementation may vary - test documents actual behavior
      try {
        const reply = await client.join('')
        // If no error thrown, check the response
        expect(reply.event).toBe('phx_reply')
      } catch (error) {
        // Error is acceptable for invalid topic
        expect(error).toBeDefined()
      }
    })
  })
})

/**
 * Compatibility Summary for Realtime Connection:
 *
 * IMPLEMENTED:
 * - WebSocket connection with API key authentication
 * - Phoenix protocol heartbeat
 * - Channel join with configuration
 * - Channel leave
 *
 * NOT YET TESTED:
 * - Connection with access token (authenticated user)
 * - Connection rejection with invalid API key
 * - Connection timeout handling
 */
