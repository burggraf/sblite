/**
 * API Response Types
 * Types for all API endpoints used by the dashboard
 */

// ============================================================================
// Common Types
// ============================================================================

export interface ApiError {
  error: string
  message?: string
  details?: unknown
}

export interface PaginationParams {
  limit?: number
  offset?: number
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

// ============================================================================
// Auth Types
// ============================================================================

export interface AuthStatus {
  authenticated: boolean
  needs_setup: boolean
}

export interface LoginRequest {
  password: string
}

export interface SetupRequest {
  password: string
}

export interface LoginResponse {
  status: string
}

// ============================================================================
// Tables & Data Types
// ============================================================================

export interface Column {
  name: string
  type: string
  nullable: boolean
  primary_key: boolean
  default_value?: string
  description?: string
}

export interface Table {
  name: string
  description?: string
  columns: Column[]
}

export interface CreateTableRequest {
  name: string
  columns: Array<{
    name: string
    type: string
    nullable?: boolean
    primary_key?: boolean
    default_value?: string
  }>
  description?: string
}

export interface AddColumnRequest {
  name: string
  type: string
  nullable?: boolean
  default_value?: string
}

export interface Row {
  [key: string]: unknown
}

export interface DataTableParams extends PaginationParams {
  order?: string
}

export interface Filter {
  column: string
  operator: 'eq' | 'neq' | 'gt' | 'gte' | 'lt' | 'lte' | 'like' | 'ilike' | 'is' | 'in'
  value: unknown
}

export interface Sort {
  column: string
  direction: 'asc' | 'desc'
}

// ============================================================================
// User Types
// ============================================================================

export interface User {
  id: string
  email: string
  created_at: string
  updated_at: string
  last_sign_in_at?: string
  email_confirmed_at?: string
  is_anonymous: boolean
  metadata?: Record<string, unknown>
}

export interface CreateUserRequest {
  email: string
  password?: string
  email_confirm?: boolean
  metadata?: Record<string, unknown>
}

export interface UpdateUserRequest {
  email?: string
  password?: string
  email_confirm?: boolean
  metadata?: Record<string, unknown>
  app_metadata?: Record<string, unknown>
}

export interface InviteUserRequest {
  email: string
}

export interface UsersFilter extends PaginationParams {
  filter?: 'all' | 'regular' | 'anonymous'
}

// ============================================================================
// RLS Policy Types
// ============================================================================

export interface Policy {
  id: number
  table: string
  name: string
  using: string
  check: string
  enabled: boolean
  created_at: string
}

export interface CreatePolicyRequest {
  table: string
  name: string
  using: string
  check: string
  enabled?: boolean
}

export interface UpdatePolicyRequest {
  name?: string
  using?: string
  check?: string
  enabled?: boolean
}

export interface TableRLSInfo {
  table: string
  rls_enabled: boolean
  policy_count: number
}

export interface PolicyTestRequest {
  policy: string
  user_id?: string
}

export interface PolicyTestResult {
  affected_rows: number
  explanation?: string
}

// ============================================================================
// Storage Types
// ============================================================================

export interface Bucket {
  id: string
  name: string
  public: boolean
  file_size_limit: number | null
  allowed_mime_types: string[] | null
  created_at: string
  updated_at: string
}

export interface CreateBucketRequest {
  id: string
  name?: string
  public?: boolean
  file_size_limit?: number | null
  allowed_mime_types?: string[] | null
}

export interface UpdateBucketRequest {
  name?: string
  public?: boolean
  file_size_limit?: number | null
  allowed_mime_types?: string[] | null
}

export interface StorageObject {
  name: string
  size: number
  created_at: string
  updated_at: string
  last_accessed_at?: string
  metadata?: Record<string, unknown>
}

export interface StoragePolicy {
  id: number
  bucket_id: string
  name: string
  using: string
  check: string
  enabled: boolean
}

// ============================================================================
// Edge Functions Types
// ============================================================================

export interface EdgeFunction {
  name: string
  status: 'active' | 'error' | 'loading' | 'unknown'
  created_at: string
  updated_at: string
}

export interface FunctionFile {
  name: string
  content: string
}

export interface CreateFunctionRequest {
  name: string
  template?: string
}

export interface FunctionConfig {
  verify_jwt: boolean
}

export interface SecretInfo {
  name: string
  // Value is not returned by the API
}

export interface SetSecretRequest {
  value: string
}

export interface FunctionTestRequest {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  headers?: Record<string, string>
  body?: unknown
}

export interface FunctionTestResponse {
  status: number
  headers: Record<string, string>
  body: unknown
  duration_ms: number
}

export interface RuntimeStatus {
  installed: boolean
  version?: string
  error?: string
}

// ============================================================================
// Settings Types
// ============================================================================

export interface ServerSettings {
  version: string
  host: string
  port: number
  db_path: string
  log_mode: string
  log_file?: string
  log_db?: string
}

export interface AuthSettings {
  allow_anonymous: boolean
  anonymous_user_count: number
  site_url?: string
  email_confirm_required: boolean
  jwt_secret?: string
}

export interface OAuthProvider {
  provider: 'google' | 'github'
  enabled: boolean
  client_id?: string
  client_secret?: string
  redirect_urls?: string[]
}

export interface OAuthSettings {
  providers: OAuthProvider[]
  redirect_urls: string[]
}

export interface StorageSettings {
  backend: 'local' | 's3'
  local?: {
    path: string
  }
  s3?: {
    endpoint: string
    region: string
    bucket: string
    access_key: string
    secret_key: string
    path_style: boolean
  }
}

export interface MailSettings {
  mode: 'log' | 'catch' | 'smtp'
  smtp?: {
    host: string
    port: number
    username: string
    password: string
    from: string
  }
}

export interface EmailTemplate {
  type: 'signup' | 'invite' | 'email_change' | 'password_reset' | 'magic_link'
  subject: string
  body: string
}

// ============================================================================
// Logs Types
// ============================================================================

export interface LogLevel {
  level: 'debug' | 'info' | 'warn' | 'error'
}

export interface LogQuery extends PaginationParams {
  level?: LogLevel['level']
  source?: string
  request_id?: string
  user_id?: string
  search?: string
  start?: string
  end?: string
}

export interface LogEntry {
  id: number
  timestamp: string
  level: string
  message: string
  source?: string
  request_id?: string
  user_id?: string
  extra?: Record<string, unknown>
}

// ============================================================================
// API Console Types
// ============================================================================

export interface ApiConsoleRequest {
  method: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  url: string
  headers: Record<string, string>
  body?: unknown
  use_anon_key: boolean
}

export interface ApiConsoleHistoryItem extends ApiConsoleRequest {
  id: string
  timestamp: string
  response?: {
    status: number
    headers: Record<string, string>
    body: unknown
  }
}

// ============================================================================
// SQL Browser Types
// ============================================================================

export interface SqlQueryRequest {
  query: string
  pg_mode?: boolean
}

export interface SqlQueryResult {
  columns: string[]
  rows: unknown[][]
  duration_ms: number
  affected_rows?: number
}

export interface SqlHistoryItem {
  id: string
  query: string
  timestamp: string
  pg_mode: boolean
}

// ============================================================================
// API Docs Types
// ============================================================================

export interface ApiDocsTable {
  name: string
  description?: string
  columns: Array<{
    name: string
    type: string
    nullable: boolean
    primary_key: boolean
    description?: string
  }>
}

export interface ApiDocsFunction {
  name: string
  description?: string
  parameters: Array<{
    name: string
    type: string
    required: boolean
    description?: string
  }>
  returns: string
}

// ============================================================================
// Realtime Types
// ============================================================================

export interface RealtimeStats {
  connections: number
  channels: number
}

export interface RealtimeChannel {
  name: string
  connections: number
}

// ============================================================================
// Observability Types
// ============================================================================

export interface ObservabilityMetrics {
  request_rate: number
  error_rate: number
  latency_p50: number
  latency_p95: number
  latency_p99: number
}

export interface Trace {
  id: string
  timestamp: string
  duration_ms: number
  method: string
  path: string
  status: number
  spans: TraceSpan[]
}

export interface TraceSpan {
  name: string
  duration_ms: number
}

// ============================================================================
// Migration Types
// ============================================================================

export interface MigrationProject {
  id: string
  name: string
  organization: string
  region: string
}

export interface MigrationItem {
  type: 'table' | 'view' | 'function' | 'trigger'
  name: string
  size: number
}

export interface MigrationStep {
  step: 'connect' | 'select' | 'review' | 'migrate' | 'verify'
  status: 'pending' | 'in_progress' | 'completed' | 'failed'
  data?: unknown
}

export interface MigrationVerifyResults {
  basic_passed: boolean
  integrity_passed: boolean
  functional_passed: boolean
  details: {
    tables?: number
    rows?: number
    functions?: number
    issues?: string[]
  }
}

// ============================================================================
// Mail Catcher Types
// ============================================================================

export interface MailStatus {
  enabled: boolean
}

export interface Email {
  id: number
  to: string
  subject: string
  received_at: string
  html_body: string
  text_body: string
}

// ============================================================================
// Export Types
// ============================================================================

export interface SchemaExportOptions {
  format?: 'sql' | 'json'
}

export interface DataExportOptions {
  table: string
  format?: 'sql' | 'json' | 'csv'
}
