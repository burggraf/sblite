import { config } from "dotenv"
config({ path: ".env.local" })

import { createClient } from "@supabase/supabase-js"
import { parse } from "csv-parse/sync"

const SUPABASE_URL = process.env.VITE_SUPABASE_URL || "http://localhost:8080"
const SUPABASE_KEY = process.env.VITE_SUPABASE_ANON_KEY || ""
const CSV_URL =
  "https://media.githubusercontent.com/media/burggraf/datasets/refs/heads/main/seinfeld/episode_info.csv"

const supabase = createClient(SUPABASE_URL, SUPABASE_KEY)

interface CsvRow {
  ID: string
  Season: string
  EpisodeNo: string
  Title: string
  AirDate: string
  Writers: string
  Director: string
  SEID: string
}

async function main() {
  console.log("Fetching Seinfeld episode info CSV...")
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

  console.log(`Found ${records.length} episodes`)

  // Transform to database format
  const rows = records.map((r) => ({
    id: parseInt(r.ID, 10),
    season: r.Season ? Math.round(parseFloat(r.Season)) : null,
    episode_no: r.EpisodeNo ? Math.round(parseFloat(r.EpisodeNo)) : null,
    title: r.Title || "",
    air_date: r.AirDate || null,
    writers: r.Writers || null,
    director: r.Director || null,
    seid: r.SEID || null,
  }))

  // Filter out rows with missing required fields
  const validRows = rows.filter((r) => r.seid && r.title && r.season !== null)
  console.log(`${validRows.length} valid records after filtering`)

  // Batch insert in chunks of 50
  const BATCH_SIZE = 50
  let inserted = 0

  for (let i = 0; i < validRows.length; i += BATCH_SIZE) {
    const batch = validRows.slice(i, i + BATCH_SIZE)
    const { error } = await supabase.from("episode_info").insert(batch)

    if (error) {
      console.error(`Error inserting batch at ${i}:`, error.message)
      throw error
    }

    inserted += batch.length
    console.log(`Imported ${inserted}/${validRows.length} episodes`)
  }

  console.log("Import complete!")
}

main().catch((err) => {
  console.error("Import failed:", err)
  process.exit(1)
})
