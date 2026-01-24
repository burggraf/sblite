# PostgreSQL Wire Protocol

**Status:** Implemented
**Version:** 1.0
**Last Updated:** 2026-01-24

## Overview

sblite includes a PostgreSQL wire protocol server that allows native PostgreSQL clients to connect directly. This enables tools like `psql`, pgAdmin, DBeaver, and any application using PostgreSQL drivers (libpq, pg, psycopg2, etc.) to connect to sblite as if it were a PostgreSQL database.

## Quick Start

### Start the Server with PostgreSQL Protocol

```bash
# Enable PostgreSQL protocol on port 5432
./sblite serve --pg-port 5432

# With password authentication
./sblite serve --pg-port 5432 --pg-password mysecretpassword

# Disable authentication entirely (development only!)
./sblite serve --pg-port 5432 --pg-no-auth
```

### Connect with psql

```bash
# Connect (no auth)
psql -h localhost -p 5432 -d sblite

# Connect with password
psql -h localhost -p 5432 -d sblite -U sblite
# Enter password when prompted
```

### Connect with Other Tools

**pgAdmin:**
1. Add New Server
2. Host: `localhost`
3. Port: `5432`
4. Database: `sblite`
5. Username: `sblite`
6. Password: (your configured password)

**DBeaver:**
1. New Database Connection → PostgreSQL
2. Host: `localhost`
3. Port: `5432`
4. Database: `sblite`
5. Username: `sblite`

**Node.js (pg):**
```javascript
import pg from 'pg';

const client = new pg.Client({
  host: 'localhost',
  port: 5432,
  database: 'sblite',
  password: 'mysecretpassword'
});

await client.connect();
const result = await client.query('SELECT * FROM users');
console.log(result.rows);
```

**Python (psycopg2):**
```python
import psycopg2

conn = psycopg2.connect(
    host="localhost",
    port=5432,
    database="sblite",
    password="mysecretpassword"
)
cur = conn.cursor()
cur.execute("SELECT * FROM users")
print(cur.fetchall())
```

## Configuration

### CLI Flags

| Flag | Environment Variable | Default | Description |
|------|---------------------|---------|-------------|
| `--pg-port` | - | (disabled) | TCP port for PostgreSQL wire protocol |
| `--pg-password` | - | (none) | Password for client authentication |
| `--pg-no-auth` | - | `false` | Disable authentication (development only) |

### Combined Usage

The PostgreSQL wire protocol runs alongside the HTTP server:

```bash
# HTTP on 8080, PostgreSQL on 5432
./sblite serve --port 8080 --pg-port 5432

# All features enabled
./sblite serve --port 8080 --pg-port 5432 --realtime --functions
```

## Supported Features

### SQL Operations

All standard SQL operations work through the wire protocol:

```sql
-- DDL
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT);
ALTER TABLE users ADD COLUMN created_at TEXT;
DROP TABLE users;

-- DML
INSERT INTO users (name, email) VALUES ('Alice', 'alice@example.com');
UPDATE users SET name = 'Bob' WHERE id = 1;
DELETE FROM users WHERE id = 1;

-- Queries
SELECT * FROM users WHERE name LIKE 'A%' ORDER BY created_at DESC LIMIT 10;
```

### PostgreSQL Syntax

All PostgreSQL syntax translations work through the wire protocol:

```sql
-- Arrays
SELECT ARRAY[1, 2, 3];
SELECT * FROM users WHERE id = ANY(ARRAY[1, 2, 3]);

-- Window functions
SELECT name, ROW_NUMBER() OVER (PARTITION BY dept ORDER BY salary DESC) FROM employees;

-- Regex operators
SELECT * FROM users WHERE email ~ '^admin@';
SELECT * FROM users WHERE name ~* 'john';  -- case-insensitive

-- Date/time
SELECT NOW(), CURRENT_TIMESTAMP;
SELECT * FROM events WHERE created_at > NOW() - INTERVAL '7 days';

-- JSON operators
SELECT metadata->'name' FROM users;
SELECT * FROM users WHERE metadata->>'role' = 'admin';

-- Type casts
SELECT '123'::integer, id::text FROM users;
```

See [PostgreSQL Translation](postgres-translation.md) for the complete list of supported syntax.

### Catalog Queries

Common PostgreSQL catalog queries are emulated for client compatibility:

| Query | Response |
|-------|----------|
| `SELECT version()` | `sblite 1.0.0, compatible with PostgreSQL 15.0` |
| `SELECT current_database()` | `sblite` |
| `SELECT current_user` | `sblite` |
| `SELECT current_schema` | `sblite` |
| `SHOW server_version` | `15.0` |
| `SHOW server_encoding` | `UTF8` |
| `SHOW client_encoding` | `UTF8` |
| `SHOW transaction isolation level` | `serializable` |

### Information Schema

Basic `information_schema.tables` queries are supported:

```sql
SELECT table_schema, table_name, table_type
FROM information_schema.tables
WHERE table_schema = 'public';
```

This maps to SQLite's `sqlite_master` table.

### SET/SHOW Statements

SET statements are acknowledged but ignored (SQLite doesn't have these settings):

```sql
SET search_path TO public;  -- Acknowledged, no effect
SET client_encoding = 'UTF8';  -- Acknowledged, no effect
SHOW search_path;  -- Returns 'public'
```

## Limitations

### Not Supported

The following PostgreSQL features are not available through the wire protocol:

- **Transactions spanning multiple statements** - Each statement auto-commits
- **Prepared statements with parameters** - Use literal values
- **COPY protocol** - Use INSERT statements
- **Listen/Notify** - Use the WebSocket realtime API instead
- **Stored procedures (PL/pgSQL)** - Use sblite's SQL functions or edge functions
- **pg_catalog queries** - Return empty results
- **System columns** (ctid, xmin, etc.) - Not available

### Type Mappings

SQLite types are reported as PostgreSQL OIDs:

| SQLite Type | PostgreSQL OID | PostgreSQL Type |
|-------------|----------------|-----------------|
| INTEGER | 20 | int8 (bigint) |
| TEXT | 25 | text |
| REAL | 701 | float8 |
| BLOB | 17 | bytea |
| NULL | 0 | unknown |

### psql Meta-Commands

Most `psql` meta-commands (`\d`, `\dt`, `\l`) require `pg_catalog` queries which return empty results. Use standard SQL instead:

```sql
-- Instead of \dt
SELECT name FROM sqlite_master WHERE type = 'table';

-- Instead of \d tablename
PRAGMA table_info(tablename);
```

## Security

### Authentication

By default, the wire protocol requires a password if `--pg-password` is set:

```bash
# Require password authentication
./sblite serve --pg-port 5432 --pg-password mysecretpassword
```

Without `--pg-password` or with `--pg-no-auth`, connections are allowed without authentication. This is suitable for local development but **never use in production**.

### Network Security

The wire protocol binds to the same host as the HTTP server (default: `localhost`). To accept remote connections:

```bash
# Accept connections from any interface (use with caution)
./sblite serve --host 0.0.0.0 --pg-port 5432 --pg-password secret
```

For production deployments:
- Always use a strong password
- Use a firewall to restrict access to port 5432
- Consider using SSH tunnels or VPN for remote access
- Use TLS termination via a reverse proxy

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                     sblite Server                           │
├─────────────────────────┬──────────────────────────────────┤
│  HTTP Server (Chi)      │  PostgreSQL Wire Protocol        │
│  - Port 8080 (default)  │  - Port 5432 (--pg-port)        │
│  - REST API             │  - psql-wire library             │
│  - Dashboard            │  - Query translation             │
│  - Storage API          │  - Catalog emulation             │
├─────────────────────────┴──────────────────────────────────┤
│                   pgtranslate Layer                         │
│  - PostgreSQL → SQLite syntax translation                   │
│  - Arrays, window functions, regex operators               │
├────────────────────────────────────────────────────────────┤
│                    SQLite (WAL mode)                        │
└────────────────────────────────────────────────────────────┘
```

### Components

- **psql-wire**: Go library implementing the PostgreSQL wire protocol
- **pgtranslate**: Query translation layer (PostgreSQL → SQLite)
- **Catalog handler**: Emulates pg_catalog and information_schema queries
- **Type mapper**: Converts SQLite types to PostgreSQL OIDs

## Troubleshooting

### Connection Refused

```
psql: error: connection refused
```

Ensure the server is running with `--pg-port`:
```bash
./sblite serve --pg-port 5432
```

### Authentication Failed

```
psql: error: password authentication failed
```

Check that you're using the correct password set with `--pg-password`.

### Query Errors

If a query works in the dashboard but fails via psql:

1. Check if the query uses unsupported PostgreSQL features
2. Try the query in the dashboard SQL browser with PostgreSQL mode enabled
3. View the translated query to understand what's being sent to SQLite

### Column Definition Errors

```
ERROR: unexpected columns
```

This indicates an internal error. Please file a bug report with:
- The exact query that caused the error
- sblite version
- How you connected (psql version, client library)

## Related Documentation

- [PostgreSQL Translation](postgres-translation.md) - Detailed syntax translation reference
- [Realtime Subscriptions](realtime.md) - Alternative to Listen/Notify
- [RPC Functions](rpc-functions.md) - Server-side functions callable via SQL
