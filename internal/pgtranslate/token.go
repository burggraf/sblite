// Package pgtranslate provides PostgreSQL to SQLite SQL translation.
package pgtranslate

import (
	"fmt"
	"strings"
	"unicode"
)

// TokenType represents the type of a token.
type TokenType int

const (
	// Special tokens
	TokenEOF TokenType = iota
	TokenError
	TokenWhitespace

	// Literals
	TokenIdent     // identifier (unquoted)
	TokenQIdent    // quoted identifier ("name")
	TokenString    // string literal ('value')
	TokenNumber    // numeric literal (123, 45.67)
	TokenDollarStr // dollar-quoted string ($$body$$, $tag$body$tag$)

	// Operators
	TokenPlus       // +
	TokenMinus      // -
	TokenStar       // *
	TokenSlash      // /
	TokenPercent    // %
	TokenCaret      // ^
	TokenEq         // =
	TokenNe         // <> or !=
	TokenLt         // <
	TokenLe         // <=
	TokenGt         // >
	TokenGe         // >=
	TokenConcat     // ||
	TokenCast       // ::
	TokenJsonArrow      // ->
	TokenJsonArrow2     // ->>
	TokenRegexMatch     // ~
	TokenRegexMatchCI   // ~*
	TokenRegexNotMatch  // !~
	TokenRegexNotMatchCI // !~*

	// Punctuation
	TokenLParen    // (
	TokenRParen    // )
	TokenLBracket  // [
	TokenRBracket  // ]
	TokenComma     // ,
	TokenSemicolon // ;
	TokenDot       // .
	TokenColon     // :

	// Keywords (subset - expanded as needed)
	TokenSelect
	TokenFrom
	TokenWhere
	TokenAnd
	TokenOr
	TokenNot
	TokenAs
	TokenNull
	TokenTrue
	TokenFalse
	TokenIs
	TokenIn
	TokenLike
	TokenILike
	TokenBetween
	TokenCase
	TokenWhen
	TokenThen
	TokenElse
	TokenEnd
	TokenOrder
	TokenBy
	TokenAsc
	TokenDesc
	TokenLimit
	TokenOffset
	TokenGroup
	TokenHaving
	TokenJoin
	TokenLeft
	TokenRight
	TokenInner
	TokenOuter
	TokenFull
	TokenCross
	TokenOn
	TokenUsing
	TokenDistinct
	TokenAll
	TokenUnion
	TokenIntersect
	TokenExcept
	TokenInsert
	TokenInto
	TokenValues
	TokenUpdate
	TokenSet
	TokenDelete
	TokenCreate
	TokenTable
	TokenFunction
	TokenReplace
	TokenDrop
	TokenAlter
	TokenIndex
	TokenPrimary
	TokenKey
	TokenForeign
	TokenReferences
	TokenUnique
	TokenCheck
	TokenDefault
	TokenConstraint
	TokenReturning
	TokenConflict
	TokenDo
	TokenNothing
	TokenWith
	TokenRecursive
	TokenCast_kw // CAST keyword
	TokenExtract
	TokenInterval
	TokenExists
	TokenAny
	TokenSome
	TokenCoalesce
	TokenNullif
	TokenGreatest
	TokenLeast
	TokenCount
	TokenSum
	TokenAvg
	TokenMin
	TokenMax
	TokenNow
	TokenLanguage
	TokenSql
	TokenReturns
	TokenSetof
	TokenImmutable
	TokenStable
	TokenVolatile
	TokenSecurity
	TokenDefiner
	TokenInvoker
	TokenIf
	TokenArray
	TokenOver
	TokenPartition
	TokenRows
	TokenRange
	TokenGroups
	TokenUnbounded
	TokenPreceding
	TokenFollowing
	TokenCurrent
)

// Token represents a lexical token with its position.
type Token struct {
	Type    TokenType
	Value   string
	Pos     Position
	Literal string // original literal for strings/numbers
}

// Position represents a position in the source.
type Position struct {
	Offset int // byte offset
	Line   int // 1-based line number
	Column int // 1-based column number
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// keywords maps keyword strings to token types (case-insensitive).
var keywords = map[string]TokenType{
	"SELECT":    TokenSelect,
	"FROM":      TokenFrom,
	"WHERE":     TokenWhere,
	"AND":       TokenAnd,
	"OR":        TokenOr,
	"NOT":       TokenNot,
	"AS":        TokenAs,
	"NULL":      TokenNull,
	"TRUE":      TokenTrue,
	"FALSE":     TokenFalse,
	"IS":        TokenIs,
	"IN":        TokenIn,
	"LIKE":      TokenLike,
	"ILIKE":     TokenILike,
	"BETWEEN":   TokenBetween,
	"CASE":      TokenCase,
	"WHEN":      TokenWhen,
	"THEN":      TokenThen,
	"ELSE":      TokenElse,
	"END":       TokenEnd,
	"ORDER":     TokenOrder,
	"BY":        TokenBy,
	"ASC":       TokenAsc,
	"DESC":      TokenDesc,
	"LIMIT":     TokenLimit,
	"OFFSET":    TokenOffset,
	"GROUP":     TokenGroup,
	"HAVING":    TokenHaving,
	"JOIN":      TokenJoin,
	"LEFT":      TokenLeft,
	"RIGHT":     TokenRight,
	"INNER":     TokenInner,
	"OUTER":     TokenOuter,
	"FULL":      TokenFull,
	"CROSS":     TokenCross,
	"ON":        TokenOn,
	"USING":     TokenUsing,
	"DISTINCT":  TokenDistinct,
	"ALL":       TokenAll,
	"UNION":     TokenUnion,
	"INTERSECT": TokenIntersect,
	"EXCEPT":    TokenExcept,
	"INSERT":    TokenInsert,
	"INTO":      TokenInto,
	"VALUES":    TokenValues,
	"UPDATE":    TokenUpdate,
	"SET":       TokenSet,
	"DELETE":    TokenDelete,
	"CREATE":    TokenCreate,
	"TABLE":     TokenTable,
	"FUNCTION":  TokenFunction,
	"REPLACE":   TokenReplace,
	"DROP":      TokenDrop,
	"ALTER":     TokenAlter,
	"INDEX":     TokenIndex,
	"PRIMARY":   TokenPrimary,
	"KEY":       TokenKey,
	"FOREIGN":   TokenForeign,
	"REFERENCES": TokenReferences,
	"UNIQUE":    TokenUnique,
	"CHECK":     TokenCheck,
	"DEFAULT":   TokenDefault,
	"CONSTRAINT": TokenConstraint,
	"RETURNING": TokenReturning,
	"CONFLICT":  TokenConflict,
	"DO":        TokenDo,
	"NOTHING":   TokenNothing,
	"WITH":      TokenWith,
	"RECURSIVE": TokenRecursive,
	"CAST":      TokenCast_kw,
	"EXTRACT":   TokenExtract,
	"INTERVAL":  TokenInterval,
	"EXISTS":    TokenExists,
	"ANY":       TokenAny,
	"SOME":      TokenSome,
	"COALESCE":  TokenCoalesce,
	"NULLIF":    TokenNullif,
	"GREATEST":  TokenGreatest,
	"LEAST":     TokenLeast,
	"COUNT":     TokenCount,
	"SUM":       TokenSum,
	"AVG":       TokenAvg,
	"MIN":       TokenMin,
	"MAX":       TokenMax,
	"NOW":       TokenNow,
	"LANGUAGE":  TokenLanguage,
	"SQL":       TokenSql,
	"RETURNS":   TokenReturns,
	"SETOF":     TokenSetof,
	"IMMUTABLE": TokenImmutable,
	"STABLE":    TokenStable,
	"VOLATILE":  TokenVolatile,
	"SECURITY":  TokenSecurity,
	"DEFINER":   TokenDefiner,
	"INVOKER":   TokenInvoker,
	"IF":        TokenIf,
	"ARRAY":     TokenArray,
	"OVER":      TokenOver,
	"PARTITION": TokenPartition,
	"ROWS":      TokenRows,
	"RANGE":     TokenRange,
	"GROUPS":    TokenGroups,
	"UNBOUNDED": TokenUnbounded,
	"PRECEDING": TokenPreceding,
	"FOLLOWING": TokenFollowing,
	"CURRENT":   TokenCurrent,
}

// Lexer tokenizes SQL input.
type Lexer struct {
	input   string
	pos     int // current position in input
	readPos int // reading position (after current char)
	ch      byte
	line    int
	col     int
}

// NewLexer creates a new lexer for the given input.
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input: input,
		line:  1,
		col:   0,
	}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	if l.ch == '\n' {
		l.line++
		l.col = 0
	} else {
		l.col++
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) peekChars(n int) string {
	end := l.readPos + n - 1
	if end > len(l.input) {
		end = len(l.input)
	}
	return l.input[l.pos:end]
}

func (l *Lexer) position() Position {
	return Position{Offset: l.pos, Line: l.line, Column: l.col}
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	pos := l.position()

	switch l.ch {
	case 0:
		return Token{Type: TokenEOF, Pos: pos}
	case '+':
		l.readChar()
		return Token{Type: TokenPlus, Value: "+", Pos: pos}
	case '*':
		l.readChar()
		return Token{Type: TokenStar, Value: "*", Pos: pos}
	case '/':
		l.readChar()
		return Token{Type: TokenSlash, Value: "/", Pos: pos}
	case '%':
		l.readChar()
		return Token{Type: TokenPercent, Value: "%", Pos: pos}
	case '^':
		l.readChar()
		return Token{Type: TokenCaret, Value: "^", Pos: pos}
	case '(':
		l.readChar()
		return Token{Type: TokenLParen, Value: "(", Pos: pos}
	case ')':
		l.readChar()
		return Token{Type: TokenRParen, Value: ")", Pos: pos}
	case '[':
		l.readChar()
		return Token{Type: TokenLBracket, Value: "[", Pos: pos}
	case ']':
		l.readChar()
		return Token{Type: TokenRBracket, Value: "]", Pos: pos}
	case ',':
		l.readChar()
		return Token{Type: TokenComma, Value: ",", Pos: pos}
	case ';':
		l.readChar()
		return Token{Type: TokenSemicolon, Value: ";", Pos: pos}
	case '.':
		l.readChar()
		return Token{Type: TokenDot, Value: ".", Pos: pos}
	case '=':
		l.readChar()
		return Token{Type: TokenEq, Value: "=", Pos: pos}
	case '-':
		if l.peekChar() == '>' {
			l.readChar()
			l.readChar()
			if l.ch == '>' {
				l.readChar()
				return Token{Type: TokenJsonArrow2, Value: "->>", Pos: pos}
			}
			return Token{Type: TokenJsonArrow, Value: "->", Pos: pos}
		}
		if l.peekChar() == '-' {
			// SQL comment -- skip to end of line
			l.skipLineComment()
			return l.NextToken()
		}
		l.readChar()
		return Token{Type: TokenMinus, Value: "-", Pos: pos}
	case '<':
		if l.peekChar() == '>' {
			l.readChar()
			l.readChar()
			return Token{Type: TokenNe, Value: "<>", Pos: pos}
		}
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			return Token{Type: TokenLe, Value: "<=", Pos: pos}
		}
		l.readChar()
		return Token{Type: TokenLt, Value: "<", Pos: pos}
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			return Token{Type: TokenGe, Value: ">=", Pos: pos}
		}
		l.readChar()
		return Token{Type: TokenGt, Value: ">", Pos: pos}
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			l.readChar()
			return Token{Type: TokenNe, Value: "!=", Pos: pos}
		}
		if l.peekChar() == '~' {
			l.readChar()
			l.readChar()
			if l.ch == '*' {
				l.readChar()
				return Token{Type: TokenRegexNotMatchCI, Value: "!~*", Pos: pos}
			}
			return Token{Type: TokenRegexNotMatch, Value: "!~", Pos: pos}
		}
		l.readChar()
		return Token{Type: TokenError, Value: "!", Pos: pos}
	case '~':
		l.readChar()
		if l.ch == '*' {
			l.readChar()
			return Token{Type: TokenRegexMatchCI, Value: "~*", Pos: pos}
		}
		return Token{Type: TokenRegexMatch, Value: "~", Pos: pos}
	case '|':
		if l.peekChar() == '|' {
			l.readChar()
			l.readChar()
			return Token{Type: TokenConcat, Value: "||", Pos: pos}
		}
		l.readChar()
		return Token{Type: TokenError, Value: "|", Pos: pos}
	case ':':
		if l.peekChar() == ':' {
			l.readChar()
			l.readChar()
			return Token{Type: TokenCast, Value: "::", Pos: pos}
		}
		l.readChar()
		return Token{Type: TokenColon, Value: ":", Pos: pos}
	case '\'':
		return l.readString()
	case '"':
		return l.readQuotedIdentifier()
	case '$':
		return l.readDollarQuoted()
	default:
		if isDigit(l.ch) {
			return l.readNumber()
		}
		if isIdentStart(l.ch) {
			return l.readIdentifier()
		}
		ch := l.ch
		l.readChar()
		return Token{Type: TokenError, Value: string(ch), Pos: pos}
	}
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
	// Skip block comments /* */
	if l.ch == '/' && l.peekChar() == '*' {
		l.skipBlockComment()
		l.skipWhitespace()
	}
}

func (l *Lexer) skipLineComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
	if l.ch == '\n' {
		l.readChar()
	}
}

func (l *Lexer) skipBlockComment() {
	l.readChar() // skip /
	l.readChar() // skip *
	for {
		if l.ch == 0 {
			return
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // skip *
			l.readChar() // skip /
			return
		}
		l.readChar()
	}
}

func (l *Lexer) readString() Token {
	pos := l.position()
	l.readChar() // skip opening '

	var sb strings.Builder
	for {
		if l.ch == 0 {
			return Token{Type: TokenError, Value: "unterminated string", Pos: pos}
		}
		if l.ch == '\'' {
			if l.peekChar() == '\'' {
				// Escaped single quote ''
				sb.WriteByte('\'')
				l.readChar()
				l.readChar()
				continue
			}
			l.readChar() // skip closing '
			break
		}
		sb.WriteByte(l.ch)
		l.readChar()
	}

	return Token{Type: TokenString, Value: sb.String(), Literal: "'" + sb.String() + "'", Pos: pos}
}

func (l *Lexer) readQuotedIdentifier() Token {
	pos := l.position()
	l.readChar() // skip opening "

	var sb strings.Builder
	for {
		if l.ch == 0 {
			return Token{Type: TokenError, Value: "unterminated quoted identifier", Pos: pos}
		}
		if l.ch == '"' {
			if l.peekChar() == '"' {
				// Escaped double quote ""
				sb.WriteByte('"')
				l.readChar()
				l.readChar()
				continue
			}
			l.readChar() // skip closing "
			break
		}
		sb.WriteByte(l.ch)
		l.readChar()
	}

	return Token{Type: TokenQIdent, Value: sb.String(), Literal: `"` + sb.String() + `"`, Pos: pos}
}

func (l *Lexer) readDollarQuoted() Token {
	pos := l.position()

	// Read the opening tag $tag$
	var tagBuilder strings.Builder
	tagBuilder.WriteByte('$')
	l.readChar() // skip $

	for l.ch != '$' && l.ch != 0 && isIdentChar(l.ch) {
		tagBuilder.WriteByte(l.ch)
		l.readChar()
	}

	if l.ch != '$' {
		return Token{Type: TokenError, Value: "invalid dollar quote", Pos: pos}
	}
	tagBuilder.WriteByte('$')
	l.readChar() // skip $

	tag := tagBuilder.String()

	// Read until we find the closing tag
	var bodyBuilder strings.Builder
	for {
		if l.ch == 0 {
			return Token{Type: TokenError, Value: "unterminated dollar-quoted string", Pos: pos}
		}
		if l.ch == '$' {
			// Check if this is the closing tag
			remaining := l.input[l.pos:]
			if strings.HasPrefix(remaining, tag) {
				// Skip the closing tag
				for i := 0; i < len(tag); i++ {
					l.readChar()
				}
				break
			}
		}
		bodyBuilder.WriteByte(l.ch)
		l.readChar()
	}

	return Token{Type: TokenDollarStr, Value: bodyBuilder.String(), Literal: tag + bodyBuilder.String() + tag, Pos: pos}
}

func (l *Lexer) readNumber() Token {
	pos := l.position()
	var sb strings.Builder

	// Read integer part
	for isDigit(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}

	// Check for decimal part
	if l.ch == '.' && isDigit(l.peekChar()) {
		sb.WriteByte(l.ch)
		l.readChar()
		for isDigit(l.ch) {
			sb.WriteByte(l.ch)
			l.readChar()
		}
	}

	// Check for exponent
	if l.ch == 'e' || l.ch == 'E' {
		sb.WriteByte(l.ch)
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			sb.WriteByte(l.ch)
			l.readChar()
		}
		for isDigit(l.ch) {
			sb.WriteByte(l.ch)
			l.readChar()
		}
	}

	return Token{Type: TokenNumber, Value: sb.String(), Literal: sb.String(), Pos: pos}
}

func (l *Lexer) readIdentifier() Token {
	pos := l.position()
	var sb strings.Builder

	for isIdentChar(l.ch) {
		sb.WriteByte(l.ch)
		l.readChar()
	}

	value := sb.String()
	upper := strings.ToUpper(value)

	// Check if it's a keyword
	if tok, ok := keywords[upper]; ok {
		return Token{Type: tok, Value: value, Pos: pos}
	}

	return Token{Type: TokenIdent, Value: value, Pos: pos}
}

// Tokenize returns all tokens from the input.
func (l *Lexer) Tokenize() []Token {
	var tokens []Token
	for {
		tok := l.NextToken()
		tokens = append(tokens, tok)
		if tok.Type == TokenEOF || tok.Type == TokenError {
			break
		}
	}
	return tokens
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isIdentStart(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isIdentChar(ch byte) bool {
	return isIdentStart(ch) || isDigit(ch)
}

// IsKeyword checks if a string is a SQL keyword.
func IsKeyword(s string) bool {
	_, ok := keywords[strings.ToUpper(s)]
	return ok
}

// TokenTypeName returns a human-readable name for a token type.
func TokenTypeName(t TokenType) string {
	switch t {
	case TokenEOF:
		return "EOF"
	case TokenError:
		return "ERROR"
	case TokenIdent:
		return "IDENT"
	case TokenQIdent:
		return "QUOTED_IDENT"
	case TokenString:
		return "STRING"
	case TokenNumber:
		return "NUMBER"
	case TokenDollarStr:
		return "DOLLAR_STRING"
	case TokenCast:
		return "::"
	case TokenJsonArrow:
		return "->"
	case TokenJsonArrow2:
		return "->>"
	default:
		// For keywords, return the keyword name
		for kw, tok := range keywords {
			if tok == t {
				return kw
			}
		}
		return fmt.Sprintf("TOKEN(%d)", t)
	}
}

// isAlpha checks if a rune is an ASCII letter.
func isAlpha(r rune) bool {
	return unicode.IsLetter(r)
}
