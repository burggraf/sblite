// Package fts provides full-text search functionality using SQLite FTS5.
// It manages FTS indexes, handles query translation, and keeps indexes synchronized
// with source tables via triggers.
package fts

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// Index represents a full-text search index configuration.
type Index struct {
	TableName string   `json:"table_name"`
	IndexName string   `json:"index_name"`
	Columns   []string `json:"columns"`
	Tokenizer string   `json:"tokenizer"`
	CreatedAt string   `json:"created_at,omitempty"`
}

// Manager handles FTS index operations.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new FTS manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// ftsTableName returns the virtual table name for an FTS index.
func ftsTableName(tableName, indexName string) string {
	return fmt.Sprintf("%s_fts_%s", tableName, indexName)
}

// CreateIndex creates a new FTS5 index on the specified columns.
func (m *Manager) CreateIndex(tableName, indexName string, columns []string, tokenizer string) error {
	if len(columns) == 0 {
		return fmt.Errorf("at least one column required for FTS index")
	}

	if tokenizer == "" {
		tokenizer = "unicode61"
	}

	// Validate tokenizer
	validTokenizers := map[string]bool{
		"unicode61": true,
		"porter":    true,
		"ascii":     true,
		"trigram":   true,
	}
	if !validTokenizers[tokenizer] {
		return fmt.Errorf("invalid tokenizer: %s (valid: unicode61, porter, ascii, trigram)", tokenizer)
	}

	// Check if table exists
	var tableExists int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, tableName).Scan(&tableExists)
	if err != nil {
		return fmt.Errorf("checking table existence: %w", err)
	}
	if tableExists == 0 {
		return fmt.Errorf("table %q does not exist", tableName)
	}

	// Check if index already exists
	var indexExists int
	err = m.db.QueryRow(`SELECT COUNT(*) FROM _fts_indexes WHERE table_name=? AND index_name=?`, tableName, indexName).Scan(&indexExists)
	if err != nil {
		return fmt.Errorf("checking index existence: %w", err)
	}
	if indexExists > 0 {
		return fmt.Errorf("FTS index %q already exists on table %q", indexName, tableName)
	}

	// Verify columns exist in the table
	for _, col := range columns {
		var colExists int
		err = m.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, tableName, col).Scan(&colExists)
		if err != nil {
			return fmt.Errorf("checking column %q: %w", col, err)
		}
		if colExists == 0 {
			return fmt.Errorf("column %q does not exist in table %q", col, tableName)
		}
	}

	// Find the primary key column (needed for content_rowid)
	var pkColumn string
	rows, err := m.db.Query(`SELECT name FROM pragma_table_info(?) WHERE pk = 1`, tableName)
	if err != nil {
		return fmt.Errorf("finding primary key: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&pkColumn); err != nil {
			return fmt.Errorf("scanning primary key: %w", err)
		}
	}
	if pkColumn == "" {
		return fmt.Errorf("table %q must have a primary key for FTS indexing", tableName)
	}

	ftsTable := ftsTableName(tableName, indexName)

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Create FTS5 virtual table with external content
	// Using content= for external content table to avoid data duplication
	columnList := strings.Join(columns, ", ")
	createSQL := fmt.Sprintf(
		`CREATE VIRTUAL TABLE %q USING fts5(%s, content=%q, content_rowid=%q, tokenize=%q)`,
		ftsTable, columnList, tableName, pkColumn, tokenizer,
	)
	if _, err := tx.Exec(createSQL); err != nil {
		return fmt.Errorf("creating FTS table: %w", err)
	}

	// Create sync triggers
	if err := m.createTriggers(tx, tableName, ftsTable, pkColumn, columns); err != nil {
		return fmt.Errorf("creating triggers: %w", err)
	}

	// Populate initial index using the 'rebuild' command for external content tables
	rebuildSQL := fmt.Sprintf(`INSERT INTO %q(%q) VALUES('rebuild')`, ftsTable, ftsTable)
	if _, err := tx.Exec(rebuildSQL); err != nil {
		return fmt.Errorf("populating FTS index: %w", err)
	}

	// Record index in metadata
	columnsJSON, _ := json.Marshal(columns)
	_, err = tx.Exec(
		`INSERT INTO _fts_indexes (table_name, index_name, columns, tokenizer) VALUES (?, ?, ?, ?)`,
		tableName, indexName, string(columnsJSON), tokenizer,
	)
	if err != nil {
		return fmt.Errorf("recording index metadata: %w", err)
	}

	return tx.Commit()
}

// createTriggers creates the sync triggers for an FTS index.
func (m *Manager) createTriggers(tx *sql.Tx, tableName, ftsTable, pkColumn string, columns []string) error {
	// Quote column names for SQL
	quotedColumns := make([]string, len(columns))
	for i, col := range columns {
		quotedColumns[i] = fmt.Sprintf(`"%s"`, col)
	}
	columnList := strings.Join(quotedColumns, ", ")

	// Build NEW."col", NEW."col2", ... for INSERT
	newCols := make([]string, len(columns))
	for i, col := range columns {
		newCols[i] = fmt.Sprintf(`NEW."%s"`, col)
	}
	newColList := strings.Join(newCols, ", ")

	// Build OLD."col", OLD."col2", ... for DELETE
	oldCols := make([]string, len(columns))
	for i, col := range columns {
		oldCols[i] = fmt.Sprintf(`OLD."%s"`, col)
	}
	oldColList := strings.Join(oldCols, ", ")

	// Trigger names don't need to be quoted, but table/column names do
	triggerNameBase := ftsTable

	// After INSERT trigger
	insertTrigger := fmt.Sprintf(`
		CREATE TRIGGER "%s_ai" AFTER INSERT ON "%s" BEGIN
			INSERT INTO "%s"(rowid, %s) VALUES (NEW."%s", %s);
		END`,
		triggerNameBase, tableName,
		ftsTable, columnList, pkColumn, newColList,
	)
	if _, err := tx.Exec(insertTrigger); err != nil {
		return fmt.Errorf("creating INSERT trigger: %w", err)
	}

	// After DELETE trigger
	deleteTrigger := fmt.Sprintf(`
		CREATE TRIGGER "%s_ad" AFTER DELETE ON "%s" BEGIN
			INSERT INTO "%s"("%s", rowid, %s) VALUES ('delete', OLD."%s", %s);
		END`,
		triggerNameBase, tableName,
		ftsTable, ftsTable, columnList, pkColumn, oldColList,
	)
	if _, err := tx.Exec(deleteTrigger); err != nil {
		return fmt.Errorf("creating DELETE trigger: %w", err)
	}

	// After UPDATE trigger (delete old, insert new)
	updateTrigger := fmt.Sprintf(`
		CREATE TRIGGER "%s_au" AFTER UPDATE ON "%s" BEGIN
			INSERT INTO "%s"("%s", rowid, %s) VALUES ('delete', OLD."%s", %s);
			INSERT INTO "%s"(rowid, %s) VALUES (NEW."%s", %s);
		END`,
		triggerNameBase, tableName,
		ftsTable, ftsTable, columnList, pkColumn, oldColList,
		ftsTable, columnList, pkColumn, newColList,
	)
	if _, err := tx.Exec(updateTrigger); err != nil {
		return fmt.Errorf("creating UPDATE trigger: %w", err)
	}

	return nil
}

// DropIndex removes an FTS index and its triggers.
func (m *Manager) DropIndex(tableName, indexName string) error {
	// Check if index exists
	var exists int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM _fts_indexes WHERE table_name=? AND index_name=?`, tableName, indexName).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking index existence: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("FTS index %q does not exist on table %q", indexName, tableName)
	}

	ftsTable := ftsTableName(tableName, indexName)

	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop triggers
	triggers := []string{
		fmt.Sprintf("%s_ai", ftsTable),
		fmt.Sprintf("%s_ad", ftsTable),
		fmt.Sprintf("%s_au", ftsTable),
	}
	for _, trigger := range triggers {
		if _, err := tx.Exec(fmt.Sprintf(`DROP TRIGGER IF EXISTS %q`, trigger)); err != nil {
			return fmt.Errorf("dropping trigger %s: %w", trigger, err)
		}
	}

	// Drop FTS table
	if _, err := tx.Exec(fmt.Sprintf(`DROP TABLE IF EXISTS %q`, ftsTable)); err != nil {
		return fmt.Errorf("dropping FTS table: %w", err)
	}

	// Remove from metadata
	if _, err := tx.Exec(`DELETE FROM _fts_indexes WHERE table_name=? AND index_name=?`, tableName, indexName); err != nil {
		return fmt.Errorf("removing index metadata: %w", err)
	}

	return tx.Commit()
}

// RebuildIndex rebuilds an FTS index from scratch.
func (m *Manager) RebuildIndex(tableName, indexName string) error {
	// Verify index exists
	_, err := m.GetIndex(tableName, indexName)
	if err != nil {
		return fmt.Errorf("getting index: %w", err)
	}

	ftsTable := ftsTableName(tableName, indexName)

	// For external content FTS5 tables, use the 'rebuild' command
	rebuildSQL := fmt.Sprintf(`INSERT INTO %q(%q) VALUES('rebuild')`, ftsTable, ftsTable)
	if _, err := m.db.Exec(rebuildSQL); err != nil {
		return fmt.Errorf("rebuilding FTS index: %w", err)
	}

	return nil
}

// GetIndex returns the configuration for a specific FTS index.
func (m *Manager) GetIndex(tableName, indexName string) (*Index, error) {
	var idx Index
	var columnsJSON string

	err := m.db.QueryRow(
		`SELECT table_name, index_name, columns, tokenizer, created_at FROM _fts_indexes WHERE table_name=? AND index_name=?`,
		tableName, indexName,
	).Scan(&idx.TableName, &idx.IndexName, &columnsJSON, &idx.Tokenizer, &idx.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("FTS index %q not found on table %q", indexName, tableName)
	}
	if err != nil {
		return nil, fmt.Errorf("querying index: %w", err)
	}

	if err := json.Unmarshal([]byte(columnsJSON), &idx.Columns); err != nil {
		return nil, fmt.Errorf("parsing columns: %w", err)
	}

	return &idx, nil
}

// ListIndexes returns all FTS indexes for a table.
func (m *Manager) ListIndexes(tableName string) ([]*Index, error) {
	query := `SELECT table_name, index_name, columns, tokenizer, created_at FROM _fts_indexes`
	args := []any{}
	if tableName != "" {
		query += ` WHERE table_name = ?`
		args = append(args, tableName)
	}
	query += ` ORDER BY table_name, index_name`

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying indexes: %w", err)
	}
	defer rows.Close()

	var indexes []*Index
	for rows.Next() {
		var idx Index
		var columnsJSON string
		if err := rows.Scan(&idx.TableName, &idx.IndexName, &columnsJSON, &idx.Tokenizer, &idx.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning index: %w", err)
		}
		if err := json.Unmarshal([]byte(columnsJSON), &idx.Columns); err != nil {
			return nil, fmt.Errorf("parsing columns: %w", err)
		}
		indexes = append(indexes, &idx)
	}

	return indexes, rows.Err()
}

// HasIndex checks if a table has an FTS index.
func (m *Manager) HasIndex(tableName, indexName string) (bool, error) {
	var count int
	err := m.db.QueryRow(`SELECT COUNT(*) FROM _fts_indexes WHERE table_name=? AND index_name=?`, tableName, indexName).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking index: %w", err)
	}
	return count > 0, nil
}

// FindIndexForColumn finds an FTS index that includes the given column.
func (m *Manager) FindIndexForColumn(tableName, columnName string) (*Index, error) {
	indexes, err := m.ListIndexes(tableName)
	if err != nil {
		return nil, err
	}

	for _, idx := range indexes {
		for _, col := range idx.Columns {
			if col == columnName {
				return idx, nil
			}
		}
	}

	return nil, nil // No index found (not an error)
}

// GetFTSTableName returns the FTS virtual table name for an index.
func GetFTSTableName(tableName, indexName string) string {
	return ftsTableName(tableName, indexName)
}
