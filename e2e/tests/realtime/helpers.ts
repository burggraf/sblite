/**
 * WebSocket test helpers for sblite Realtime E2E tests
 *
 * Provides a TestClient class for connecting to and interacting with
 * the sblite realtime WebSocket endpoint using Phoenix protocol.
 *
 * Note: Requires 'ws' package to be installed:
 *   npm install --save-dev ws @types/ws
 */

import { WebSocket } from 'ws'
import { TEST_CONFIG } from '../../setup/global-setup'

// WebSocket URL for realtime endpoint
const WS_URL = process.env.SBLITE_WS_URL || 'ws://localhost:8080/realtime/v1/websocket'

/**
 * Phoenix protocol message format
 */
export interface Message {
  event: string
  topic: string
  payload: Record<string, any>
  ref: string
  join_ref?: string
}

/**
 * Subscription configuration for channel joins
 */
export interface SubscriptionConfig {
  broadcast?: {
    self?: boolean
    ack?: boolean
  }
  presence?: {
    key?: string
  }
  postgres_changes?: Array<{
    event: 'INSERT' | 'UPDATE' | 'DELETE' | '*'
    schema?: string
    table: string
    filter?: string
  }>
}

/**
 * WebSocket test client for realtime testing
 *
 * Example usage:
 * ```typescript
 * const client = new TestClient()
 * await client.connect()
 *
 * // Join a channel
 * const response = await client.join('realtime:public:messages', {
 *   postgres_changes: [{ event: '*', table: 'messages' }]
 * })
 *
 * // Receive messages
 * const msg = await client.receive()
 *
 * // Clean up
 * await client.close()
 * ```
 */
export class TestClient {
  private ws: WebSocket | null = null
  private messageQueue: Message[] = []
  private messageResolvers: Array<(msg: Message) => void> = []
  private refCounter = 0
  private joinRefs: Map<string, string> = new Map()
  private apiKey: string
  private accessToken: string | null = null
  private closed = false

  /**
   * Create a new test client
   * @param apiKey - API key for authentication (defaults to anon key from TEST_CONFIG)
   */
  constructor(apiKey?: string) {
    this.apiKey = apiKey || TEST_CONFIG.SBLITE_ANON_KEY
  }

  /**
   * Set an access token for authenticated connections
   */
  setAccessToken(token: string): void {
    this.accessToken = token
  }

  /**
   * Connect to the WebSocket endpoint
   */
  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      // Build URL with authentication
      let url = `${WS_URL}?apikey=${encodeURIComponent(this.apiKey)}`
      if (this.accessToken) {
        url += `&token=${encodeURIComponent(this.accessToken)}`
      }

      this.ws = new WebSocket(url)
      this.closed = false

      const timeout = setTimeout(() => {
        reject(new Error('Connection timeout'))
      }, 5000)

      this.ws.on('open', () => {
        clearTimeout(timeout)
        resolve()
      })

      this.ws.on('error', (err) => {
        clearTimeout(timeout)
        reject(err)
      })

      this.ws.on('message', (data: Buffer) => {
        const msg = JSON.parse(data.toString()) as Message
        if (this.messageResolvers.length > 0) {
          const resolver = this.messageResolvers.shift()!
          resolver(msg)
        } else {
          this.messageQueue.push(msg)
        }
      })

      this.ws.on('close', () => {
        this.closed = true
      })
    })
  }

  /**
   * Close the WebSocket connection
   */
  async close(): Promise<void> {
    if (this.ws && !this.closed) {
      this.ws.close()
      this.ws = null
    }
    this.messageQueue = []
    this.messageResolvers = []
    this.joinRefs.clear()
  }

  /**
   * Check if the connection is open
   */
  isConnected(): boolean {
    return this.ws !== null && this.ws.readyState === WebSocket.OPEN && !this.closed
  }

  /**
   * Send a message and return the ref
   */
  send(msg: Omit<Message, 'ref'>): string {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket is not connected')
    }
    const ref = String(++this.refCounter)
    const fullMsg: Message = { ...msg, ref }
    this.ws.send(JSON.stringify(fullMsg))
    return ref
  }

  /**
   * Receive the next message with optional timeout
   */
  async receive(timeout = 5000): Promise<Message> {
    if (this.messageQueue.length > 0) {
      return this.messageQueue.shift()!
    }
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        // Remove this resolver from the queue
        const index = this.messageResolvers.indexOf(resolverFn)
        if (index > -1) {
          this.messageResolvers.splice(index, 1)
        }
        reject(new Error(`Timeout waiting for message after ${timeout}ms`))
      }, timeout)

      const resolverFn = (msg: Message) => {
        clearTimeout(timer)
        resolve(msg)
      }

      this.messageResolvers.push(resolverFn)
    })
  }

  /**
   * Try to receive a message, returns null if timeout
   */
  async tryReceive(timeout = 1000): Promise<Message | null> {
    try {
      return await this.receive(timeout)
    } catch {
      return null
    }
  }

  /**
   * Drain all pending messages
   */
  drainMessages(): Message[] {
    const messages = [...this.messageQueue]
    this.messageQueue = []
    return messages
  }

  /**
   * Join a channel
   */
  async join(topic: string, config: SubscriptionConfig = {}): Promise<Message> {
    const joinRef = String(this.refCounter + 1)
    this.joinRefs.set(topic, joinRef)

    this.send({
      event: 'phx_join',
      topic,
      payload: { config },
      join_ref: joinRef,
    })

    return this.receive()
  }

  /**
   * Leave a channel
   */
  async leave(topic: string): Promise<Message> {
    const joinRef = this.joinRefs.get(topic)
    this.send({
      event: 'phx_leave',
      topic,
      payload: {},
      join_ref: joinRef,
    })
    this.joinRefs.delete(topic)
    return this.receive()
  }

  /**
   * Send a heartbeat (Phoenix protocol keepalive)
   */
  async heartbeat(): Promise<Message> {
    this.send({
      event: 'heartbeat',
      topic: 'phoenix',
      payload: {},
    })
    return this.receive()
  }

  /**
   * Broadcast a message to a channel
   */
  async broadcast(topic: string, event: string, payload: Record<string, any>): Promise<void> {
    const joinRef = this.joinRefs.get(topic)
    this.send({
      event: 'broadcast',
      topic,
      payload: { event, payload },
      join_ref: joinRef,
    })
  }

  /**
   * Send presence state (track user)
   */
  async presenceTrack(topic: string, payload: Record<string, any>): Promise<void> {
    const joinRef = this.joinRefs.get(topic)
    this.send({
      event: 'presence',
      topic,
      payload: { type: 'track', payload },
      join_ref: joinRef,
    })
  }

  /**
   * Send presence untrack
   */
  async presenceUntrack(topic: string): Promise<void> {
    const joinRef = this.joinRefs.get(topic)
    this.send({
      event: 'presence',
      topic,
      payload: { type: 'untrack' },
      join_ref: joinRef,
    })
  }

  /**
   * Wait for a specific event type on a topic
   */
  async waitForEvent(
    topic: string,
    event: string,
    timeout = 5000
  ): Promise<Message> {
    const startTime = Date.now()

    while (Date.now() - startTime < timeout) {
      const msg = await this.tryReceive(Math.min(1000, timeout - (Date.now() - startTime)))
      if (msg && msg.topic === topic && msg.event === event) {
        return msg
      }
      // If we got a message but it wasn't what we wanted, continue waiting
    }

    throw new Error(`Timeout waiting for event '${event}' on topic '${topic}'`)
  }

  /**
   * Collect all messages for a duration
   */
  async collectMessages(duration: number): Promise<Message[]> {
    const messages: Message[] = []
    const endTime = Date.now() + duration

    while (Date.now() < endTime) {
      const msg = await this.tryReceive(Math.min(100, endTime - Date.now()))
      if (msg) {
        messages.push(msg)
      }
    }

    return messages
  }
}

/**
 * Create a connected test client
 */
export async function createTestClient(apiKey?: string): Promise<TestClient> {
  const client = new TestClient(apiKey)
  await client.connect()
  return client
}

/**
 * Create a connected test client with an access token
 */
export async function createAuthenticatedTestClient(
  accessToken: string,
  apiKey?: string
): Promise<TestClient> {
  const client = new TestClient(apiKey)
  client.setAccessToken(accessToken)
  await client.connect()
  return client
}

/**
 * Helper to generate unique channel names for testing
 */
export function uniqueChannel(prefix: string = 'test'): string {
  return `realtime:${prefix}_${Date.now()}_${Math.random().toString(36).substring(7)}`
}

/**
 * Helper to build postgres changes topic
 */
export function postgresChangesTopic(schema: string, table: string): string {
  return `realtime:${schema}:${table}`
}
