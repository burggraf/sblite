package pgwire

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestGetOID(t *testing.T) {
	tests := []struct {
		sqliteType string
		wantOID    uint32
	}{
		// Numeric types
		{"INTEGER", pgtype.Int8OID},
		{"INT", pgtype.Int8OID},
		{"SMALLINT", pgtype.Int2OID},
		{"BIGINT", pgtype.Int8OID},
		{"REAL", pgtype.Float8OID},
		{"FLOAT", pgtype.Float8OID},
		{"DOUBLE", pgtype.Float8OID},
		{"NUMERIC", pgtype.NumericOID},

		// Text types
		{"TEXT", pgtype.TextOID},
		{"VARCHAR", pgtype.VarcharOID},
		{"CHAR", pgtype.BPCharOID},
		{"CLOB", pgtype.TextOID},
		{"", pgtype.TextOID}, // Default

		// Binary types
		{"BLOB", pgtype.ByteaOID},
		{"BYTEA", pgtype.ByteaOID},

		// Boolean
		{"BOOLEAN", pgtype.BoolOID},
		{"BOOL", pgtype.BoolOID},

		// Date/Time
		{"DATE", pgtype.DateOID},
		{"TIME", pgtype.TimeOID},
		{"DATETIME", pgtype.TimestampOID},
		{"TIMESTAMP", pgtype.TimestampOID},
		{"TIMESTAMPTZ", pgtype.TimestamptzOID},

		// JSON
		{"JSON", pgtype.JSONOID},
		{"JSONB", pgtype.JSONBOID},

		// UUID
		{"UUID", pgtype.UUIDOID},

		// Unknown type defaults to TEXT
		{"UNKNOWN_TYPE", pgtype.TextOID},
		{"CUSTOM", pgtype.TextOID},
	}

	for _, tt := range tests {
		t.Run(tt.sqliteType, func(t *testing.T) {
			got := GetOID(tt.sqliteType)
			if got != tt.wantOID {
				t.Errorf("GetOID(%q) = %d, want %d", tt.sqliteType, got, tt.wantOID)
			}
		})
	}
}

func TestSQLiteToOIDMap(t *testing.T) {
	// Verify the map contains expected entries
	expectedTypes := []string{
		"INTEGER", "TEXT", "BLOB", "BOOLEAN", "DATE",
		"TIMESTAMP", "JSON", "JSONB", "UUID",
	}

	for _, typ := range expectedTypes {
		if _, ok := SQLiteToOID[typ]; !ok {
			t.Errorf("SQLiteToOID missing type %q", typ)
		}
	}
}
