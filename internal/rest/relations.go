// internal/rest/relations.go
package rest

import (
	"database/sql"
	"fmt"
	"sync"
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

// GetRelationships returns all relationships for a table, using cache if available.
// It detects both many-to-one (this table has FK) and one-to-many (other tables have FK to this table).
func (rc *RelationshipCache) GetRelationships(table string) ([]Relationship, error) {
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

		fks, err := rc.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", otherTable))
		if err != nil {
			// Log error but continue checking other tables
			continue
		}

		for fks.Next() {
			var id, seq int
			var foreignTable, localCol, foreignCol, onUpdate, onDelete, match string
			if err := fks.Scan(&id, &seq, &foreignTable, &localCol, &foreignCol, &onUpdate, &onDelete, &match); err != nil {
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
		fks.Close()
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
