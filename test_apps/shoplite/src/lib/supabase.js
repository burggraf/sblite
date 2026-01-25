import { createClient } from '@supabase/supabase-js'

// Determine Supabase URL based on protocol:
// - HTTPS: use current browser origin (assumes backend is on same domain)
// - HTTP: use .env value or localhost fallback (development)
function getSupabaseUrl() {
  if (typeof window !== 'undefined' && window.location.protocol === 'https:') {
    return window.location.origin
  }
  return import.meta.env.VITE_SUPABASE_URL || 'http://localhost:8080'
}

const supabaseUrl = getSupabaseUrl()

// Use the test anon key (signed with 'super-secret-jwt-key-please-change-in-production')
const supabaseAnonKey = import.meta.env.VITE_SUPABASE_ANON_KEY || 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJyb2xlIjoiYW5vbiIsImlzcyI6InNibGl0ZSIsImlhdCI6MTc2ODcxMDcyNX0.bihutl_eCd_6-IsVU1CgIPROlgQsM2KKYz69E149ZzQ'

// Debug logging (only in development)
if (import.meta.env.DEV) {
  console.log('Supabase URL:', supabaseUrl)
  console.log('Supabase Key:', supabaseAnonKey.substring(0, 50) + '...')
}

export const supabase = createClient(supabaseUrl, supabaseAnonKey, {
  auth: {
    autoRefreshToken: true,
    persistSession: true,
    detectSessionInUrl: true  // Enable for magic link and password reset redirects
  }
})
