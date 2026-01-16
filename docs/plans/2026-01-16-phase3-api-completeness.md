# Phase 3: API Completeness Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Achieve near-complete supabase-js compatibility by implementing remaining filters, relationship queries, pagination, and response modifiers.

**Architecture:** Builds on Phase 1 (REST API) and Phase 2 (RLS). Adds relationship detection via SQLite PRAGMA, nested select parsing, and query execution with result shaping.

**Tech Stack:** Go, SQLite, Chi router

---

## Task 1: Logical Operators (or/and/not)

**Files:**
- Modify: `internal/rest/query.go`
- Modify: `internal/rest/builder.go`
- Modify: `internal/rest/handler.go`
- Test: `internal/rest/query_test.go`

**Step 1: Add logical operator types**

In `internal/rest/query.go`, add:

```go
type LogicalFilter struct {
    Operator string   // "or", "and"
    Filters  []Filter
}

// Update Query struct
type Query struct {
    // ... existing fields
    LogicalFilters []LogicalFilter
}
```

**Step 2: Parse or/and query params**

Parse `?or=(status.eq.active,status.eq.pending)` format:

```go
func ParseLogicalFilter(operator, value string) (LogicalFilter, error) {
    // Remove surrounding parentheses
    value = strings.TrimPrefix(value, "(")
    value = strings.TrimSuffix(value, ")")

    // Split by comma (handling nested parens)
    parts := splitLogicalParts(value)

    var filters []Filter
    for _, part := range parts {
        f, err := ParseFilter(part)
        if err != nil {
            return LogicalFilter{}, err
        }
        filters = append(filters, f)
    }

    return LogicalFilter{Operator: operator, Filters: filters}, nil
}
```

**Step 3: Add not operator**

Add `not` to validOperators and handle in ToSQL:

```go
case "not":
    // not.eq.value -> != value
    // Parse the inner operator
    innerParts := strings.SplitN(f.Value, ".", 2)
    innerOp := innerParts[0]
    innerVal := innerParts[1]
    // Generate negated SQL
```

**Step 4: Generate SQL with logical grouping**

In `builder.go`:

```go
func (lf LogicalFilter) ToSQL() (string, []any) {
    var conditions []string
    var args []any

    for _, f := range lf.Filters {
        sql, filterArgs := f.ToSQL()
        conditions = append(conditions, sql)
        args = append(args, filterArgs...)
    }

    joiner := " AND "
    if lf.Operator == "or" {
        joiner = " OR "
    }

    return "(" + strings.Join(conditions, joiner) + ")", args
}
```

**Step 5: Update handler to parse or/and params**

In `handler.go`, check for `or` and `and` query params.

**Step 6: Write tests**

```go
func TestParseOrFilter(t *testing.T) {
    lf, err := ParseLogicalFilter("or", "(status.eq.active,status.eq.pending)")
    // Assert 2 filters with OR operator
}

func TestNotOperator(t *testing.T) {
    f, err := ParseFilter("status=not.eq.deleted")
    sql, args := f.ToSQL()
    // Assert: "status" != ?
}
```

**Step 7: Run tests**

```bash
go test ./internal/rest/... -v
```

**Step 8: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add or/and/not logical operators"
```

---

## Task 2: Match & Filter Methods

**Files:**
- Modify: `internal/rest/query.go`
- Modify: `internal/rest/handler.go`
- Test: `internal/rest/query_test.go`

**Step 1: Add match() support**

`match()` is shorthand for multiple `.eq()` filters:

```go
// ?match={"status":"active","priority":"high"}
func ParseMatchFilter(jsonValue string) ([]Filter, error) {
    var matches map[string]any
    if err := json.Unmarshal([]byte(jsonValue), &matches); err != nil {
        return nil, err
    }

    var filters []Filter
    for col, val := range matches {
        filters = append(filters, Filter{
            Column:   col,
            Operator: "eq",
            Value:    fmt.Sprintf("%v", val),
        })
    }
    return filters, nil
}
```

**Step 2: Add filter() raw syntax**

`filter()` allows raw PostgREST syntax:

```go
// ?column=filter.operator.value
// This is essentially what we already support, just document it
```

**Step 3: Update handler**

Check for `match` query param and expand to multiple filters.

**Step 4: Write tests**

```go
func TestMatchFilter(t *testing.T) {
    filters, err := ParseMatchFilter(`{"status":"active","priority":"high"}`)
    assert.Len(t, filters, 2)
}
```

**Step 5: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add match() filter method"
```

---

## Task 3: Count Queries

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/rest/builder.go`
- Test: `internal/rest/handler_test.go`

**Step 1: Parse Prefer header for count**

```go
func parsePreferHeader(prefer string) (count string, head bool) {
    if strings.Contains(prefer, "count=exact") {
        count = "exact"
    } else if strings.Contains(prefer, "count=planned") {
        count = "planned"
    } else if strings.Contains(prefer, "count=estimated") {
        count = "estimated"
    }
    // head is determined by request method (HEAD) not Prefer
    return
}
```

**Step 2: Build count query**

```go
func BuildCountQuery(q Query) (string, []any) {
    var args []any
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf(`SELECT COUNT(*) FROM "%s"`, q.Table))

    // Add WHERE clause (same as select)
    if len(q.Filters) > 0 {
        sb.WriteString(" WHERE ")
        // ... same filter logic
    }

    // Add RLS condition
    if q.RLSCondition != "" {
        // ... same RLS logic
    }

    return sb.String(), args
}
```

**Step 3: Execute count and add Content-Range header**

```go
func (h *Handler) HandleSelect(w http.ResponseWriter, r *http.Request) {
    // ... existing code

    prefer := r.Header.Get("Prefer")
    countType, _ := parsePreferHeader(prefer)

    var totalCount int64 = -1
    if countType != "" {
        countSQL, countArgs := BuildCountQuery(q)
        h.db.QueryRow(countSQL, countArgs...).Scan(&totalCount)
    }

    // ... execute main query

    // Set Content-Range header
    if totalCount >= 0 {
        start := q.Offset
        end := start + len(results) - 1
        if len(results) == 0 {
            end = start
        }
        w.Header().Set("Content-Range", fmt.Sprintf("%d-%d/%d", start, end, totalCount))
    }
}
```

**Step 4: Handle HEAD requests**

```go
if r.Method == "HEAD" {
    // Only return headers, no body
    w.Header().Set("Content-Range", fmt.Sprintf("0-0/%d", totalCount))
    return
}
```

**Step 5: Write tests**

```go
func TestCountExact(t *testing.T) {
    req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
    req.Header.Set("Prefer", "count=exact")
    // Assert Content-Range header present
}
```

**Step 6: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add count queries with Content-Range header"
```

---

## Task 4: Range Header Pagination

**Files:**
- Modify: `internal/rest/handler.go`
- Test: `internal/rest/handler_test.go`

**Step 1: Parse Range header**

```go
func parseRangeHeader(rangeHeader string) (start, end int, ok bool) {
    // Format: "0-24" or "items=0-24"
    rangeHeader = strings.TrimPrefix(rangeHeader, "items=")
    parts := strings.Split(rangeHeader, "-")
    if len(parts) != 2 {
        return 0, 0, false
    }

    start, err1 := strconv.Atoi(parts[0])
    end, err2 := strconv.Atoi(parts[1])
    if err1 != nil || err2 != nil {
        return 0, 0, false
    }

    return start, end, true
}
```

**Step 2: Apply Range to query**

```go
// In HandleSelect
rangeHeader := r.Header.Get("Range")
if rangeHeader != "" {
    if start, end, ok := parseRangeHeader(rangeHeader); ok {
        q.Offset = start
        q.Limit = end - start + 1
    }
}
```

**Step 3: Return 206 Partial Content when paginated**

```go
if q.Limit > 0 && len(results) == q.Limit {
    w.WriteHeader(http.StatusPartialContent)
} else {
    w.WriteHeader(http.StatusOK)
}
```

**Step 4: Write tests**

```go
func TestRangeHeader(t *testing.T) {
    req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
    req.Header.Set("Range", "0-9")
    // Assert LIMIT 10 OFFSET 0 applied
    // Assert 206 status if more results exist
}
```

**Step 5: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add Range header pagination"
```

---

## Task 5: Relationship Detection

**Files:**
- Create: `internal/rest/relations.go`
- Test: `internal/rest/relations_test.go`

**Step 1: Define relationship types**

```go
// internal/rest/relations.go
package rest

type Relationship struct {
    Name         string // Related table name
    LocalColumn  string // FK column in this table
    ForeignTable string // Referenced table
    ForeignColumn string // Referenced column (usually "id")
    Type         string // "many-to-one" or "one-to-many"
}

type RelationshipCache struct {
    db    *db.DB
    cache map[string][]Relationship // table -> relationships
    mu    sync.RWMutex
}
```

**Step 2: Detect relationships via PRAGMA**

```go
func (rc *RelationshipCache) GetRelationships(table string) ([]Relationship, error) {
    rc.mu.RLock()
    if rels, ok := rc.cache[table]; ok {
        rc.mu.RUnlock()
        return rels, nil
    }
    rc.mu.RUnlock()

    // Query foreign keys for this table (many-to-one)
    rows, err := rc.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", table))
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var rels []Relationship
    for rows.Next() {
        var id, seq int
        var foreignTable, localCol, foreignCol, onUpdate, onDelete, match string
        rows.Scan(&id, &seq, &foreignTable, &localCol, &foreignCol, &onUpdate, &onDelete, &match)

        rels = append(rels, Relationship{
            Name:          foreignTable,
            LocalColumn:   localCol,
            ForeignTable:  foreignTable,
            ForeignColumn: foreignCol,
            Type:          "many-to-one",
        })
    }

    // Also find reverse relationships (one-to-many)
    // Query all tables and check if they reference this table
    rels = append(rels, rc.findReverseRelationships(table)...)

    rc.mu.Lock()
    rc.cache[table] = rels
    rc.mu.Unlock()

    return rels, nil
}
```

**Step 3: Find reverse relationships**

```go
func (rc *RelationshipCache) findReverseRelationships(table string) []Relationship {
    var rels []Relationship

    // Get all tables
    tables, _ := rc.db.Query("SELECT name FROM sqlite_master WHERE type='table'")
    defer tables.Close()

    for tables.Next() {
        var otherTable string
        tables.Scan(&otherTable)

        // Check if otherTable has FK to our table
        fks, _ := rc.db.Query(fmt.Sprintf("PRAGMA foreign_key_list('%s')", otherTable))
        for fks.Next() {
            var id, seq int
            var foreignTable, localCol, foreignCol, onUpdate, onDelete, match string
            fks.Scan(&id, &seq, &foreignTable, &localCol, &foreignCol, &onUpdate, &onDelete, &match)

            if foreignTable == table {
                rels = append(rels, Relationship{
                    Name:          otherTable,
                    LocalColumn:   foreignCol,
                    ForeignTable:  otherTable,
                    ForeignColumn: localCol,
                    Type:          "one-to-many",
                })
            }
        }
        fks.Close()
    }

    return rels
}
```

**Step 4: Write tests**

```go
func TestRelationshipDetection(t *testing.T) {
    // Create test tables with FK
    db.Exec(`CREATE TABLE countries (id INTEGER PRIMARY KEY, name TEXT)`)
    db.Exec(`CREATE TABLE cities (id INTEGER PRIMARY KEY, name TEXT, country_id INTEGER REFERENCES countries(id))`)

    cache := NewRelationshipCache(db)

    // cities should have many-to-one to countries
    cityRels, _ := cache.GetRelationships("cities")
    assert.Contains(t, cityRels, Relationship{Name: "countries", Type: "many-to-one"})

    // countries should have one-to-many to cities
    countryRels, _ := cache.GetRelationships("countries")
    assert.Contains(t, countryRels, Relationship{Name: "cities", Type: "one-to-many"})
}
```

**Step 5: Commit**

```bash
git add internal/rest/relations.go internal/rest/relations_test.go
git commit -m "feat(rest): add relationship detection via PRAGMA"
```

---

## Task 6: Select Parser (Nested Syntax)

**Files:**
- Create: `internal/rest/select_parser.go`
- Test: `internal/rest/select_parser_test.go`

**Step 1: Define parsed select structure**

```go
// internal/rest/select_parser.go
package rest

type SelectColumn struct {
    Name     string         // Column name or "*"
    Alias    string         // Optional alias (customName:columnName)
    Relation *SelectRelation // If this is a relation
}

type SelectRelation struct {
    Name       string         // Relation/table name
    Alias      string         // Optional alias
    Inner      bool           // !inner join
    Columns    []SelectColumn // Nested columns
}

type ParsedSelect struct {
    Columns []SelectColumn
}
```

**Step 2: Parse select string**

```go
func ParseSelectString(input string) (*ParsedSelect, error) {
    if input == "" || input == "*" {
        return &ParsedSelect{Columns: []SelectColumn{{Name: "*"}}}, nil
    }

    // Split by comma, respecting parentheses
    parts := splitRespectingParens(input)

    var columns []SelectColumn
    for _, part := range parts {
        col, err := parseSelectPart(strings.TrimSpace(part))
        if err != nil {
            return nil, err
        }
        columns = append(columns, col)
    }

    return &ParsedSelect{Columns: columns}, nil
}

func parseSelectPart(part string) (SelectColumn, error) {
    // Check for relation: "table(cols)" or "alias:table(cols)"
    if idx := strings.Index(part, "("); idx != -1 {
        return parseRelationSelect(part)
    }

    // Check for alias: "alias:column"
    if idx := strings.Index(part, ":"); idx != -1 {
        return SelectColumn{
            Alias: part[:idx],
            Name:  part[idx+1:],
        }, nil
    }

    return SelectColumn{Name: part}, nil
}

func parseRelationSelect(part string) (SelectColumn, error) {
    // Handle: "alias:table!inner(cols)" or "table(cols)"
    parenIdx := strings.Index(part, "(")
    prefix := part[:parenIdx]
    inner := strings.Contains(prefix, "!inner")
    prefix = strings.Replace(prefix, "!inner", "", 1)

    var alias, name string
    if colonIdx := strings.Index(prefix, ":"); colonIdx != -1 {
        alias = prefix[:colonIdx]
        name = prefix[colonIdx+1:]
    } else {
        name = prefix
    }

    // Parse nested columns
    nestedStr := part[parenIdx+1 : len(part)-1]
    nested, err := ParseSelectString(nestedStr)
    if err != nil {
        return SelectColumn{}, err
    }

    return SelectColumn{
        Alias: alias,
        Relation: &SelectRelation{
            Name:    name,
            Alias:   alias,
            Inner:   inner,
            Columns: nested.Columns,
        },
    }, nil
}
```

**Step 3: Write tests**

```go
func TestParseSelectSimple(t *testing.T) {
    p, _ := ParseSelectString("id, name, email")
    assert.Len(t, p.Columns, 3)
}

func TestParseSelectWithRelation(t *testing.T) {
    p, _ := ParseSelectString("id, name, country(id, name)")
    assert.Len(t, p.Columns, 3)
    assert.NotNil(t, p.Columns[2].Relation)
    assert.Equal(t, "country", p.Columns[2].Relation.Name)
}

func TestParseSelectWithAlias(t *testing.T) {
    p, _ := ParseSelectString("from:sender_id(name), to:receiver_id(name)")
    assert.Equal(t, "from", p.Columns[0].Relation.Alias)
    assert.Equal(t, "sender_id", p.Columns[0].Relation.Name)
}

func TestParseSelectInnerJoin(t *testing.T) {
    p, _ := ParseSelectString("name, country!inner(name)")
    assert.True(t, p.Columns[1].Relation.Inner)
}

func TestParseSelectTwoLevel(t *testing.T) {
    p, _ := ParseSelectString("name, country(name, continent(name))")
    rel := p.Columns[1].Relation
    assert.NotNil(t, rel.Columns[1].Relation)
    assert.Equal(t, "continent", rel.Columns[1].Relation.Name)
}
```

**Step 4: Commit**

```bash
git add internal/rest/select_parser.go internal/rest/select_parser_test.go
git commit -m "feat(rest): add nested select syntax parser"
```

---

## Task 7: Relationship Query Execution

**Files:**
- Modify: `internal/rest/handler.go`
- Create: `internal/rest/relation_query.go`
- Test: `internal/rest/relation_query_test.go`

**Step 1: Execute main query with relation detection**

```go
// internal/rest/relation_query.go
package rest

type RelationQueryExecutor struct {
    db       *db.DB
    relCache *RelationshipCache
}

func (rqe *RelationQueryExecutor) ExecuteWithRelations(q Query, parsed *ParsedSelect) ([]map[string]any, error) {
    // 1. Extract base columns (non-relation)
    baseColumns := extractBaseColumns(parsed)

    // 2. Execute main query
    mainQ := q
    mainQ.Select = baseColumns
    mainSQL, args := BuildSelectQuery(mainQ)
    rows, err := rqe.db.Query(mainSQL, args...)
    if err != nil {
        return nil, err
    }
    results, _ := scanRows(rows)
    rows.Close()

    // 3. For each relation, execute sub-query and embed
    for _, col := range parsed.Columns {
        if col.Relation != nil {
            rqe.embedRelation(results, q.Table, col.Relation)
        }
    }

    return results, nil
}
```

**Step 2: Embed many-to-one relations**

```go
func (rqe *RelationQueryExecutor) embedRelation(results []map[string]any, mainTable string, rel *SelectRelation) error {
    // Find the relationship definition
    rels, _ := rqe.relCache.GetRelationships(mainTable)
    var relDef *Relationship
    for _, r := range rels {
        if r.Name == rel.Name || r.ForeignTable == rel.Name {
            relDef = &r
            break
        }
    }

    if relDef == nil {
        return fmt.Errorf("unknown relation: %s", rel.Name)
    }

    if relDef.Type == "many-to-one" {
        return rqe.embedManyToOne(results, relDef, rel)
    } else {
        return rqe.embedOneToMany(results, relDef, rel)
    }
}

func (rqe *RelationQueryExecutor) embedManyToOne(results []map[string]any, relDef *Relationship, rel *SelectRelation) error {
    // Collect all foreign key values
    var fkValues []any
    for _, row := range results {
        if fk, ok := row[relDef.LocalColumn]; ok && fk != nil {
            fkValues = append(fkValues, fk)
        }
    }

    if len(fkValues) == 0 {
        return nil
    }

    // Query related table
    cols := extractColumnNames(rel.Columns)
    placeholders := make([]string, len(fkValues))
    for i := range fkValues {
        placeholders[i] = "?"
    }

    sql := fmt.Sprintf(`SELECT %s, "%s" FROM "%s" WHERE "%s" IN (%s)`,
        strings.Join(cols, ", "),
        relDef.ForeignColumn,
        relDef.ForeignTable,
        relDef.ForeignColumn,
        strings.Join(placeholders, ", "))

    rows, _ := rqe.db.Query(sql, fkValues...)
    relResults, _ := scanRows(rows)
    rows.Close()

    // Index by foreign key
    relIndex := make(map[any]map[string]any)
    for _, relRow := range relResults {
        key := relRow[relDef.ForeignColumn]
        delete(relRow, relDef.ForeignColumn) // Don't include FK in nested result
        relIndex[key] = relRow
    }

    // Embed into results
    embedName := rel.Alias
    if embedName == "" {
        embedName = rel.Name
    }

    for _, row := range results {
        fk := row[relDef.LocalColumn]
        if relData, ok := relIndex[fk]; ok {
            row[embedName] = relData
        } else {
            row[embedName] = nil
        }
    }

    return nil
}
```

**Step 3: Embed one-to-many relations**

```go
func (rqe *RelationQueryExecutor) embedOneToMany(results []map[string]any, relDef *Relationship, rel *SelectRelation) error {
    // Collect all primary key values
    var pkValues []any
    for _, row := range results {
        if pk, ok := row[relDef.LocalColumn]; ok && pk != nil {
            pkValues = append(pkValues, pk)
        }
    }

    if len(pkValues) == 0 {
        return nil
    }

    // Query related table
    cols := extractColumnNames(rel.Columns)
    // Include the FK column for matching
    cols = append(cols, relDef.ForeignColumn)

    placeholders := make([]string, len(pkValues))
    for i := range pkValues {
        placeholders[i] = "?"
    }

    sql := fmt.Sprintf(`SELECT %s FROM "%s" WHERE "%s" IN (%s)`,
        strings.Join(cols, ", "),
        relDef.ForeignTable,
        relDef.ForeignColumn,
        strings.Join(placeholders, ", "))

    rows, _ := rqe.db.Query(sql, pkValues...)
    relResults, _ := scanRows(rows)
    rows.Close()

    // Group by foreign key
    relGroups := make(map[any][]map[string]any)
    for _, relRow := range relResults {
        fk := relRow[relDef.ForeignColumn]
        delete(relRow, relDef.ForeignColumn)
        relGroups[fk] = append(relGroups[fk], relRow)
    }

    // Embed into results
    embedName := rel.Alias
    if embedName == "" {
        embedName = rel.Name
    }

    for _, row := range results {
        pk := row[relDef.LocalColumn]
        if relData, ok := relGroups[pk]; ok {
            row[embedName] = relData
        } else {
            row[embedName] = []map[string]any{}
        }
    }

    return nil
}
```

**Step 4: Update handler to use relation executor**

In `handler.go`, when select contains relations, use RelationQueryExecutor.

**Step 5: Write tests**

```go
func TestManyToOneRelation(t *testing.T) {
    // Setup: cities -> countries
    // Query: cities with country(name)
    // Assert: each city has embedded country object
}

func TestOneToManyRelation(t *testing.T) {
    // Setup: countries -> cities
    // Query: countries with cities(name)
    // Assert: each country has embedded cities array
}

func TestTwoLevelNesting(t *testing.T) {
    // Setup: cities -> countries -> continents
    // Query: cities with country(name, continent(name))
    // Assert: nested structure
}
```

**Step 6: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): implement relationship query execution"
```

---

## Task 8: Inner Joins & Multi-Refs

**Files:**
- Modify: `internal/rest/relation_query.go`
- Modify: `internal/rest/select_parser.go`
- Test: `internal/rest/relation_query_test.go`

**Step 1: Handle !inner joins**

When `!inner` is specified, use INNER JOIN instead of LEFT JOIN behavior (filter out nulls):

```go
func (rqe *RelationQueryExecutor) embedManyToOne(...) error {
    // ... existing code

    // If inner join, filter out rows without matching relation
    if rel.Inner {
        filtered := make([]map[string]any, 0)
        for _, row := range results {
            if row[embedName] != nil {
                filtered = append(filtered, row)
            }
        }
        // Update results slice in place
        copy(results, filtered)
        results = results[:len(filtered)]
    }
}
```

**Step 2: Handle multiple references to same table**

Support `from:sender_id(name), to:receiver_id(name)` syntax:

```go
func (rqe *RelationQueryExecutor) findRelationByAlias(mainTable string, rel *SelectRelation) (*Relationship, error) {
    rels, _ := rqe.relCache.GetRelationships(mainTable)

    // If alias looks like a column name (e.g., "sender_id"), find FK by that column
    for _, r := range rels {
        if r.LocalColumn == rel.Name {
            return &r, nil
        }
        if r.Name == rel.Name || r.ForeignTable == rel.Name {
            return &r, nil
        }
    }

    return nil, fmt.Errorf("unknown relation: %s", rel.Name)
}
```

**Step 3: Write tests**

```go
func TestInnerJoin(t *testing.T) {
    // Cities without countries should be filtered out
    // when using country!inner(name)
}

func TestMultipleRefsToSameTable(t *testing.T) {
    // Messages with from:sender_id(name), to:receiver_id(name)
    // Both should resolve to users table via different FKs
}
```

**Step 4: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add inner joins and multi-reference support"
```

---

## Task 9: Filter/Order on Related Tables

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/rest/query.go`
- Test: `internal/rest/handler_test.go`

**Step 1: Parse dotted column references**

Support `?country.name=eq.Canada` and `.order('country(name)')`:

```go
type Filter struct {
    Column        string
    Operator      string
    Value         string
    RelatedTable  string // If filtering on related table
    RelatedColumn string
}

func ParseFilter(input string) (Filter, error) {
    // Check for dotted column: "country.name=eq.Canada"
    parts := strings.SplitN(input, "=", 2)
    column := parts[0]

    if dotIdx := strings.Index(column, "."); dotIdx != -1 {
        // Related table filter
        return Filter{
            RelatedTable:  column[:dotIdx],
            RelatedColumn: column[dotIdx+1:],
            Operator:      // parse operator
            Value:         // parse value
        }, nil
    }
    // ... existing code
}
```

**Step 2: Apply related filters via subquery or join**

```go
func BuildSelectQueryWithRelatedFilters(q Query, relCache *RelationshipCache) (string, []any) {
    // For related filters, add EXISTS subquery
    for _, f := range q.Filters {
        if f.RelatedTable != "" {
            rel := relCache.FindRelation(q.Table, f.RelatedTable)
            // Add: AND EXISTS (SELECT 1 FROM related WHERE related.fk = main.id AND related.col = ?)
        }
    }
}
```

**Step 3: Handle order by related column**

```go
func ParseOrder(orderParam string) []OrderBy {
    // Check for "relation(column)" format
    if match := relationOrderRegex.FindStringSubmatch(part); match != nil {
        return OrderBy{
            RelatedTable: match[1],
            Column:       match[2],
            Desc:         strings.HasSuffix(part, ".desc"),
        }
    }
}
```

**Step 4: Write tests**

```go
func TestFilterOnRelatedTable(t *testing.T) {
    // GET /cities?country.name=eq.Canada
    // Should return only cities in Canada
}

func TestOrderByRelatedColumn(t *testing.T) {
    // GET /cities?order=country(name)
    // Should order cities by their country's name
}
```

**Step 5: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add filter and order on related tables"
```

---

## Task 10: CSV Response Format

**Files:**
- Modify: `internal/rest/handler.go`
- Create: `internal/rest/csv.go`
- Test: `internal/rest/csv_test.go`

**Step 1: Check Accept header for CSV**

```go
func (h *Handler) HandleSelect(w http.ResponseWriter, r *http.Request) {
    // ... existing code

    accept := r.Header.Get("Accept")
    if strings.Contains(accept, "text/csv") {
        h.writeCSV(w, results)
        return
    }

    // ... existing JSON response
}
```

**Step 2: Implement CSV writer**

```go
// internal/rest/csv.go
package rest

import (
    "encoding/csv"
    "fmt"
    "net/http"
)

func (h *Handler) writeCSV(w http.ResponseWriter, results []map[string]any) {
    if len(results) == 0 {
        w.Header().Set("Content-Type", "text/csv")
        return
    }

    w.Header().Set("Content-Type", "text/csv")

    writer := csv.NewWriter(w)
    defer writer.Flush()

    // Get column headers from first row
    var headers []string
    for key := range results[0] {
        headers = append(headers, key)
    }
    sort.Strings(headers) // Consistent ordering
    writer.Write(headers)

    // Write data rows
    for _, row := range results {
        var values []string
        for _, h := range headers {
            values = append(values, fmt.Sprintf("%v", row[h]))
        }
        writer.Write(values)
    }
}
```

**Step 3: Write tests**

```go
func TestCSVResponse(t *testing.T) {
    req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
    req.Header.Set("Accept", "text/csv")

    // Assert Content-Type: text/csv
    // Assert valid CSV format
}
```

**Step 4: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add CSV response format"
```

---

## Task 11: Explain Modifier

**Files:**
- Modify: `internal/rest/handler.go`
- Test: `internal/rest/handler_test.go`

**Step 1: Parse explain option**

```go
// Check for Prefer: explain=true
if strings.Contains(prefer, "explain") {
    h.handleExplain(w, q)
    return
}
```

**Step 2: Execute EXPLAIN and return**

```go
func (h *Handler) handleExplain(w http.ResponseWriter, q Query) {
    sqlStr, args := BuildSelectQuery(q)
    explainSQL := "EXPLAIN QUERY PLAN " + sqlStr

    rows, err := h.db.Query(explainSQL, args...)
    if err != nil {
        h.writeError(w, http.StatusInternalServerError, "explain_error", err.Error())
        return
    }
    defer rows.Close()

    var plan []map[string]any
    for rows.Next() {
        var id, parent, notused int
        var detail string
        rows.Scan(&id, &parent, &notused, &detail)
        plan = append(plan, map[string]any{
            "id":     id,
            "parent": parent,
            "detail": detail,
        })
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "sql":  sqlStr,
        "args": args,
        "plan": plan,
    })
}
```

**Step 3: Write tests**

```go
func TestExplainModifier(t *testing.T) {
    req := httptest.NewRequest("GET", "/rest/v1/todos", nil)
    req.Header.Set("Prefer", "explain")

    // Assert response contains sql, args, plan
}
```

**Step 4: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add explain modifier for query plans"
```

---

## Task 12: Upsert Options

**Files:**
- Modify: `internal/rest/handler.go`
- Modify: `internal/rest/builder.go`
- Test: `internal/rest/handler_test.go`

**Step 1: Parse onConflict option**

```go
// Prefer: resolution=merge-duplicates,on-conflict=column1,column2
func parseUpsertOptions(prefer string) (onConflict []string, ignoreDuplicates bool) {
    if strings.Contains(prefer, "resolution=ignore-duplicates") {
        ignoreDuplicates = true
    }

    // Parse on-conflict columns
    if match := onConflictRegex.FindStringSubmatch(prefer); match != nil {
        onConflict = strings.Split(match[1], ",")
    }

    return
}
```

**Step 2: Update BuildUpsertQuery**

```go
func BuildUpsertQuery(table string, data map[string]any, onConflict []string, ignoreDuplicates bool) (string, []any) {
    // ... existing insert part

    conflictCols := onConflict
    if len(conflictCols) == 0 {
        conflictCols = []string{"id"} // Default
    }

    if ignoreDuplicates {
        sql := fmt.Sprintf(
            `INSERT INTO "%s" (%s) VALUES (%s) ON CONFLICT (%s) DO NOTHING`,
            table,
            strings.Join(quotedCols, ", "),
            strings.Join(placeholders, ", "),
            strings.Join(conflictCols, ", "),
        )
        return sql, args
    }

    // DO UPDATE SET ...
    sql := fmt.Sprintf(
        `INSERT INTO "%s" (%s) VALUES (%s) ON CONFLICT (%s) DO UPDATE SET %s`,
        table,
        strings.Join(quotedCols, ", "),
        strings.Join(placeholders, ", "),
        strings.Join(conflictCols, ", "),
        strings.Join(updateClauses, ", "),
    )

    return sql, args
}
```

**Step 3: Write tests**

```go
func TestUpsertOnConflict(t *testing.T) {
    // Upsert with custom conflict column
    req := httptest.NewRequest("POST", "/rest/v1/users", body)
    req.Header.Set("Prefer", "resolution=merge-duplicates,on-conflict=email")
    // Assert ON CONFLICT (email) used
}

func TestUpsertIgnoreDuplicates(t *testing.T) {
    req := httptest.NewRequest("POST", "/rest/v1/users", body)
    req.Header.Set("Prefer", "resolution=ignore-duplicates")
    // Assert DO NOTHING used
}
```

**Step 4: Commit**

```bash
git add internal/rest/
git commit -m "feat(rest): add onConflict and ignoreDuplicates upsert options"
```

---

## Task 13: OpenAPI Schema Generation

**Files:**
- Create: `internal/rest/openapi.go`
- Create: `internal/rest/openapi_types.go`
- Modify: `internal/server/server.go`
- Test: `internal/rest/openapi_test.go`

**Step 1: Define OpenAPI types**

```go
// internal/rest/openapi_types.go
package rest

type OpenAPISpec struct {
    OpenAPI    string                `json:"openapi"`
    Info       OpenAPIInfo           `json:"info"`
    Paths      map[string]PathItem   `json:"paths"`
    Components OpenAPIComponents     `json:"components"`
}

type OpenAPIInfo struct {
    Title   string `json:"title"`
    Version string `json:"version"`
}

type PathItem struct {
    Get    *Operation `json:"get,omitempty"`
    Post   *Operation `json:"post,omitempty"`
    Patch  *Operation `json:"patch,omitempty"`
    Delete *Operation `json:"delete,omitempty"`
}

type Operation struct {
    Summary     string              `json:"summary"`
    Parameters  []Parameter         `json:"parameters,omitempty"`
    RequestBody *RequestBody        `json:"requestBody,omitempty"`
    Responses   map[string]Response `json:"responses"`
}

// ... more types
```

**Step 2: Generate schema from SQLite**

```go
// internal/rest/openapi.go
package rest

func GenerateOpenAPISpec(db *db.DB) (*OpenAPISpec, error) {
    spec := &OpenAPISpec{
        OpenAPI: "3.0.0",
        Info: OpenAPIInfo{
            Title:   "sblite REST API",
            Version: "1.0.0",
        },
        Paths:      make(map[string]PathItem),
        Components: OpenAPIComponents{Schemas: make(map[string]Schema)},
    }

    // Get all tables
    tables, _ := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name NOT LIKE 'auth_%' AND name NOT LIKE '_%'")
    defer tables.Close()

    for tables.Next() {
        var tableName string
        tables.Scan(&tableName)

        // Generate schema for table
        schema := generateTableSchema(db, tableName)
        spec.Components.Schemas[tableName] = schema

        // Generate paths
        spec.Paths["/rest/v1/"+tableName] = generateTablePaths(tableName)
    }

    return spec, nil
}

func generateTableSchema(db *db.DB, table string) Schema {
    rows, _ := db.Query(fmt.Sprintf("PRAGMA table_info('%s')", table))
    defer rows.Close()

    schema := Schema{
        Type:       "object",
        Properties: make(map[string]Schema),
    }

    for rows.Next() {
        var cid int
        var name, colType string
        var notNull, pk int
        var dflt any
        rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk)

        propSchema := Schema{Type: sqliteTypeToOpenAPI(colType)}
        if notNull == 0 {
            propSchema.Nullable = true
        }
        schema.Properties[name] = propSchema
    }

    return schema
}

func sqliteTypeToOpenAPI(sqliteType string) string {
    switch strings.ToUpper(sqliteType) {
    case "INTEGER":
        return "integer"
    case "REAL":
        return "number"
    case "BLOB":
        return "string" // format: binary
    default:
        return "string"
    }
}
```

**Step 3: Add endpoint**

In `server.go`:

```go
r.Get("/rest/v1/", s.handleOpenAPI)

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
    spec, err := rest.GenerateOpenAPISpec(s.db)
    if err != nil {
        // error response
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(spec)
}
```

**Step 4: Write tests**

```go
func TestOpenAPIGeneration(t *testing.T) {
    spec, _ := GenerateOpenAPISpec(db)

    assert.Equal(t, "3.0.0", spec.OpenAPI)
    assert.Contains(t, spec.Paths, "/rest/v1/todos")
    assert.Contains(t, spec.Components.Schemas, "todos")
}
```

**Step 5: Commit**

```bash
git add internal/rest/openapi*.go internal/server/
git commit -m "feat(rest): add OpenAPI schema generation"
```

---

## Task 14: E2E Tests

**Files:**
- Create: `e2e/tests/filters/logical.test.ts`
- Create: `e2e/tests/relations/relations.test.ts`
- Create: `e2e/tests/modifiers/count.test.ts`
- Create: `e2e/tests/modifiers/csv.test.ts`

**Step 1: Logical operator tests**

```typescript
// e2e/tests/filters/logical.test.ts
describe('Logical Filters', () => {
    it('should support or() filter', async () => {
        const { data } = await supabase
            .from('characters')
            .select('*')
            .or('homeworld.eq.Tatooine,homeworld.eq.Alderaan')

        expect(data.length).toBeGreaterThan(0)
        data.forEach(row => {
            expect(['Tatooine', 'Alderaan']).toContain(row.homeworld)
        })
    })

    it('should support not() filter', async () => {
        const { data } = await supabase
            .from('characters')
            .select('*')
            .not('homeworld', 'eq', 'Tatooine')

        data.forEach(row => {
            expect(row.homeworld).not.toBe('Tatooine')
        })
    })
})
```

**Step 2: Relationship tests**

```typescript
// e2e/tests/relations/relations.test.ts
describe('Relationship Queries', () => {
    it('should embed many-to-one relation', async () => {
        const { data } = await supabase
            .from('cities')
            .select('name, country:country_id(name)')

        expect(data[0].country).toBeDefined()
        expect(data[0].country.name).toBeDefined()
    })

    it('should embed one-to-many relation', async () => {
        const { data } = await supabase
            .from('countries')
            .select('name, cities(name)')

        expect(Array.isArray(data[0].cities)).toBe(true)
    })

    it('should support inner joins', async () => {
        const { data } = await supabase
            .from('instruments')
            .select('name, orchestral_sections!inner(name)')

        // All results should have a section
        data.forEach(row => {
            expect(row.orchestral_sections).not.toBeNull()
        })
    })
})
```

**Step 3: Count and pagination tests**

```typescript
// e2e/tests/modifiers/count.test.ts
describe('Count Queries', () => {
    it('should return count with exact option', async () => {
        const { data, count } = await supabase
            .from('characters')
            .select('*', { count: 'exact' })

        expect(count).toBe(5)
    })

    it('should support head option for count only', async () => {
        const { data, count } = await supabase
            .from('characters')
            .select('*', { count: 'exact', head: true })

        expect(data).toBeNull()
        expect(count).toBe(5)
    })
})
```

**Step 4: Run tests**

```bash
cd e2e && npm test
```

**Step 5: Commit**

```bash
git add e2e/tests/
git commit -m "test: add E2E tests for Phase 3 features"
```

---

## Task 15: Update COMPATIBILITY.md

**Files:**
- Modify: `e2e/COMPATIBILITY.md`

**Step 1: Update filter status**

Mark as ✅:
- `not()`
- `or()`
- `match()`
- `filter()`

**Step 2: Update modifier status**

Mark as ✅:
- `csv()`
- `explain()`
- Count queries

**Step 3: Update relationship status**

Mark as ✅:
- Query referenced tables
- Query nested foreign tables
- Filter through referenced tables
- Query with inner join

**Step 4: Commit**

```bash
git add e2e/COMPATIBILITY.md
git commit -m "docs: update compatibility matrix for Phase 3"
```

---

## Summary

Phase 3 delivers:

1. **Logical Operators** - `or()`, `and()`, `not()`, `match()`, `filter()`
2. **Relationship Queries** - Foreign table embedding, column renaming, inner joins, multi-refs
3. **Count & Pagination** - `count=exact`, Range header, Content-Range, head option
4. **Response Formats** - CSV output, explain modifier
5. **Upsert Options** - `onConflict`, `ignoreDuplicates`
6. **OpenAPI Generation** - Auto-generated API documentation

**Outcome:** Near-complete supabase-js compatibility for data operations.

**Remaining gaps after Phase 3:**
- PostgreSQL arrays/ranges (SQLite limitation)
- Full-text search (future: FTS5)
- Realtime subscriptions (Phase 5)
- Storage API (Phase 5)
