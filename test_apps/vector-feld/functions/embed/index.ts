import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

const GOOGLE_API_KEY = Deno.env.get("GOOGLE_API_KEY")

serve(async (req: Request) => {
  // CORS is handled by sblite, so we just handle the request

  if (req.method === "OPTIONS") {
    return new Response(null, { status: 204 })
  }

  if (req.method !== "POST") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers: { "Content-Type": "application/json" },
    })
  }

  if (!GOOGLE_API_KEY) {
    return new Response(JSON.stringify({ error: "GOOGLE_API_KEY not configured" }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    })
  }

  try {
    const { text } = await req.json()

    if (!text || typeof text !== "string") {
      return new Response(JSON.stringify({ error: "Missing or invalid 'text' field" }), {
        status: 400,
        headers: { "Content-Type": "application/json" },
      })
    }

    // Call Gemini embedding API
    const response = await fetch(
      `https://generativelanguage.googleapis.com/v1beta/models/text-embedding-004:embedContent?key=${GOOGLE_API_KEY}`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          content: { parts: [{ text }] },
          taskType: "RETRIEVAL_QUERY",
        }),
      }
    )

    if (!response.ok) {
      const error = await response.text()
      console.error("Gemini API error:", error)
      return new Response(JSON.stringify({ error: "Embedding API failed" }), {
        status: 502,
        headers: { "Content-Type": "application/json" },
      })
    }

    const data = await response.json()
    const embedding = data.embedding?.values

    if (!embedding) {
      return new Response(JSON.stringify({ error: "No embedding returned" }), {
        status: 502,
        headers: { "Content-Type": "application/json" },
      })
    }

    return new Response(JSON.stringify({ embedding }), {
      headers: { "Content-Type": "application/json" },
    })
  } catch (err) {
    console.error("Error:", err)
    return new Response(JSON.stringify({ error: "Internal server error" }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    })
  }
})
