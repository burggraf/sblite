import { createClient } from '@supabase/supabase-js'

// Use same-origin URL in development (Vite proxy handles cross-origin)
// The proxy is configured in vite.config.js to forward /auth and /rest to localhost:8080
const supabaseUrl = import.meta.env.VITE_SUPABASE_URL || 'http://localhost:8080'

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
    detectSessionInUrl: false
  }
})
