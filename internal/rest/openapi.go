// internal/rest/openapi.go
package rest

import (
	"database/sql"
	"fmt"
	"strings"
)

// GenerateOpenAPISpec generates an OpenAPI 3.0 specification from the database schema.
// It introspects SQLite tables and generates schemas and paths for each user table.
// Internal tables (auth_%, sqlite_%, and tables starting with _) are excluded.
func GenerateOpenAPISpec(db *sql.DB) (*OpenAPISpec, error) {
	spec := &OpenAPISpec{
		OpenAPI: "3.0.0",
		Info: OpenAPIInfo{
			Title:       "sblite REST API",
			Description: "Auto-generated REST API for SQLite database tables",
			Version:     "1.0.0",
		},
		Paths: make(map[string]PathItem),
		Components: OpenAPIComponents{
			Schemas: make(map[string]Schema),
			SecuritySchemes: map[string]SecurityScheme{
				"bearerAuth": {
					Type:         "http",
					Scheme:       "bearer",
					BearerFormat: "JWT",
					Description:  "JWT Authorization header using the Bearer scheme",
				},
				"apiKey": {
					Type:        "apiKey",
					Name:        "apikey",
					In:          "header",
					Description: "API key passed in the apikey header",
				},
			},
		},
	}

	// Get all user tables (exclude internal tables)
	tables, err := getUserTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %w", err)
	}

	for _, tableName := range tables {
		// Generate schema for table
		schema, required, err := generateTableSchema(db, tableName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for table %s: %w", tableName, err)
		}
		schema.Required = required
		spec.Components.Schemas[tableName] = schema

		// Generate paths for table
		spec.Paths["/rest/v1/"+tableName] = generateTablePaths(tableName)
	}

	return spec, nil
}

// getUserTables returns a list of user tables, excluding internal tables.
// Excluded tables:
// - sqlite_% (SQLite system tables)
// - auth_% (sblite authentication tables)
// - _% (internal tables by convention)
func getUserTables(db *sql.DB) ([]string, error) {
	// Note: In SQL LIKE, _ is a single-character wildcard, so we must escape it
	// using ESCAPE clause when we want to match a literal underscore at the start.
	query := `
		SELECT name FROM sqlite_master
		WHERE type='table'
		AND name NOT LIKE 'sqlite\_%' ESCAPE '\'
		AND name NOT LIKE 'auth\_%' ESCAPE '\'
		AND name NOT LIKE '\_%' ESCAPE '\'
		ORDER BY name
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, rows.Err()
}

// generateTableSchema generates an OpenAPI Schema for a table by introspecting its columns.
func generateTableSchema(db *sql.DB, table string) (Schema, []string, error) {
	// Use PRAGMA table_info to get column information
	// Escape single quotes to prevent SQL injection via table names
	escapedTable := strings.ReplaceAll(table, "'", "''")
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", escapedTable))
	if err != nil {
		return Schema{}, nil, err
	}
	defer rows.Close()

	schema := Schema{
		Type:       "object",
		Properties: make(map[string]Schema),
	}
	var required []string

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt any
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return Schema{}, nil, err
		}

		propSchema := sqliteTypeToSchema(colType)

		// If column can be null, mark as nullable
		if notNull == 0 && pk == 0 {
			propSchema.Nullable = true
		}

		// If column is NOT NULL, add to required (unless it has a default or is PK)
		if notNull == 1 && dflt == nil && pk == 0 {
			required = append(required, name)
		}

		schema.Properties[name] = propSchema
	}

	return schema, required, rows.Err()
}

// sqliteTypeToSchema converts SQLite column types to OpenAPI Schema types.
func sqliteTypeToSchema(sqliteType string) Schema {
	// SQLite type affinity is flexible, so we normalize common patterns
	upperType := strings.ToUpper(sqliteType)

	// Handle INTEGER types
	if strings.Contains(upperType, "INT") {
		return Schema{Type: "integer"}
	}

	// Handle REAL/FLOAT/DOUBLE types
	if strings.Contains(upperType, "REAL") ||
		strings.Contains(upperType, "FLOAT") ||
		strings.Contains(upperType, "DOUBLE") {
		return Schema{Type: "number", Format: "double"}
	}

	// Handle BLOB type
	if strings.Contains(upperType, "BLOB") {
		return Schema{Type: "string", Format: "binary"}
	}

	// Handle BOOLEAN (SQLite stores as INTEGER but semantically boolean)
	if strings.Contains(upperType, "BOOL") {
		return Schema{Type: "boolean"}
	}

	// Handle DATE/TIME types
	// Check more specific types first (DATETIME/TIMESTAMP before DATE/TIME)
	if strings.Contains(upperType, "TIMESTAMP") || strings.Contains(upperType, "DATETIME") {
		return Schema{Type: "string", Format: "date-time"}
	}
	if strings.Contains(upperType, "DATE") {
		return Schema{Type: "string", Format: "date"}
	}
	if strings.Contains(upperType, "TIME") {
		return Schema{Type: "string", Format: "time"}
	}

	// Handle UUID type (custom, stored as TEXT)
	if strings.Contains(upperType, "UUID") {
		return Schema{Type: "string", Format: "uuid"}
	}

	// Handle JSON type (stored as TEXT in SQLite)
	if strings.Contains(upperType, "JSON") {
		return Schema{Type: "object"}
	}

	// Default to string for TEXT, VARCHAR, CHAR, and unknown types
	return Schema{Type: "string"}
}

// generateTablePaths generates the PathItem with all CRUD operations for a table.
func generateTablePaths(tableName string) PathItem {
	return PathItem{
		Get:    generateGetOperation(tableName),
		Post:   generatePostOperation(tableName),
		Patch:  generatePatchOperation(tableName),
		Delete: generateDeleteOperation(tableName),
	}
}

// generateGetOperation generates the GET operation for selecting rows.
func generateGetOperation(tableName string) *Operation {
	return &Operation{
		Summary:     fmt.Sprintf("Get rows from %s", tableName),
		Description: fmt.Sprintf("Retrieve rows from the %s table with optional filtering, ordering, and pagination.", tableName),
		OperationID: fmt.Sprintf("get%s", capitalizeFirst(tableName)),
		Tags:        []string{tableName},
		Parameters:  generateQueryParameters(),
		Responses: map[string]Response{
			"200": {
				Description: "Successful response",
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{
							Type: "array",
							Items: &Schema{
								Ref: fmt.Sprintf("#/components/schemas/%s", tableName),
							},
						},
					},
				},
				Headers: map[string]Header{
					"Content-Range": {
						Description: "Pagination info in format: start-end/total",
						Schema:      &Schema{Type: "string"},
					},
				},
			},
			"400": errorResponse("Bad request"),
			"401": errorResponse("Unauthorized"),
		},
		Security: []SecurityReq{
			{"bearerAuth": {}},
			{"apiKey": {}},
		},
	}
}

// generatePostOperation generates the POST operation for inserting rows.
func generatePostOperation(tableName string) *Operation {
	return &Operation{
		Summary:     fmt.Sprintf("Insert rows into %s", tableName),
		Description: fmt.Sprintf("Insert one or more rows into the %s table. Supports bulk insert with JSON array.", tableName),
		OperationID: fmt.Sprintf("create%s", capitalizeFirst(tableName)),
		Tags:        []string{tableName},
		Parameters: []Parameter{
			{
				Name:        "Prefer",
				In:          "header",
				Description: "Request preferences: return=representation to return inserted rows",
				Schema:      &Schema{Type: "string", Enum: []any{"return=representation", "return=minimal"}},
			},
		},
		RequestBody: &RequestBody{
			Description: "Row data to insert",
			Required:    true,
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Ref: fmt.Sprintf("#/components/schemas/%s", tableName),
					},
				},
			},
		},
		Responses: map[string]Response{
			"201": {
				Description: "Created",
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{
							Type: "array",
							Items: &Schema{
								Ref: fmt.Sprintf("#/components/schemas/%s", tableName),
							},
						},
					},
				},
			},
			"400": errorResponse("Bad request"),
			"401": errorResponse("Unauthorized"),
		},
		Security: []SecurityReq{
			{"bearerAuth": {}},
			{"apiKey": {}},
		},
	}
}

// generatePatchOperation generates the PATCH operation for updating rows.
func generatePatchOperation(tableName string) *Operation {
	return &Operation{
		Summary:     fmt.Sprintf("Update rows in %s", tableName),
		Description: fmt.Sprintf("Update rows in the %s table matching the specified filters.", tableName),
		OperationID: fmt.Sprintf("update%s", capitalizeFirst(tableName)),
		Tags:        []string{tableName},
		Parameters: append(generateFilterParameters(), Parameter{
			Name:        "Prefer",
			In:          "header",
			Description: "Request preferences: return=representation to return updated rows",
			Schema:      &Schema{Type: "string", Enum: []any{"return=representation", "return=minimal"}},
		}),
		RequestBody: &RequestBody{
			Description: "Fields to update",
			Required:    true,
			Content: map[string]MediaType{
				"application/json": {
					Schema: &Schema{
						Type: "object",
					},
				},
			},
		},
		Responses: map[string]Response{
			"200": {
				Description: "Updated",
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{
							Type: "array",
							Items: &Schema{
								Ref: fmt.Sprintf("#/components/schemas/%s", tableName),
							},
						},
					},
				},
			},
			"400": errorResponse("Bad request"),
			"401": errorResponse("Unauthorized"),
		},
		Security: []SecurityReq{
			{"bearerAuth": {}},
			{"apiKey": {}},
		},
	}
}

// generateDeleteOperation generates the DELETE operation for removing rows.
func generateDeleteOperation(tableName string) *Operation {
	return &Operation{
		Summary:     fmt.Sprintf("Delete rows from %s", tableName),
		Description: fmt.Sprintf("Delete rows from the %s table matching the specified filters. At least one filter is required.", tableName),
		OperationID: fmt.Sprintf("delete%s", capitalizeFirst(tableName)),
		Tags:        []string{tableName},
		Parameters: append(generateFilterParameters(), Parameter{
			Name:        "Prefer",
			In:          "header",
			Description: "Request preferences: return=representation to return deleted rows",
			Schema:      &Schema{Type: "string", Enum: []any{"return=representation", "return=minimal"}},
		}),
		Responses: map[string]Response{
			"204": {
				Description: "Deleted (no content)",
			},
			"200": {
				Description: "Deleted (with representation)",
				Content: map[string]MediaType{
					"application/json": {
						Schema: &Schema{
							Type: "array",
							Items: &Schema{
								Ref: fmt.Sprintf("#/components/schemas/%s", tableName),
							},
						},
					},
				},
			},
			"400": errorResponse("Bad request - filter required"),
			"401": errorResponse("Unauthorized"),
		},
		Security: []SecurityReq{
			{"bearerAuth": {}},
			{"apiKey": {}},
		},
	}
}

// generateQueryParameters returns common query parameters for GET requests.
func generateQueryParameters() []Parameter {
	return []Parameter{
		{
			Name:        "select",
			In:          "query",
			Description: "Columns to return (comma-separated). Supports embedded relations: select=*,posts(*)",
			Schema:      &Schema{Type: "string", Default: "*"},
		},
		{
			Name:        "order",
			In:          "query",
			Description: "Order by column(s): column.asc or column.desc (comma-separated for multiple)",
			Schema:      &Schema{Type: "string"},
		},
		{
			Name:        "limit",
			In:          "query",
			Description: "Maximum number of rows to return",
			Schema:      &Schema{Type: "integer"},
		},
		{
			Name:        "offset",
			In:          "query",
			Description: "Number of rows to skip",
			Schema:      &Schema{Type: "integer"},
		},
		{
			Name:        "Range",
			In:          "header",
			Description: "Pagination range header: 0-24 or items=0-24",
			Schema:      &Schema{Type: "string"},
		},
		{
			Name:        "Prefer",
			In:          "header",
			Description: "Request preferences: count=exact for total count",
			Schema:      &Schema{Type: "string", Enum: []any{"count=exact", "count=planned", "count=estimated"}},
		},
	}
}

// generateFilterParameters returns common filter parameters for mutating requests.
func generateFilterParameters() []Parameter {
	return []Parameter{
		{
			Name:        "column",
			In:          "query",
			Description: "Filter by column value using PostgREST syntax: column=operator.value (e.g., id=eq.1, status=in.(active,pending))",
			Schema:      &Schema{Type: "string"},
		},
	}
}

// errorResponse generates a standard error response.
func errorResponse(description string) Response {
	return Response{
		Description: description,
		Content: map[string]MediaType{
			"application/json": {
				Schema: &Schema{
					Type: "object",
					Properties: map[string]Schema{
						"error":   {Type: "string"},
						"message": {Type: "string"},
					},
				},
			},
		},
	}
}

// capitalizeFirst capitalizes the first letter of a string.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
