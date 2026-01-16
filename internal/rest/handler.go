// internal/rest/handler.go
package rest

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markb/sblite/internal/db"
	"github.com/markb/sblite/internal/rls"
)

type Handler struct {
	db       *db.DB
	enforcer *rls.Enforcer
}

func NewHandler(database *db.DB, enforcer *rls.Enforcer) *Handler {
	return &Handler{db: database, enforcer: enforcer}
}

// GetAuthContextFromRequest extracts auth context from request for RLS
// The server middleware stores claims in context with string key "claims"
func GetAuthContextFromRequest(r *http.Request) *rls.AuthContext {
	claimsValue := r.Context().Value("claims")
	if claimsValue == nil {
		return nil
	}

	claims, ok := claimsValue.(*jwt.MapClaims)
	if !ok {
		return nil
	}

	ctx := &rls.AuthContext{
		Claims: *claims,
	}

	if sub, ok := (*claims)["sub"].(string); ok {
		ctx.UserID = sub
	}
	if email, ok := (*claims)["email"].(string); ok {
		ctx.Email = email
	}
	if role, ok := (*claims)["role"].(string); ok {
		ctx.Role = role
	}

	return ctx
}

// Reserved query params that are not filters
var reservedParams = map[string]bool{
	"select": true,
	"order":  true,
	"limit":  true,
	"offset": true,
	"or":     true, // handled separately as logical filter
	"and":    true, // handled separately as logical filter
}

func (h *Handler) parseQueryParams(r *http.Request) Query {
	q := Query{
		Table:  chi.URLParam(r, "table"),
		Select: ParseSelect(r.URL.Query().Get("select")),
		Order:  ParseOrder(r.URL.Query().Get("order")),
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		q.Limit, _ = strconv.Atoi(limit)
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		q.Offset, _ = strconv.Atoi(offset)
	}

	// Parse filters from query params
	for key, values := range r.URL.Query() {
		if reservedParams[key] {
			continue
		}
		for _, value := range values {
			filterStr := fmt.Sprintf("%s=%s", key, value)
			if filter, err := ParseFilter(filterStr); err == nil {
				q.Filters = append(q.Filters, filter)
			}
		}
	}

	// Parse logical filters (or/and)
	for _, orValue := range r.URL.Query()["or"] {
		if lf, err := ParseLogicalFilter("or", orValue); err == nil {
			q.LogicalFilters = append(q.LogicalFilters, lf)
		}
	}
	for _, andValue := range r.URL.Query()["and"] {
		if lf, err := ParseLogicalFilter("and", andValue); err == nil {
			q.LogicalFilters = append(q.LogicalFilters, lf)
		}
	}

	return q
}

func (h *Handler) HandleSelect(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	// Apply RLS conditions if enforcer is configured
	if h.enforcer != nil {
		authCtx := GetAuthContextFromRequest(r)
		rlsCondition, err := h.enforcer.GetSelectConditions(q.Table, authCtx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "rls_error", "Failed to apply RLS")
			return
		}
		if rlsCondition != "" {
			q.RLSCondition = rlsCondition
		}
	}

	// Check for single() modifier via Accept header
	accept := r.Header.Get("Accept")
	wantSingle := strings.Contains(accept, "application/vnd.pgrst.object+json")

	sqlStr, args := BuildSelectQuery(q)
	rows, err := h.db.Query(sqlStr, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "query_error", err.Error())
		return
	}
	defer rows.Close()

	results, err := h.scanRows(rows)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "scan_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Handle single() modifier
	if wantSingle {
		if len(results) == 0 {
			h.writeError(w, http.StatusNotAcceptable, "PGRST116", "JSON object requested, multiple (or no) rows returned")
			return
		}
		if len(results) > 1 {
			h.writeError(w, http.StatusNotAcceptable, "PGRST116", "JSON object requested, multiple (or no) rows returned")
			return
		}
		json.NewEncoder(w).Encode(results[0])
		return
	}

	json.NewEncoder(w).Encode(results)
}

func (h *Handler) HandleInsert(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")
	selectCols := ParseSelect(r.URL.Query().Get("select"))
	prefer := r.Header.Get("Prefer")
	returnRepresentation := strings.Contains(prefer, "return=representation")
	isUpsert := strings.Contains(prefer, "resolution=merge-duplicates")

	// Try to decode as array first (bulk insert), then as single object
	var records []map[string]any
	decoder := json.NewDecoder(r.Body)

	// Peek at first character to determine if array or object
	var rawData json.RawMessage
	if err := decoder.Decode(&rawData); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	// Try array first
	if err := json.Unmarshal(rawData, &records); err != nil {
		// Try single object
		var single map[string]any
		if err := json.Unmarshal(rawData, &single); err != nil {
			h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
			return
		}
		records = []map[string]any{single}
	}

	var insertedIDs []int64
	for _, data := range records {
		var sqlStr string
		var args []any
		if isUpsert {
			sqlStr, args = BuildUpsertQuery(table, data)
		} else {
			sqlStr, args = BuildInsertQuery(table, data)
		}
		result, err := h.db.Exec(sqlStr, args...)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "insert_error", err.Error())
			return
		}
		if returnRepresentation {
			// For upsert, we may not get a new ID if it was an update
			// Try to get the ID from the data itself
			if id, ok := data["id"]; ok {
				switch v := id.(type) {
				case float64:
					insertedIDs = append(insertedIDs, int64(v))
				case int64:
					insertedIDs = append(insertedIDs, v)
				case int:
					insertedIDs = append(insertedIDs, int64(v))
				}
			} else {
				lastID, _ := result.LastInsertId()
				insertedIDs = append(insertedIDs, lastID)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// Return representation if requested
	if returnRepresentation && len(insertedIDs) > 0 {
		results := h.selectByIDs(table, selectCols, insertedIDs)
		json.NewEncoder(w).Encode(results)
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"inserted": true})
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)
	prefer := r.Header.Get("Prefer")
	returnRepresentation := strings.Contains(prefer, "return=representation")

	// Apply RLS
	if h.enforcer != nil {
		authCtx := GetAuthContextFromRequest(r)
		rlsCondition, err := h.enforcer.GetUpdateConditions(q.Table, authCtx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "rls_error", "Failed to apply RLS")
			return
		}
		if rlsCondition != "" {
			q.RLSCondition = rlsCondition
		}
	}

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	// If returning representation, first get the IDs of rows to be updated
	var affectedIDs []int64
	if returnRepresentation {
		affectedIDs = h.getMatchingIDsWithRLS(q)
	}

	sqlStr, args := BuildUpdateQuery(q, data)
	_, err := h.db.Exec(sqlStr, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Return representation if requested
	if returnRepresentation {
		if len(affectedIDs) > 0 {
			results := h.selectByIDs(q.Table, q.Select, affectedIDs)
			json.NewEncoder(w).Encode(results)
		} else {
			json.NewEncoder(w).Encode([]map[string]any{})
		}
		return
	}

	json.NewEncoder(w).Encode(map[string]any{"updated": true})
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)
	prefer := r.Header.Get("Prefer")
	returnRepresentation := strings.Contains(prefer, "return=representation")

	// Apply RLS
	if h.enforcer != nil {
		authCtx := GetAuthContextFromRequest(r)
		rlsCondition, err := h.enforcer.GetDeleteConditions(q.Table, authCtx)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, "rls_error", "Failed to apply RLS")
			return
		}
		if rlsCondition != "" {
			q.RLSCondition = rlsCondition
		}
	}

	if len(q.Filters) == 0 && len(q.LogicalFilters) == 0 {
		h.writeError(w, http.StatusBadRequest, "missing_filter", "DELETE requires at least one filter")
		return
	}

	// If returning representation, first get the data of rows to be deleted
	var deletedRows []map[string]any
	if returnRepresentation {
		deletedRows = h.selectMatchingWithRLS(q)
	}

	sqlStr, args := BuildDeleteQuery(q)
	_, err := h.db.Exec(sqlStr, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "delete_error", err.Error())
		return
	}

	// Return representation if requested
	if returnRepresentation {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(deletedRows)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) scanRows(rows *sql.Rows) ([]map[string]any, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]any)
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	if results == nil {
		results = []map[string]any{}
	}

	return results, nil
}

func (h *Handler) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   code,
		"message": message,
	})
}

// selectByIDs retrieves rows by their IDs
func (h *Handler) selectByIDs(table string, selectCols []string, ids []int64) []map[string]any {
	if len(ids) == 0 {
		return []map[string]any{}
	}

	// Build IN clause for IDs
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	q := Query{Table: table, Select: selectCols}
	sqlStr, _ := BuildSelectQuery(q)
	sqlStr += fmt.Sprintf(" WHERE \"id\" IN (%s)", strings.Join(placeholders, ", "))

	rows, err := h.db.Query(sqlStr, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()

	results, _ := h.scanRows(rows)
	return results
}

// getMatchingIDs returns IDs of rows matching the filters
func (h *Handler) getMatchingIDs(table string, filters []Filter) []int64 {
	q := Query{Table: table, Select: []string{"id"}, Filters: filters}
	sqlStr, args := BuildSelectQuery(q)

	rows, err := h.db.Query(sqlStr, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// selectMatching retrieves rows matching the filters
func (h *Handler) selectMatching(table string, selectCols []string, filters []Filter) []map[string]any {
	q := Query{Table: table, Select: selectCols, Filters: filters}
	sqlStr, args := BuildSelectQuery(q)

	rows, err := h.db.Query(sqlStr, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()

	results, _ := h.scanRows(rows)
	return results
}

// getMatchingIDsWithRLS returns IDs of rows matching the query (including RLS condition)
func (h *Handler) getMatchingIDsWithRLS(q Query) []int64 {
	selectQ := Query{Table: q.Table, Select: []string{"id"}, Filters: q.Filters, LogicalFilters: q.LogicalFilters, RLSCondition: q.RLSCondition}
	sqlStr, args := BuildSelectQuery(selectQ)

	rows, err := h.db.Query(sqlStr, args...)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// selectMatchingWithRLS retrieves rows matching the query (including RLS condition)
func (h *Handler) selectMatchingWithRLS(q Query) []map[string]any {
	selectQ := Query{Table: q.Table, Select: q.Select, Filters: q.Filters, LogicalFilters: q.LogicalFilters, RLSCondition: q.RLSCondition}
	sqlStr, args := BuildSelectQuery(selectQ)

	rows, err := h.db.Query(sqlStr, args...)
	if err != nil {
		return []map[string]any{}
	}
	defer rows.Close()

	results, _ := h.scanRows(rows)
	return results
}
