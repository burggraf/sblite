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
	"github.com/markb/sblite/internal/db"
)

type Handler struct {
	db *db.DB
}

func NewHandler(database *db.DB) *Handler {
	return &Handler{db: database}
}

// Reserved query params that are not filters
var reservedParams = map[string]bool{
	"select": true,
	"order":  true,
	"limit":  true,
	"offset": true,
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

	return q
}

func (h *Handler) HandleSelect(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	sql, args := BuildSelectQuery(q)
	rows, err := h.db.Query(sql, args...)
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
	json.NewEncoder(w).Encode(results)
}

func (h *Handler) HandleInsert(w http.ResponseWriter, r *http.Request) {
	table := chi.URLParam(r, "table")

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	sql, args := BuildInsertQuery(table, data)
	result, err := h.db.Exec(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "insert_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)

	// Return representation if requested
	prefer := r.Header.Get("Prefer")
	if strings.Contains(prefer, "return=representation") {
		lastID, _ := result.LastInsertId()
		q := Query{Table: table, Select: []string{"*"}, Filters: []Filter{{Column: "id", Operator: "eq", Value: fmt.Sprintf("%d", lastID)}}}
		selectSQL, selectArgs := BuildSelectQuery(q)
		rows, _ := h.db.Query(selectSQL, selectArgs...)
		defer rows.Close()
		results, _ := h.scanRows(rows)
		if len(results) > 0 {
			json.NewEncoder(w).Encode(results[0])
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]any{"inserted": true})
}

func (h *Handler) HandleUpdate(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body")
		return
	}

	sql, args := BuildUpdateQuery(q.Table, data, q.Filters)
	_, err := h.db.Exec(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"updated": true})
}

func (h *Handler) HandleDelete(w http.ResponseWriter, r *http.Request) {
	q := h.parseQueryParams(r)

	if len(q.Filters) == 0 {
		h.writeError(w, http.StatusBadRequest, "missing_filter", "DELETE requires at least one filter")
		return
	}

	sql, args := BuildDeleteQuery(q.Table, q.Filters)
	_, err := h.db.Exec(sql, args...)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "delete_error", err.Error())
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
