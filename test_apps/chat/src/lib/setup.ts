import { supabase } from './supabase/client'

const SETUP_KEY = 'realtime-demo-setup-complete'

export async function ensureTodosTable(): Promise<{ success: boolean; error?: string }> {
  // Check if we've already set up successfully in this session
  if (sessionStorage.getItem(SETUP_KEY)) {
    return { success: true }
  }

  try {
    // Try to select from todos table to check if it exists
    const { error: selectError } = await supabase
      .from('todos')
      .select('id')
      .limit(1)

    if (!selectError) {
      // Table exists
      sessionStorage.setItem(SETUP_KEY, 'true')
      return { success: true }
    }

    // Table doesn't exist - create it via admin API
    // We need the service role key for this
    const serviceKey = import.meta.env.VITE_SUPABASE_SERVICE_KEY

    if (!serviceKey) {
      return {
        success: false,
        error: 'VITE_SUPABASE_SERVICE_KEY not set. Please create the todos table manually or add the service key to .env.local'
      }
    }

    const response = await fetch(`${import.meta.env.VITE_SUPABASE_URL}/admin/v1/tables`, {
      method: 'POST',
      headers: {
        'Authorization': `Bearer ${serviceKey}`,
        'Content-Type': 'application/json'
      },
      body: JSON.stringify({
        name: 'todos',
        columns: [
          { name: 'id', type: 'uuid', primary: true, default: 'gen_random_uuid()' },
          { name: 'title', type: 'text', nullable: false },
          { name: 'completed', type: 'integer', default: '0' },
          { name: 'author', type: 'text', nullable: false },
          { name: 'created_at', type: 'timestamptz', nullable: false, default: 'now()' }
        ]
      })
    })

    if (!response.ok) {
      const text = await response.text()
      return {
        success: false,
        error: `Failed to create todos table: ${text}`
      }
    }

    sessionStorage.setItem(SETUP_KEY, 'true')
    return { success: true }
  } catch (err) {
    return {
      success: false,
      error: `Setup error: ${err instanceof Error ? err.message : String(err)}`
    }
  }
}
