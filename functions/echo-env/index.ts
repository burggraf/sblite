// echo-env edge function for testing environment variable injection
// Returns information about the environment (safe values only)

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

const corsHeaders = {
  'Access-Control-Allow-Origin': '*',
  'Access-Control-Allow-Headers': 'authorization, x-client-info, apikey, content-type',
}

serve(async (req: Request) => {
  // Handle CORS preflight requests
  if (req.method === 'OPTIONS') {
    return new Response('ok', { headers: corsHeaders })
  }

  try {
    // Return safe environment information
    const data = {
      supabase_url: Deno.env.get('SUPABASE_URL') || null,
      has_anon_key: !!Deno.env.get('SUPABASE_ANON_KEY'),
      has_service_key: !!Deno.env.get('SUPABASE_SERVICE_ROLE_KEY'),
      // Echo back authorization header presence
      has_auth_header: !!req.headers.get('Authorization'),
      timestamp: new Date().toISOString(),
    }

    return new Response(
      JSON.stringify(data),
      {
        headers: {
          ...corsHeaders,
          'Content-Type': 'application/json',
        },
      },
    )
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      {
        status: 500,
        headers: {
          ...corsHeaders,
          'Content-Type': 'application/json',
        },
      },
    )
  }
})
