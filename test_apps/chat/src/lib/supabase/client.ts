import { createClient as createSupabaseClient } from '@supabase/supabase-js'

// Determine Supabase URL based on protocol:
// - HTTPS: use current browser origin (assumes backend is on same domain)
// - HTTP: use .env value (development)
function getSupabaseUrl(): string {
  if (typeof window !== 'undefined' && window.location.protocol === 'https:') {
    return window.location.origin
  }
  return import.meta.env.VITE_SUPABASE_URL!
}

export const supabase = createSupabaseClient(
  getSupabaseUrl(),
  import.meta.env.VITE_SUPABASE_PUBLISHABLE_OR_ANON_KEY!
)
