package pgtranslate

import (
	"fmt"
	"strings"
)

// Dialect represents the target SQL dialect.
type Dialect int

const (
	DialectPostgreSQL Dialect = iota
	DialectSQLite
)

// Generator generates SQL from AST nodes.
type Generator struct {
	dialect       Dialect
	funcMapper    *FunctionMapper
	typeMapper    *TypeMapper
	indent        int
	indentStr     string
	inDefaultExpr bool // true when generating a DEFAULT expression
}

// GeneratorOption configures the generator.
type GeneratorOption func(*Generator)

// WithDialect sets the target dialect.
func WithDialect(d Dialect) GeneratorOption {
	return func(g *Generator) {
		g.dialect = d
	}
}

// WithIndent sets the indentation string.
func WithIndent(s string) GeneratorOption {
	return func(g *Generator) {
		g.indentStr = s
	}
}

// NewGenerator creates a new SQL generator.
func NewGenerator(opts ...GeneratorOption) *Generator {
	g := &Generator{
		dialect:    DialectSQLite,
		funcMapper: NewFunctionMapper(),
		typeMapper: NewTypeMapper(),
		indentStr:  "  ",
	}
	for _, opt := range opts {
		opt(g)
	}
	return g
}

// Generate generates SQL from an AST node.
func (g *Generator) Generate(node Node) (string, error) {
	switch n := node.(type) {
	case Expr:
		return g.generateExpr(n)
	case Stmt:
		return g.generateStmt(n)
	default:
		return "", fmt.Errorf("unknown node type: %T", node)
	}
}

// generateExpr generates SQL for an expression.
func (g *Generator) generateExpr(expr Expr) (string, error) {
	switch e := expr.(type) {
	case *Identifier:
		return g.generateIdentifier(e), nil
	case *Literal:
		return g.generateLiteral(e), nil
	case *BinaryOp:
		return g.generateBinaryOp(e)
	case *UnaryOp:
		return g.generateUnaryOp(e)
	case *FunctionCall:
		return g.generateFunctionCall(e)
	case *TypeCast:
		return g.generateTypeCast(e)
	case *CastExpr:
		return g.generateCastExpr(e)
	case *JsonAccess:
		return g.generateJsonAccess(e)
	case *ParenExpr:
		return g.generateParenExpr(e)
	case *ArrayExpr:
		return g.generateArrayExpr(e)
	case *ArraySubscript:
		return g.generateArraySubscript(e)
	case *CaseExpr:
		return g.generateCaseExpr(e)
	case *BetweenExpr:
		return g.generateBetweenExpr(e)
	case *InExpr:
		return g.generateInExpr(e)
	case *IsExpr:
		return g.generateIsExpr(e)
	case *ExistsExpr:
		return g.generateExistsExpr(e)
	case *ExtractExpr:
		return g.generateExtractExpr(e)
	case *IntervalExpr:
		return g.generateIntervalExpr(e)
	case *SubqueryExpr:
		return g.generateSubqueryExpr(e)
	case *QualifiedRef:
		return g.generateQualifiedRef(e), nil
	case *StarExpr:
		return g.generateStarExpr(e), nil
	case *RawSQL:
		return e.Text, nil
	default:
		return "", fmt.Errorf("unknown expression type: %T", expr)
	}
}

func (g *Generator) generateIdentifier(id *Identifier) string {
	if id.Quoted {
		return `"` + id.Name + `"`
	}
	// Check if identifier needs quoting (reserved word or special chars)
	if g.needsQuoting(id.Name) {
		return `"` + id.Name + `"`
	}
	return id.Name
}

func (g *Generator) needsQuoting(name string) bool {
	// Check if it's a keyword
	if IsKeyword(name) {
		return true
	}
	// Check for special characters
	for _, ch := range name {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
			return true
		}
	}
	return false
}

func (g *Generator) generateLiteral(lit *Literal) string {
	switch lit.Type {
	case LitString:
		// Escape single quotes
		escaped := strings.ReplaceAll(lit.Value, "'", "''")
		return "'" + escaped + "'"
	case LitNumber:
		return lit.Value
	case LitBoolean:
		if g.dialect == DialectSQLite {
			if strings.ToUpper(lit.Value) == "TRUE" {
				return "1"
			}
			return "0"
		}
		return lit.Value
	case LitNull:
		return "NULL"
	case LitDollarQuoted:
		if g.dialect == DialectSQLite {
			// Convert to single-quoted string
			escaped := strings.ReplaceAll(lit.Value, "'", "''")
			return "'" + escaped + "'"
		}
		return "$$" + lit.Value + "$$"
	default:
		return lit.Value
	}
}

func (g *Generator) generateBinaryOp(op *BinaryOp) (string, error) {
	left, err := g.generateExpr(op.Left)
	if err != nil {
		return "", err
	}
	right, err := g.generateExpr(op.Right)
	if err != nil {
		return "", err
	}

	// Handle ILIKE for SQLite (convert to LIKE with case-insensitive handling)
	operator := op.Op
	if g.dialect == DialectSQLite && strings.ToUpper(operator) == "ILIKE" {
		operator = "LIKE"
	}
	if g.dialect == DialectSQLite && strings.ToUpper(operator) == "NOT ILIKE" {
		operator = "NOT LIKE"
	}

	return fmt.Sprintf("%s %s %s", left, operator, right), nil
}

func (g *Generator) generateUnaryOp(op *UnaryOp) (string, error) {
	operand, err := g.generateExpr(op.Operand)
	if err != nil {
		return "", err
	}

	if op.Op == "NOT" {
		return fmt.Sprintf("NOT %s", operand), nil
	}
	return fmt.Sprintf("%s%s", op.Op, operand), nil
}

func (g *Generator) generateFunctionCall(call *FunctionCall) (string, error) {
	// Special handling for gen_random_uuid() in DEFAULT expressions for SQLite
	// SQLite doesn't support SELECT subqueries in DEFAULT clauses, so we use gen_uuid()
	// which is a simpler placeholder that gets stored in _columns metadata
	if g.dialect == DialectSQLite && g.inDefaultExpr {
		upperName := strings.ToUpper(call.Name)
		if upperName == "GEN_RANDOM_UUID" {
			return "gen_uuid()", nil
		}
	}

	// Check for function mapping
	if g.dialect == DialectSQLite {
		if mapped, ok := g.funcMapper.MapToSQLite(call); ok {
			return mapped, nil
		}
	} else {
		if mapped, ok := g.funcMapper.MapToPostgreSQL(call); ok {
			return mapped, nil
		}
	}

	// Generate normally
	var sb strings.Builder
	sb.WriteString(call.Name)
	sb.WriteString("(")

	if call.Star {
		sb.WriteString("*")
	} else {
		if call.Distinct {
			sb.WriteString("DISTINCT ")
		}

		for i, arg := range call.Args {
			if i > 0 {
				sb.WriteString(", ")
			}
			argStr, err := g.generateExpr(arg)
			if err != nil {
				return "", err
			}
			sb.WriteString(argStr)
		}

		// ORDER BY in aggregate
		if len(call.OrderBy) > 0 {
			sb.WriteString(" ORDER BY ")
			for i, ob := range call.OrderBy {
				if i > 0 {
					sb.WriteString(", ")
				}
				obStr, err := g.generateExpr(ob.Expr)
				if err != nil {
					return "", err
				}
				sb.WriteString(obStr)
				if ob.Desc {
					sb.WriteString(" DESC")
				}
			}
		}
	}

	sb.WriteString(")")
	return sb.String(), nil
}

func (g *Generator) generateTypeCast(cast *TypeCast) (string, error) {
	expr, err := g.generateExpr(cast.Expr)
	if err != nil {
		return "", err
	}

	if g.dialect == DialectSQLite {
		// SQLite doesn't have :: cast syntax - either remove or use CAST
		mappedType := g.typeMapper.MapToSQLite(cast.TypeName)
		if mappedType == "" {
			// Type cast can be removed (e.g., ::uuid, ::text)
			return expr, nil
		}
		// Use CAST syntax for types that need conversion
		return fmt.Sprintf("CAST(%s AS %s)", expr, mappedType), nil
	}

	// PostgreSQL: use :: syntax
	typeName := cast.TypeName
	if len(cast.TypeArgs) > 0 {
		typeName += "(" + strings.Join(cast.TypeArgs, ", ") + ")"
	}
	return fmt.Sprintf("%s::%s", expr, typeName), nil
}

func (g *Generator) generateCastExpr(cast *CastExpr) (string, error) {
	expr, err := g.generateExpr(cast.Expr)
	if err != nil {
		return "", err
	}

	typeName := cast.TypeName
	if g.dialect == DialectSQLite {
		typeName = g.typeMapper.MapToSQLite(cast.TypeName)
		if typeName == "" {
			typeName = "TEXT" // Default
		}
	}

	if len(cast.TypeArgs) > 0 {
		typeName += "(" + strings.Join(cast.TypeArgs, ", ") + ")"
	}

	return fmt.Sprintf("CAST(%s AS %s)", expr, typeName), nil
}

func (g *Generator) generateJsonAccess(ja *JsonAccess) (string, error) {
	expr, err := g.generateExpr(ja.Expr)
	if err != nil {
		return "", err
	}
	key, err := g.generateExpr(ja.Key)
	if err != nil {
		return "", err
	}

	if g.dialect == DialectSQLite {
		// Convert to json_extract
		// key is either a string literal or number
		var jsonPath string
		switch k := ja.Key.(type) {
		case *Literal:
			if k.Type == LitString {
				jsonPath = "$." + k.Value
			} else if k.Type == LitNumber {
				jsonPath = "$[" + k.Value + "]"
			}
		default:
			// Dynamic key - use json_extract with concatenation
			return fmt.Sprintf("json_extract(%s, '$.' || %s)", expr, key), nil
		}
		return fmt.Sprintf("json_extract(%s, '%s')", expr, jsonPath), nil
	}

	// PostgreSQL
	if ja.AsText {
		return fmt.Sprintf("%s->>%s", expr, key), nil
	}
	return fmt.Sprintf("%s->%s", expr, key), nil
}

func (g *Generator) generateParenExpr(pe *ParenExpr) (string, error) {
	inner, err := g.generateExpr(pe.Expr)
	if err != nil {
		return "", err
	}
	return "(" + inner + ")", nil
}

func (g *Generator) generateArrayExpr(ae *ArrayExpr) (string, error) {
	if g.dialect == DialectSQLite {
		// Convert to JSON array
		var parts []string
		for _, elem := range ae.Elements {
			elemStr, err := g.generateExpr(elem)
			if err != nil {
				return "", err
			}
			parts = append(parts, elemStr)
		}
		return fmt.Sprintf("json_array(%s)", strings.Join(parts, ", ")), nil
	}

	// PostgreSQL
	var parts []string
	for _, elem := range ae.Elements {
		elemStr, err := g.generateExpr(elem)
		if err != nil {
			return "", err
		}
		parts = append(parts, elemStr)
	}
	return fmt.Sprintf("ARRAY[%s]", strings.Join(parts, ", ")), nil
}

func (g *Generator) generateArraySubscript(as *ArraySubscript) (string, error) {
	array, err := g.generateExpr(as.Array)
	if err != nil {
		return "", err
	}
	index, err := g.generateExpr(as.Index)
	if err != nil {
		return "", err
	}

	if g.dialect == DialectSQLite {
		// Use json_extract for array access
		return fmt.Sprintf("json_extract(%s, '$[' || (%s - 1) || ']')", array, index), nil
	}

	return fmt.Sprintf("%s[%s]", array, index), nil
}

func (g *Generator) generateCaseExpr(ce *CaseExpr) (string, error) {
	var sb strings.Builder
	sb.WriteString("CASE")

	if ce.Operand != nil {
		operand, err := g.generateExpr(ce.Operand)
		if err != nil {
			return "", err
		}
		sb.WriteString(" ")
		sb.WriteString(operand)
	}

	for _, when := range ce.Whens {
		whenExpr, err := g.generateExpr(when.When)
		if err != nil {
			return "", err
		}
		thenExpr, err := g.generateExpr(when.Then)
		if err != nil {
			return "", err
		}
		sb.WriteString(" WHEN ")
		sb.WriteString(whenExpr)
		sb.WriteString(" THEN ")
		sb.WriteString(thenExpr)
	}

	if ce.Else != nil {
		elseExpr, err := g.generateExpr(ce.Else)
		if err != nil {
			return "", err
		}
		sb.WriteString(" ELSE ")
		sb.WriteString(elseExpr)
	}

	sb.WriteString(" END")
	return sb.String(), nil
}

func (g *Generator) generateBetweenExpr(be *BetweenExpr) (string, error) {
	expr, err := g.generateExpr(be.Expr)
	if err != nil {
		return "", err
	}
	low, err := g.generateExpr(be.Low)
	if err != nil {
		return "", err
	}
	high, err := g.generateExpr(be.High)
	if err != nil {
		return "", err
	}

	if be.Not {
		return fmt.Sprintf("%s NOT BETWEEN %s AND %s", expr, low, high), nil
	}
	return fmt.Sprintf("%s BETWEEN %s AND %s", expr, low, high), nil
}

func (g *Generator) generateInExpr(ie *InExpr) (string, error) {
	expr, err := g.generateExpr(ie.Expr)
	if err != nil {
		return "", err
	}

	var listStr string
	if ie.Query != nil {
		query, err := g.generateStmt(ie.Query)
		if err != nil {
			return "", err
		}
		listStr = query
	} else {
		var parts []string
		for _, item := range ie.List {
			itemStr, err := g.generateExpr(item)
			if err != nil {
				return "", err
			}
			parts = append(parts, itemStr)
		}
		listStr = strings.Join(parts, ", ")
	}

	if ie.Not {
		return fmt.Sprintf("%s NOT IN (%s)", expr, listStr), nil
	}
	return fmt.Sprintf("%s IN (%s)", expr, listStr), nil
}

func (g *Generator) generateIsExpr(ie *IsExpr) (string, error) {
	expr, err := g.generateExpr(ie.Expr)
	if err != nil {
		return "", err
	}

	value := ie.Value
	if g.dialect == DialectSQLite {
		// SQLite uses IS NULL/IS NOT NULL, but for booleans use = 1/= 0
		switch value {
		case "TRUE":
			return fmt.Sprintf("%s = 1", expr), nil
		case "FALSE":
			return fmt.Sprintf("%s = 0", expr), nil
		case "NOT TRUE":
			return fmt.Sprintf("%s != 1", expr), nil
		case "NOT FALSE":
			return fmt.Sprintf("%s != 0", expr), nil
		}
	}

	return fmt.Sprintf("%s IS %s", expr, value), nil
}

func (g *Generator) generateExistsExpr(ee *ExistsExpr) (string, error) {
	query, err := g.generateStmt(ee.Query)
	if err != nil {
		return "", err
	}

	if ee.Not {
		return fmt.Sprintf("NOT EXISTS (%s)", query), nil
	}
	return fmt.Sprintf("EXISTS (%s)", query), nil
}

func (g *Generator) generateExtractExpr(ee *ExtractExpr) (string, error) {
	expr, err := g.generateExpr(ee.Expr)
	if err != nil {
		return "", err
	}

	if g.dialect == DialectSQLite {
		// Convert to strftime
		formatMap := map[string]string{
			"YEAR":    "%Y",
			"MONTH":   "%m",
			"DAY":     "%d",
			"HOUR":    "%H",
			"MINUTE":  "%M",
			"SECOND":  "%S",
			"DOW":     "%w", // day of week
			"DOY":     "%j", // day of year
			"WEEK":    "%W",
			"QUARTER": "", // needs special handling
		}

		format, ok := formatMap[ee.Field]
		if !ok || format == "" {
			// Unsupported field, return as-is and let SQLite handle it
			return fmt.Sprintf("EXTRACT(%s FROM %s)", ee.Field, expr), nil
		}

		return fmt.Sprintf("CAST(strftime('%s', %s) AS INTEGER)", format, expr), nil
	}

	return fmt.Sprintf("EXTRACT(%s FROM %s)", ee.Field, expr), nil
}

func (g *Generator) generateIntervalExpr(ie *IntervalExpr) (string, error) {
	if g.dialect == DialectSQLite {
		// Convert to SQLite datetime modifier format
		// PostgreSQL: INTERVAL '1 day' -> SQLite: '+1 day'
		return "'" + g.convertIntervalToSQLite(ie.Value) + "'", nil
	}

	return fmt.Sprintf("INTERVAL '%s'", ie.Value), nil
}

func (g *Generator) convertIntervalToSQLite(interval string) string {
	// Parse the interval and convert to SQLite format
	// "1 day" -> "+1 day"
	// "2 hours" -> "+2 hours"
	interval = strings.TrimSpace(interval)
	if !strings.HasPrefix(interval, "+") && !strings.HasPrefix(interval, "-") {
		interval = "+" + interval
	}
	return interval
}

func (g *Generator) generateSubqueryExpr(se *SubqueryExpr) (string, error) {
	query, err := g.generateStmt(se.Query)
	if err != nil {
		return "", err
	}
	return "(" + query + ")", nil
}

func (g *Generator) generateQualifiedRef(qr *QualifiedRef) string {
	return qr.Table + "." + qr.Column
}

func (g *Generator) generateStarExpr(se *StarExpr) string {
	if se.Table != "" {
		return se.Table + ".*"
	}
	return "*"
}

// generateStmt generates SQL for a statement.
func (g *Generator) generateStmt(stmt Stmt) (string, error) {
	switch s := stmt.(type) {
	case *SelectStmt:
		return g.generateSelectStmt(s)
	case *InsertStmt:
		return g.generateInsertStmt(s)
	case *UpdateStmt:
		return g.generateUpdateStmt(s)
	case *DeleteStmt:
		return g.generateDeleteStmt(s)
	case *CreateTableStmt:
		return g.generateCreateTableStmt(s)
	case *CreateFunctionStmt:
		return g.generateCreateFunctionStmt(s)
	case *DropStmt:
		return g.generateDropStmt(s)
	case *RawSQL:
		return s.Text, nil
	default:
		return "", fmt.Errorf("unknown statement type: %T", stmt)
	}
}

func (g *Generator) generateSelectStmt(stmt *SelectStmt) (string, error) {
	var sb strings.Builder

	// WITH clause
	if stmt.With != nil {
		with, err := g.generateWithClause(stmt.With)
		if err != nil {
			return "", err
		}
		sb.WriteString(with)
		sb.WriteString(" ")
	}

	sb.WriteString("SELECT")

	if stmt.Distinct {
		sb.WriteString(" DISTINCT")
	}

	// Columns
	for i, col := range stmt.Columns {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(" ")
		colStr, err := g.generateExpr(col.Expr)
		if err != nil {
			return "", err
		}
		sb.WriteString(colStr)
		if col.Alias != "" {
			sb.WriteString(" AS ")
			sb.WriteString(col.Alias)
		}
	}

	// FROM
	if stmt.From != nil && len(stmt.From.Tables) > 0 {
		sb.WriteString(" FROM ")
		for i, table := range stmt.From.Tables {
			if i > 0 {
				sb.WriteString(", ")
			}
			tableStr, err := g.generateTableRef(table)
			if err != nil {
				return "", err
			}
			sb.WriteString(tableStr)
		}
	}

	// WHERE
	if stmt.Where != nil {
		sb.WriteString(" WHERE ")
		where, err := g.generateExpr(stmt.Where)
		if err != nil {
			return "", err
		}
		sb.WriteString(where)
	}

	// GROUP BY
	if len(stmt.GroupBy) > 0 {
		sb.WriteString(" GROUP BY ")
		for i, expr := range stmt.GroupBy {
			if i > 0 {
				sb.WriteString(", ")
			}
			exprStr, err := g.generateExpr(expr)
			if err != nil {
				return "", err
			}
			sb.WriteString(exprStr)
		}
	}

	// HAVING
	if stmt.Having != nil {
		sb.WriteString(" HAVING ")
		having, err := g.generateExpr(stmt.Having)
		if err != nil {
			return "", err
		}
		sb.WriteString(having)
	}

	// ORDER BY
	if len(stmt.OrderBy) > 0 {
		sb.WriteString(" ORDER BY ")
		for i, ob := range stmt.OrderBy {
			if i > 0 {
				sb.WriteString(", ")
			}
			obStr, err := g.generateExpr(ob.Expr)
			if err != nil {
				return "", err
			}
			sb.WriteString(obStr)
			if ob.Desc {
				sb.WriteString(" DESC")
			}
		}
	}

	// LIMIT
	if stmt.Limit != nil {
		sb.WriteString(" LIMIT ")
		limit, err := g.generateExpr(stmt.Limit)
		if err != nil {
			return "", err
		}
		sb.WriteString(limit)
	}

	// OFFSET
	if stmt.Offset != nil {
		sb.WriteString(" OFFSET ")
		offset, err := g.generateExpr(stmt.Offset)
		if err != nil {
			return "", err
		}
		sb.WriteString(offset)
	}

	return sb.String(), nil
}

func (g *Generator) generateWithClause(with *WithClause) (string, error) {
	var sb strings.Builder
	sb.WriteString("WITH")
	if with.Recursive {
		sb.WriteString(" RECURSIVE")
	}

	for i, cte := range with.CTEs {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(" ")
		sb.WriteString(cte.Name)
		if len(cte.Columns) > 0 {
			sb.WriteString("(")
			sb.WriteString(strings.Join(cte.Columns, ", "))
			sb.WriteString(")")
		}
		sb.WriteString(" AS (")
		query, err := g.generateSelectStmt(cte.Query)
		if err != nil {
			return "", err
		}
		sb.WriteString(query)
		sb.WriteString(")")
	}

	return sb.String(), nil
}

func (g *Generator) generateTableRef(ref *TableRef) (string, error) {
	var sb strings.Builder

	if ref.Subquery != nil {
		query, err := g.generateSelectStmt(ref.Subquery)
		if err != nil {
			return "", err
		}
		sb.WriteString("(")
		sb.WriteString(query)
		sb.WriteString(")")
	} else {
		sb.WriteString(ref.Name)
	}

	if ref.Alias != "" {
		sb.WriteString(" AS ")
		sb.WriteString(ref.Alias)
	}

	if ref.Join != nil {
		join, err := g.generateJoinClause(ref.Join)
		if err != nil {
			return "", err
		}
		sb.WriteString(" ")
		sb.WriteString(join)
	}

	return sb.String(), nil
}

func (g *Generator) generateJoinClause(join *JoinClause) (string, error) {
	var sb strings.Builder

	sb.WriteString(join.Type)
	sb.WriteString(" JOIN ")

	table, err := g.generateTableRef(join.Table)
	if err != nil {
		return "", err
	}
	sb.WriteString(table)

	if join.On != nil {
		sb.WriteString(" ON ")
		on, err := g.generateExpr(join.On)
		if err != nil {
			return "", err
		}
		sb.WriteString(on)
	} else if len(join.Using) > 0 {
		sb.WriteString(" USING (")
		sb.WriteString(strings.Join(join.Using, ", "))
		sb.WriteString(")")
	}

	return sb.String(), nil
}

func (g *Generator) generateInsertStmt(stmt *InsertStmt) (string, error) {
	var sb strings.Builder

	// Handle ON CONFLICT DO NOTHING for SQLite
	if g.dialect == DialectSQLite && stmt.OnConflict != nil && stmt.OnConflict.DoNothing {
		sb.WriteString("INSERT OR IGNORE INTO ")
	} else {
		sb.WriteString("INSERT INTO ")
	}

	sb.WriteString(stmt.Table)

	if len(stmt.Columns) > 0 {
		sb.WriteString(" (")
		sb.WriteString(strings.Join(stmt.Columns, ", "))
		sb.WriteString(")")
	}

	if stmt.Query != nil {
		sb.WriteString(" ")
		query, err := g.generateSelectStmt(stmt.Query)
		if err != nil {
			return "", err
		}
		sb.WriteString(query)
	} else {
		sb.WriteString(" VALUES ")
		for i, row := range stmt.Values {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("(")
			for j, val := range row {
				if j > 0 {
					sb.WriteString(", ")
				}
				valStr, err := g.generateExpr(val)
				if err != nil {
					return "", err
				}
				sb.WriteString(valStr)
			}
			sb.WriteString(")")
		}
	}

	// ON CONFLICT (for PostgreSQL or non-DO NOTHING cases)
	if stmt.OnConflict != nil && g.dialect == DialectPostgreSQL {
		sb.WriteString(" ON CONFLICT")
		if len(stmt.OnConflict.Target) > 0 {
			sb.WriteString(" (")
			sb.WriteString(strings.Join(stmt.OnConflict.Target, ", "))
			sb.WriteString(")")
		}
		if stmt.OnConflict.DoNothing {
			sb.WriteString(" DO NOTHING")
		} else if len(stmt.OnConflict.Updates) > 0 {
			sb.WriteString(" DO UPDATE SET ")
			for i, upd := range stmt.OnConflict.Updates {
				if i > 0 {
					sb.WriteString(", ")
				}
				val, err := g.generateExpr(upd.Value)
				if err != nil {
					return "", err
				}
				sb.WriteString(upd.Column)
				sb.WriteString(" = ")
				sb.WriteString(val)
			}
		}
	}

	// RETURNING
	if len(stmt.Returning) > 0 {
		sb.WriteString(" RETURNING ")
		for i, ret := range stmt.Returning {
			if i > 0 {
				sb.WriteString(", ")
			}
			retStr, err := g.generateExpr(ret)
			if err != nil {
				return "", err
			}
			sb.WriteString(retStr)
		}
	}

	return sb.String(), nil
}

func (g *Generator) generateUpdateStmt(stmt *UpdateStmt) (string, error) {
	var sb strings.Builder

	sb.WriteString("UPDATE ")
	sb.WriteString(stmt.Table)
	if stmt.Alias != "" {
		sb.WriteString(" AS ")
		sb.WriteString(stmt.Alias)
	}

	sb.WriteString(" SET ")
	for i, assign := range stmt.Set {
		if i > 0 {
			sb.WriteString(", ")
		}
		val, err := g.generateExpr(assign.Value)
		if err != nil {
			return "", err
		}
		sb.WriteString(assign.Column)
		sb.WriteString(" = ")
		sb.WriteString(val)
	}

	if stmt.Where != nil {
		sb.WriteString(" WHERE ")
		where, err := g.generateExpr(stmt.Where)
		if err != nil {
			return "", err
		}
		sb.WriteString(where)
	}

	if len(stmt.Returning) > 0 {
		sb.WriteString(" RETURNING ")
		for i, ret := range stmt.Returning {
			if i > 0 {
				sb.WriteString(", ")
			}
			retStr, err := g.generateExpr(ret)
			if err != nil {
				return "", err
			}
			sb.WriteString(retStr)
		}
	}

	return sb.String(), nil
}

func (g *Generator) generateDeleteStmt(stmt *DeleteStmt) (string, error) {
	var sb strings.Builder

	sb.WriteString("DELETE FROM ")
	sb.WriteString(stmt.Table)
	if stmt.Alias != "" {
		sb.WriteString(" AS ")
		sb.WriteString(stmt.Alias)
	}

	if stmt.Where != nil {
		sb.WriteString(" WHERE ")
		where, err := g.generateExpr(stmt.Where)
		if err != nil {
			return "", err
		}
		sb.WriteString(where)
	}

	if len(stmt.Returning) > 0 {
		sb.WriteString(" RETURNING ")
		for i, ret := range stmt.Returning {
			if i > 0 {
				sb.WriteString(", ")
			}
			retStr, err := g.generateExpr(ret)
			if err != nil {
				return "", err
			}
			sb.WriteString(retStr)
		}
	}

	return sb.String(), nil
}

func (g *Generator) generateCreateTableStmt(stmt *CreateTableStmt) (string, error) {
	var sb strings.Builder

	sb.WriteString("CREATE TABLE ")
	if stmt.IfNotExists {
		sb.WriteString("IF NOT EXISTS ")
	}
	sb.WriteString(stmt.Name)
	sb.WriteString(" (\n")

	for i, col := range stmt.Columns {
		if i > 0 {
			sb.WriteString(",\n")
		}
		sb.WriteString(g.indentStr)
		colDef, err := g.generateColumnDef(col)
		if err != nil {
			return "", err
		}
		sb.WriteString(colDef)
	}

	for _, constraint := range stmt.Constraints {
		sb.WriteString(",\n")
		sb.WriteString(g.indentStr)
		constr, err := g.generateTableConstraint(constraint)
		if err != nil {
			return "", err
		}
		sb.WriteString(constr)
	}

	sb.WriteString("\n)")

	return sb.String(), nil
}

func (g *Generator) generateColumnDef(col *ColumnDef) (string, error) {
	var sb strings.Builder

	sb.WriteString(col.Name)
	sb.WriteString(" ")

	typeName := col.TypeName
	if g.dialect == DialectSQLite {
		typeName = g.typeMapper.MapToSQLite(col.TypeName)
		if typeName == "" {
			typeName = "TEXT"
		}
	}
	sb.WriteString(typeName)

	if len(col.TypeArgs) > 0 && g.dialect == DialectPostgreSQL {
		sb.WriteString("(")
		sb.WriteString(strings.Join(col.TypeArgs, ", "))
		sb.WriteString(")")
	}

	if col.PrimaryKey {
		sb.WriteString(" PRIMARY KEY")
	}

	if col.NotNull {
		sb.WriteString(" NOT NULL")
	}

	if col.Unique {
		sb.WriteString(" UNIQUE")
	}

	if col.Default != nil {
		sb.WriteString(" DEFAULT ")
		// Set context for DEFAULT expression generation
		g.inDefaultExpr = true
		def, err := g.generateExpr(col.Default)
		g.inDefaultExpr = false
		if err != nil {
			return "", err
		}
		sb.WriteString(def)
	}

	return sb.String(), nil
}

func (g *Generator) generateTableConstraint(constr *TableConstraint) (string, error) {
	var sb strings.Builder

	if constr.Name != "" {
		sb.WriteString("CONSTRAINT ")
		sb.WriteString(constr.Name)
		sb.WriteString(" ")
	}

	switch constr.Type {
	case "PRIMARY KEY":
		sb.WriteString("PRIMARY KEY (")
		sb.WriteString(strings.Join(constr.Columns, ", "))
		sb.WriteString(")")
	case "UNIQUE":
		sb.WriteString("UNIQUE (")
		sb.WriteString(strings.Join(constr.Columns, ", "))
		sb.WriteString(")")
	case "FOREIGN KEY":
		sb.WriteString("FOREIGN KEY (")
		sb.WriteString(strings.Join(constr.Columns, ", "))
		sb.WriteString(") REFERENCES ")
		sb.WriteString(constr.References.Table)
		sb.WriteString("(")
		sb.WriteString(constr.References.Column)
		sb.WriteString(")")
	case "CHECK":
		sb.WriteString("CHECK (")
		check, err := g.generateExpr(constr.Check)
		if err != nil {
			return "", err
		}
		sb.WriteString(check)
		sb.WriteString(")")
	}

	return sb.String(), nil
}

func (g *Generator) generateCreateFunctionStmt(stmt *CreateFunctionStmt) (string, error) {
	// CREATE FUNCTION is PostgreSQL-specific
	// For SQLite, we store the function metadata but don't create actual functions
	if g.dialect == DialectSQLite {
		return "", fmt.Errorf("CREATE FUNCTION not supported in SQLite dialect")
	}

	var sb strings.Builder

	sb.WriteString("CREATE ")
	if stmt.OrReplace {
		sb.WriteString("OR REPLACE ")
	}
	sb.WriteString("FUNCTION ")
	sb.WriteString(stmt.Name)
	sb.WriteString("(")

	for i, arg := range stmt.Args {
		if i > 0 {
			sb.WriteString(", ")
		}
		if arg.Name != "" {
			sb.WriteString(arg.Name)
			sb.WriteString(" ")
		}
		sb.WriteString(arg.TypeName)
		if arg.Default != nil {
			sb.WriteString(" DEFAULT ")
			def, err := g.generateExpr(arg.Default)
			if err != nil {
				return "", err
			}
			sb.WriteString(def)
		}
	}

	sb.WriteString(")")

	if stmt.Returns != nil {
		sb.WriteString(" RETURNS ")
		if stmt.Returns.IsSetOf {
			sb.WriteString("SETOF ")
		}
		if stmt.Returns.IsTable {
			sb.WriteString("TABLE(")
			for i, col := range stmt.Returns.TableCols {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(col.Name)
				sb.WriteString(" ")
				sb.WriteString(col.TypeName)
			}
			sb.WriteString(")")
		} else {
			sb.WriteString(stmt.Returns.TypeName)
		}
	}

	sb.WriteString(" LANGUAGE ")
	sb.WriteString(stmt.Language)

	if stmt.Volatility != "" {
		sb.WriteString(" ")
		sb.WriteString(stmt.Volatility)
	}

	if stmt.Security != "" {
		sb.WriteString(" SECURITY ")
		sb.WriteString(stmt.Security)
	}

	sb.WriteString(" AS $$")
	sb.WriteString(stmt.Body)
	sb.WriteString("$$")

	return sb.String(), nil
}

func (g *Generator) generateDropStmt(stmt *DropStmt) (string, error) {
	var sb strings.Builder

	sb.WriteString("DROP ")
	sb.WriteString(stmt.Type)
	sb.WriteString(" ")
	if stmt.IfExists {
		sb.WriteString("IF EXISTS ")
	}
	sb.WriteString(stmt.Name)

	return sb.String(), nil
}
