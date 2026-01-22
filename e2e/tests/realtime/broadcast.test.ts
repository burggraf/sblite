/**
 * Realtime - Broadcast Tests
 *
 * Tests broadcasting messages between clients on the same channel.
 * Broadcast allows clients to send messages to other connected clients
 * without persisting to the database.
 */

import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { TestClient, uniqueChannel } from './helpers'

describe('Realtime Broadcast', () => {
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

  describe('1. Basic Broadcast', () => {
    it('broadcasts message to other subscribers', async () => {
      const channel = uniqueChannel('broadcast')

      await client1.join(channel, { broadcast: { self: false } })
      await client2.join(channel, { broadcast: { self: false } })

      // Small delay to ensure both clients are subscribed
      await new Promise((resolve) => setTimeout(resolve, 50))

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'test-event',
          payload: { message: 'hello' },
        },
      })

      const msg = await client2.receive()
      expect(msg.event).toBe('broadcast')
      expect(msg.payload.event).toBe('test-event')
      expect(msg.payload.payload.message).toBe('hello')
    })

    it('broadcasts with custom event name', async () => {
      const channel = uniqueChannel('custom-event')

      await client1.join(channel, { broadcast: { self: false } })
      await client2.join(channel, { broadcast: { self: false } })

      await new Promise((resolve) => setTimeout(resolve, 50))

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'user:typing',
          payload: { user_id: 'user-123' },
        },
      })

      const msg = await client2.receive()
      expect(msg.payload.event).toBe('user:typing')
      expect(msg.payload.payload.user_id).toBe('user-123')
    })

    it('broadcasts complex payload', async () => {
      const channel = uniqueChannel('complex-payload')

      await client1.join(channel, { broadcast: { self: false } })
      await client2.join(channel, { broadcast: { self: false } })

      await new Promise((resolve) => setTimeout(resolve, 50))

      const complexPayload = {
        nested: {
          data: [1, 2, 3],
          object: { key: 'value' },
        },
        timestamp: '2024-01-01T00:00:00Z',
        count: 42,
        active: true,
      }

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'complex-data',
          payload: complexPayload,
        },
      })

      const msg = await client2.receive()
      expect(msg.payload.payload).toEqual(complexPayload)
    })
  })

  describe('2. Self Broadcast Setting', () => {
    it('does not receive own broadcast when self=false', async () => {
      const channel = uniqueChannel('no-self')

      await client1.join(channel, { broadcast: { self: false } })

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'test-event',
          payload: { message: 'hello' },
        },
      })

      // Should timeout because self=false means no echo
      const msg = await client1.tryReceive(500)
      expect(msg).toBeNull()
    })

    it('receives own broadcast when self=true', async () => {
      const channel = uniqueChannel('with-self')

      await client1.join(channel, { broadcast: { self: true } })

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'test-event',
          payload: { message: 'hello' },
        },
      })

      const msg = await client1.receive()
      expect(msg.event).toBe('broadcast')
      expect(msg.payload.payload.message).toBe('hello')
    })
  })

  describe('3. Acknowledgment', () => {
    it('sends ack when ack=true', async () => {
      const channel = uniqueChannel('with-ack')

      await client1.join(channel, { broadcast: { ack: true, self: false } })

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'test-event',
          payload: { message: 'hello' },
        },
      })

      const reply = await client1.receive()
      expect(reply.event).toBe('phx_reply')
      expect(reply.payload.status).toBe('ok')
    })

    it('does not send ack when ack=false', async () => {
      const channel = uniqueChannel('no-ack')

      await client1.join(channel, { broadcast: { ack: false, self: false } })

      client1.send({
        event: 'broadcast',
        topic: channel,
        payload: {
          type: 'broadcast',
          event: 'test-event',
          payload: { message: 'hello' },
        },
      })

      // Should not receive ack
      const msg = await client1.tryReceive(500)
      expect(msg).toBeNull()
    })
  })

  describe('4. Multi-Client Broadcast', () => {
    it('broadcasts to all subscribers', async () => {
      const client3 = new TestClient()
      await client3.connect()

      const channel = uniqueChannel('multi-client')

      try {
        await client1.join(channel, { broadcast: { self: false } })
        await client2.join(channel, { broadcast: { self: false } })
        await client3.join(channel, { broadcast: { self: false } })

        await new Promise((resolve) => setTimeout(resolve, 50))

        client1.send({
          event: 'broadcast',
          topic: channel,
          payload: {
            type: 'broadcast',
            event: 'announcement',
            payload: { message: 'hello all' },
          },
        })

        // Both client2 and client3 should receive the message
        const [msg2, msg3] = await Promise.all([
          client2.receive(),
          client3.receive(),
        ])

        expect(msg2.payload.payload.message).toBe('hello all')
        expect(msg3.payload.payload.message).toBe('hello all')
      } finally {
        await client3.close()
      }
    })

    it('only broadcasts to subscribers of the same channel', async () => {
      const channel1 = uniqueChannel('channel-1')
      const channel2 = uniqueChannel('channel-2')

      await client1.join(channel1, { broadcast: { self: false } })
      await client2.join(channel2, { broadcast: { self: false } })

      await new Promise((resolve) => setTimeout(resolve, 50))

      client1.send({
        event: 'broadcast',
        topic: channel1,
        payload: {
          type: 'broadcast',
          event: 'test',
          payload: { message: 'channel1 only' },
        },
      })

      // Client2 is on different channel, should not receive
      const msg = await client2.tryReceive(500)
      expect(msg).toBeNull()
    })
  })

  describe('5. Rapid Broadcast', () => {
    it('handles multiple rapid broadcasts', async () => {
      const channel = uniqueChannel('rapid')

      await client1.join(channel, { broadcast: { self: false } })
      await client2.join(channel, { broadcast: { self: false } })

      await new Promise((resolve) => setTimeout(resolve, 50))

      // Send 5 rapid messages
      for (let i = 0; i < 5; i++) {
        client1.send({
          event: 'broadcast',
          topic: channel,
          payload: {
            type: 'broadcast',
            event: 'rapid',
            payload: { index: i },
          },
        })
      }

      // Collect messages
      const messages = await client2.collectMessages(1000)

      // Should receive all 5 messages
      expect(messages.length).toBeGreaterThanOrEqual(5)

      const indices = messages
        .filter((m) => m.event === 'broadcast' && m.payload.event === 'rapid')
        .map((m) => m.payload.payload.index)
        .sort((a, b) => a - b)

      expect(indices).toEqual([0, 1, 2, 3, 4])
    })
  })
})

/**
 * Compatibility Summary for Realtime Broadcast:
 *
 * IMPLEMENTED:
 * - Basic message broadcasting between clients
 * - Self broadcast control (self=true/false)
 * - Acknowledgment control (ack=true/false)
 * - Custom event names
 * - Complex payload support
 * - Multi-client broadcasting
 *
 * SUPABASE FEATURES:
 * - Broadcast is the simplest realtime feature
 * - Messages are not persisted to database
 * - Useful for presence, typing indicators, etc.
 */
