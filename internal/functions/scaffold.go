package functions

import (
	"fmt"
	"strings"
)

// defaultFunctionTemplate returns the default function template.
func defaultFunctionTemplate(name string) string {
	return fmt.Sprintf(`// %s edge function
// https://supabase.com/docs/guides/functions

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"

serve(async (req: Request) => {
  try {
    const { name } = await req.json()

    const data = {
      message: %s,
    }

    return new Response(
      JSON.stringify(data),
      { headers: { "Content-Type": "application/json" } },
    )
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      {
        status: 400,
        headers: { "Content-Type": "application/json" }
      },
    )
  }
})
`, name, "`Hello ${name}!`")
}

// supabaseClientTemplate returns a template that uses the Supabase client.
func supabaseClientTemplate(name string) string {
	return fmt.Sprintf(`// %s edge function with Supabase client
// https://supabase.com/docs/guides/functions

import { serve } from "https://deno.land/std@0.168.0/http/server.ts"
import { createClient } from "https://esm.sh/@supabase/supabase-js@2"

serve(async (req: Request) => {
  try {
    // Create Supabase client using auto-injected env vars
    const authHeader = req.headers.get('Authorization')
    const supabase = createClient(
      Deno.env.get('SUPABASE_URL') ?? '',
      Deno.env.get('SUPABASE_ANON_KEY') ?? '',
      {
        global: {
          headers: authHeader ? { Authorization: authHeader } : {},
        },
      }
    )

    // Example: Query data from a table
    // const { data, error } = await supabase.from('your_table').select()

    // Example: Get authenticated user (requires valid JWT in Authorization header)
    // const { data: { user }, error: authError } = await supabase.auth.getUser()

    const data = {
      message: %s,
    }

    return new Response(
      JSON.stringify(data),
      { headers: { "Content-Type": "application/json" } },
    )
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      {
        status: 500,
        headers: { "Content-Type": "application/json" }
      },
    )
  }
})
`, name, fmt.Sprintf("`Hello from %s!`", name))
}

// corsTemplate returns a template with CORS handling.
func corsTemplate(name string) string {
	return fmt.Sprintf(`// %s edge function with CORS support
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
    const { name } = await req.json()

    const data = {
      message: %s,
    }

    return new Response(
      JSON.stringify(data),
      {
        headers: {
          ...corsHeaders,
          "Content-Type": "application/json"
        }
      },
    )
  } catch (error) {
    return new Response(
      JSON.stringify({ error: error.message }),
      {
        status: 400,
        headers: {
          ...corsHeaders,
          "Content-Type": "application/json"
        }
      },
    )
  }
})
`, name, "`Hello ${name}!`")
}

// TemplateType represents the type of function template.
type TemplateType string

const (
	TemplateDefault   TemplateType = "default"
	TemplateSupabase  TemplateType = "supabase"
	TemplateCORS      TemplateType = "cors"
)

// GetTemplate returns the template content for the given type.
func GetTemplate(templateType TemplateType, name string) string {
	switch templateType {
	case TemplateSupabase:
		return supabaseClientTemplate(name)
	case TemplateCORS:
		return corsTemplate(name)
	default:
		return defaultFunctionTemplate(name)
	}
}

// AvailableTemplates returns a list of available template types.
func AvailableTemplates() []string {
	return []string{
		string(TemplateDefault),
		string(TemplateSupabase),
		string(TemplateCORS),
	}
}

// ValidateFunctionName validates a function name.
func ValidateFunctionName(name string) error {
	if name == "" {
		return fmt.Errorf("function name cannot be empty")
	}
	if len(name) > 64 {
		return fmt.Errorf("function name too long (max 64 characters)")
	}
	if name[0] == '.' || name[0] == '_' {
		return fmt.Errorf("function name cannot start with . or _")
	}
	if strings.ContainsAny(name, "/\\:*?\"<>|") {
		return fmt.Errorf("function name contains invalid characters")
	}
	// Reserved names
	reserved := []string{"_shared", "node_modules", "dist", "build"}
	for _, r := range reserved {
		if strings.EqualFold(name, r) {
			return fmt.Errorf("function name %q is reserved", name)
		}
	}
	return nil
}
