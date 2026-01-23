import { config } from "dotenv"
config({ path: ".env.local" })

import { createClient } from "@supabase/supabase-js"
import { parse } from "csv-parse/sync"

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || "http://localhost:8080"
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || ""
const CSV_URL =
  "https://media.githubusercontent.com/media/burggraf/datasets/refs/heads/main/seinfeld/scripts.csv"

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)

interface CsvRow {
  ID: string
  Character: string
  Dialogue: string
  EpisodeNo: string
  SEID: string
  Season: string
}

async function main() {
  console.log("Fetching Seinfeld scripts CSV...")
  const response = await fetch(CSV_URL)
  if (!response.ok) {
    throw new Error(`Failed to fetch CSV: ${response.statusText}`)
  }
  const csvText = await response.text()

  console.log("Parsing CSV...")
  const records: CsvRow[] = parse(csvText, {
    columns: true,
    skip_empty_lines: true,
    trim: true,
  })

  console.log(`Found ${records.length} dialogue lines`)

  // Filter out empty dialogue
  const validRecords = records.filter((r) => r.Dialogue && r.Dialogue.trim())
  console.log(`${validRecords.length} valid records after filtering`)

  // Transform to database format
  const rows = validRecords.map((r) => ({
    id: parseInt(r.ID, 10),
    character: r.Character,
    dialogue: r.Dialogue,
    episode_no: r.EpisodeNo ? Math.round(parseFloat(r.EpisodeNo)) : null,
    seid: r.SEID || null,
    season: r.Season ? Math.round(parseFloat(r.Season)) : null,
  }))

  // Batch insert in chunks of 100
  const BATCH_SIZE = 100
  let inserted = 0

  for (let i = 0; i < rows.length; i += BATCH_SIZE) {
    const batch = rows.slice(i, i + BATCH_SIZE)
    const { error } = await supabase.from("scripts").insert(batch)

    if (error) {
      console.error(`Error inserting batch at ${i}:`, error.message)
      throw error
    }

    inserted += batch.length
    console.log(`Imported ${inserted}/${rows.length} rows`)
  }

  console.log("Import complete!")
}

main().catch((err) => {
  console.error("Import failed:", err)
  process.exit(1)
})
