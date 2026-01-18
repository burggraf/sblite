// internal/rest/relations.go
package rest

import (
	"database/sql"
	"fmt"
	"sync"
	"unicode"
)

// Relationship describes a foreign key relationship between tables.
type Relationship struct {
	Name          string // Related table name (used as the key in select queries)
	LocalColumn   string // FK column in this table (for many-to-one) or referenced column (for one-to-many)
	ForeignTable  string // Referenced table
	ForeignColumn string // Referenced column (usually "id")
	Type          string // "many-to-one" or "one-to-many"
}

// RelationshipCache provides thread-safe caching of table relationships
// detected via SQLite PRAGMA foreign_key_list.
type RelationshipCache struct {
	db    *sql.DB
	cache map[string][]Relationship
	mu    sync.RWMutex
}

// NewRelationshipCache creates a new RelationshipCache with the given database connection.
func NewRelationshipCache(db *sql.DB) *RelationshipCache {
	return &RelationshipCache{
		db:    db,
		cache: make(map[string][]Relationship),
	}
}

// isValidTableName validates that a table name contains only safe characters
// (letters, digits, and underscores) to prevent SQL injection in PRAGMA queries.
func isValidTableName(name string) bool {
	if len(name) == 0 {
		return false
	}
	for _, r := range name {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}

// GetRelationships returns all relationships for a table, using cache if available.
// It detects both many-to-one (this table has FK) and one-to-many (other tables have FK to this table).
func (rc *RelationshipCache) GetRelationships(table string) ([]Relationship, error) {
	// Validate table name to prevent SQL injection in PRAGMA queries
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	// Check cache first (read lock)
	rc.mu.RLock()
	if rels, ok := rc.cache[table]; ok {
		rc.mu.RUnlock()
		return rels, nil
	}
	rc.mu.RUnlock()

	// Query foreign keys for this table (many-to-one relationships)
	// PRAGMA foreign_key_list returns: id, seq, table, from, to, on_update, on_delete, match
	rows, err := rc.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys for %s: %w", table, err)
	}
	defer rows.Close()

	var rels []Relationship
	for rows.Next() {
		var id, seq int
		var foreignTable, localCol, foreignCol, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &foreignTable, &localCol, &foreignCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}

		rels = append(rels, Relationship{
			Name:          foreignTable,
			LocalColumn:   localCol,
			ForeignTable:  foreignTable,
			ForeignColumn: foreignCol,
			Type:          "many-to-one",
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign keys: %w", err)
	}

	// Also find reverse relationships (one-to-many)
	reverseRels, err := rc.findReverseRelationships(table)
	if err != nil {
		return nil, err
	}
	rels = append(rels, reverseRels...)

	// Cache the results (write lock)
	rc.mu.Lock()
	rc.cache[table] = rels
	rc.mu.Unlock()

	return rels, nil
}

// findReverseRelationships finds tables that have foreign keys pointing to the given table.
// These are one-to-many relationships from the perspective of the given table.
func (rc *RelationshipCache) findReverseRelationships(table string) ([]Relationship, error) {
	var rels []Relationship

	// Get all tables in the database
	tables, err := rc.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer tables.Close()

	var tableNames []string
	for tables.Next() {
		var tableName string
		if err := tables.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	if err := tables.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	// Check each table's foreign keys to see if they reference our table
	for _, otherTable := range tableNames {
		// Note: We don't skip self-referencing tables because a table can have
		// an FK to itself (e.g., employees.manager_id -> employees.id)

		// Table names from sqlite_master should always be valid, but validate
		// to be defensive against any edge cases
		if !isValidTableName(otherTable) {
			continue
		}

		fks, err := rc.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", otherTable))
		if err != nil {
			// Skip tables we can't query foreign keys for (e.g., permission issues)
			// This is expected in some scenarios and not worth failing the entire operation
			continue
		}

		var scanErr error
		for fks.Next() {
			var id, seq int
			var foreignTable, localCol, foreignCol, onUpdate, onDelete, match string
			if err := fks.Scan(&id, &seq, &foreignTable, &localCol, &foreignCol, &onUpdate, &onDelete, &match); err != nil {
				// Record scan error but continue processing remaining rows
				scanErr = err
				continue
			}

			if foreignTable == table {
				rels = append(rels, Relationship{
					Name:          otherTable,
					LocalColumn:   foreignCol, // The column in our table being referenced
					ForeignTable:  otherTable,
					ForeignColumn: localCol, // The FK column in the other table
					Type:          "one-to-many",
				})
			}
		}

		// Check for iteration errors after the loop
		if err := fks.Err(); err != nil {
			fks.Close()
			return nil, fmt.Errorf("error iterating foreign keys for %s: %w", otherTable, err)
		}
		fks.Close()

		// If we had scan errors but no iteration errors, we can still continue
		// but log that some rows may have been skipped
		_ = scanErr // Acknowledge the error was captured; rows were skipped
	}

	return rels, nil
}

// InvalidateCache clears the cache for a specific table or all tables if table is empty.
func (rc *RelationshipCache) InvalidateCache(table string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if table == "" {
		rc.cache = make(map[string][]Relationship)
	} else {
		delete(rc.cache, table)
	}
}

// FindRelationship looks up a specific relationship by name for a given table.
// Returns nil if no relationship with that name exists.
func (rc *RelationshipCache) FindRelationship(table, relationName string) (*Relationship, error) {
	rels, err := rc.GetRelationships(table)
	if err != nil {
		return nil, err
	}

	for _, rel := range rels {
		if rel.Name == relationName {
			return &rel, nil
		}
	}

	return nil, nil
}

// FindRelationshipWithHint looks up a relationship with an optional FK hint.
// If hint is provided, it matches against the FK column name.
// If hint is empty, it falls back to FindRelationship.
// Returns an error if the hint doesn't match any FK.
func (rc *RelationshipCache) FindRelationshipWithHint(table, relationName, hint string) (*Relationship, error) {
	if hint == "" {
		return rc.FindRelationship(table, relationName)
	}

	rels, err := rc.GetRelationships(table)
	if err != nil {
		return nil, err
	}

	// Find relationships matching the relation name and hint
	var matching []*Relationship
	var availableHints []string

	for i := range rels {
		rel := &rels[i]
		if rel.Name == relationName || rel.ForeignTable == relationName {
			// Collect available hints for error message
			availableHints = append(availableHints, rel.LocalColumn)

			// Check if the hint matches the FK column
			if rel.LocalColumn == hint {
				matching = append(matching, rel)
			}
		}
	}

	if len(matching) == 0 {
		if len(availableHints) == 0 {
			return nil, nil // No relationship found
		}
		return nil, fmt.Errorf("no foreign key '%s' found. Available: %v", hint, availableHints)
	}

	return matching[0], nil
}

// JunctionInfo describes a many-to-many junction table.
type JunctionInfo struct {
	JunctionTable  string // The junction table name (e.g., "user_roles")
	SourceColumn   string // FK column pointing to source table
	SourceRef      string // Column in source table being referenced
	TargetColumn   string // FK column pointing to target table
	TargetRef      string // Column in target table being referenced
}

// getTablePrimaryKeyColumns returns the primary key column(s) for a table.
func (rc *RelationshipCache) getTablePrimaryKeyColumns(table string) ([]string, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk
	// pk is 1 for primary key columns (or > 1 for composite PK position)
	rows, err := rc.db.Query(fmt.Sprintf("PRAGMA table_info('%s')", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query table info for %s: %w", table, err)
	}
	defer rows.Close()

	var pkCols []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notnull, &dfltValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan table info: %w", err)
		}
		if pk > 0 {
			pkCols = append(pkCols, name)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating table info: %w", err)
	}

	return pkCols, nil
}

// getTableForeignKeys returns all foreign keys for a table.
func (rc *RelationshipCache) getTableForeignKeys(table string) ([]struct {
	LocalColumn   string
	ForeignTable  string
	ForeignColumn string
}, error) {
	if !isValidTableName(table) {
		return nil, fmt.Errorf("invalid table name: %s", table)
	}

	rows, err := rc.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", table))
	if err != nil {
		return nil, fmt.Errorf("failed to query foreign keys for %s: %w", table, err)
	}
	defer rows.Close()

	var fks []struct {
		LocalColumn   string
		ForeignTable  string
		ForeignColumn string
	}

	for rows.Next() {
		var id, seq int
		var foreignTable, localCol, foreignCol, onUpdate, onDelete, match string
		if err := rows.Scan(&id, &seq, &foreignTable, &localCol, &foreignCol, &onUpdate, &onDelete, &match); err != nil {
			return nil, fmt.Errorf("failed to scan foreign key: %w", err)
		}
		fks = append(fks, struct {
			LocalColumn   string
			ForeignTable  string
			ForeignColumn string
		}{localCol, foreignTable, foreignCol})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating foreign keys: %w", err)
	}

	return fks, nil
}

// isJunctionTable checks if a table is a "strict junction" table for M2M relationships.
// A strict junction table has:
// 1. Exactly 2 foreign keys pointing to different tables
// 2. Both FK columns are part of the table's primary key
func (rc *RelationshipCache) isJunctionTable(table string) (bool, string, string, error) {
	// Get foreign keys
	fks, err := rc.getTableForeignKeys(table)
	if err != nil {
		return false, "", "", err
	}

	// Must have exactly 2 FKs to different tables
	if len(fks) != 2 {
		return false, "", "", nil
	}
	if fks[0].ForeignTable == fks[1].ForeignTable {
		return false, "", "", nil // Both FKs point to same table
	}

	// Get primary key columns
	pkCols, err := rc.getTablePrimaryKeyColumns(table)
	if err != nil {
		return false, "", "", err
	}

	// Both FK columns must be in the primary key
	pkSet := make(map[string]bool)
	for _, col := range pkCols {
		pkSet[col] = true
	}

	if !pkSet[fks[0].LocalColumn] || !pkSet[fks[1].LocalColumn] {
		return false, "", "", nil // FK columns not part of PK
	}

	return true, fks[0].ForeignTable, fks[1].ForeignTable, nil
}

// FindM2MRelationship finds a many-to-many relationship between source and target tables
// through a junction table. Returns nil if no such relationship exists.
func (rc *RelationshipCache) FindM2MRelationship(sourceTable, targetTable string) (*JunctionInfo, error) {
	if !isValidTableName(sourceTable) || !isValidTableName(targetTable) {
		return nil, fmt.Errorf("invalid table name")
	}

	// Get all tables to check for junction tables
	// Note: Use ESCAPE to properly handle underscore (which is a LIKE wildcard)
	tables, err := rc.db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'auth_%' AND name NOT LIKE '\\_%%' ESCAPE '\\'")
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer tables.Close()

	var tableNames []string
	for tables.Next() {
		var tableName string
		if err := tables.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}

	if err := tables.Err(); err != nil {
		return nil, fmt.Errorf("error iterating tables: %w", err)
	}

	// Check each table to see if it's a junction between source and target
	for _, table := range tableNames {
		if table == sourceTable || table == targetTable {
			continue
		}

		isJunction, table1, table2, err := rc.isJunctionTable(table)
		if err != nil {
			continue // Skip tables we can't check
		}

		if !isJunction {
			continue
		}

		// Check if this junction connects source and target
		if (table1 == sourceTable && table2 == targetTable) ||
			(table1 == targetTable && table2 == sourceTable) {

			// Get FK details to build JunctionInfo
			fks, err := rc.getTableForeignKeys(table)
			if err != nil {
				return nil, err
			}

			var info JunctionInfo
			info.JunctionTable = table

			for _, fk := range fks {
				if fk.ForeignTable == sourceTable {
					info.SourceColumn = fk.LocalColumn
					info.SourceRef = fk.ForeignColumn
				} else if fk.ForeignTable == targetTable {
					info.TargetColumn = fk.LocalColumn
					info.TargetRef = fk.ForeignColumn
				}
			}

			return &info, nil
		}
	}

	return nil, nil // No junction table found
}
