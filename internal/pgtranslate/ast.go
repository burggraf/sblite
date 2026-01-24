package pgtranslate

// Node is the interface implemented by all AST nodes.
type Node interface {
	// Position returns the position of the first character of the node.
	Position() Position
	// nodeType returns a string identifying the node type for debugging.
	nodeType() string
}

// Expr is the interface implemented by all expression nodes.
type Expr interface {
	Node
	exprNode()
}

// Stmt is the interface implemented by all statement nodes.
type Stmt interface {
	Node
	stmtNode()
}

// ----------------------------------------------------------------------------
// Expression Nodes
// ----------------------------------------------------------------------------

// Identifier represents an identifier (column name, table name, etc.).
type Identifier struct {
	Pos    Position
	Name   string
	Quoted bool // true if it was a quoted identifier ("name")
}

func (n *Identifier) Position() Position { return n.Pos }
func (n *Identifier) nodeType() string   { return "Identifier" }
func (n *Identifier) exprNode()          {}

// Literal represents a literal value (string, number, boolean, null).
type Literal struct {
	Pos   Position
	Type  LiteralType
	Value string // the raw value
}

type LiteralType int

const (
	LitString LiteralType = iota
	LitNumber
	LitBoolean
	LitNull
	LitDollarQuoted
)

func (n *Literal) Position() Position { return n.Pos }
func (n *Literal) nodeType() string   { return "Literal" }
func (n *Literal) exprNode()          {}

// BinaryOp represents a binary operation (a + b, a AND b, etc.).
type BinaryOp struct {
	Pos   Position
	Op    string
	Left  Expr
	Right Expr
}

func (n *BinaryOp) Position() Position { return n.Pos }
func (n *BinaryOp) nodeType() string   { return "BinaryOp" }
func (n *BinaryOp) exprNode()          {}

// UnaryOp represents a unary operation (NOT x, -x).
type UnaryOp struct {
	Pos     Position
	Op      string
	Operand Expr
}

func (n *UnaryOp) Position() Position { return n.Pos }
func (n *UnaryOp) nodeType() string   { return "UnaryOp" }
func (n *UnaryOp) exprNode()          {}

// FunctionCall represents a function call (func(arg1, arg2)).
type FunctionCall struct {
	Pos      Position
	Name     string
	Args     []Expr
	Distinct bool // for COUNT(DISTINCT x)
	Star     bool // for COUNT(*)
	OrderBy  []OrderByExpr // for aggregate functions with ORDER BY
}

func (n *FunctionCall) Position() Position { return n.Pos }
func (n *FunctionCall) nodeType() string   { return "FunctionCall" }
func (n *FunctionCall) exprNode()          {}

// TypeCast represents a PostgreSQL type cast (expr::type).
type TypeCast struct {
	Pos      Position
	Expr     Expr
	TypeName string
	TypeArgs []string // for types like vector(256) or numeric(10,2)
}

func (n *TypeCast) Position() Position { return n.Pos }
func (n *TypeCast) nodeType() string   { return "TypeCast" }
func (n *TypeCast) exprNode()          {}

// CastExpr represents a CAST(expr AS type) expression.
type CastExpr struct {
	Pos      Position
	Expr     Expr
	TypeName string
	TypeArgs []string
}

func (n *CastExpr) Position() Position { return n.Pos }
func (n *CastExpr) nodeType() string   { return "CastExpr" }
func (n *CastExpr) exprNode()          {}

// JsonAccess represents JSON field access (expr->'key' or expr->>'key').
type JsonAccess struct {
	Pos      Position
	Expr     Expr
	Key      Expr   // can be string literal or number for array access
	AsText   bool   // true for ->> (returns text), false for -> (returns JSON)
}

func (n *JsonAccess) Position() Position { return n.Pos }
func (n *JsonAccess) nodeType() string   { return "JsonAccess" }
func (n *JsonAccess) exprNode()          {}

// ParenExpr represents a parenthesized expression ((expr)).
type ParenExpr struct {
	Pos  Position
	Expr Expr
}

func (n *ParenExpr) Position() Position { return n.Pos }
func (n *ParenExpr) nodeType() string   { return "ParenExpr" }
func (n *ParenExpr) exprNode()          {}

// ArrayExpr represents an array literal (ARRAY[1,2,3] or '{1,2,3}').
type ArrayExpr struct {
	Pos      Position
	Elements []Expr
}

func (n *ArrayExpr) Position() Position { return n.Pos }
func (n *ArrayExpr) nodeType() string   { return "ArrayExpr" }
func (n *ArrayExpr) exprNode()          {}

// ArraySubscript represents array subscript access (arr[1]).
type ArraySubscript struct {
	Pos   Position
	Array Expr
	Index Expr
}

func (n *ArraySubscript) Position() Position { return n.Pos }
func (n *ArraySubscript) nodeType() string   { return "ArraySubscript" }
func (n *ArraySubscript) exprNode()          {}

// CaseExpr represents a CASE expression.
type CaseExpr struct {
	Pos     Position
	Operand Expr          // for CASE x WHEN ..., nil for CASE WHEN ...
	Whens   []*CaseWhen
	Else    Expr          // optional ELSE clause
}

func (n *CaseExpr) Position() Position { return n.Pos }
func (n *CaseExpr) nodeType() string   { return "CaseExpr" }
func (n *CaseExpr) exprNode()          {}

// CaseWhen represents a WHEN clause in a CASE expression.
type CaseWhen struct {
	Pos    Position
	When   Expr
	Then   Expr
}

func (n *CaseWhen) Position() Position { return n.Pos }
func (n *CaseWhen) nodeType() string   { return "CaseWhen" }

// BetweenExpr represents a BETWEEN expression (x BETWEEN a AND b).
type BetweenExpr struct {
	Pos     Position
	Expr    Expr
	Low     Expr
	High    Expr
	Not     bool
}

func (n *BetweenExpr) Position() Position { return n.Pos }
func (n *BetweenExpr) nodeType() string   { return "BetweenExpr" }
func (n *BetweenExpr) exprNode()          {}

// InExpr represents an IN expression (x IN (1, 2, 3)).
type InExpr struct {
	Pos    Position
	Expr   Expr
	List   []Expr    // for IN (a, b, c)
	Query  *SelectStmt // for IN (SELECT ...)
	Not    bool
}

func (n *InExpr) Position() Position { return n.Pos }
func (n *InExpr) nodeType() string   { return "InExpr" }
func (n *InExpr) exprNode()          {}

// IsExpr represents an IS expression (x IS NULL, x IS NOT NULL, x IS TRUE).
type IsExpr struct {
	Pos   Position
	Expr  Expr
	Value string // "NULL", "NOT NULL", "TRUE", "FALSE", "UNKNOWN"
}

func (n *IsExpr) Position() Position { return n.Pos }
func (n *IsExpr) nodeType() string   { return "IsExpr" }
func (n *IsExpr) exprNode()          {}

// ExistsExpr represents an EXISTS expression.
type ExistsExpr struct {
	Pos   Position
	Query *SelectStmt
	Not   bool
}

func (n *ExistsExpr) Position() Position { return n.Pos }
func (n *ExistsExpr) nodeType() string   { return "ExistsExpr" }
func (n *ExistsExpr) exprNode()          {}

// ExtractExpr represents EXTRACT(field FROM expr).
type ExtractExpr struct {
	Pos   Position
	Field string // YEAR, MONTH, DAY, HOUR, MINUTE, SECOND, etc.
	Expr  Expr
}

func (n *ExtractExpr) Position() Position { return n.Pos }
func (n *ExtractExpr) nodeType() string   { return "ExtractExpr" }
func (n *ExtractExpr) exprNode()          {}

// IntervalExpr represents an INTERVAL expression.
type IntervalExpr struct {
	Pos   Position
	Value string // the interval string, e.g., "1 day"
}

func (n *IntervalExpr) Position() Position { return n.Pos }
func (n *IntervalExpr) nodeType() string   { return "IntervalExpr" }
func (n *IntervalExpr) exprNode()          {}

// SubqueryExpr represents a subquery used as an expression.
type SubqueryExpr struct {
	Pos   Position
	Query *SelectStmt
}

func (n *SubqueryExpr) Position() Position { return n.Pos }
func (n *SubqueryExpr) nodeType() string   { return "SubqueryExpr" }
func (n *SubqueryExpr) exprNode()          {}

// QualifiedRef represents a qualified column reference (table.column).
type QualifiedRef struct {
	Pos    Position
	Table  string
	Column string
}

func (n *QualifiedRef) Position() Position { return n.Pos }
func (n *QualifiedRef) nodeType() string   { return "QualifiedRef" }
func (n *QualifiedRef) exprNode()          {}

// StarExpr represents the * in SELECT *.
type StarExpr struct {
	Pos   Position
	Table string // optional table qualifier for table.*
}

func (n *StarExpr) Position() Position { return n.Pos }
func (n *StarExpr) nodeType() string   { return "StarExpr" }
func (n *StarExpr) exprNode()          {}

// ----------------------------------------------------------------------------
// Statement Nodes
// ----------------------------------------------------------------------------

// SelectStmt represents a SELECT statement.
type SelectStmt struct {
	Pos       Position
	With      *WithClause
	Distinct  bool
	Columns   []*SelectColumn
	From      *FromClause
	Where     Expr
	GroupBy   []Expr
	Having    Expr
	OrderBy   []OrderByExpr
	Limit     Expr
	Offset    Expr
	Union     *SetOperation
}

func (n *SelectStmt) Position() Position { return n.Pos }
func (n *SelectStmt) nodeType() string   { return "SelectStmt" }
func (n *SelectStmt) stmtNode()          {}

// SelectColumn represents a column in SELECT clause.
type SelectColumn struct {
	Pos   Position
	Expr  Expr
	Alias string
}

func (n *SelectColumn) Position() Position { return n.Pos }
func (n *SelectColumn) nodeType() string   { return "SelectColumn" }

// FromClause represents the FROM clause.
type FromClause struct {
	Pos    Position
	Tables []*TableRef
}

func (n *FromClause) Position() Position { return n.Pos }
func (n *FromClause) nodeType() string   { return "FromClause" }

// TableRef represents a table reference in FROM clause.
type TableRef struct {
	Pos      Position
	Name     string
	Alias    string
	Subquery *SelectStmt // for (SELECT ...) AS alias
	Join     *JoinClause
}

func (n *TableRef) Position() Position { return n.Pos }
func (n *TableRef) nodeType() string   { return "TableRef" }

// JoinClause represents a JOIN clause.
type JoinClause struct {
	Pos       Position
	Type      string // INNER, LEFT, RIGHT, FULL, CROSS
	Table     *TableRef
	On        Expr
	Using     []string
}

func (n *JoinClause) Position() Position { return n.Pos }
func (n *JoinClause) nodeType() string   { return "JoinClause" }

// OrderByExpr represents an expression in ORDER BY clause.
type OrderByExpr struct {
	Pos       Position
	Expr      Expr
	Desc      bool
	NullsFirst *bool // nil means default, true means NULLS FIRST, false means NULLS LAST
}

func (n *OrderByExpr) Position() Position { return n.Pos }
func (n *OrderByExpr) nodeType() string   { return "OrderByExpr" }

// WithClause represents a WITH clause (CTE).
type WithClause struct {
	Pos       Position
	Recursive bool
	CTEs      []*CTE
}

func (n *WithClause) Position() Position { return n.Pos }
func (n *WithClause) nodeType() string   { return "WithClause" }

// CTE represents a Common Table Expression.
type CTE struct {
	Pos     Position
	Name    string
	Columns []string
	Query   *SelectStmt
}

func (n *CTE) Position() Position { return n.Pos }
func (n *CTE) nodeType() string   { return "CTE" }

// SetOperation represents UNION, INTERSECT, EXCEPT.
type SetOperation struct {
	Pos   Position
	Type  string // UNION, INTERSECT, EXCEPT
	All   bool
	Right *SelectStmt
}

func (n *SetOperation) Position() Position { return n.Pos }
func (n *SetOperation) nodeType() string   { return "SetOperation" }

// InsertStmt represents an INSERT statement.
type InsertStmt struct {
	Pos        Position
	Table      string
	Columns    []string
	Values     [][]Expr
	Query      *SelectStmt // for INSERT ... SELECT
	OnConflict *OnConflict
	Returning  []Expr
}

func (n *InsertStmt) Position() Position { return n.Pos }
func (n *InsertStmt) nodeType() string   { return "InsertStmt" }
func (n *InsertStmt) stmtNode()          {}

// OnConflict represents ON CONFLICT clause.
type OnConflict struct {
	Pos       Position
	Target    []string // conflict columns
	DoNothing bool
	Updates   []*Assignment // for DO UPDATE SET
	Where     Expr
}

func (n *OnConflict) Position() Position { return n.Pos }
func (n *OnConflict) nodeType() string   { return "OnConflict" }

// Assignment represents a column assignment (col = expr).
type Assignment struct {
	Pos    Position
	Column string
	Value  Expr
}

func (n *Assignment) Position() Position { return n.Pos }
func (n *Assignment) nodeType() string   { return "Assignment" }

// UpdateStmt represents an UPDATE statement.
type UpdateStmt struct {
	Pos       Position
	Table     string
	Alias     string
	Set       []*Assignment
	From      *FromClause
	Where     Expr
	Returning []Expr
}

func (n *UpdateStmt) Position() Position { return n.Pos }
func (n *UpdateStmt) nodeType() string   { return "UpdateStmt" }
func (n *UpdateStmt) stmtNode()          {}

// DeleteStmt represents a DELETE statement.
type DeleteStmt struct {
	Pos       Position
	Table     string
	Alias     string
	Using     *FromClause
	Where     Expr
	Returning []Expr
}

func (n *DeleteStmt) Position() Position { return n.Pos }
func (n *DeleteStmt) nodeType() string   { return "DeleteStmt" }
func (n *DeleteStmt) stmtNode()          {}

// CreateTableStmt represents a CREATE TABLE statement.
type CreateTableStmt struct {
	Pos         Position
	IfNotExists bool
	Name        string
	Columns     []*ColumnDef
	Constraints []*TableConstraint
}

func (n *CreateTableStmt) Position() Position { return n.Pos }
func (n *CreateTableStmt) nodeType() string   { return "CreateTableStmt" }
func (n *CreateTableStmt) stmtNode()          {}

// ColumnDef represents a column definition.
type ColumnDef struct {
	Pos         Position
	Name        string
	TypeName    string
	TypeArgs    []string // for types like numeric(10,2) or vector(256)
	NotNull     bool
	Default     Expr
	PrimaryKey  bool
	Unique      bool
	References  *ForeignKeyRef
	Check       Expr
}

func (n *ColumnDef) Position() Position { return n.Pos }
func (n *ColumnDef) nodeType() string   { return "ColumnDef" }

// ForeignKeyRef represents a REFERENCES clause.
type ForeignKeyRef struct {
	Pos      Position
	Table    string
	Column   string
	OnDelete string // CASCADE, SET NULL, etc.
	OnUpdate string
}

func (n *ForeignKeyRef) Position() Position { return n.Pos }
func (n *ForeignKeyRef) nodeType() string   { return "ForeignKeyRef" }

// TableConstraint represents a table-level constraint.
type TableConstraint struct {
	Pos        Position
	Name       string
	Type       string // PRIMARY KEY, UNIQUE, FOREIGN KEY, CHECK
	Columns    []string
	References *ForeignKeyRef
	Check      Expr
}

func (n *TableConstraint) Position() Position { return n.Pos }
func (n *TableConstraint) nodeType() string   { return "TableConstraint" }

// CreateFunctionStmt represents a CREATE FUNCTION statement.
type CreateFunctionStmt struct {
	Pos        Position
	OrReplace  bool
	Name       string
	Args       []*FunctionArg
	Returns    *FunctionReturn
	Language   string
	Volatility string // IMMUTABLE, STABLE, VOLATILE
	Security   string // DEFINER, INVOKER
	Body       string // the function body (dollar-quoted)
}

func (n *CreateFunctionStmt) Position() Position { return n.Pos }
func (n *CreateFunctionStmt) nodeType() string   { return "CreateFunctionStmt" }
func (n *CreateFunctionStmt) stmtNode()          {}

// FunctionArg represents a function argument.
type FunctionArg struct {
	Pos      Position
	Name     string
	TypeName string
	Default  Expr
	Mode     string // IN, OUT, INOUT, VARIADIC
}

func (n *FunctionArg) Position() Position { return n.Pos }
func (n *FunctionArg) nodeType() string   { return "FunctionArg" }

// FunctionReturn represents the RETURNS clause.
type FunctionReturn struct {
	Pos       Position
	TypeName  string
	IsSetOf   bool
	IsTable   bool
	TableCols []*ColumnDef // for RETURNS TABLE(...)
}

func (n *FunctionReturn) Position() Position { return n.Pos }
func (n *FunctionReturn) nodeType() string   { return "FunctionReturn" }

// DropStmt represents DROP TABLE/FUNCTION/INDEX statement.
type DropStmt struct {
	Pos      Position
	Type     string // TABLE, FUNCTION, INDEX
	IfExists bool
	Name     string
}

func (n *DropStmt) Position() Position { return n.Pos }
func (n *DropStmt) nodeType() string   { return "DropStmt" }
func (n *DropStmt) stmtNode()          {}

// RawSQL represents unparsed SQL that we pass through.
type RawSQL struct {
	Pos  Position
	Text string
}

func (n *RawSQL) Position() Position { return n.Pos }
func (n *RawSQL) nodeType() string   { return "RawSQL" }
func (n *RawSQL) stmtNode()          {}
func (n *RawSQL) exprNode()          {}
