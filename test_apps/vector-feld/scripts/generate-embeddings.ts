import { config } from "dotenv"
config({ path: ".env.local" })

import { createClient } from "@supabase/supabase-js"
import { GoogleGenerativeAI, TaskType } from "@google/generative-ai"

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || "http://localhost:8080"
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || ""
const GOOGLE_API_KEY = process.env.GOOGLE_API_KEY || ""

if (!GOOGLE_API_KEY) {
  console.error("GOOGLE_API_KEY environment variable is required")
  process.exit(1)
}

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)
const genAI = new GoogleGenerativeAI(GOOGLE_API_KEY)

// Batch configuration
// Gemini API limits:
// - 20,000 tokens total per batch request
// - Up to 250 texts per batch request
// - 2,048 tokens max per individual text
// Rate limits (free tier): 100 requests/minute
const MAX_BATCH_SIZE = 100 // Conservative limit (API allows 250)
const MAX_BATCH_TOKENS = 18000 // Leave headroom under 20k limit
const RATE_LIMIT_DELAY = 1100 // 1.1 seconds between batch requests

// Rough token estimation: ~4 chars per token for English text
function estimateTokens(text: string): number {
  return Math.ceil(text.length / 4)
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

interface Row {
  id: number
  dialogue: string
}

async function batchEmbedWithRetry(
  texts: string[],
  maxRetries = 3
): Promise<number[][]> {
  const model = genAI.getGenerativeModel({ model: "text-embedding-004" })

  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      const result = await model.batchEmbedContents({
        requests: texts.map((text) => ({
          content: { parts: [{ text }], role: "user" },
          taskType: TaskType.RETRIEVAL_DOCUMENT,
        })),
      })
      return result.embeddings.map((e) => e.values)
    } catch (error) {
      const err = error as Error & { status?: number }
      if (err.status === 429 && attempt < maxRetries) {
        // Rate limited, exponential backoff
        const waitTime = RATE_LIMIT_DELAY * Math.pow(2, attempt)
        console.log(`Rate limited, waiting ${(waitTime / 1000).toFixed(1)}s...`)
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
    // Fetch more rows than we might need, then dynamically batch by token count
    const { data: allRows, error: fetchError } = await supabase
      .from("scripts")
      .select("id, dialogue")
      .is("embedding", null)
      .order("id")
      .limit(MAX_BATCH_SIZE * 2) // Fetch extra to allow dynamic batching

    if (fetchError) {
      console.error("Fetch error:", fetchError.message)
      throw fetchError
    }

    if (!allRows || allRows.length === 0) {
      break
    }

    // Build batch dynamically based on token count
    const rows: Row[] = []
    let batchTokens = 0

    for (const row of allRows as Row[]) {
      const tokens = estimateTokens(row.dialogue)
      if (rows.length > 0 && (batchTokens + tokens > MAX_BATCH_TOKENS || rows.length >= MAX_BATCH_SIZE)) {
        break // This row would exceed limits, stop here
      }
      rows.push(row)
      batchTokens += tokens
    }

    try {
      // Generate embeddings for entire batch in one API call
      const texts = rows.map((r) => r.dialogue)
      const embeddings = await batchEmbedWithRetry(texts)

      // Update rows sequentially to avoid overwhelming SQLite
      let updateErrors = 0
      for (let i = 0; i < rows.length; i++) {
        const { error } = await supabase
          .from("scripts")
          .update({ embedding: JSON.stringify(embeddings[i]) })
          .eq("id", rows[i].id)

        if (error) {
          updateErrors++
          if (updateErrors <= 3) {
            console.error(`Update error for row ${rows[i].id}:`, error.message)
          }
        }
      }
      if (updateErrors > 0) {
        console.error(`${updateErrors} update errors in batch`)
      }

      processed += rows.length - updateErrors
      const elapsed = (Date.now() - startTime) / 1000
      const rate = processed / elapsed
      const remaining = totalCount - processed
      const eta = remaining / rate

      console.log(
        `Embedded ${processed}/${totalCount} (${Math.round((processed / totalCount) * 100)}%) ` +
          `| batch: ${rows.length} (~${batchTokens} tokens) | ${rate.toFixed(1)}/sec | ETA: ${Math.round(eta / 60)}m ${Math.round(eta % 60)}s`
      )

      // Rate limiting delay between batches
      await sleep(RATE_LIMIT_DELAY)
    } catch (error) {
      console.error("Batch error:", error)
      // On batch failure, wait longer before retry
      await sleep(RATE_LIMIT_DELAY * 5)
    }
  }

  const totalTime = (Date.now() - startTime) / 1000
  const avgRate = processed / totalTime
  console.log(
    `\nComplete! Processed ${processed} rows in ${(totalTime / 60).toFixed(1)} minutes ` +
      `(${avgRate.toFixed(1)} rows/sec)`
  )
}

main().catch((err) => {
  console.error("Embedding generation failed:", err)
  process.exit(1)
})
