package pgtranslate

import (
	"fmt"
	"strings"
)

// Parser parses SQL into an AST.
type Parser struct {
	lexer   *Lexer
	current Token
	peek    Token
	errors  []error
}

// NewParser creates a new parser for the given input.
func NewParser(input string) *Parser {
	p := &Parser{
		lexer: NewLexer(input),
	}
	// Read two tokens to initialize current and peek
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.current = p.peek
	p.peek = p.lexer.NextToken()
}

func (p *Parser) curIs(t TokenType) bool {
	return p.current.Type == t
}

func (p *Parser) peekIs(t TokenType) bool {
	return p.peek.Type == t
}

func (p *Parser) expect(t TokenType) error {
	if !p.curIs(t) {
		return fmt.Errorf("expected %s, got %s at %s", TokenTypeName(t), TokenTypeName(p.current.Type), p.current.Pos)
	}
	p.nextToken()
	return nil
}

func (p *Parser) expectPeek(t TokenType) error {
	if !p.peekIs(t) {
		return fmt.Errorf("expected %s, got %s at %s", TokenTypeName(t), TokenTypeName(p.peek.Type), p.peek.Pos)
	}
	p.nextToken()
	return nil
}

// Errors returns any parse errors.
func (p *Parser) Errors() []error {
	return p.errors
}

// ParseExpr parses a single expression.
func (p *Parser) ParseExpr() (Expr, error) {
	return p.parseExpr(0)
}

// Operator precedence levels (higher = tighter binding)
const (
	precLowest     = 0
	precOr         = 1
	precAnd        = 2
	precNot        = 3
	precIs         = 4
	precComparison = 5
	precBetween    = 6
	precIn         = 7
	precLike       = 8
	precAddSub     = 9
	precMulDiv     = 10
	precUnary      = 11
	precCast       = 12
	precJson       = 13
	precCall       = 14
)

func (p *Parser) precedence(t TokenType) int {
	switch t {
	case TokenOr:
		return precOr
	case TokenAnd:
		return precAnd
	case TokenEq, TokenNe, TokenLt, TokenLe, TokenGt, TokenGe:
		return precComparison
	case TokenLike, TokenILike:
		return precLike
	case TokenPlus, TokenMinus:
		return precAddSub
	case TokenStar, TokenSlash, TokenPercent:
		return precMulDiv
	case TokenConcat:
		return precAddSub
	case TokenCast:
		return precCast
	case TokenJsonArrow, TokenJsonArrow2:
		return precJson
	default:
		return precLowest
	}
}

// parseExpr is the main Pratt parser entry point.
func (p *Parser) parseExpr(minPrec int) (Expr, error) {
	left, err := p.parsePrefixExpr()
	if err != nil {
		return nil, err
	}

	for {
		prec := p.precedence(p.current.Type)
		if prec <= minPrec {
			break
		}

		left, err = p.parseInfixExpr(left, prec)
		if err != nil {
			return nil, err
		}
	}

	return left, nil
}

func (p *Parser) parsePrefixExpr() (Expr, error) {
	switch p.current.Type {
	case TokenIdent:
		return p.parseIdentOrCall()
	case TokenQIdent:
		ident := &Identifier{
			Pos:    p.current.Pos,
			Name:   p.current.Value,
			Quoted: true,
		}
		p.nextToken()
		return ident, nil
	case TokenString:
		lit := &Literal{
			Pos:   p.current.Pos,
			Type:  LitString,
			Value: p.current.Value,
		}
		p.nextToken()
		return lit, nil
	case TokenNumber:
		lit := &Literal{
			Pos:   p.current.Pos,
			Type:  LitNumber,
			Value: p.current.Value,
		}
		p.nextToken()
		return lit, nil
	case TokenDollarStr:
		lit := &Literal{
			Pos:   p.current.Pos,
			Type:  LitDollarQuoted,
			Value: p.current.Value,
		}
		p.nextToken()
		return lit, nil
	case TokenTrue, TokenFalse:
		lit := &Literal{
			Pos:   p.current.Pos,
			Type:  LitBoolean,
			Value: strings.ToUpper(p.current.Value),
		}
		p.nextToken()
		return lit, nil
	case TokenNull:
		lit := &Literal{
			Pos:   p.current.Pos,
			Type:  LitNull,
			Value: "NULL",
		}
		p.nextToken()
		return lit, nil
	case TokenLParen:
		return p.parseParenExpr()
	case TokenNot:
		return p.parseUnaryNot()
	case TokenMinus:
		return p.parseUnaryMinus()
	case TokenCase:
		return p.parseCaseExpr()
	case TokenExtract:
		return p.parseExtract()
	case TokenInterval:
		return p.parseInterval()
	case TokenCast_kw:
		return p.parseCastExpr()
	case TokenExists:
		return p.parseExists()
	case TokenCoalesce, TokenNullif, TokenGreatest, TokenLeast:
		return p.parseBuiltinFunc()
	case TokenNow:
		return p.parseNow()
	case TokenCount, TokenSum, TokenAvg, TokenMin, TokenMax:
		return p.parseAggregateFunc()
	case TokenStar:
		star := &StarExpr{Pos: p.current.Pos}
		p.nextToken()
		return star, nil
	default:
		return nil, fmt.Errorf("unexpected token %s at %s", TokenTypeName(p.current.Type), p.current.Pos)
	}
}

func (p *Parser) parseIdentOrCall() (Expr, error) {
	pos := p.current.Pos
	name := p.current.Value
	p.nextToken()

	// Check for qualified reference (table.column)
	if p.curIs(TokenDot) {
		p.nextToken()
		if p.curIs(TokenStar) {
			// table.*
			star := &StarExpr{Pos: pos, Table: name}
			p.nextToken()
			return star, nil
		}
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected identifier after '.', got %s at %s", TokenTypeName(p.current.Type), p.current.Pos)
		}
		colName := p.current.Value
		p.nextToken()
		return &QualifiedRef{Pos: pos, Table: name, Column: colName}, nil
	}

	// Check for function call
	if p.curIs(TokenLParen) {
		return p.parseFunctionCall(pos, name)
	}

	return &Identifier{Pos: pos, Name: name}, nil
}

func (p *Parser) parseFunctionCall(pos Position, name string) (Expr, error) {
	p.nextToken() // skip (

	call := &FunctionCall{Pos: pos, Name: name}

	// Check for COUNT(*)
	if strings.ToUpper(name) == "COUNT" && p.curIs(TokenStar) {
		call.Star = true
		p.nextToken()
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return call, nil
	}

	// Check for DISTINCT
	if p.curIs(TokenDistinct) {
		call.Distinct = true
		p.nextToken()
	}

	// Parse arguments
	if !p.curIs(TokenRParen) {
		for {
			arg, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			call.Args = append(call.Args, arg)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken() // skip comma
		}
	}

	// Check for ORDER BY in aggregate
	if p.curIs(TokenOrder) {
		p.nextToken() // skip ORDER
		if err := p.expect(TokenBy); err != nil {
			return nil, err
		}
		for {
			orderExpr, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			ob := OrderByExpr{Pos: orderExpr.Position(), Expr: orderExpr}
			if p.curIs(TokenDesc) {
				ob.Desc = true
				p.nextToken()
			} else if p.curIs(TokenAsc) {
				p.nextToken()
			}
			call.OrderBy = append(call.OrderBy, ob)
			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return call, nil
}

func (p *Parser) parseParenExpr() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip (

	// Check for subquery
	if p.curIs(TokenSelect) || p.curIs(TokenWith) {
		query, err := p.ParseSelect()
		if err != nil {
			return nil, err
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return &SubqueryExpr{Pos: pos, Query: query}, nil
	}

	expr, err := p.parseExpr(precLowest)
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return &ParenExpr{Pos: pos, Expr: expr}, nil
}

func (p *Parser) parseUnaryNot() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip NOT

	// Check for NOT EXISTS
	if p.curIs(TokenExists) {
		return p.parseExists()
	}

	operand, err := p.parseExpr(precNot)
	if err != nil {
		return nil, err
	}

	return &UnaryOp{Pos: pos, Op: "NOT", Operand: operand}, nil
}

func (p *Parser) parseUnaryMinus() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip -

	operand, err := p.parseExpr(precUnary)
	if err != nil {
		return nil, err
	}

	return &UnaryOp{Pos: pos, Op: "-", Operand: operand}, nil
}

func (p *Parser) parseInfixExpr(left Expr, prec int) (Expr, error) {
	switch p.current.Type {
	case TokenCast:
		return p.parseTypeCast(left)
	case TokenJsonArrow, TokenJsonArrow2:
		return p.parseJsonAccess(left)
	case TokenIs:
		return p.parseIsExpr(left)
	case TokenIn:
		return p.parseInExpr(left, false)
	case TokenNot:
		// Check for NOT IN, NOT LIKE, NOT BETWEEN
		p.nextToken() // skip NOT
		switch p.current.Type {
		case TokenIn:
			return p.parseInExpr(left, true)
		case TokenLike, TokenILike:
			return p.parseLikeExpr(left, true)
		case TokenBetween:
			return p.parseBetweenExpr(left, true)
		default:
			return nil, fmt.Errorf("expected IN, LIKE, or BETWEEN after NOT at %s", p.current.Pos)
		}
	case TokenBetween:
		return p.parseBetweenExpr(left, false)
	case TokenLike, TokenILike:
		return p.parseLikeExpr(left, false)
	case TokenLBracket:
		return p.parseArraySubscript(left)
	default:
		return p.parseBinaryOp(left, prec)
	}
}

func (p *Parser) parseBinaryOp(left Expr, prec int) (Expr, error) {
	pos := p.current.Pos
	op := p.current.Value
	opType := p.current.Type
	p.nextToken()

	// Map token types to operator strings
	switch opType {
	case TokenAnd:
		op = "AND"
	case TokenOr:
		op = "OR"
	case TokenConcat:
		op = "||"
	}

	right, err := p.parseExpr(prec)
	if err != nil {
		return nil, err
	}

	return &BinaryOp{Pos: pos, Op: op, Left: left, Right: right}, nil
}

func (p *Parser) parseTypeCast(left Expr) (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip ::

	typeName, typeArgs, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}

	return &TypeCast{Pos: pos, Expr: left, TypeName: typeName, TypeArgs: typeArgs}, nil
}

func (p *Parser) parseTypeName() (string, []string, error) {
	if !p.curIs(TokenIdent) {
		return "", nil, fmt.Errorf("expected type name, got %s at %s", TokenTypeName(p.current.Type), p.current.Pos)
	}

	typeName := p.current.Value
	p.nextToken()

	var typeArgs []string

	// Check for type arguments like (256) or (10, 2)
	if p.curIs(TokenLParen) {
		p.nextToken()
		for {
			if !p.curIs(TokenNumber) && !p.curIs(TokenIdent) {
				return "", nil, fmt.Errorf("expected type argument at %s", p.current.Pos)
			}
			typeArgs = append(typeArgs, p.current.Value)
			p.nextToken()
			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
		if err := p.expect(TokenRParen); err != nil {
			return "", nil, err
		}
	}

	// Check for array notation []
	if p.curIs(TokenLBracket) {
		p.nextToken()
		if err := p.expect(TokenRBracket); err != nil {
			return "", nil, err
		}
		typeName += "[]"
	}

	return typeName, typeArgs, nil
}

func (p *Parser) parseJsonAccess(left Expr) (Expr, error) {
	pos := p.current.Pos
	asText := p.current.Type == TokenJsonArrow2
	p.nextToken()

	key, err := p.parsePrefixExpr()
	if err != nil {
		return nil, err
	}

	return &JsonAccess{Pos: pos, Expr: left, Key: key, AsText: asText}, nil
}

func (p *Parser) parseIsExpr(left Expr) (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip IS

	var value string
	if p.curIs(TokenNot) {
		p.nextToken()
		if p.curIs(TokenNull) {
			value = "NOT NULL"
		} else if p.curIs(TokenTrue) {
			value = "NOT TRUE"
		} else if p.curIs(TokenFalse) {
			value = "NOT FALSE"
		} else {
			return nil, fmt.Errorf("expected NULL, TRUE, or FALSE after IS NOT at %s", p.current.Pos)
		}
	} else if p.curIs(TokenNull) {
		value = "NULL"
	} else if p.curIs(TokenTrue) {
		value = "TRUE"
	} else if p.curIs(TokenFalse) {
		value = "FALSE"
	} else if p.curIs(TokenDistinct) {
		// IS DISTINCT FROM
		p.nextToken() // skip DISTINCT
		if !p.curIs(TokenFrom) {
			return nil, fmt.Errorf("expected FROM after IS DISTINCT at %s", p.current.Pos)
		}
		p.nextToken() // skip FROM
		right, err := p.parseExpr(precComparison)
		if err != nil {
			return nil, err
		}
		return &BinaryOp{Pos: pos, Op: "IS DISTINCT FROM", Left: left, Right: right}, nil
	} else {
		return nil, fmt.Errorf("expected NULL, TRUE, FALSE, or NOT after IS at %s", p.current.Pos)
	}

	p.nextToken() // skip NULL/TRUE/FALSE

	return &IsExpr{Pos: pos, Expr: left, Value: value}, nil
}

func (p *Parser) parseInExpr(left Expr, not bool) (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip IN

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	inExpr := &InExpr{Pos: pos, Expr: left, Not: not}

	// Check for subquery
	if p.curIs(TokenSelect) || p.curIs(TokenWith) {
		query, err := p.ParseSelect()
		if err != nil {
			return nil, err
		}
		inExpr.Query = query
	} else {
		// Parse list
		for {
			val, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			inExpr.List = append(inExpr.List, val)
			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return inExpr, nil
}

func (p *Parser) parseBetweenExpr(left Expr, not bool) (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip BETWEEN

	low, err := p.parseExpr(precBetween)
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenAnd); err != nil {
		return nil, err
	}

	high, err := p.parseExpr(precBetween)
	if err != nil {
		return nil, err
	}

	return &BetweenExpr{Pos: pos, Expr: left, Low: low, High: high, Not: not}, nil
}

func (p *Parser) parseLikeExpr(left Expr, not bool) (Expr, error) {
	pos := p.current.Pos
	op := "LIKE"
	if p.current.Type == TokenILike {
		op = "ILIKE"
	}
	p.nextToken()

	right, err := p.parseExpr(precLike)
	if err != nil {
		return nil, err
	}

	if not {
		op = "NOT " + op
	}

	return &BinaryOp{Pos: pos, Op: op, Left: left, Right: right}, nil
}

func (p *Parser) parseArraySubscript(left Expr) (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip [

	index, err := p.parseExpr(precLowest)
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenRBracket); err != nil {
		return nil, err
	}

	return &ArraySubscript{Pos: pos, Array: left, Index: index}, nil
}

func (p *Parser) parseCaseExpr() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip CASE

	caseExpr := &CaseExpr{Pos: pos}

	// Check for simple CASE (CASE expr WHEN ...)
	if !p.curIs(TokenWhen) {
		operand, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		caseExpr.Operand = operand
	}

	// Parse WHEN clauses
	for p.curIs(TokenWhen) {
		whenPos := p.current.Pos
		p.nextToken() // skip WHEN

		when, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}

		if err := p.expect(TokenThen); err != nil {
			return nil, err
		}

		then, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}

		caseExpr.Whens = append(caseExpr.Whens, &CaseWhen{Pos: whenPos, When: when, Then: then})
	}

	// Parse optional ELSE
	if p.curIs(TokenElse) {
		p.nextToken()
		elseExpr, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		caseExpr.Else = elseExpr
	}

	if err := p.expect(TokenEnd); err != nil {
		return nil, err
	}

	return caseExpr, nil
}

func (p *Parser) parseExtract() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip EXTRACT

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	// Parse field (YEAR, MONTH, DAY, etc.)
	if !p.curIs(TokenIdent) {
		return nil, fmt.Errorf("expected field name in EXTRACT at %s", p.current.Pos)
	}
	field := strings.ToUpper(p.current.Value)
	p.nextToken()

	// Expect FROM
	if !p.curIs(TokenFrom) {
		return nil, fmt.Errorf("expected FROM in EXTRACT at %s", p.current.Pos)
	}
	p.nextToken()

	expr, err := p.parseExpr(precLowest)
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return &ExtractExpr{Pos: pos, Field: field, Expr: expr}, nil
}

func (p *Parser) parseInterval() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip INTERVAL

	if !p.curIs(TokenString) {
		return nil, fmt.Errorf("expected string after INTERVAL at %s", p.current.Pos)
	}

	value := p.current.Value
	p.nextToken()

	return &IntervalExpr{Pos: pos, Value: value}, nil
}

func (p *Parser) parseCastExpr() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip CAST

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	expr, err := p.parseExpr(precLowest)
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenAs); err != nil {
		return nil, err
	}

	typeName, typeArgs, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return &CastExpr{Pos: pos, Expr: expr, TypeName: typeName, TypeArgs: typeArgs}, nil
}

func (p *Parser) parseExists() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip EXISTS

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	query, err := p.ParseSelect()
	if err != nil {
		return nil, err
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return &ExistsExpr{Pos: pos, Query: query}, nil
}

func (p *Parser) parseBuiltinFunc() (Expr, error) {
	pos := p.current.Pos
	name := strings.ToUpper(p.current.Value)
	p.nextToken()

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	call := &FunctionCall{Pos: pos, Name: name}

	for {
		arg, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		call.Args = append(call.Args, arg)
		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return call, nil
}

func (p *Parser) parseNow() (Expr, error) {
	pos := p.current.Pos
	p.nextToken() // skip NOW

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}
	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return &FunctionCall{Pos: pos, Name: "NOW"}, nil
}

func (p *Parser) parseAggregateFunc() (Expr, error) {
	pos := p.current.Pos
	name := strings.ToUpper(p.current.Value)
	p.nextToken()

	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	call := &FunctionCall{Pos: pos, Name: name}

	// Check for COUNT(*)
	if name == "COUNT" && p.curIs(TokenStar) {
		call.Star = true
		p.nextToken()
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
		return call, nil
	}

	// Check for DISTINCT
	if p.curIs(TokenDistinct) {
		call.Distinct = true
		p.nextToken()
	}

	// Parse arguments
	for {
		arg, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		call.Args = append(call.Args, arg)
		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return call, nil
}

// ParseSelect parses a SELECT statement with full support for CTEs and set operations.
func (p *Parser) ParseSelect() (*SelectStmt, error) {
	pos := p.current.Pos
	stmt := &SelectStmt{Pos: pos}

	// Parse WITH clause (CTEs)
	if p.curIs(TokenWith) {
		with, err := p.parseWithClause()
		if err != nil {
			return nil, err
		}
		stmt.With = with
	}

	if err := p.expect(TokenSelect); err != nil {
		return nil, err
	}

	// Check for DISTINCT
	if p.curIs(TokenDistinct) {
		stmt.Distinct = true
		p.nextToken()
	}

	// Parse select columns
	for {
		colPos := p.current.Pos
		expr, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}

		col := &SelectColumn{Pos: colPos, Expr: expr}

		// Check for alias
		if p.curIs(TokenAs) {
			p.nextToken()
			if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
				return nil, fmt.Errorf("expected alias after AS at %s", p.current.Pos)
			}
			col.Alias = p.current.Value
			p.nextToken()
		} else if p.curIs(TokenIdent) && !isSelectClauseKeyword(p.current.Type) {
			// Implicit alias (but not if it's a keyword like FROM, WHERE, etc.)
			col.Alias = p.current.Value
			p.nextToken()
		}

		stmt.Columns = append(stmt.Columns, col)

		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	// Parse FROM clause
	if p.curIs(TokenFrom) {
		p.nextToken()
		stmt.From = &FromClause{Pos: p.current.Pos}

		for {
			tableRef, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From.Tables = append(stmt.From.Tables, tableRef)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	// Parse WHERE clause
	if p.curIs(TokenWhere) {
		p.nextToken()
		where, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// Parse GROUP BY
	if p.curIs(TokenGroup) {
		p.nextToken()
		if err := p.expect(TokenBy); err != nil {
			return nil, err
		}
		for {
			expr, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			stmt.GroupBy = append(stmt.GroupBy, expr)
			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	// Parse HAVING
	if p.curIs(TokenHaving) {
		p.nextToken()
		having, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		stmt.Having = having
	}

	// Parse ORDER BY
	if p.curIs(TokenOrder) {
		p.nextToken()
		if err := p.expect(TokenBy); err != nil {
			return nil, err
		}
		for {
			expr, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			ob := OrderByExpr{Pos: expr.Position(), Expr: expr}
			if p.curIs(TokenDesc) {
				ob.Desc = true
				p.nextToken()
			} else if p.curIs(TokenAsc) {
				p.nextToken()
			}
			stmt.OrderBy = append(stmt.OrderBy, ob)
			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	// Parse LIMIT
	if p.curIs(TokenLimit) {
		p.nextToken()
		limit, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	// Parse OFFSET
	if p.curIs(TokenOffset) {
		p.nextToken()
		offset, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		stmt.Offset = offset
	}

	// Parse UNION/INTERSECT/EXCEPT
	if p.curIs(TokenUnion) || p.curIs(TokenIntersect) || p.curIs(TokenExcept) {
		setOp, err := p.parseSetOperation()
		if err != nil {
			return nil, err
		}
		stmt.Union = setOp
	}

	return stmt, nil
}

// parseWithClause parses a WITH clause (CTEs).
func (p *Parser) parseWithClause() (*WithClause, error) {
	pos := p.current.Pos
	p.nextToken() // skip WITH

	with := &WithClause{Pos: pos}

	// Check for RECURSIVE
	if p.curIs(TokenRecursive) {
		with.Recursive = true
		p.nextToken()
	}

	// Parse CTEs
	for {
		cte, err := p.parseCTE()
		if err != nil {
			return nil, err
		}
		with.CTEs = append(with.CTEs, cte)

		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	return with, nil
}

// parseCTE parses a single CTE.
func (p *Parser) parseCTE() (*CTE, error) {
	pos := p.current.Pos

	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected CTE name at %s", p.current.Pos)
	}

	cte := &CTE{Pos: pos, Name: p.current.Value}
	p.nextToken()

	// Optional column list
	if p.curIs(TokenLParen) {
		p.nextToken()
		for {
			if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
				return nil, fmt.Errorf("expected column name in CTE at %s", p.current.Pos)
			}
			cte.Columns = append(cte.Columns, p.current.Value)
			p.nextToken()

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	}

	// AS keyword
	if err := p.expect(TokenAs); err != nil {
		return nil, err
	}

	// Opening paren for subquery
	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	// Parse the CTE query
	query, err := p.ParseSelect()
	if err != nil {
		return nil, err
	}
	cte.Query = query

	// Closing paren
	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return cte, nil
}

// parseSetOperation parses UNION/INTERSECT/EXCEPT.
func (p *Parser) parseSetOperation() (*SetOperation, error) {
	pos := p.current.Pos
	setOp := &SetOperation{Pos: pos}

	switch p.current.Type {
	case TokenUnion:
		setOp.Type = "UNION"
	case TokenIntersect:
		setOp.Type = "INTERSECT"
	case TokenExcept:
		setOp.Type = "EXCEPT"
	default:
		return nil, fmt.Errorf("expected UNION, INTERSECT, or EXCEPT at %s", p.current.Pos)
	}
	p.nextToken()

	// Check for ALL
	if p.curIs(TokenAll) {
		setOp.All = true
		p.nextToken()
	}

	// Parse the right side SELECT
	right, err := p.ParseSelect()
	if err != nil {
		return nil, err
	}
	setOp.Right = right

	return setOp, nil
}

// isSelectClauseKeyword checks if a token is a keyword that could follow SELECT columns.
func isSelectClauseKeyword(t TokenType) bool {
	switch t {
	case TokenFrom, TokenWhere, TokenGroup, TokenHaving, TokenOrder, TokenLimit, TokenOffset,
		TokenUnion, TokenIntersect, TokenExcept, TokenJoin, TokenLeft, TokenRight,
		TokenInner, TokenOuter, TokenFull, TokenCross, TokenOn:
		return true
	}
	return false
}

func (p *Parser) parseTableRef() (*TableRef, error) {
	pos := p.current.Pos

	ref := &TableRef{Pos: pos}

	// Check for subquery
	if p.curIs(TokenLParen) {
		p.nextToken()
		query, err := p.ParseSelect()
		if err != nil {
			return nil, err
		}
		ref.Subquery = query
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	} else {
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected table name at %s", p.current.Pos)
		}
		ref.Name = p.current.Value
		p.nextToken()
	}

	// Check for alias
	if p.curIs(TokenAs) {
		p.nextToken()
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected alias after AS at %s", p.current.Pos)
		}
		ref.Alias = p.current.Value
		p.nextToken()
	} else if p.curIs(TokenIdent) && !isJoinKeyword(p.current.Type) {
		ref.Alias = p.current.Value
		p.nextToken()
	}

	// Check for JOINs (simplified - full implementation in Phase 3)
	for isJoinKeyword(p.current.Type) {
		join, err := p.parseJoinClause()
		if err != nil {
			return nil, err
		}
		// Chain joins: the last table ref gets the join
		if ref.Join == nil {
			ref.Join = join
		} else {
			// Find the last join in the chain and attach
			last := ref.Join
			for last.Table.Join != nil {
				last = last.Table.Join
			}
			last.Table.Join = join
		}
	}

	return ref, nil
}

func isJoinKeyword(t TokenType) bool {
	switch t {
	case TokenJoin, TokenLeft, TokenRight, TokenInner, TokenOuter, TokenFull, TokenCross:
		return true
	}
	return false
}

func (p *Parser) parseJoinClause() (*JoinClause, error) {
	pos := p.current.Pos
	join := &JoinClause{Pos: pos}

	// Parse join type
	switch p.current.Type {
	case TokenLeft:
		join.Type = "LEFT"
		p.nextToken()
		if p.curIs(TokenOuter) {
			p.nextToken()
		}
	case TokenRight:
		join.Type = "RIGHT"
		p.nextToken()
		if p.curIs(TokenOuter) {
			p.nextToken()
		}
	case TokenFull:
		join.Type = "FULL"
		p.nextToken()
		if p.curIs(TokenOuter) {
			p.nextToken()
		}
	case TokenCross:
		join.Type = "CROSS"
		p.nextToken()
	case TokenInner:
		join.Type = "INNER"
		p.nextToken()
	default:
		join.Type = "INNER"
	}

	if err := p.expect(TokenJoin); err != nil {
		return nil, err
	}

	// Parse table reference
	table, err := p.parseTableRef()
	if err != nil {
		return nil, err
	}
	join.Table = table

	// Parse ON or USING clause
	if p.curIs(TokenOn) {
		p.nextToken()
		on, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		join.On = on
	} else if p.curIs(TokenUsing) {
		p.nextToken()
		if err := p.expect(TokenLParen); err != nil {
			return nil, err
		}
		for {
			if !p.curIs(TokenIdent) {
				return nil, fmt.Errorf("expected column name in USING at %s", p.current.Pos)
			}
			join.Using = append(join.Using, p.current.Value)
			p.nextToken()
			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	}

	return join, nil
}

// ParseInsert parses an INSERT statement.
func (p *Parser) ParseInsert() (*InsertStmt, error) {
	pos := p.current.Pos

	if err := p.expect(TokenInsert); err != nil {
		return nil, err
	}

	if err := p.expect(TokenInto); err != nil {
		return nil, err
	}

	stmt := &InsertStmt{Pos: pos}

	// Table name
	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected table name at %s", p.current.Pos)
	}
	stmt.Table = p.current.Value
	p.nextToken()

	// Optional column list
	if p.curIs(TokenLParen) {
		p.nextToken()
		for {
			if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
				return nil, fmt.Errorf("expected column name at %s", p.current.Pos)
			}
			stmt.Columns = append(stmt.Columns, p.current.Value)
			p.nextToken()

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	}

	// VALUES or SELECT
	if p.curIs(TokenValues) {
		p.nextToken()
		// Parse value rows
		for {
			if err := p.expect(TokenLParen); err != nil {
				return nil, err
			}

			var row []Expr
			for {
				val, err := p.parseExpr(precLowest)
				if err != nil {
					return nil, err
				}
				row = append(row, val)

				if !p.curIs(TokenComma) {
					break
				}
				p.nextToken()
			}
			stmt.Values = append(stmt.Values, row)

			if err := p.expect(TokenRParen); err != nil {
				return nil, err
			}

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	} else if p.curIs(TokenSelect) || p.curIs(TokenWith) {
		query, err := p.ParseSelect()
		if err != nil {
			return nil, err
		}
		stmt.Query = query
	} else if p.curIs(TokenDefault) {
		// DEFAULT VALUES
		p.nextToken()
		if err := p.expect(TokenValues); err != nil {
			return nil, err
		}
		// No values to add - the InsertStmt will have empty Values
	} else {
		return nil, fmt.Errorf("expected VALUES or SELECT at %s", p.current.Pos)
	}

	// ON CONFLICT clause
	if p.curIs(TokenOn) {
		p.nextToken()
		if err := p.expect(TokenConflict); err != nil {
			return nil, err
		}

		conflict := &OnConflict{Pos: p.current.Pos}

		// Optional conflict target
		if p.curIs(TokenLParen) {
			p.nextToken()
			for {
				if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
					return nil, fmt.Errorf("expected column name in ON CONFLICT at %s", p.current.Pos)
				}
				conflict.Target = append(conflict.Target, p.current.Value)
				p.nextToken()

				if !p.curIs(TokenComma) {
					break
				}
				p.nextToken()
			}
			if err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
		}

		// DO NOTHING or DO UPDATE
		if err := p.expect(TokenDo); err != nil {
			return nil, err
		}

		if p.curIs(TokenNothing) {
			conflict.DoNothing = true
			p.nextToken()
		} else if p.curIs(TokenUpdate) {
			p.nextToken()
			if err := p.expect(TokenSet); err != nil {
				return nil, err
			}

			// Parse SET assignments
			for {
				if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
					return nil, fmt.Errorf("expected column name in SET at %s", p.current.Pos)
				}
				assign := &Assignment{Pos: p.current.Pos, Column: p.current.Value}
				p.nextToken()

				if err := p.expect(TokenEq); err != nil {
					return nil, err
				}

				val, err := p.parseExpr(precLowest)
				if err != nil {
					return nil, err
				}
				assign.Value = val
				conflict.Updates = append(conflict.Updates, assign)

				if !p.curIs(TokenComma) {
					break
				}
				p.nextToken()
			}

			// Optional WHERE clause for DO UPDATE
			if p.curIs(TokenWhere) {
				p.nextToken()
				where, err := p.parseExpr(precLowest)
				if err != nil {
					return nil, err
				}
				conflict.Where = where
			}
		} else {
			return nil, fmt.Errorf("expected NOTHING or UPDATE after DO at %s", p.current.Pos)
		}

		stmt.OnConflict = conflict
	}

	// RETURNING clause
	if p.curIs(TokenReturning) {
		p.nextToken()
		for {
			expr, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			stmt.Returning = append(stmt.Returning, expr)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	return stmt, nil
}

// ParseUpdate parses an UPDATE statement.
func (p *Parser) ParseUpdate() (*UpdateStmt, error) {
	pos := p.current.Pos

	if err := p.expect(TokenUpdate); err != nil {
		return nil, err
	}

	stmt := &UpdateStmt{Pos: pos}

	// Table name
	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected table name at %s", p.current.Pos)
	}
	stmt.Table = p.current.Value
	p.nextToken()

	// Optional alias
	if p.curIs(TokenAs) {
		p.nextToken()
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected alias after AS at %s", p.current.Pos)
		}
		stmt.Alias = p.current.Value
		p.nextToken()
	} else if p.curIs(TokenIdent) && !p.curIs(TokenSet) {
		stmt.Alias = p.current.Value
		p.nextToken()
	}

	// SET clause
	if err := p.expect(TokenSet); err != nil {
		return nil, err
	}

	for {
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected column name in SET at %s", p.current.Pos)
		}
		assign := &Assignment{Pos: p.current.Pos, Column: p.current.Value}
		p.nextToken()

		if err := p.expect(TokenEq); err != nil {
			return nil, err
		}

		val, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		assign.Value = val
		stmt.Set = append(stmt.Set, assign)

		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	// Optional FROM clause (PostgreSQL extension)
	if p.curIs(TokenFrom) {
		p.nextToken()
		stmt.From = &FromClause{Pos: p.current.Pos}

		for {
			tableRef, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From.Tables = append(stmt.From.Tables, tableRef)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	// WHERE clause
	if p.curIs(TokenWhere) {
		p.nextToken()
		where, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// RETURNING clause
	if p.curIs(TokenReturning) {
		p.nextToken()
		for {
			expr, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			stmt.Returning = append(stmt.Returning, expr)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	return stmt, nil
}

// ParseDelete parses a DELETE statement.
func (p *Parser) ParseDelete() (*DeleteStmt, error) {
	pos := p.current.Pos

	if err := p.expect(TokenDelete); err != nil {
		return nil, err
	}

	if err := p.expect(TokenFrom); err != nil {
		return nil, err
	}

	stmt := &DeleteStmt{Pos: pos}

	// Table name
	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected table name at %s", p.current.Pos)
	}
	stmt.Table = p.current.Value
	p.nextToken()

	// Optional alias
	if p.curIs(TokenAs) {
		p.nextToken()
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected alias after AS at %s", p.current.Pos)
		}
		stmt.Alias = p.current.Value
		p.nextToken()
	} else if p.curIs(TokenIdent) && !p.curIs(TokenWhere) && !p.curIs(TokenUsing) && !p.curIs(TokenReturning) {
		stmt.Alias = p.current.Value
		p.nextToken()
	}

	// Optional USING clause (PostgreSQL extension)
	if p.curIs(TokenUsing) {
		p.nextToken()
		stmt.Using = &FromClause{Pos: p.current.Pos}

		for {
			tableRef, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.Using.Tables = append(stmt.Using.Tables, tableRef)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	// WHERE clause
	if p.curIs(TokenWhere) {
		p.nextToken()
		where, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// RETURNING clause
	if p.curIs(TokenReturning) {
		p.nextToken()
		for {
			expr, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			stmt.Returning = append(stmt.Returning, expr)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	return stmt, nil
}

// ParseCreateTable parses a CREATE TABLE statement.
func (p *Parser) ParseCreateTable() (*CreateTableStmt, error) {
	pos := p.current.Pos

	if err := p.expect(TokenCreate); err != nil {
		return nil, err
	}

	if err := p.expect(TokenTable); err != nil {
		return nil, err
	}

	stmt := &CreateTableStmt{Pos: pos}

	// IF NOT EXISTS
	if p.curIs(TokenIf) {
		p.nextToken()
		if !p.curIs(TokenNot) {
			return nil, fmt.Errorf("expected NOT after IF at %s", p.current.Pos)
		}
		p.nextToken()
		if !p.curIs(TokenExists) {
			return nil, fmt.Errorf("expected EXISTS after IF NOT at %s", p.current.Pos)
		}
		p.nextToken()
		stmt.IfNotExists = true
	}

	// Table name
	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected table name at %s", p.current.Pos)
	}
	stmt.Name = p.current.Value
	p.nextToken()

	// Opening paren
	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	// Parse columns and constraints
	for {
		if p.curIs(TokenRParen) {
			break
		}

		// Check for table-level constraint
		if p.curIs(TokenConstraint) || p.curIs(TokenPrimary) || p.curIs(TokenUnique) ||
			p.curIs(TokenForeign) || p.curIs(TokenCheck) {
			constraint, err := p.parseTableConstraint()
			if err != nil {
				return nil, err
			}
			stmt.Constraints = append(stmt.Constraints, constraint)
		} else {
			// Column definition
			col, err := p.parseColumnDef()
			if err != nil {
				return nil, err
			}
			stmt.Columns = append(stmt.Columns, col)
		}

		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	// Closing paren
	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return stmt, nil
}

// parseColumnDef parses a column definition.
func (p *Parser) parseColumnDef() (*ColumnDef, error) {
	pos := p.current.Pos

	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected column name at %s", p.current.Pos)
	}

	col := &ColumnDef{Pos: pos, Name: p.current.Value}
	p.nextToken()

	// Type name
	typeName, typeArgs, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}
	col.TypeName = typeName
	col.TypeArgs = typeArgs

	// Column constraints
	for {
		switch {
		case p.curIs(TokenPrimary):
			p.nextToken()
			if err := p.expect(TokenKey); err != nil {
				return nil, err
			}
			col.PrimaryKey = true
		case p.curIs(TokenNot):
			p.nextToken()
			if err := p.expect(TokenNull); err != nil {
				return nil, err
			}
			col.NotNull = true
		case p.curIs(TokenNull):
			p.nextToken()
			col.NotNull = false
		case p.curIs(TokenUnique):
			p.nextToken()
			col.Unique = true
		case p.curIs(TokenDefault):
			p.nextToken()
			def, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			col.Default = def
		case p.curIs(TokenReferences):
			ref, err := p.parseForeignKeyRef()
			if err != nil {
				return nil, err
			}
			col.References = ref
		case p.curIs(TokenCheck):
			p.nextToken()
			if err := p.expect(TokenLParen); err != nil {
				return nil, err
			}
			check, err := p.parseExpr(precLowest)
			if err != nil {
				return nil, err
			}
			col.Check = check
			if err := p.expect(TokenRParen); err != nil {
				return nil, err
			}
		case p.curIs(TokenConstraint):
			p.nextToken()
			if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
				return nil, fmt.Errorf("expected constraint name at %s", p.current.Pos)
			}
			p.nextToken()
			continue
		default:
			return col, nil
		}
	}
}

// parseForeignKeyRef parses a REFERENCES clause.
func (p *Parser) parseForeignKeyRef() (*ForeignKeyRef, error) {
	pos := p.current.Pos
	p.nextToken() // skip REFERENCES

	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected table name in REFERENCES at %s", p.current.Pos)
	}

	ref := &ForeignKeyRef{Pos: pos, Table: p.current.Value}
	p.nextToken()

	if p.curIs(TokenLParen) {
		p.nextToken()
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected column name in REFERENCES at %s", p.current.Pos)
		}
		ref.Column = p.current.Value
		p.nextToken()
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	}

	for p.curIs(TokenOn) {
		p.nextToken()
		if p.curIs(TokenDelete) {
			p.nextToken()
			action, err := p.parseFKAction()
			if err != nil {
				return nil, err
			}
			ref.OnDelete = action
		} else if p.curIs(TokenUpdate) {
			p.nextToken()
			action, err := p.parseFKAction()
			if err != nil {
				return nil, err
			}
			ref.OnUpdate = action
		} else {
			return nil, fmt.Errorf("expected DELETE or UPDATE after ON at %s", p.current.Pos)
		}
	}

	return ref, nil
}

// parseFKAction parses a foreign key action.
func (p *Parser) parseFKAction() (string, error) {
	switch {
	case p.curIs(TokenIdent) && strings.ToUpper(p.current.Value) == "CASCADE":
		p.nextToken()
		return "CASCADE", nil
	case p.curIs(TokenIdent) && strings.ToUpper(p.current.Value) == "RESTRICT":
		p.nextToken()
		return "RESTRICT", nil
	case p.curIs(TokenIdent) && strings.ToUpper(p.current.Value) == "NO":
		p.nextToken()
		if !p.curIs(TokenIdent) || strings.ToUpper(p.current.Value) != "ACTION" {
			return "", fmt.Errorf("expected ACTION after NO at %s", p.current.Pos)
		}
		p.nextToken()
		return "NO ACTION", nil
	case p.curIs(TokenSet):
		p.nextToken()
		if p.curIs(TokenNull) {
			p.nextToken()
			return "SET NULL", nil
		} else if p.curIs(TokenDefault) {
			p.nextToken()
			return "SET DEFAULT", nil
		}
		return "", fmt.Errorf("expected NULL or DEFAULT after SET at %s", p.current.Pos)
	default:
		return "", fmt.Errorf("expected foreign key action at %s", p.current.Pos)
	}
}

// parseTableConstraint parses a table-level constraint.
func (p *Parser) parseTableConstraint() (*TableConstraint, error) {
	pos := p.current.Pos
	constraint := &TableConstraint{Pos: pos}

	if p.curIs(TokenConstraint) {
		p.nextToken()
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected constraint name at %s", p.current.Pos)
		}
		constraint.Name = p.current.Value
		p.nextToken()
	}

	switch {
	case p.curIs(TokenPrimary):
		p.nextToken()
		if err := p.expect(TokenKey); err != nil {
			return nil, err
		}
		constraint.Type = "PRIMARY KEY"
		cols, err := p.parseColumnList()
		if err != nil {
			return nil, err
		}
		constraint.Columns = cols

	case p.curIs(TokenUnique):
		p.nextToken()
		constraint.Type = "UNIQUE"
		cols, err := p.parseColumnList()
		if err != nil {
			return nil, err
		}
		constraint.Columns = cols

	case p.curIs(TokenForeign):
		p.nextToken()
		if err := p.expect(TokenKey); err != nil {
			return nil, err
		}
		constraint.Type = "FOREIGN KEY"
		cols, err := p.parseColumnList()
		if err != nil {
			return nil, err
		}
		constraint.Columns = cols
		ref, err := p.parseForeignKeyRef()
		if err != nil {
			return nil, err
		}
		constraint.References = ref

	case p.curIs(TokenCheck):
		p.nextToken()
		constraint.Type = "CHECK"
		if err := p.expect(TokenLParen); err != nil {
			return nil, err
		}
		check, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		constraint.Check = check
		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("expected constraint type at %s", p.current.Pos)
	}

	return constraint, nil
}

// parseColumnList parses (col1, col2, ...).
func (p *Parser) parseColumnList() ([]string, error) {
	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	var cols []string
	for {
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected column name at %s", p.current.Pos)
		}
		cols = append(cols, p.current.Value)
		p.nextToken()

		if !p.curIs(TokenComma) {
			break
		}
		p.nextToken()
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	return cols, nil
}

// ParseDrop parses a DROP statement.
func (p *Parser) ParseDrop() (*DropStmt, error) {
	pos := p.current.Pos

	if err := p.expect(TokenDrop); err != nil {
		return nil, err
	}

	stmt := &DropStmt{Pos: pos}

	switch {
	case p.curIs(TokenTable):
		stmt.Type = "TABLE"
		p.nextToken()
	case p.curIs(TokenFunction):
		stmt.Type = "FUNCTION"
		p.nextToken()
	case p.curIs(TokenIndex):
		stmt.Type = "INDEX"
		p.nextToken()
	default:
		return nil, fmt.Errorf("expected TABLE, FUNCTION, or INDEX after DROP at %s", p.current.Pos)
	}

	if p.curIs(TokenIf) {
		p.nextToken()
		if err := p.expect(TokenExists); err != nil {
			return nil, err
		}
		stmt.IfExists = true
	}

	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected name at %s", p.current.Pos)
	}
	stmt.Name = p.current.Value
	p.nextToken()

	return stmt, nil
}

// ParseCreateFunction parses a CREATE [OR REPLACE] FUNCTION statement.
func (p *Parser) ParseCreateFunction() (*CreateFunctionStmt, error) {
	pos := p.current.Pos

	if err := p.expect(TokenCreate); err != nil {
		return nil, err
	}

	stmt := &CreateFunctionStmt{Pos: pos}

	// OR REPLACE
	if p.curIs(TokenOr) {
		p.nextToken()
		if err := p.expect(TokenReplace); err != nil {
			return nil, err
		}
		stmt.OrReplace = true
	}

	if err := p.expect(TokenFunction); err != nil {
		return nil, err
	}

	// Function name
	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected function name at %s", p.current.Pos)
	}
	stmt.Name = p.current.Value
	p.nextToken()

	// Arguments
	if err := p.expect(TokenLParen); err != nil {
		return nil, err
	}

	if !p.curIs(TokenRParen) {
		for {
			arg, err := p.parseFunctionArg()
			if err != nil {
				return nil, err
			}
			stmt.Args = append(stmt.Args, arg)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}
	}

	if err := p.expect(TokenRParen); err != nil {
		return nil, err
	}

	// RETURNS clause
	if err := p.expect(TokenReturns); err != nil {
		return nil, err
	}

	ret, err := p.parseFunctionReturn()
	if err != nil {
		return nil, err
	}
	stmt.Returns = ret

	// Function options (LANGUAGE, IMMUTABLE/STABLE/VOLATILE, SECURITY)
	for {
		switch {
		case p.curIs(TokenLanguage):
			p.nextToken()
			if !p.curIs(TokenIdent) && !p.curIs(TokenSql) {
				return nil, fmt.Errorf("expected language name at %s", p.current.Pos)
			}
			stmt.Language = strings.ToLower(p.current.Value)
			p.nextToken()

		case p.curIs(TokenImmutable):
			stmt.Volatility = "IMMUTABLE"
			p.nextToken()

		case p.curIs(TokenStable):
			stmt.Volatility = "STABLE"
			p.nextToken()

		case p.curIs(TokenVolatile):
			stmt.Volatility = "VOLATILE"
			p.nextToken()

		case p.curIs(TokenSecurity):
			p.nextToken()
			if p.curIs(TokenDefiner) {
				stmt.Security = "DEFINER"
				p.nextToken()
			} else if p.curIs(TokenInvoker) {
				stmt.Security = "INVOKER"
				p.nextToken()
			} else {
				return nil, fmt.Errorf("expected DEFINER or INVOKER after SECURITY at %s", p.current.Pos)
			}

		case p.curIs(TokenAs):
			p.nextToken()
			// Expect dollar-quoted string
			if !p.curIs(TokenDollarStr) {
				return nil, fmt.Errorf("expected dollar-quoted body at %s", p.current.Pos)
			}
			stmt.Body = p.current.Value
			p.nextToken()
			return stmt, nil

		default:
			// No more options, but we need the body
			return nil, fmt.Errorf("expected AS $$ body $$ at %s", p.current.Pos)
		}
	}
}

// parseFunctionArg parses a single function argument.
func (p *Parser) parseFunctionArg() (*FunctionArg, error) {
	pos := p.current.Pos
	arg := &FunctionArg{Pos: pos}

	// Optional mode (IN, OUT, INOUT, VARIADIC)
	if p.curIs(TokenIdent) {
		mode := strings.ToUpper(p.current.Value)
		if mode == "IN" || mode == "OUT" || mode == "INOUT" || mode == "VARIADIC" {
			arg.Mode = mode
			p.nextToken()
		}
	}

	// Argument name
	if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
		return nil, fmt.Errorf("expected argument name at %s", p.current.Pos)
	}
	arg.Name = p.current.Value
	p.nextToken()

	// Type name
	typeName, _, err := p.parseTypeName()
	if err != nil {
		return nil, err
	}
	arg.TypeName = typeName

	// Optional DEFAULT
	if p.curIs(TokenDefault) {
		p.nextToken()
		def, err := p.parseExpr(precLowest)
		if err != nil {
			return nil, err
		}
		arg.Default = def
	}

	return arg, nil
}

// parseFunctionReturn parses a RETURNS clause.
func (p *Parser) parseFunctionReturn() (*FunctionReturn, error) {
	pos := p.current.Pos
	ret := &FunctionReturn{Pos: pos}

	// SETOF
	if p.curIs(TokenSetof) {
		ret.IsSetOf = true
		p.nextToken()
	}

	// TABLE(col1 type1, col2 type2, ...)
	if p.curIs(TokenTable) {
		ret.IsTable = true
		ret.IsSetOf = true // TABLE implies set
		p.nextToken()

		if err := p.expect(TokenLParen); err != nil {
			return nil, err
		}

		for {
			col, err := p.parseColumnDef()
			if err != nil {
				return nil, err
			}
			ret.TableCols = append(ret.TableCols, col)

			if !p.curIs(TokenComma) {
				break
			}
			p.nextToken()
		}

		if err := p.expect(TokenRParen); err != nil {
			return nil, err
		}
	} else {
		// Simple type name
		if !p.curIs(TokenIdent) && !p.curIs(TokenQIdent) {
			return nil, fmt.Errorf("expected return type at %s", p.current.Pos)
		}
		ret.TypeName = p.current.Value
		p.nextToken()
	}

	return ret, nil
}

// IsCreateFunctionSQL checks if SQL is a CREATE FUNCTION statement.
func IsCreateFunctionSQL(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "CREATE FUNCTION") ||
		strings.HasPrefix(normalized, "CREATE OR REPLACE FUNCTION")
}

// IsDropFunctionSQL checks if SQL is a DROP FUNCTION statement.
func IsDropFunctionSQL(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "DROP FUNCTION")
}

// ParseCreateFunctionSQL parses a CREATE FUNCTION statement from raw SQL.
// This is a convenience function that handles the full SQL string.
func ParseCreateFunctionSQL(sql string) (*CreateFunctionStmt, error) {
	parser := NewParser(sql)
	return parser.ParseCreateFunction()
}

// ParseDropFunctionSQL parses a DROP FUNCTION statement from raw SQL.
func ParseDropFunctionSQL(sql string) (*DropStmt, error) {
	parser := NewParser(sql)
	return parser.ParseDrop()
}
