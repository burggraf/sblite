// hello-world edge function for testing
// https://supabase.com/docs/guides/functions

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
    // Handle different content types
    let body: any = {}
    const contentType = req.headers.get('content-type') || ''

    if (contentType.includes('application/json')) {
      const text = await req.text()
      if (text) {
        body = JSON.parse(text)
      }
    }

    const name = body.name || 'World'

    const data = {
      message: `Hello ${name}!`,
      method: req.method,
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
        status: 400,
        headers: {
          ...corsHeaders,
          'Content-Type': 'application/json',
        },
      },
    )
  }
})
