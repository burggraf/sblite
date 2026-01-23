import { config } from "dotenv"
config({ path: ".env.local" })

import { createClient } from "@supabase/supabase-js"
import { GoogleGenerativeAI } from "@google/generative-ai"

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || "http://localhost:8080"
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || ""
const GOOGLE_API_KEY = process.env.GOOGLE_API_KEY || ""

if (!GOOGLE_API_KEY) {
  console.error("GOOGLE_API_KEY environment variable is required")
  process.exit(1)
}

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)
const genAI = new GoogleGenerativeAI(GOOGLE_API_KEY)

// Rate limiting: Gemini free tier is 15 requests/minute
const RATE_LIMIT_DELAY = 4500 // 4.5 seconds between requests (safe margin)

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

async function embedWithRetry(
  text: string,
  maxRetries = 3
): Promise<number[]> {
  const model = genAI.getGenerativeModel({ model: "text-embedding-004" })

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      const result = await model.embedContent(text)
      return result.embedding.values
    } catch (error) {
      const err = error as Error & { status?: number }
      if (err.status === 429 && attempt < maxRetries) {
        // Rate limited, wait longer
        const waitTime = RATE_LIMIT_DELAY * attempt * 2
        console.log(`Rate limited, waiting ${waitTime / 1000}s...`)
        await sleep(waitTime)
        continue
      }
      throw error
    }
  }

  throw new Error("Max retries exceeded")
}

async function main() {
  // Get count of rows needing embeddings
  const { count: totalCount } = await supabase
    .from("scripts")
    .select("*", { count: "exact", head: true })
    .is("embedding", null)

  if (!totalCount) {
    console.log("All rows already have embeddings!")
    return
  }

  console.log(`${totalCount} rows need embeddings`)

  let processed = 0
  const startTime = Date.now()

  while (true) {
    // Fetch next batch of rows without embeddings
    const { data: rows, error: fetchError } = await supabase
      .from("scripts")
      .select("id, dialogue")
      .is("embedding", null)
      .order("id")
      .limit(10)

    if (fetchError) {
      console.error("Fetch error:", fetchError.message)
      throw fetchError
    }

    if (!rows || rows.length === 0) {
      break
    }

    for (const row of rows) {
      try {
        // Generate embedding
        const embedding = await embedWithRetry(row.dialogue)

        // Update row with embedding
        const { error: updateError } = await supabase
          .from("scripts")
          .update({ embedding: JSON.stringify(embedding) })
          .eq("id", row.id)

        if (updateError) {
          console.error(`Update error for row ${row.id}:`, updateError.message)
          continue
        }

        processed++
        const elapsed = (Date.now() - startTime) / 1000
        const rate = processed / elapsed
        const remaining = totalCount - processed
        const eta = remaining / rate

        console.log(
          `Embedded ${processed}/${totalCount} (${Math.round((processed / totalCount) * 100)}%) ` +
            `- ETA: ${Math.round(eta / 60)}m`
        )

        // Rate limiting delay
        await sleep(RATE_LIMIT_DELAY)
      } catch (error) {
        console.error(`Error processing row ${row.id}:`, error)
        // Continue with next row
      }
    }
  }

  const totalTime = (Date.now() - startTime) / 1000 / 60
  console.log(`\nComplete! Processed ${processed} rows in ${totalTime.toFixed(1)} minutes`)
}

main().catch((err) => {
  console.error("Embedding generation failed:", err)
  process.exit(1)
})
