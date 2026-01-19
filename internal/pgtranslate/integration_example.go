package pgtranslate

// Example integration into the dashboard SQL browser
// This shows how handleExecuteSQL would be modified

/*
// In internal/dashboard/handler.go:

func (h *Handler) handleExecuteSQL(w http.ResponseWriter, r *http.Request) {
	var req SQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Query) == "" {
		http.Error(w, `{"error": "Query cannot be empty"}`, http.StatusBadRequest)
		return
	}

	// NEW: PostgreSQL syntax translation
	translator := pgtranslate.NewTranslator()
	translatedQuery, wasTranslated := translator.TranslateWithFallback(req.Query)

	// Detect query type (using translated query)
	queryType := detectQueryType(translatedQuery)

	startTime := time.Now()
	var response SQLResponse
	response.Type = queryType
	response.OriginalQuery = req.Query  // NEW: Include original for debugging
	response.TranslatedQuery = translatedQuery  // NEW: Show what was executed
	response.WasTranslated = wasTranslated  // NEW: Indicate if translation occurred

	if queryType == "SELECT" || queryType == "PRAGMA" {
		// Execute the TRANSLATED query
		rows, err := h.db.Query(translatedQuery)
		if err != nil {
			response.Error = err.Error()
			response.ExecutionTimeMs = time.Since(startTime).Milliseconds()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(response)
			return
		}
		defer rows.Close()

		// ... rest of existing code ...
	} else {
		// For non-SELECT queries (INSERT, UPDATE, DELETE, etc.)
		result, err := h.db.Exec(translatedQuery)  // Use translated query
		// ... rest of existing code ...
	}
}

// NEW: Update SQLResponse type to include translation info
type SQLResponse struct {
	Type             string          `json:"type"`
	Columns          []string        `json:"columns,omitempty"`
	Rows             [][]interface{} `json:"rows,omitempty"`
	RowCount         int             `json:"row_count"`
	RowsAffected     int64           `json:"rows_affected,omitempty"`
	LastInsertID     int64           `json:"last_insert_id,omitempty"`
	ExecutionTimeMs  int64           `json:"execution_time_ms"`
	Error            string          `json:"error,omitempty"`
	OriginalQuery    string          `json:"original_query,omitempty"`    // NEW
	TranslatedQuery  string          `json:"translated_query,omitempty"`  // NEW
	WasTranslated    bool            `json:"was_translated"`              // NEW
}
*/

// Frontend changes in internal/dashboard/static/app.js:
/*
async executeSQLQuery(query) {
    // ... existing code ...

    const data = await res.json();

    if (data.error) {
        this.state.sqlBrowser.error = data.error;

        // NEW: Show if translation occurred
        if (data.was_translated) {
            this.state.sqlBrowser.error += '\n\nTranslated query: ' + data.translated_query;
        }
    } else {
        this.state.sqlBrowser.results = data;

        // NEW: Show translation info in UI
        if (data.was_translated) {
            console.log('PostgreSQL query translated to SQLite:');
            console.log('Original:', data.original_query);
            console.log('Translated:', data.translated_query);
        }

        // Add to history
        const historyEntry = {
            id: Date.now().toString(),
            query: query,
            translatedQuery: data.translated_query,  // NEW
            wasTranslated: data.was_translated,      // NEW
            rowCount: data.row_count,
            type: data.type,
            timestamp: Date.now()
        };

        // ... rest of existing code ...
    }
}
*/

// UI Enhancement: Add a toggle in the SQL browser
/*
<div class="sql-browser-header">
    <h2>SQL Browser</h2>
    <div class="sql-options">
        <label>
            <input type="checkbox" id="postgres-mode" checked>
            PostgreSQL Syntax Mode
        </label>
        <button onclick="App.showSQLHelp()">Syntax Help</button>
    </div>
</div>

// When checkbox is unchecked, bypass translation:
const postgresMode = document.getElementById('postgres-mode').checked;
const bodyData = {
    query: query,
    postgres_mode: postgresMode  // NEW: Send preference to backend
};
*/
