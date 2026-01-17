# Dashboard Table Management - Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add full table management UI to the sblite dashboard with data browsing, inline editing, and schema management.

**Architecture:** Dashboard handler proxies requests to admin/REST APIs with session auth. Frontend is vanilla JS with state management for tables, data grid, and modals.

**Tech Stack:** Go (Chi router), Vanilla JavaScript, CSS, Playwright for E2E tests

---

## Task 1: Dashboard API - Table List Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`
- Test: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

Add to `internal/dashboard/handler_test.go`:

```go
func TestHandlerListTables(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create a test table first
	_, err := h.db.Exec(`CREATE TABLE test_items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)

	// Register in schema
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('test_items', 'id', 'text', false, true)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('test_items', 'name', 'text', true, false)`)
	require.NoError(t, err)

	// Setup session
	token := setupTestSession(t, h)

	req := httptest.NewRequest("GET", "/_/api/tables", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var tables []map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &tables)
	require.NoError(t, err)
	require.Len(t, tables, 1)
	require.Equal(t, "test_items", tables[0]["name"])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandlerListTables -v`
Expected: FAIL (route not found, 404)

**Step 3: Implement the endpoint**

Add to `internal/dashboard/handler.go` in `RegisterRoutes`:

```go
// Table management API routes (require auth)
r.Route("/api/tables", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/", h.handleListTables)
})
```

Add the middleware and handler:

```go
func (h *Handler) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil || cookie.Value == "" || !h.sessions.Validate(cookie.Value) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "Unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) handleListTables(w http.ResponseWriter, r *http.Request) {
	rows, err := h.db.Query(`SELECT DISTINCT table_name FROM _columns ORDER BY table_name`)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to list tables"})
		return
	}
	defer rows.Close()

	var tables []map[string]interface{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, map[string]interface{}{"name": name})
	}

	if tables == nil {
		tables = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandlerListTables -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add table list API endpoint"
```

---

## Task 2: Dashboard API - Get Table Schema Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

```go
func TestHandlerGetTableSchema(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create test table with columns
	_, err := h.db.Exec(`CREATE TABLE products (id TEXT PRIMARY KEY, name TEXT NOT NULL, price INTEGER)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES
		('products', 'id', 'uuid', false, true),
		('products', 'name', 'text', false, false),
		('products', 'price', 'integer', true, false)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("GET", "/_/api/tables/products", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var schema map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &schema)
	require.NoError(t, err)
	require.Equal(t, "products", schema["name"])

	columns := schema["columns"].([]interface{})
	require.Len(t, columns, 3)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandlerGetTableSchema -v`
Expected: FAIL

**Step 3: Implement the endpoint**

Add route in `RegisterRoutes`:

```go
r.Route("/api/tables", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/", h.handleListTables)
	r.Get("/{name}", h.handleGetTableSchema)
})
```

Add handler:

```go
func (h *Handler) handleGetTableSchema(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	rows, err := h.db.Query(`SELECT column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns WHERE table_name = ? ORDER BY column_name`, tableName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get schema"})
		return
	}
	defer rows.Close()

	var columns []map[string]interface{}
	for rows.Next() {
		var name, pgType string
		var nullable, primary bool
		var defaultVal sql.NullString
		if err := rows.Scan(&name, &pgType, &nullable, &defaultVal, &primary); err != nil {
			continue
		}
		col := map[string]interface{}{
			"name":     name,
			"type":     pgType,
			"nullable": nullable,
			"primary":  primary,
		}
		if defaultVal.Valid {
			col["default"] = defaultVal.String
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"name":    tableName,
		"columns": columns,
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandlerGetTableSchema -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add get table schema API endpoint"
```

---

## Task 3: Dashboard API - Create Table Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

```go
func TestHandlerCreateTable(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	token := setupTestSession(t, h)

	body := `{"name":"orders","columns":[{"name":"id","type":"uuid","primary":true},{"name":"total","type":"integer","nullable":true}]}`
	req := httptest.NewRequest("POST", "/_/api/tables", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	// Verify table exists
	var count int
	err := h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'orders'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 2, count)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandlerCreateTable -v`
Expected: FAIL

**Step 3: Implement the endpoint**

Add route:

```go
r.Route("/api/tables", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/", h.handleListTables)
	r.Post("/", h.handleCreateTable)
	r.Get("/{name}", h.handleGetTableSchema)
})
```

Add handler:

```go
type CreateTableRequest struct {
	Name    string `json:"name"`
	Columns []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Default  string `json:"default,omitempty"`
		Primary  bool   `json:"primary"`
	} `json:"columns"`
}

func (h *Handler) handleCreateTable(w http.ResponseWriter, r *http.Request) {
	var req CreateTableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request body"})
		return
	}

	if req.Name == "" || len(req.Columns) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Name and columns required"})
		return
	}

	// Build CREATE TABLE SQL
	var colDefs []string
	var primaryKeys []string
	for _, col := range req.Columns {
		sqlType := pgTypeToSQLite(col.Type)
		def := fmt.Sprintf(`"%s" %s`, col.Name, sqlType)
		if !col.Nullable {
			def += " NOT NULL"
		}
		if col.Default != "" {
			def += " DEFAULT " + col.Default
		}
		colDefs = append(colDefs, def)
		if col.Primary {
			primaryKeys = append(primaryKeys, fmt.Sprintf(`"%s"`, col.Name))
		}
	}
	if len(primaryKeys) > 0 {
		colDefs = append(colDefs, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}

	createSQL := fmt.Sprintf(`CREATE TABLE "%s" (%s)`, req.Name, strings.Join(colDefs, ", "))

	tx, err := h.db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(createSQL); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Register columns in metadata
	for _, col := range req.Columns {
		_, err := tx.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary) VALUES (?, ?, ?, ?, ?, ?)`,
			req.Name, col.Name, col.Type, col.Nullable, col.Default, col.Primary)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to register column"})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"name": req.Name, "columns": req.Columns})
}

func pgTypeToSQLite(pgType string) string {
	switch pgType {
	case "integer", "boolean":
		return "INTEGER"
	case "bytea":
		return "BLOB"
	default:
		return "TEXT"
	}
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandlerCreateTable -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add create table API endpoint"
```

---

## Task 4: Dashboard API - Delete Table Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

```go
func TestHandlerDeleteTable(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create table first
	_, err := h.db.Exec(`CREATE TABLE to_delete (id TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('to_delete', 'id', 'text', true, false)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("DELETE", "/_/api/tables/to_delete", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify table is gone
	var count int
	err = h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'to_delete'`).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandlerDeleteTable -v`
Expected: FAIL

**Step 3: Implement the endpoint**

Add route:

```go
r.Delete("/{name}", h.handleDeleteTable)
```

Add handler:

```go
func (h *Handler) handleDeleteTable(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	if tableName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Drop the table
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, tableName)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Remove metadata
	if _, err := tx.Exec(`DELETE FROM _columns WHERE table_name = ?`, tableName); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to remove metadata"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandlerDeleteTable -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add delete table API endpoint"
```

---

## Task 5: Dashboard API - Data Select Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

```go
func TestHandlerSelectData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	// Create and populate table
	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'Apple'), ('2', 'Banana'), ('3', 'Cherry')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("GET", "/_/api/data/items?limit=2&offset=0", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var result map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)

	rows := result["rows"].([]interface{})
	require.Len(t, rows, 2)
	require.Equal(t, float64(3), result["total"])
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandlerSelectData -v`
Expected: FAIL

**Step 3: Implement the endpoint**

Add route:

```go
r.Route("/api/data", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/{table}", h.handleSelectData)
})
```

Add handler:

```go
func (h *Handler) handleSelectData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")
	if tableName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table name required"})
		return
	}

	limit := 25
	offset := 0
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	// Get total count
	var total int
	err := h.db.QueryRow(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, tableName)).Scan(&total)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Table not found"})
		return
	}

	// Get rows
	query := fmt.Sprintf(`SELECT * FROM "%s" LIMIT %d OFFSET %d`, tableName, limit, offset)
	rows, err := h.db.Query(query)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	var results []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"rows":   results,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandlerSelectData -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add data select API endpoint with pagination"
```

---

## Task 6: Dashboard API - Insert, Update, Delete Data Endpoints

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing tests**

```go
func TestHandlerInsertData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"id":"new-1","name":"New Item"}`
	req := httptest.NewRequest("POST", "/_/api/data/items", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = 'new-1'`).Scan(&count)
	require.Equal(t, 1, count)
}

func TestHandlerUpdateData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'Old Name')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"name":"New Name"}`
	req := httptest.NewRequest("PATCH", "/_/api/data/items?id=eq.1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var name string
	h.db.QueryRow(`SELECT name FROM items WHERE id = '1'`).Scan(&name)
	require.Equal(t, "New Name", name)
}

func TestHandlerDeleteData(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'To Delete')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("DELETE", "/_/api/data/items?id=eq.1", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM items WHERE id = '1'`).Scan(&count)
	require.Equal(t, 0, count)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/dashboard/... -run "TestHandlerInsertData|TestHandlerUpdateData|TestHandlerDeleteData" -v`
Expected: FAIL

**Step 3: Implement the endpoints**

Add routes:

```go
r.Route("/api/data", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/{table}", h.handleSelectData)
	r.Post("/{table}", h.handleInsertData)
	r.Patch("/{table}", h.handleUpdateData)
	r.Delete("/{table}", h.handleDeleteData)
})
```

Add handlers:

```go
func (h *Handler) handleInsertData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	var columns []string
	var placeholders []string
	var values []interface{}
	for col, val := range data {
		columns = append(columns, fmt.Sprintf(`"%s"`, col))
		placeholders = append(placeholders, "?")
		values = append(values, val)
	}

	query := fmt.Sprintf(`INSERT INTO "%s" (%s) VALUES (%s)`,
		tableName, strings.Join(columns, ", "), strings.Join(placeholders, ", "))

	if _, err := h.db.Exec(query, values...); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleUpdateData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON"})
		return
	}

	// Build SET clause
	var setClauses []string
	var values []interface{}
	for col, val := range data {
		setClauses = append(setClauses, fmt.Sprintf(`"%s" = ?`, col))
		values = append(values, val)
	}

	// Parse filter from query string (simple eq filter)
	whereClause, whereValues := h.parseSimpleFilter(r.URL.Query())
	values = append(values, whereValues...)

	query := fmt.Sprintf(`UPDATE "%s" SET %s %s`, tableName, strings.Join(setClauses, ", "), whereClause)

	result, err := h.db.Exec(query, values...)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	affected, _ := result.RowsAffected()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"updated": affected})
}

func (h *Handler) handleDeleteData(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "table")

	whereClause, whereValues := h.parseSimpleFilter(r.URL.Query())
	if whereClause == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Filter required for delete"})
		return
	}

	query := fmt.Sprintf(`DELETE FROM "%s" %s`, tableName, whereClause)

	if _, err := h.db.Exec(query, whereValues...); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) parseSimpleFilter(query url.Values) (string, []interface{}) {
	var conditions []string
	var values []interface{}

	for key, vals := range query {
		if key == "limit" || key == "offset" || key == "order" {
			continue
		}
		if len(vals) > 0 {
			val := vals[0]
			if strings.HasPrefix(val, "eq.") {
				conditions = append(conditions, fmt.Sprintf(`"%s" = ?`, key))
				values = append(values, strings.TrimPrefix(val, "eq."))
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(conditions, " AND "), values
}
```

Add import for `net/url` at the top.

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/dashboard/... -run "TestHandlerInsertData|TestHandlerUpdateData|TestHandlerDeleteData" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add insert, update, delete data API endpoints"
```

---

## Task 7: Dashboard API - Add Column Endpoint

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing test**

```go
func TestHandlerAddColumn(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('items', 'id', 'text', false, true)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"name":"description","type":"text","nullable":true}`
	req := httptest.NewRequest("POST", "/_/api/tables/items/columns", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	// Verify column exists in metadata
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'items' AND column_name = 'description'`).Scan(&count)
	require.Equal(t, 1, count)
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/dashboard/... -run TestHandlerAddColumn -v`
Expected: FAIL

**Step 3: Implement the endpoint**

Add route:

```go
r.Route("/api/tables", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/", h.handleListTables)
	r.Post("/", h.handleCreateTable)
	r.Get("/{name}", h.handleGetTableSchema)
	r.Delete("/{name}", h.handleDeleteTable)
	r.Post("/{name}/columns", h.handleAddColumn)
})
```

Add handler:

```go
func (h *Handler) handleAddColumn(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")

	var col struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Nullable bool   `json:"nullable"`
		Default  string `json:"default,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&col); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	sqlType := pgTypeToSQLite(col.Type)
	alterSQL := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, tableName, col.Name, sqlType)
	if col.Default != "" {
		alterSQL += " DEFAULT " + col.Default
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	if _, err := tx.Exec(alterSQL); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	_, err = tx.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, default_value, is_primary) VALUES (?, ?, ?, ?, ?, ?)`,
		tableName, col.Name, col.Type, col.Nullable, col.Default, false)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to register column"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(col)
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/dashboard/... -run TestHandlerAddColumn -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add column API endpoint"
```

---

## Task 8: Dashboard API - Rename and Drop Column Endpoints

**Files:**
- Modify: `internal/dashboard/handler.go`
- Modify: `internal/dashboard/handler_test.go`

**Step 1: Write the failing tests**

```go
func TestHandlerRenameColumn(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT, old_name TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('items', 'old_name', 'text', true, false)`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	body := `{"new_name":"new_name"}`
	req := httptest.NewRequest("PATCH", "/_/api/tables/items/columns/old_name", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'items' AND column_name = 'new_name'`).Scan(&count)
	require.Equal(t, 1, count)
}

func TestHandlerDropColumn(t *testing.T) {
	h, dbPath := setupTestHandler(t)
	defer os.Remove(dbPath)

	_, err := h.db.Exec(`CREATE TABLE items (id TEXT PRIMARY KEY, to_drop TEXT)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO _columns (table_name, column_name, pg_type, is_nullable, is_primary) VALUES ('items', 'id', 'text', false, true), ('items', 'to_drop', 'text', true, false)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO items VALUES ('1', 'value')`)
	require.NoError(t, err)

	token := setupTestSession(t, h)

	req := httptest.NewRequest("DELETE", "/_/api/tables/items/columns/to_drop", nil)
	req.AddCookie(&http.Cookie{Name: "_sblite_session", Value: token})
	w := httptest.NewRecorder()

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify column is gone from metadata
	var count int
	h.db.QueryRow(`SELECT COUNT(*) FROM _columns WHERE table_name = 'items' AND column_name = 'to_drop'`).Scan(&count)
	require.Equal(t, 0, count)

	// Verify data preserved in remaining columns
	var id string
	h.db.QueryRow(`SELECT id FROM items`).Scan(&id)
	require.Equal(t, "1", id)
}
```

**Step 2: Run tests to verify they fail**

Run: `go test ./internal/dashboard/... -run "TestHandlerRenameColumn|TestHandlerDropColumn" -v`
Expected: FAIL

**Step 3: Implement the endpoints**

Add routes:

```go
r.Route("/api/tables", func(r chi.Router) {
	r.Use(h.requireAuth)
	r.Get("/", h.handleListTables)
	r.Post("/", h.handleCreateTable)
	r.Get("/{name}", h.handleGetTableSchema)
	r.Delete("/{name}", h.handleDeleteTable)
	r.Post("/{name}/columns", h.handleAddColumn)
	r.Patch("/{name}/columns/{column}", h.handleRenameColumn)
	r.Delete("/{name}/columns/{column}", h.handleDropColumn)
})
```

Add handlers:

```go
func (h *Handler) handleRenameColumn(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	oldName := chi.URLParam(r, "column")

	var req struct {
		NewName string `json:"new_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.NewName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "new_name required"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	alterSQL := fmt.Sprintf(`ALTER TABLE "%s" RENAME COLUMN "%s" TO "%s"`, tableName, oldName, req.NewName)
	if _, err := tx.Exec(alterSQL); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	if _, err := tx.Exec(`UPDATE _columns SET column_name = ? WHERE table_name = ? AND column_name = ?`,
		req.NewName, tableName, oldName); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update metadata"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"name": req.NewName})
}

func (h *Handler) handleDropColumn(w http.ResponseWriter, r *http.Request) {
	tableName := chi.URLParam(r, "name")
	columnName := chi.URLParam(r, "column")

	// Get remaining columns
	rows, err := h.db.Query(`SELECT column_name, pg_type, is_nullable, default_value, is_primary
		FROM _columns WHERE table_name = ? AND column_name != ? ORDER BY column_name`, tableName, columnName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to get columns"})
		return
	}
	defer rows.Close()

	type colInfo struct {
		name, pgType         string
		nullable, primary    bool
		defaultVal           sql.NullString
	}
	var remainingCols []colInfo
	var colNames []string

	for rows.Next() {
		var c colInfo
		rows.Scan(&c.name, &c.pgType, &c.nullable, &c.defaultVal, &c.primary)
		remainingCols = append(remainingCols, c)
		colNames = append(colNames, fmt.Sprintf(`"%s"`, c.name))
	}

	if len(remainingCols) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Cannot drop last column"})
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// Create new table without the column
	var colDefs []string
	var primaryKeys []string
	for _, c := range remainingCols {
		def := fmt.Sprintf(`"%s" %s`, c.name, pgTypeToSQLite(c.pgType))
		if !c.nullable {
			def += " NOT NULL"
		}
		if c.defaultVal.Valid {
			def += " DEFAULT " + c.defaultVal.String
		}
		colDefs = append(colDefs, def)
		if c.primary {
			primaryKeys = append(primaryKeys, fmt.Sprintf(`"%s"`, c.name))
		}
	}
	if len(primaryKeys) > 0 {
		colDefs = append(colDefs, "PRIMARY KEY ("+strings.Join(primaryKeys, ", ")+")")
	}

	newTableSQL := fmt.Sprintf(`CREATE TABLE "%s_new" (%s)`, tableName, strings.Join(colDefs, ", "))
	if _, err := tx.Exec(newTableSQL); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Copy data
	copySQL := fmt.Sprintf(`INSERT INTO "%s_new" SELECT %s FROM "%s"`, tableName, strings.Join(colNames, ", "), tableName)
	if _, err := tx.Exec(copySQL); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Drop old, rename new
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE "%s"`, tableName)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	if _, err := tx.Exec(fmt.Sprintf(`ALTER TABLE "%s_new" RENAME TO "%s"`, tableName, tableName)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	// Update metadata
	if _, err := tx.Exec(`DELETE FROM _columns WHERE table_name = ? AND column_name = ?`, tableName, columnName); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to update metadata"})
		return
	}

	if err := tx.Commit(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to commit"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

**Step 4: Run tests to verify they pass**

Run: `go test ./internal/dashboard/... -run "TestHandlerRenameColumn|TestHandlerDropColumn" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add internal/dashboard/handler.go internal/dashboard/handler_test.go
git commit -m "feat(dashboard): add rename and drop column API endpoints"
```

---

## Task 9: Frontend - Table List and Selection

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Update app.js state and methods**

Replace the `renderContent` method for 'tables' case and add table management state:

```javascript
// Add to App.state:
tables: {
    list: [],
    selected: null,
    schema: null,
    data: [],
    page: 1,
    pageSize: 25,
    totalRows: 0,
    selectedRows: new Set(),
    editingCell: null,
    loading: false,
},

// Add methods:
async loadTables() {
    try {
        const res = await fetch('/_/api/tables');
        if (res.ok) {
            this.state.tables.list = await res.json();
        }
    } catch (e) {
        this.state.error = 'Failed to load tables';
    }
    this.render();
},

async selectTable(name) {
    this.state.tables.selected = name;
    this.state.tables.page = 1;
    this.state.tables.selectedRows = new Set();
    await this.loadTableSchema(name);
    await this.loadTableData();
},

async loadTableSchema(name) {
    try {
        const res = await fetch(`/_/api/tables/${name}`);
        if (res.ok) {
            this.state.tables.schema = await res.json();
        }
    } catch (e) {
        this.state.error = 'Failed to load schema';
    }
},

async loadTableData() {
    const { selected, page, pageSize } = this.state.tables;
    if (!selected) return;

    this.state.tables.loading = true;
    this.render();

    try {
        const offset = (page - 1) * pageSize;
        const res = await fetch(`/_/api/data/${selected}?limit=${pageSize}&offset=${offset}`);
        if (res.ok) {
            const data = await res.json();
            this.state.tables.data = data.rows;
            this.state.tables.totalRows = data.total;
        }
    } catch (e) {
        this.state.error = 'Failed to load data';
    }
    this.state.tables.loading = false;
    this.render();
},

// Update renderContent for tables:
renderContent() {
    switch (this.state.currentView) {
        case 'tables':
            return this.renderTablesView();
        // ... other cases unchanged
    }
},

renderTablesView() {
    return `
        <div class="tables-layout">
            <div class="table-list-panel">
                <div class="panel-header">
                    <span>Tables</span>
                    <button class="btn btn-primary btn-sm" onclick="App.showCreateTableModal()">+ New</button>
                </div>
                <div class="table-list">
                    ${this.state.tables.list.length === 0
                        ? '<div class="empty-state">No tables yet</div>'
                        : this.state.tables.list.map(t => `
                            <div class="table-list-item ${this.state.tables.selected === t.name ? 'active' : ''}"
                                 onclick="App.selectTable('${t.name}')">
                                ${t.name}
                            </div>
                        `).join('')}
                </div>
            </div>
            <div class="table-content-panel">
                ${this.state.tables.selected ? this.renderTableContent() : '<div class="empty-state">Select a table</div>'}
            </div>
        </div>
    `;
},

renderTableContent() {
    if (this.state.tables.loading) {
        return '<div class="loading">Loading...</div>';
    }
    return `
        <div class="table-toolbar">
            <h2>${this.state.tables.selected}</h2>
            <div class="toolbar-actions">
                <button class="btn btn-secondary btn-sm" onclick="App.showAddRowModal()">+ Add Row</button>
                <button class="btn btn-secondary btn-sm" onclick="App.showSchemaModal()">Schema</button>
                <button class="btn btn-secondary btn-sm" onclick="App.confirmDeleteTable()">Delete Table</button>
            </div>
        </div>
        ${this.renderDataGrid()}
        ${this.renderPagination()}
    `;
},
```

**Step 2: Add CSS for table layout**

Add to `style.css`:

```css
/* Tables view layout */
.tables-layout {
    display: flex;
    gap: 1rem;
    height: calc(100vh - 4rem);
}

.table-list-panel {
    width: 200px;
    flex-shrink: 0;
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    display: flex;
    flex-direction: column;
}

.panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem;
    border-bottom: 1px solid var(--border);
    font-weight: 500;
}

.table-list {
    flex: 1;
    overflow-y: auto;
    padding: 0.5rem;
}

.table-list-item {
    padding: 0.5rem 0.75rem;
    border-radius: 0.375rem;
    cursor: pointer;
    margin-bottom: 0.25rem;
}

.table-list-item:hover {
    background: var(--bg-tertiary);
}

.table-list-item.active {
    background: var(--accent-muted);
    color: var(--accent);
}

.table-content-panel {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-width: 0;
}

.table-toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1rem;
}

.toolbar-actions {
    display: flex;
    gap: 0.5rem;
}

.btn-sm {
    padding: 0.375rem 0.75rem;
    font-size: 0.8125rem;
}

.empty-state {
    color: var(--text-secondary);
    text-align: center;
    padding: 2rem;
}
```

**Step 3: Update init to load tables when authenticated**

```javascript
async init() {
    this.loadTheme();
    await this.checkAuth();
    if (this.state.authenticated) {
        await this.loadTables();
    }
    this.render();
},
```

**Step 4: Test manually**

Run: `go build -o sblite . && ./sblite serve --db test.db`
Visit: `http://localhost:8080/_/`
Expected: Table list panel visible, can select tables

**Step 5: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): add table list UI with selection"
```

---

## Task 10: Frontend - Data Grid with Pagination

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add data grid render method**

```javascript
renderDataGrid() {
    const { schema, data, selectedRows } = this.state.tables;
    if (!schema || !schema.columns) return '';

    const columns = schema.columns;
    const primaryKey = columns.find(c => c.primary)?.name || columns[0]?.name;

    return `
        <div class="data-grid-container">
            <table class="data-grid">
                <thead>
                    <tr>
                        <th class="checkbox-col">
                            <input type="checkbox" onchange="App.toggleAllRows(this.checked)"
                                ${data.length > 0 && selectedRows.size === data.length ? 'checked' : ''}>
                        </th>
                        ${columns.map(col => `
                            <th>${col.name}<span class="col-type">${col.type}</span></th>
                        `).join('')}
                        <th class="actions-col"></th>
                    </tr>
                </thead>
                <tbody>
                    ${data.length === 0
                        ? `<tr><td colspan="${columns.length + 2}" class="empty-state">No data</td></tr>`
                        : data.map(row => this.renderDataRow(row, columns, primaryKey)).join('')}
                </tbody>
            </table>
        </div>
    `;
},

renderDataRow(row, columns, primaryKey) {
    const rowId = row[primaryKey];
    const isSelected = this.state.tables.selectedRows.has(rowId);

    return `
        <tr class="${isSelected ? 'selected' : ''}">
            <td class="checkbox-col">
                <input type="checkbox" ${isSelected ? 'checked' : ''}
                    onchange="App.toggleRow('${rowId}', this.checked)">
            </td>
            ${columns.map(col => `
                <td class="data-cell"
                    onclick="App.startCellEdit('${rowId}', '${col.name}')"
                    data-row="${rowId}" data-col="${col.name}">
                    ${this.formatCellValue(row[col.name], col.type)}
                </td>
            `).join('')}
            <td class="actions-col">
                <button class="btn-icon" onclick="App.showEditRowModal('${rowId}')">Edit</button>
                <button class="btn-icon" onclick="App.confirmDeleteRow('${rowId}')">Delete</button>
            </td>
        </tr>
    `;
},

formatCellValue(value, type) {
    if (value === null || value === undefined) return '<span class="null-value">NULL</span>';
    if (type === 'boolean') return value ? 'true' : 'false';
    if (type === 'jsonb') return '<span class="json-value">{...}</span>';
    const str = String(value);
    return str.length > 50 ? str.substring(0, 50) + '...' : str;
},

toggleRow(rowId, checked) {
    if (checked) {
        this.state.tables.selectedRows.add(rowId);
    } else {
        this.state.tables.selectedRows.delete(rowId);
    }
    this.render();
},

toggleAllRows(checked) {
    const { data, schema } = this.state.tables;
    const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

    if (checked) {
        data.forEach(row => this.state.tables.selectedRows.add(row[primaryKey]));
    } else {
        this.state.tables.selectedRows.clear();
    }
    this.render();
},

renderPagination() {
    const { page, pageSize, totalRows } = this.state.tables;
    const totalPages = Math.ceil(totalRows / pageSize);

    return `
        <div class="pagination">
            <div class="pagination-info">
                ${totalRows} rows | Page ${page} of ${totalPages || 1}
            </div>
            <div class="pagination-controls">
                <select onchange="App.changePageSize(this.value)">
                    <option value="25" ${pageSize === 25 ? 'selected' : ''}>25</option>
                    <option value="50" ${pageSize === 50 ? 'selected' : ''}>50</option>
                    <option value="100" ${pageSize === 100 ? 'selected' : ''}>100</option>
                </select>
                <button class="btn btn-secondary btn-sm" onclick="App.prevPage()" ${page <= 1 ? 'disabled' : ''}>Prev</button>
                <button class="btn btn-secondary btn-sm" onclick="App.nextPage()" ${page >= totalPages ? 'disabled' : ''}>Next</button>
            </div>
        </div>
    `;
},

changePageSize(size) {
    this.state.tables.pageSize = parseInt(size);
    this.state.tables.page = 1;
    this.loadTableData();
},

prevPage() {
    if (this.state.tables.page > 1) {
        this.state.tables.page--;
        this.loadTableData();
    }
},

nextPage() {
    const totalPages = Math.ceil(this.state.tables.totalRows / this.state.tables.pageSize);
    if (this.state.tables.page < totalPages) {
        this.state.tables.page++;
        this.loadTableData();
    }
},
```

**Step 2: Add data grid CSS**

```css
/* Data grid */
.data-grid-container {
    flex: 1;
    overflow: auto;
    border: 1px solid var(--border);
    border-radius: 0.5rem;
}

.data-grid {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.875rem;
}

.data-grid th, .data-grid td {
    padding: 0.5rem 0.75rem;
    text-align: left;
    border-bottom: 1px solid var(--border);
}

.data-grid th {
    background: var(--bg-secondary);
    font-weight: 500;
    position: sticky;
    top: 0;
    z-index: 1;
}

.col-type {
    display: block;
    font-size: 0.75rem;
    color: var(--text-muted);
    font-weight: normal;
}

.data-grid tbody tr:hover {
    background: var(--bg-tertiary);
}

.data-grid tbody tr.selected {
    background: var(--accent-muted);
}

.checkbox-col {
    width: 40px;
    text-align: center;
}

.actions-col {
    width: 100px;
    text-align: right;
}

.data-cell {
    cursor: pointer;
    max-width: 200px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.null-value {
    color: var(--text-muted);
    font-style: italic;
}

.json-value {
    color: var(--accent);
}

.btn-icon {
    background: none;
    border: none;
    color: var(--text-secondary);
    cursor: pointer;
    padding: 0.25rem 0.5rem;
    font-size: 0.75rem;
}

.btn-icon:hover {
    color: var(--text-primary);
}

/* Pagination */
.pagination {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 0;
    margin-top: 0.5rem;
}

.pagination-info {
    color: var(--text-secondary);
    font-size: 0.875rem;
}

.pagination-controls {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}

.pagination-controls select {
    padding: 0.375rem;
    border: 1px solid var(--border);
    border-radius: 0.375rem;
    background: var(--bg-secondary);
    color: var(--text-primary);
}
```

**Step 3: Test manually**

Visit dashboard, select a table with data, verify grid displays and pagination works.

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): add data grid with pagination"
```

---

## Task 11: Frontend - Inline Cell Editing

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add inline editing methods**

```javascript
startCellEdit(rowId, column) {
    this.state.tables.editingCell = { rowId, column };
    this.render();

    // Focus the input after render
    setTimeout(() => {
        const input = document.querySelector('.cell-input');
        if (input) {
            input.focus();
            input.select();
        }
    }, 0);
},

cancelCellEdit() {
    this.state.tables.editingCell = null;
    this.render();
},

async saveCellEdit(rowId, column, value) {
    const { selected, schema } = this.state.tables;
    const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

    try {
        const res = await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${rowId}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ [column]: value || null })
        });

        if (!res.ok) {
            const err = await res.json();
            this.state.error = err.error || 'Failed to update';
        }
    } catch (e) {
        this.state.error = 'Failed to update';
    }

    this.state.tables.editingCell = null;
    await this.loadTableData();
},

handleCellKeydown(e, rowId, column) {
    if (e.key === 'Enter') {
        this.saveCellEdit(rowId, column, e.target.value);
    } else if (e.key === 'Escape') {
        this.cancelCellEdit();
    }
},
```

**Step 2: Update renderDataRow to handle editing state**

Update the cell rendering in `renderDataRow`:

```javascript
renderDataRow(row, columns, primaryKey) {
    const rowId = row[primaryKey];
    const isSelected = this.state.tables.selectedRows.has(rowId);
    const { editingCell } = this.state.tables;

    return `
        <tr class="${isSelected ? 'selected' : ''}">
            <td class="checkbox-col">
                <input type="checkbox" ${isSelected ? 'checked' : ''}
                    onchange="App.toggleRow('${rowId}', this.checked)">
            </td>
            ${columns.map(col => {
                const isEditing = editingCell?.rowId === rowId && editingCell?.column === col.name;
                const value = row[col.name];

                if (isEditing) {
                    return `
                        <td class="data-cell editing">
                            <input type="text" class="cell-input" value="${value ?? ''}"
                                onblur="App.saveCellEdit('${rowId}', '${col.name}', this.value)"
                                onkeydown="App.handleCellKeydown(event, '${rowId}', '${col.name}')">
                        </td>
                    `;
                }
                return `
                    <td class="data-cell"
                        onclick="App.startCellEdit('${rowId}', '${col.name}')">
                        ${this.formatCellValue(value, col.type)}
                    </td>
                `;
            }).join('')}
            <td class="actions-col">
                <button class="btn-icon" onclick="App.showEditRowModal('${rowId}')">Edit</button>
                <button class="btn-icon" onclick="App.confirmDeleteRow('${rowId}')">Delete</button>
            </td>
        </tr>
    `;
},
```

**Step 3: Add editing CSS**

```css
.data-cell.editing {
    padding: 0;
}

.cell-input {
    width: 100%;
    padding: 0.5rem 0.75rem;
    border: 2px solid var(--accent);
    background: var(--bg-primary);
    color: var(--text-primary);
    font-size: inherit;
    outline: none;
}
```

**Step 4: Test manually**

Click a cell, edit value, press Enter to save or Escape to cancel.

**Step 5: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): add inline cell editing"
```

---

## Task 12: Frontend - Create Table Modal

**Files:**
- Modify: `internal/dashboard/static/app.js`
- Modify: `internal/dashboard/static/style.css`

**Step 1: Add modal state and methods**

```javascript
// Add to state:
modal: {
    type: null,  // 'createTable', 'addRow', 'editRow', 'schema', 'addColumn'
    data: {}
},

showCreateTableModal() {
    this.state.modal = {
        type: 'createTable',
        data: {
            name: '',
            columns: [{ name: 'id', type: 'uuid', primary: true, nullable: false }]
        }
    };
    this.render();
},

closeModal() {
    this.state.modal = { type: null, data: {} };
    this.render();
},

updateModalData(field, value) {
    this.state.modal.data[field] = value;
    this.render();
},

addColumnToModal() {
    this.state.modal.data.columns.push({ name: '', type: 'text', nullable: true, primary: false });
    this.render();
},

removeColumnFromModal(index) {
    this.state.modal.data.columns.splice(index, 1);
    this.render();
},

updateModalColumn(index, field, value) {
    this.state.modal.data.columns[index][field] = value;
    this.render();
},

async createTable() {
    const { name, columns } = this.state.modal.data;
    if (!name || columns.length === 0) {
        this.state.error = 'Name and at least one column required';
        this.render();
        return;
    }

    try {
        const res = await fetch('/_/api/tables', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ name, columns })
        });

        if (res.ok) {
            this.closeModal();
            await this.loadTables();
            this.selectTable(name);
        } else {
            const err = await res.json();
            this.state.error = err.error || 'Failed to create table';
            this.render();
        }
    } catch (e) {
        this.state.error = 'Failed to create table';
        this.render();
    }
},

renderModals() {
    const { type, data } = this.state.modal;
    if (!type) return '';

    let content = '';
    switch (type) {
        case 'createTable':
            content = this.renderCreateTableModal();
            break;
        // Other modal types will be added
    }

    return `
        <div class="modal-overlay" onclick="App.closeModal()">
            <div class="modal" onclick="event.stopPropagation()">
                ${content}
            </div>
        </div>
    `;
},

renderCreateTableModal() {
    const { name, columns } = this.state.modal.data;
    const types = ['uuid', 'text', 'integer', 'boolean', 'timestamptz', 'jsonb', 'numeric', 'bytea'];

    return `
        <div class="modal-header">
            <h3>Create Table</h3>
            <button class="btn-icon" onclick="App.closeModal()">×</button>
        </div>
        <div class="modal-body">
            <div class="form-group">
                <label class="form-label">Table Name</label>
                <input type="text" class="form-input" value="${name}"
                    onchange="App.updateModalData('name', this.value)" placeholder="my_table">
            </div>

            <div class="form-group">
                <label class="form-label">Columns</label>
                ${columns.map((col, i) => `
                    <div class="column-row">
                        <input type="text" class="form-input" value="${col.name}" placeholder="column_name"
                            onchange="App.updateModalColumn(${i}, 'name', this.value)">
                        <select class="form-input" onchange="App.updateModalColumn(${i}, 'type', this.value)">
                            ${types.map(t => `<option value="${t}" ${col.type === t ? 'selected' : ''}>${t}</option>`).join('')}
                        </select>
                        <label><input type="checkbox" ${col.primary ? 'checked' : ''}
                            onchange="App.updateModalColumn(${i}, 'primary', this.checked)"> PK</label>
                        <label><input type="checkbox" ${col.nullable ? 'checked' : ''}
                            onchange="App.updateModalColumn(${i}, 'nullable', this.checked)"> Null</label>
                        <button class="btn-icon" onclick="App.removeColumnFromModal(${i})">×</button>
                    </div>
                `).join('')}
                <button class="btn btn-secondary btn-sm" onclick="App.addColumnToModal()">+ Add Column</button>
            </div>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
            <button class="btn btn-primary" onclick="App.createTable()">Create Table</button>
        </div>
    `;
},
```

**Step 2: Update render to include modals**

```javascript
renderDashboard() {
    // ... existing code ...
    return `
        <div class="layout">
            <!-- existing sidebar and main content -->
        </div>
        ${this.renderModals()}
    `;
},
```

**Step 3: Add modal CSS**

```css
/* Modal */
.modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
}

.modal {
    background: var(--bg-secondary);
    border: 1px solid var(--border);
    border-radius: 0.5rem;
    width: 100%;
    max-width: 500px;
    max-height: 80vh;
    overflow: auto;
}

.modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1rem;
    border-bottom: 1px solid var(--border);
}

.modal-header h3 {
    margin: 0;
}

.modal-body {
    padding: 1rem;
}

.modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    padding: 1rem;
    border-top: 1px solid var(--border);
}

.column-row {
    display: flex;
    gap: 0.5rem;
    align-items: center;
    margin-bottom: 0.5rem;
}

.column-row .form-input {
    flex: 1;
}

.column-row label {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    font-size: 0.8125rem;
    white-space: nowrap;
}
```

**Step 4: Test manually**

Click "+ New" button, fill form, create table.

**Step 5: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): add create table modal"
```

---

## Task 13: Frontend - Add Row and Edit Row Modals

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add row modal methods**

```javascript
showAddRowModal() {
    const { schema } = this.state.tables;
    if (!schema) return;

    const data = {};
    schema.columns.forEach(col => {
        data[col.name] = col.type === 'uuid' ? crypto.randomUUID() : '';
    });

    this.state.modal = { type: 'addRow', data };
    this.render();
},

showEditRowModal(rowId) {
    const { data, schema } = this.state.tables;
    const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;
    const row = data.find(r => r[primaryKey] === rowId);

    if (row) {
        this.state.modal = { type: 'editRow', data: { ...row, _rowId: rowId } };
        this.render();
    }
},

async saveRow() {
    const { type, data } = this.state.modal;
    const { selected } = this.state.tables;
    const isNew = type === 'addRow';

    const rowData = { ...data };
    delete rowData._rowId;

    try {
        let res;
        if (isNew) {
            res = await fetch(`/_/api/data/${selected}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(rowData)
            });
        } else {
            const { schema } = this.state.tables;
            const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;
            res = await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${data._rowId}`, {
                method: 'PATCH',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(rowData)
            });
        }

        if (res.ok) {
            this.closeModal();
            await this.loadTableData();
        } else {
            const err = await res.json();
            this.state.error = err.error || 'Failed to save';
            this.render();
        }
    } catch (e) {
        this.state.error = 'Failed to save';
        this.render();
    }
},

updateRowField(field, value) {
    this.state.modal.data[field] = value;
},

// Add to renderModals switch:
case 'addRow':
case 'editRow':
    content = this.renderRowModal();
    break;

renderRowModal() {
    const { type, data } = this.state.modal;
    const { schema } = this.state.tables;
    const isNew = type === 'addRow';

    return `
        <div class="modal-header">
            <h3>${isNew ? 'Add Row' : 'Edit Row'}</h3>
            <button class="btn-icon" onclick="App.closeModal()">×</button>
        </div>
        <div class="modal-body">
            ${schema.columns.map(col => `
                <div class="form-group">
                    <label class="form-label">${col.name} <span class="col-type">${col.type}</span></label>
                    <input type="text" class="form-input" value="${data[col.name] ?? ''}"
                        onchange="App.updateRowField('${col.name}', this.value)"
                        ${col.primary && !isNew ? 'disabled' : ''}>
                </div>
            `).join('')}
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="App.closeModal()">Cancel</button>
            <button class="btn btn-primary" onclick="App.saveRow()">${isNew ? 'Add' : 'Save'}</button>
        </div>
    `;
},
```

**Step 2: Test manually**

Click "+ Add Row", fill form, save. Click "Edit" on a row, modify, save.

**Step 3: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add row and edit row modals"
```

---

## Task 14: Frontend - Delete Operations

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add delete methods**

```javascript
async confirmDeleteRow(rowId) {
    if (!confirm('Delete this row?')) return;

    const { selected, schema } = this.state.tables;
    const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

    try {
        const res = await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${rowId}`, {
            method: 'DELETE'
        });

        if (res.ok) {
            await this.loadTableData();
        } else {
            this.state.error = 'Failed to delete';
            this.render();
        }
    } catch (e) {
        this.state.error = 'Failed to delete';
        this.render();
    }
},

async deleteSelectedRows() {
    const { selectedRows, selected, schema } = this.state.tables;
    if (selectedRows.size === 0) return;

    if (!confirm(`Delete ${selectedRows.size} row(s)?`)) return;

    const primaryKey = schema.columns.find(c => c.primary)?.name || schema.columns[0]?.name;

    for (const rowId of selectedRows) {
        try {
            await fetch(`/_/api/data/${selected}?${primaryKey}=eq.${rowId}`, {
                method: 'DELETE'
            });
        } catch (e) {
            // Continue with others
        }
    }

    this.state.tables.selectedRows.clear();
    await this.loadTableData();
},

async confirmDeleteTable() {
    const { selected } = this.state.tables;
    if (!confirm(`Delete table "${selected}"? This cannot be undone.`)) return;

    try {
        const res = await fetch(`/_/api/tables/${selected}`, {
            method: 'DELETE'
        });

        if (res.ok) {
            this.state.tables.selected = null;
            this.state.tables.schema = null;
            this.state.tables.data = [];
            await this.loadTables();
        } else {
            this.state.error = 'Failed to delete table';
            this.render();
        }
    } catch (e) {
        this.state.error = 'Failed to delete table';
        this.render();
    }
},
```

**Step 2: Add bulk delete button to toolbar**

Update `renderTableContent`:

```javascript
renderTableContent() {
    const { selectedRows } = this.state.tables;
    // ... existing code ...
    return `
        <div class="table-toolbar">
            <h2>${this.state.tables.selected}</h2>
            <div class="toolbar-actions">
                ${selectedRows.size > 0 ? `
                    <button class="btn btn-secondary btn-sm" style="color: var(--error)"
                        onclick="App.deleteSelectedRows()">Delete (${selectedRows.size})</button>
                ` : ''}
                <button class="btn btn-secondary btn-sm" onclick="App.showAddRowModal()">+ Add Row</button>
                <button class="btn btn-secondary btn-sm" onclick="App.showSchemaModal()">Schema</button>
                <button class="btn btn-secondary btn-sm" onclick="App.confirmDeleteTable()">Delete Table</button>
            </div>
        </div>
        ${this.renderDataGrid()}
        ${this.renderPagination()}
    `;
},
```

**Step 3: Test manually**

Select rows, click delete. Delete single row. Delete table.

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js
git commit -m "feat(dashboard): add delete operations for rows and tables"
```

---

## Task 15: Frontend - Schema Management Modal

**Files:**
- Modify: `internal/dashboard/static/app.js`

**Step 1: Add schema modal methods**

```javascript
showSchemaModal() {
    this.state.modal = { type: 'schema', data: {} };
    this.render();
},

showAddColumnModal() {
    this.state.modal = {
        type: 'addColumn',
        data: { name: '', type: 'text', nullable: true }
    };
    this.render();
},

async addColumn() {
    const { data } = this.state.modal;
    const { selected } = this.state.tables;

    try {
        const res = await fetch(`/_/api/tables/${selected}/columns`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(data)
        });

        if (res.ok) {
            this.closeModal();
            await this.loadTableSchema(selected);
            await this.loadTableData();
        } else {
            const err = await res.json();
            this.state.error = err.error || 'Failed to add column';
            this.render();
        }
    } catch (e) {
        this.state.error = 'Failed to add column';
        this.render();
    }
},

async renameColumn(oldName) {
    const newName = prompt('New column name:', oldName);
    if (!newName || newName === oldName) return;

    const { selected } = this.state.tables;

    try {
        const res = await fetch(`/_/api/tables/${selected}/columns/${oldName}`, {
            method: 'PATCH',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ new_name: newName })
        });

        if (res.ok) {
            await this.loadTableSchema(selected);
            await this.loadTableData();
        } else {
            const err = await res.json();
            this.state.error = err.error || 'Failed to rename column';
        }
    } catch (e) {
        this.state.error = 'Failed to rename column';
    }
    this.render();
},

async dropColumn(colName) {
    if (!confirm(`Drop column "${colName}"? Data in this column will be lost.`)) return;

    const { selected } = this.state.tables;

    try {
        const res = await fetch(`/_/api/tables/${selected}/columns/${colName}`, {
            method: 'DELETE'
        });

        if (res.ok) {
            await this.loadTableSchema(selected);
            await this.loadTableData();
        } else {
            const err = await res.json();
            this.state.error = err.error || 'Failed to drop column';
        }
    } catch (e) {
        this.state.error = 'Failed to drop column';
    }
    this.render();
},

// Add to renderModals switch:
case 'schema':
    content = this.renderSchemaModal();
    break;
case 'addColumn':
    content = this.renderAddColumnModal();
    break;

renderSchemaModal() {
    const { schema } = this.state.tables;

    return `
        <div class="modal-header">
            <h3>Schema: ${schema.name}</h3>
            <button class="btn-icon" onclick="App.closeModal()">×</button>
        </div>
        <div class="modal-body">
            <table class="schema-table">
                <thead>
                    <tr><th>Column</th><th>Type</th><th>Nullable</th><th>Primary</th><th></th></tr>
                </thead>
                <tbody>
                    ${schema.columns.map(col => `
                        <tr>
                            <td>${col.name}</td>
                            <td>${col.type}</td>
                            <td>${col.nullable ? 'Yes' : 'No'}</td>
                            <td>${col.primary ? 'Yes' : ''}</td>
                            <td>
                                <button class="btn-icon" onclick="App.renameColumn('${col.name}')">Rename</button>
                                ${!col.primary ? `<button class="btn-icon" onclick="App.dropColumn('${col.name}')">Drop</button>` : ''}
                            </td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="App.showAddColumnModal()">+ Add Column</button>
            <button class="btn btn-primary" onclick="App.closeModal()">Done</button>
        </div>
    `;
},

renderAddColumnModal() {
    const { data } = this.state.modal;
    const types = ['uuid', 'text', 'integer', 'boolean', 'timestamptz', 'jsonb', 'numeric', 'bytea'];

    return `
        <div class="modal-header">
            <h3>Add Column</h3>
            <button class="btn-icon" onclick="App.closeModal()">×</button>
        </div>
        <div class="modal-body">
            <div class="form-group">
                <label class="form-label">Column Name</label>
                <input type="text" class="form-input" value="${data.name}"
                    onchange="App.updateModalData('name', this.value)">
            </div>
            <div class="form-group">
                <label class="form-label">Type</label>
                <select class="form-input" onchange="App.updateModalData('type', this.value)">
                    ${types.map(t => `<option value="${t}" ${data.type === t ? 'selected' : ''}>${t}</option>`).join('')}
                </select>
            </div>
            <div class="form-group">
                <label><input type="checkbox" ${data.nullable ? 'checked' : ''}
                    onchange="App.updateModalData('nullable', this.checked)"> Nullable</label>
            </div>
        </div>
        <div class="modal-footer">
            <button class="btn btn-secondary" onclick="App.showSchemaModal()">Back</button>
            <button class="btn btn-primary" onclick="App.addColumn()">Add Column</button>
        </div>
    `;
},
```

**Step 2: Add schema table CSS**

```css
.schema-table {
    width: 100%;
    border-collapse: collapse;
}

.schema-table th, .schema-table td {
    padding: 0.5rem;
    text-align: left;
    border-bottom: 1px solid var(--border);
}

.schema-table th {
    font-weight: 500;
    color: var(--text-secondary);
}
```

**Step 3: Test manually**

Click "Schema" button, view columns, add/rename/drop columns.

**Step 4: Commit**

```bash
git add internal/dashboard/static/app.js internal/dashboard/static/style.css
git commit -m "feat(dashboard): add schema management modal"
```

---

## Task 16: Playwright Tests for Table Management

**Files:**
- Create: `e2e/tests/dashboard/tables.test.ts`

**Step 1: Write comprehensive tests**

```typescript
import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';
import path from 'path';

const TEST_PASSWORD = 'testpassword123';

async function ensureSetup(request: any) {
  const status = await request.get('/_/api/auth/status');
  const data = await status.json();
  if (data.needs_setup) {
    await request.post('/_/api/auth/setup', { data: { password: TEST_PASSWORD } });
  }
}

async function login(page: any, context: any) {
  await context.clearCookies();
  await page.goto('/_/');
  await page.waitForFunction(() => {
    const app = document.getElementById('app');
    return app && !app.innerHTML.includes('Loading');
  }, { timeout: 5000 });

  const needsLogin = await page.locator('.auth-container').isVisible();
  if (needsLogin) {
    await page.locator('#password').fill(TEST_PASSWORD);
    await page.getByRole('button', { name: 'Sign In' }).click();
  }
  await page.waitForSelector('.sidebar', { timeout: 5000 });
}

async function cleanupTestTables(request: any) {
  const tables = await request.get('/_/api/tables');
  const list = await tables.json();
  for (const t of list) {
    if (t.name.startsWith('test_')) {
      await request.delete(`/_/api/tables/${t.name}`);
    }
  }
}

test.describe('Table Management', () => {
  test.beforeAll(async ({ request }) => {
    await ensureSetup(request);
  });

  test.beforeEach(async ({ request }) => {
    await cleanupTestTables(request);
  });

  test('displays table list panel', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await expect(page.locator('.table-list-panel')).toBeVisible();
    await expect(page.locator('.panel-header')).toContainText('Tables');
  });

  test('creates new table via modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await login(page, context);

    await page.getByRole('button', { name: '+ New' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input[placeholder="my_table"]').fill('test_products');
    await page.locator('.modal .column-row input[placeholder="column_name"]').first().fill('id');

    await page.getByRole('button', { name: 'Create Table' }).click();

    await expect(page.locator('.table-list-item').filter({ hasText: 'test_products' })).toBeVisible({ timeout: 5000 });
  });

  test('selects table and shows data grid', async ({ page, request, context }) => {
    await ensureSetup(request);
    // Create table via API
    await request.post('/_/api/tables', {
      data: { name: 'test_items', columns: [{ name: 'id', type: 'text', primary: true }] }
    });

    await login(page, context);

    await page.locator('.table-list-item').filter({ hasText: 'test_items' }).click();

    await expect(page.locator('.data-grid')).toBeVisible({ timeout: 5000 });
    await expect(page.locator('.table-toolbar h2')).toContainText('test_items');
  });

  test('adds row via modal', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_add_row', columns: [
        { name: 'id', type: 'text', primary: true },
        { name: 'name', type: 'text', nullable: true }
      ]}
    });

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_add_row' }).click();
    await page.waitForSelector('.data-grid');

    await page.getByRole('button', { name: '+ Add Row' }).click();
    await page.waitForSelector('.modal');

    await page.locator('.modal input').first().fill('row-1');
    await page.locator('.modal input').nth(1).fill('Test Name');
    await page.getByRole('button', { name: 'Add' }).click();

    await expect(page.locator('.data-grid')).toContainText('row-1');
  });

  test('inline edits cell', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_inline', columns: [
        { name: 'id', type: 'text', primary: true },
        { name: 'value', type: 'text', nullable: true }
      ]}
    });
    await request.post('/_/api/data/test_inline', { data: { id: 'edit-1', value: 'original' } });

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_inline' }).click();
    await page.waitForSelector('.data-grid');

    // Click cell to edit
    await page.locator('.data-cell').filter({ hasText: 'original' }).click();
    await page.locator('.cell-input').fill('updated');
    await page.locator('.cell-input').press('Enter');

    await expect(page.locator('.data-grid')).toContainText('updated');
  });

  test('deletes table', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_delete_me', columns: [{ name: 'id', type: 'text', primary: true }] }
    });

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_delete_me' }).click();
    await page.waitForSelector('.data-grid');

    page.on('dialog', dialog => dialog.accept());
    await page.getByRole('button', { name: 'Delete Table' }).click();

    await expect(page.locator('.table-list-item').filter({ hasText: 'test_delete_me' })).not.toBeVisible({ timeout: 5000 });
  });

  test('pagination works', async ({ page, request, context }) => {
    await ensureSetup(request);
    await request.post('/_/api/tables', {
      data: { name: 'test_pagination', columns: [{ name: 'id', type: 'integer', primary: true }] }
    });
    // Insert 30 rows
    for (let i = 1; i <= 30; i++) {
      await request.post('/_/api/data/test_pagination', { data: { id: i } });
    }

    await login(page, context);
    await page.locator('.table-list-item').filter({ hasText: 'test_pagination' }).click();
    await page.waitForSelector('.data-grid');

    await expect(page.locator('.pagination-info')).toContainText('30 rows');
    await expect(page.locator('.pagination-info')).toContainText('Page 1');

    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.locator('.pagination-info')).toContainText('Page 2');
  });
});
```

**Step 2: Run tests**

```bash
cd e2e && npm run test:dashboard
```

Expected: All tests pass

**Step 3: Commit**

```bash
git add e2e/tests/dashboard/tables.test.ts
git commit -m "test(dashboard): add Playwright tests for table management"
```

---

## Task 17: Final Integration and Cleanup

**Step 1: Run all Go tests**

```bash
go test ./internal/dashboard/... -v
```

Expected: All dashboard tests pass

**Step 2: Run all Playwright tests**

```bash
cd e2e && npm run test:dashboard
```

Expected: All tests pass

**Step 3: Final commit**

```bash
git add -A
git commit -m "feat(dashboard): complete Phase 3 table management implementation"
```
