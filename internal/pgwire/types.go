package pgwire

import (
	"github.com/jackc/pgx/v5/pgtype"
)

// SQLiteToOID maps SQLite column types to PostgreSQL OIDs.
// This allows PostgreSQL clients to understand the column types returned by sblite.
var SQLiteToOID = map[string]uint32{
	// Numeric types
	"INTEGER":  pgtype.Int8OID,
	"INT":      pgtype.Int8OID,
	"SMALLINT": pgtype.Int2OID,
	"BIGINT":   pgtype.Int8OID,
	"REAL":     pgtype.Float8OID,
	"FLOAT":    pgtype.Float8OID,
	"DOUBLE":   pgtype.Float8OID,
	"NUMERIC":  pgtype.NumericOID,

	// Text types
	"TEXT":     pgtype.TextOID,
	"VARCHAR":  pgtype.VarcharOID,
	"CHAR":     pgtype.BPCharOID,
	"CLOB":     pgtype.TextOID,
	"":         pgtype.TextOID, // Default

	// Binary types
	"BLOB":  pgtype.ByteaOID,
	"BYTEA": pgtype.ByteaOID,

	// Boolean
	"BOOLEAN": pgtype.BoolOID,
	"BOOL":    pgtype.BoolOID,

	// Date/Time
	"DATE":        pgtype.DateOID,
	"TIME":        pgtype.TimeOID,
	"DATETIME":    pgtype.TimestampOID,
	"TIMESTAMP":   pgtype.TimestampOID,
	"TIMESTAMPTZ": pgtype.TimestamptzOID,

	// JSON
	"JSON":  pgtype.JSONOID,
	"JSONB": pgtype.JSONBOID,

	// UUID
	"UUID": pgtype.UUIDOID,
}

// GetOID returns the PostgreSQL OID for a SQLite column type.
func GetOID(sqliteType string) uint32 {
	if oid, ok := SQLiteToOID[sqliteType]; ok {
		return oid
	}
	return pgtype.TextOID // Default to text
}
