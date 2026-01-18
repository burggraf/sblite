package functions

import (
	"os"
	"path/filepath"
)

// mainServiceTemplate is the edge-runtime main router service.
// This routes incoming requests to the appropriate user function workers.
const mainServiceTemplate = `// sblite edge functions main router
// This file is auto-generated and should not be edited manually.

const FUNCTIONS_PATH = Deno.env.get("SBLITE_FUNCTIONS_PATH") || "/functions";

Deno.serve(async (req: Request) => {
  const url = new URL(req.url);
  const pathname = url.pathname;

  // Health check endpoint
  if (pathname === "/health" || pathname === "/_internal/health") {
    return new Response(JSON.stringify({ status: "ok" }), {
      headers: { "Content-Type": "application/json" },
    });
  }

  // Extract function name from path
  // Expected format: /{function-name} or /{function-name}/...
  const pathParts = pathname.split("/").filter(Boolean);
  if (pathParts.length === 0) {
    return new Response(
      JSON.stringify({ error: "FunctionNotFound", message: "No function specified" }),
      { status: 404, headers: { "Content-Type": "application/json" } }
    );
  }

  const functionName = pathParts[0];

  // Skip reserved paths
  if (functionName.startsWith("_") || functionName.startsWith(".")) {
    return new Response(
      JSON.stringify({ error: "FunctionNotFound", message: "Function not found" }),
      { status: 404, headers: { "Content-Type": "application/json" } }
    );
  }

  // Build the service path
  const servicePath = ` + "`${FUNCTIONS_PATH}/${functionName}`" + `;

  try {
    // Check if the function exists by looking for index.ts or index.js
    let entrypoint = "";
    try {
      await Deno.stat(` + "`${servicePath}/index.ts`" + `);
      entrypoint = "index.ts";
    } catch {
      try {
        await Deno.stat(` + "`${servicePath}/index.js`" + `);
        entrypoint = "index.js";
      } catch {
        return new Response(
          JSON.stringify({ error: "FunctionNotFound", message: ` + "`Function '${functionName}' not found`" + ` }),
          { status: 404, headers: { "Content-Type": "application/json" } }
        );
      }
    }

    // Create or reuse worker for this function
    const worker = await EdgeRuntime.userWorkers.create({
      servicePath,
      memoryLimitMb: 150,
      workerTimeoutMs: 5 * 60 * 1000, // 5 minutes
      noModuleCache: false,
      importMapPath: null,
      envVars: Object.entries(Deno.env.toObject()),
      forceCreate: false,
      netAccessDisabled: false,
      cpuTimeSoftLimitMs: 50000,
      cpuTimeHardLimitMs: 100000,
    });

    // Forward to worker with the original request
    // The worker will handle the request based on pathname
    const response = await worker.fetch(req);
    return response;
  } catch (error) {
    console.error(` + "`Error invoking function '${functionName}':`, error);" + `

    // Handle specific error types
    if (error.message?.includes("worker boot")) {
      return new Response(
        JSON.stringify({
          error: "FunctionBootError",
          message: ` + "`Function '${functionName}' failed to start: ${error.message}`" + `
        }),
        { status: 500, headers: { "Content-Type": "application/json" } }
      );
    }

    return new Response(
      JSON.stringify({
        error: "FunctionInvocationError",
        message: error.message || "Failed to invoke function"
      }),
      { status: 500, headers: { "Content-Type": "application/json" } }
    );
  }
});
`

// ensureMainService ensures the main router service exists in the functions directory.
func ensureMainService(functionsDir string) (string, error) {
	mainDir := filepath.Join(functionsDir, "_main")
	mainFile := filepath.Join(mainDir, "index.ts")

	// Create _main directory if it doesn't exist
	if err := os.MkdirAll(mainDir, 0755); err != nil {
		return "", err
	}

	// Write or update the main service file
	if err := os.WriteFile(mainFile, []byte(mainServiceTemplate), 0644); err != nil {
		return "", err
	}

	return mainDir, nil
}
