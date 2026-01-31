/**
 * API client for sblite dashboard
 * All requests go through the Vite proxy during development
 * In production, they go directly to the Go backend
 */

const API_BASE = "/_/api"

export interface Table {
  name: string
  description?: string
  columns: Column[]
}

export interface Column {
  name: string
  type: string
  nullable: boolean
  primary_key: boolean
  description?: string
}

export interface User {
  id: string
  email: string
  created_at: string
  last_sign_in_at?: string
  metadata?: Record<string, unknown>
}

class ApiClient {
  private baseUrl: string

  constructor(baseUrl: string = API_BASE) {
    this.baseUrl = baseUrl
  }

  private async request<T>(
    endpoint: string,
    options?: RequestInit
  ): Promise<T> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      headers: {
        "Content-Type": "application/json",
        ...options?.headers,
      },
      ...options,
    })

    if (!response.ok) {
      throw new Error(`API error: ${response.statusText}`)
    }

    return response.json()
  }

  // Tables
  async getTables(): Promise<Table[]> {
    return this.request<Table[]>("/tables")
  }

  async getTable(name: string): Promise<Table> {
    return this.request<Table>(`/tables/${name}`)
  }

  async createTable(data: {
    name: string
    columns: Array<{ name: string; type: string; nullable?: boolean }>
  }): Promise<Table> {
    return this.request("/tables", {
      method: "POST",
      body: JSON.stringify(data),
    })
  }

  async deleteTable(name: string): Promise<void> {
    return this.request(`/tables/${name}`, { method: "DELETE" })
  }

  // Data
  async getData(table: string, params?: {
    limit?: number
    offset?: number
    order?: string
  }): Promise<{ data: unknown[]; total: number }> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set("limit", params.limit.toString())
    if (params?.offset) searchParams.set("offset", params.offset.toString())
    if (params?.order) searchParams.set("order", params.order)

    const query = searchParams.toString()
    return this.request(`/data/${table}${query ? `?${query}` : ""}`)
  }

  // Users
  async getUsers(params?: {
    limit?: number
    offset?: number
    filter?: "all" | "regular" | "anonymous"
  }): Promise<{ users: User[]; total: number }> {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set("limit", params.limit.toString())
    if (params?.offset) searchParams.set("offset", params.offset.toString())
    if (params?.filter) searchParams.set("filter", params.filter)

    const query = searchParams.toString()
    return this.request(`/users${query ? `?${query}` : ""}`)
  }

  // Settings
  async getServerSettings(): Promise<{
    version: string
    host: string
    port: number
    db_path: string
  }> {
    return this.request("/settings/server")
  }
}

export const api = new ApiClient()
