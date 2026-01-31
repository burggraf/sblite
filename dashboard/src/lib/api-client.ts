/**
 * API Client
 * Type-safe API client for all dashboard endpoints
 */

const API_BASE = '/_/api'

/**
 * Helper to make API requests with proper error handling
 */
async function request<T>(
  endpoint: string,
  options?: RequestInit
): Promise<T> {
  const url = `${API_BASE}${endpoint}`

  const response = await fetch(url, {
    headers: {
      'Content-Type': 'application/json',
      ...options?.headers,
    },
    ...options,
  })

  if (!response.ok) {
    const error = await response.json().catch(() => ({ error: response.statusText }))
    throw new Error(error.error || error.message || 'API request failed')
  }

  return response.json()
}

/**
 * Helper for POST requests
 */
async function post<T>(endpoint: string, data?: unknown): Promise<T> {
  return request<T>(endpoint, {
    method: 'POST',
    body: JSON.stringify(data),
  })
}

/**
 * Helper for PUT requests
 */
async function put<T>(endpoint: string, data?: unknown): Promise<T> {
  return request<T>(endpoint, {
    method: 'PUT',
    body: JSON.stringify(data),
  })
}

/**
 * Helper for DELETE requests
 */
async function del<T>(endpoint: string): Promise<T> {
  return request<T>(endpoint, {
    method: 'DELETE',
  })
}

/**
 * Helper for PATCH requests
 */
async function patch<T>(endpoint: string, data?: unknown): Promise<T> {
  return request<T>(endpoint, {
    method: 'PATCH',
    body: JSON.stringify(data),
  })
}

// ============================================================================
// Auth API
// ============================================================================

export const authApi = {
  getStatus: () => request<{ authenticated: boolean; needs_setup: boolean }>('/auth/status'),
  setup: (password: string) => post<{ status: string }>('/auth/setup', { password }),
  login: (password: string) => post<{ status: string }>('/auth/login', { password }),
  logout: () => post<void>('/auth/logout'),
}

// ============================================================================
// Tables API
// ============================================================================

export const tablesApi = {
  list: () => request<string[]>('/tables'),

  get: (name: string) => request<{ name: string; description: string; columns: unknown }>(`/tables/${name}`),

  create: (data: {
    name: string
    columns: Array<{ name: string; type: string; nullable?: boolean }>
    description?: string
  }) => post<{ name: string }>('/tables', data),

  delete: (name: string) => del<void>(`/tables/${name}`),

  setDescription: (name: string, description: string) =>
    patch<void>(`/tables/${name}/description`, { description }),

  // Columns
  addColumn: (tableName: string, data: { name: string; type: string; nullable?: boolean }) =>
    post<void>(`/tables/${tableName}/columns`, data),

  renameColumn: (tableName: string, columnName: string, newName: string) =>
    patch<void>(`/tables/${tableName}/columns/${columnName}`, { name: newName }),

  deleteColumn: (tableName: string, columnName: string) =>
    del<void>(`/tables/${tableName}/columns/${columnName}`),

  // FTS
  listFts: (tableName: string) => request<Array<{ name: string; columns: string[] }>>(`/tables/${tableName}/fts`),

  createFts: (tableName: string, data: { name: string; columns: string[] }) =>
    post<void>(`/tables/${tableName}/fts`, data),

  deleteFts: (tableName: string, indexName: string) =>
    del<void>(`/tables/${tableName}/fts/${indexName}`),

  // RLS
  getRls: (tableName: string) => request<{ enabled: boolean }>(`/tables/${tableName}/rls`),

  toggleRls: (tableName: string, enabled: boolean) =>
    put<void>(`/tables/${tableName}/rls`, { enabled }),
}

// ============================================================================
// Data API
// ============================================================================

export const dataApi = {
  list: (table: string, params?: {
    limit?: number
    offset?: number
    order?: string
  }) => {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    if (params?.order) searchParams.set('order', params.order)

    const query = searchParams.toString()
    return request<{ data: unknown[]; total: number }>(`/data/${table}${query ? `?${query}` : ''}`)
  },

  create: (table: string, row: Record<string, unknown>) =>
    post<unknown>(`/data/${table}`, row),

  update: (table: string, row: Record<string, unknown>) =>
    patch<unknown>(`/data/${table}`, row),

  upsert: (table: string, row: Record<string, unknown>) =>
    put<unknown>(`/data/${table}`, row),

  delete: (table: string, filters: Record<string, unknown>) =>
    request<{ deleted: number }>(`/data/${table}`, {
      method: 'DELETE',
      body: JSON.stringify(filters),
    }),
}

// ============================================================================
// Users API
// ============================================================================

export interface User {
  id: string
  email: string | null
  is_anonymous: boolean
  created_at: string | null
  updated_at: string | null
  last_sign_in_at: string | null
  email_confirmed_at: string | null
  user_metadata: Record<string, unknown>
  app_metadata: Record<string, unknown>
}

export interface UsersListResponse {
  users: User[]
  total: number
}

export const usersApi = {
  list: (params?: {
    limit?: number
    offset?: number
    filter?: 'all' | 'regular' | 'anonymous'
  }) => {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    if (params?.filter) searchParams.set('filter', params.filter)

    const query = searchParams.toString()
    return request<UsersListResponse>(`/users${query ? `?${query}` : ''}`)
  },

  get: (id: string) => request<User>(`/users/${id}`),

  create: (data: { email: string; password?: string; email_confirm?: boolean }) =>
    post<{ id: string }>('/users', data),

  invite: (data: { email: string }) => post<void>('/users/invite', data),

  update: (id: string, data: {
    email?: string
    password?: string
    email_confirm?: boolean
    user_metadata?: Record<string, unknown>
    app_metadata?: Record<string, unknown>
  }) => patch<void>(`/users/${id}`, data),

  delete: (id: string) => del<void>(`/users/${id}`),
}

// ============================================================================
// Policies API
// ============================================================================

export const policiesApi = {
  list: () => request<unknown[]>('/policies'),

  get: (id: number) => request<unknown>(`/policies/${id}`),

  create: (data: {
    table: string
    name: string
    using: string
    check: string
  }) => post<{ id: number }>('/policies', data),

  update: (id: number, data: {
    name?: string
    using?: string
    check?: string
    enabled?: boolean
  }) => patch<void>(`/policies/${id}`, data),

  delete: (id: number) => del<void>(`/policies/${id}`),

  test: (data: { policy: string; user_id?: string }) =>
    post<{ affected_rows: number; explanation?: string }>('/policies/test', data),
}

// ============================================================================
// Storage API
// ============================================================================

export const storageApi = {
  // Buckets
  listBuckets: () => request<unknown[]>('/storage/buckets'),

  createBucket: (data: {
    id: string
    name?: string
    public?: boolean
    file_size_limit?: number
    allowed_mime_types?: string[]
  }) => post<void>('/storage/buckets', data),

  getBucket: (id: string) => request<unknown>(`/storage/buckets/${id}`),

  updateBucket: (id: string, data: {
    name?: string
    public?: boolean
    file_size_limit?: number
    allowed_mime_types?: string[]
  }) => put<void>(`/storage/buckets/${id}`, data),

  deleteBucket: (id: string) => del<void>(`/storage/buckets/${id}`),

  emptyBucket: (id: string) => post<void>(`/storage/buckets/${id}/empty`),

  // Objects
  listObjects: (bucket: string, data?: {
    path?: string
    limit?: number
    offset?: number
  }) => post<unknown[]>('/storage/objects/list', { bucket, ...data }),

  upload: (bucket: string, path: string, file: File, onProgress?: (progress: number) => void) => {
    return new Promise((resolve, reject) => {
      const formData = new FormData()
      formData.append('file', file)

      const xhr = new XMLHttpRequest()

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable && onProgress) {
          onProgress((e.loaded / e.total) * 100)
        }
      })

      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          resolve(xhr.response)
        } else {
          reject(new Error(xhr.statusText))
        }
      })

      xhr.addEventListener('error', () => reject(new Error('Upload failed')))
      xhr.addEventListener('abort', () => reject(new Error('Upload cancelled')))

      xhr.open('POST', `${API_BASE}/storage/objects/upload/${bucket}/${path}`)
      xhr.send(formData)
    })
  },

  download: (bucket: string, path: string) => {
    const url = `/storage/v1/object/${bucket}/${path}`
    window.open(url, '_blank')
  },

  deleteObjects: (data: { bucket: string; paths: string[] }) =>
    request<void>('/storage/objects', {
      method: 'DELETE',
      body: JSON.stringify(data),
    }),

  // Policies
  listPolicies: () => request<unknown[]>('/settings/storage/policies'),

  createPolicy: (data: {
    bucket_id: string
    name: string
    using: string
    check: string
  }) => post<void>('/settings/storage/policies', data),

  updatePolicy: (id: number, data: {
    name?: string
    using?: string
    check?: string
    enabled?: boolean
  }) => patch<void>(`/settings/storage/policies/${id}`, data),

  deletePolicy: (id: number) => del<void>(`/settings/storage/policies/${id}`),
}

// ============================================================================
// Functions API
// ============================================================================

export const functionsApi = {
  list: () => request<unknown[]>('/functions'),

  get: (name: string) => request<unknown>(`/functions/${name}`),

  create: (data: { name: string; template?: string }) =>
    post<void>('/functions', data),

  delete: (name: string) => del<void>(`/functions/${name}`),

  // Files
  getFiles: (name: string) => request<unknown[]>(`/functions/${name}/files`),

  saveFile: (name: string, path: string, content: string) =>
    post<void>(`/functions/${name}/files`, { path, content }),

  // Config
  getConfig: (name: string) => request<unknown>(`/functions/${name}/config`),

  updateConfig: (name: string, data: { verify_jwt?: boolean }) =>
    patch<void>(`/functions/${name}/config`, data),

  // Secrets
  listSecrets: () => request<string[]>('/secrets'),

  setSecret: (name: string, value: string) =>
    post<void>('/secrets', { name, value }),

  deleteSecret: (name: string) => del<void>(`/secrets/${name}`),

  // Runtime
  getStatus: () => request<{ installed: boolean; version?: string }>('/functions/status'),

  install: (version?: string) => post<void>('/functions/install', { version }),

  // Test
  test: (name: string, data: {
    method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
    headers?: Record<string, string>
    body?: unknown
  }) => post<unknown>(`/functions/${name}/test`, data),
}

// ============================================================================
// Settings API
// ============================================================================

export const settingsApi = {
  // Server
  getServer: () => request<{
    version: string
    host: string
    port: number
    db_path: string
  }>('/settings/server'),

  // Auth
  getAuth: () => request<{
    allow_anonymous: boolean
    anonymous_user_count: number
    site_url?: string
    email_confirm_required: boolean
  }>('/settings/auth'),

  updateAuth: (data: {
    allow_anonymous?: boolean
    site_url?: string
    email_confirm_required?: boolean
  }) => patch<void>('/settings/auth', data),

  regenerateJwtSecret: () => post<void>('/settings/auth/regenerate'),

  // OAuth
  getOAuth: () => request<{
    providers: unknown[]
    redirect_urls: string[]
  }>('/settings/oauth'),

  updateOAuth: (data: {
    provider: string
    enabled?: boolean
    client_id?: string
    client_secret?: string
  }) => patch<void>('/settings/oauth', data),

  // Redirect URLs
  addRedirectUrl: (url: string) =>
    post<void>('/settings/oauth/redirect-urls', { url }),

  deleteRedirectUrl: (url: string) =>
    request<void>(`/settings/oauth/redirect-urls?url=${encodeURIComponent(url)}`, {
      method: 'DELETE',
    }),

  // Storage
  getStorage: () => request<{
    backend: 'local' | 's3'
    local?: { path: string }
    s3?: unknown
  }>('/settings/storage'),

  updateStorage: (data: {
    backend: 'local' | 's3'
    local?: { path: string }
    s3?: {
      endpoint: string
      region: string
      bucket: string
      access_key: string
      secret_key: string
      path_style?: boolean
    }
  }) => patch<void>('/settings/storage', data),

  testStorage: () => post<void>('/settings/storage/test'),

  // Mail
  getMail: () => request<{
    mode: 'log' | 'catch' | 'smtp'
    smtp?: unknown
  }>('/settings/mail'),

  updateMail: (data: {
    mode: 'log' | 'catch' | 'smtp'
    smtp?: {
      host: string
      port: number
      username: string
      password: string
      from: string
    }
  }) => patch<void>('/settings/mail', data),

  // Email Templates
  getTemplates: () => request<unknown[]>('/settings/templates'),

  getTemplate: (type: string) => request<unknown>(`/settings/templates/${type}`),

  updateTemplate: (type: string, data: { subject: string; body: string }) =>
    patch<void>(`/settings/templates/${type}`, data),

  resetTemplate: (type: string) => post<void>(`/settings/templates/${type}/reset`),

  // API Keys
  getApiKeys: () => request<{
    anon: string
    service_role: string
  }>('/apikeys'),
}

// ============================================================================
// Logs API
// ============================================================================

export const logsApi = {
  query: (params?: {
    limit?: number
    offset?: number
    level?: string
    source?: string
    search?: string
  }) => {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())
    if (params?.level) searchParams.set('level', params.level)
    if (params?.source) searchParams.set('source', params.source)
    if (params?.search) searchParams.set('search', params.search)

    const query = searchParams.toString()
    return request<{ logs: unknown[]; total: number }>(`/logs${query ? `?${query}` : ''}`)
  },

  tail: () => {
    const eventSource = new EventSource(`${API_BASE}/logs/tail`)
    return eventSource
  },

  getBuffer: () => request<unknown[]>('/logs/buffer'),

  getConfig: () => request<{
    mode: string
    file?: string
    db?: string
  }>('/logs/config'),
}

// ============================================================================
// SQL Browser API
// ============================================================================

export const sqlApi = {
  execute: (data: { query: string; pg_mode?: boolean }) =>
    post<{
      columns: string[]
      rows: unknown[][]
      duration_ms: number
      affected_rows?: number
    }>('/sql', data),
}

// ============================================================================
// API Docs API
// ============================================================================

export const apiDocsApi = {
  getTables: () => request<unknown[]>('/apidocs/tables'),

  getTable: (name: string) => request<unknown>(`/apidocs/tables/${name}`),

  updateTableDescription: (name: string, description: string) =>
    patch<void>(`/apidocs/tables/${name}/description`, { description }),

  updateColumnDescription: (name: string, column: string, description: string) =>
    patch<void>(`/apidocs/tables/${name}/columns/${column}/description`, { description }),

  getFunctions: () => request<unknown[]>('/apidocs/functions'),

  getFunction: (name: string) => request<unknown>(`/apidocs/functions/${name}`),

  updateFunctionDescription: (name: string, description: string) =>
    patch<void>(`/apidocs/functions/${name}/description`, { description }),
}

// ============================================================================
// Realtime API
// ============================================================================

export const realtimeApi = {
  getStats: () => request<{
    connections: number
    channels: number
  }>('/settings/realtime'),

  getChannels: () => request<unknown[]>('/settings/realtime/channels'),
}

// ============================================================================
// Observability API
// ============================================================================

export const observabilityApi = {
  getMetrics: () => request<{
    request_rate: number
    error_rate: number
    latency_p50: number
    latency_p95: number
    latency_p99: number
  }>('/settings/observability/metrics'),

  getTraces: (params?: {
    limit?: number
    offset?: number
  }) => {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())

    const query = searchParams.toString()
    return request<{ traces: unknown[]; total: number }>(`/settings/observability/traces${query ? `?${query}` : ''}`)
  },

  getTrace: (id: string) => request<unknown>(`/settings/observability/traces/${id}`),
}

// ============================================================================
// Migration API
// ============================================================================

export const migrationApi = {
  list: () => request<unknown[]>('/migrations'),

  get: (id: string) => request<unknown>(`/migration/${id}`),

  create: () => post<{ id: string }>('/migration'),

  start: (id: string, data: {
    access_token: string
    project_ref?: string
  }) => post<void>(`/migration/${id}/start`, data),

  connect: (id: string, password: string) =>
    post<void>(`/migration/${id}/connect`, { password }),

  getProjects: (id: string) => request<unknown[]>(`/migration/${id}/projects`),

  selectItems: (id: string, items: unknown[]) =>
    post<void>(`/migration/${id}/select`, { items }),

  run: (id: string) => post<void>(`/migration/${id}/run`),

  verify: (id: string) => request<unknown>(`/migration/${id}/verify`),

  delete: (id: string) => del<void>(`/migration/${id}`),
}

// ============================================================================
// Mail Catcher API
// ============================================================================

export const mailApi = {
  getStatus: () => request<{ enabled: boolean }>('/mail/status'),

  list: (params?: { limit?: number; offset?: number }) => {
    const searchParams = new URLSearchParams()
    if (params?.limit) searchParams.set('limit', params.limit.toString())
    if (params?.offset) searchParams.set('offset', params.offset.toString())

    const query = searchParams.toString()
    return request<{ emails: unknown[]; total: number }>(`/mail/emails${query ? `?${query}` : ''}`)
  },

  get: (id: number) => request<unknown>(`/mail/emails/${id}`),

  delete: (id: number) => del<void>(`/mail/emails/${id}`),

  clear: () => del<void>('/mail/emails'),
}

// ============================================================================
// Export API
// ============================================================================

export const exportApi = {
  schema: (format?: 'sql' | 'json') => {
    const url = `/export/schema${format ? `?format=${format}` : ''}`
    window.open(`${API_BASE}${url}`, '_blank')
  },

  data: (table: string, format?: 'sql' | 'json' | 'csv') => {
    const url = `/export/data?table=${table}${format ? `&format=${format}` : ''}`
    window.open(`${API_BASE}${url}`, '_blank')
  },

  backup: () => {
    window.open(`${API_BASE}/export/backup`, '_blank')
  },
}

// ============================================================================
// Convenience exports
// ============================================================================

export const api = {
  auth: authApi,
  tables: tablesApi,
  data: dataApi,
  users: usersApi,
  policies: policiesApi,
  storage: storageApi,
  functions: functionsApi,
  settings: settingsApi,
  logs: logsApi,
  sql: sqlApi,
  apiDocs: apiDocsApi,
  realtime: realtimeApi,
  observability: observabilityApi,
  migration: migrationApi,
  mail: mailApi,
  export: exportApi,
}
