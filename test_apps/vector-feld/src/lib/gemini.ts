import { supabase } from "./supabase"

export async function embedText(text: string): Promise<number[]> {
  const { data, error } = await supabase.functions.invoke("embed", {
    body: { text },
  })

  if (error) {
    throw new Error(`Embedding failed: ${error.message}`)
  }

  if (!data?.embedding) {
    throw new Error("No embedding returned from function")
  }

  return data.embedding
}
