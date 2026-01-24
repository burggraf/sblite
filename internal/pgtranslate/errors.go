package pgtranslate

import (
	"fmt"
	"strings"
)

// ParseError represents a parsing error with position information.
type ParseError struct {
	Message  string
	Pos      Position
	Source   string // the source SQL being parsed
	Expected string // what was expected
	Got      string // what was found
}

// Error implements the error interface.
func (e *ParseError) Error() string {
	if e.Pos.Line > 0 {
		return fmt.Sprintf("parse error at line %d, column %d: %s", e.Pos.Line, e.Pos.Column, e.Message)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

// Verbose returns a detailed error message with source context.
func (e *ParseError) Verbose() string {
	var sb strings.Builder

	sb.WriteString(e.Error())
	sb.WriteString("\n")

	if e.Source != "" && e.Pos.Line > 0 {
		lines := strings.Split(e.Source, "\n")
		lineIdx := e.Pos.Line - 1
		if lineIdx >= 0 && lineIdx < len(lines) {
			line := lines[lineIdx]
			sb.WriteString("  ")
			sb.WriteString(line)
			sb.WriteString("\n")

			// Add pointer to the error position
			sb.WriteString("  ")
			for i := 1; i < e.Pos.Column; i++ {
				if i-1 < len(line) && line[i-1] == '\t' {
					sb.WriteString("\t")
				} else {
					sb.WriteString(" ")
				}
			}
			sb.WriteString("^")
			sb.WriteString("\n")
		}
	}

	if e.Expected != "" && e.Got != "" {
		sb.WriteString(fmt.Sprintf("  expected: %s\n", e.Expected))
		sb.WriteString(fmt.Sprintf("  got: %s\n", e.Got))
	}

	return sb.String()
}

// NewParseError creates a new parse error.
func NewParseError(message string, pos Position) *ParseError {
	return &ParseError{
		Message: message,
		Pos:     pos,
	}
}

// NewParseErrorWithSource creates a new parse error with source context.
func NewParseErrorWithSource(message string, pos Position, source string) *ParseError {
	return &ParseError{
		Message: message,
		Pos:     pos,
		Source:  source,
	}
}

// NewExpectedError creates a parse error for unexpected token.
func NewExpectedError(expected, got string, pos Position) *ParseError {
	return &ParseError{
		Message:  fmt.Sprintf("expected %s, got %s", expected, got),
		Pos:      pos,
		Expected: expected,
		Got:      got,
	}
}

// TranslationError represents an error during SQL translation.
type TranslationError struct {
	Message string
	Pos     Position
	Node    Node
	Cause   error
}

// Error implements the error interface.
func (e *TranslationError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("translation error at %s: %s: %v", e.Pos, e.Message, e.Cause)
	}
	if e.Pos.Line > 0 {
		return fmt.Sprintf("translation error at %s: %s", e.Pos, e.Message)
	}
	return fmt.Sprintf("translation error: %s", e.Message)
}

// Unwrap returns the underlying cause.
func (e *TranslationError) Unwrap() error {
	return e.Cause
}

// NewTranslationError creates a new translation error.
func NewTranslationError(message string, node Node) *TranslationError {
	var pos Position
	if node != nil {
		pos = node.Position()
	}
	return &TranslationError{
		Message: message,
		Pos:     pos,
		Node:    node,
	}
}

// WrapTranslationError wraps an error as a translation error.
func WrapTranslationError(cause error, message string, node Node) *TranslationError {
	var pos Position
	if node != nil {
		pos = node.Position()
	}
	return &TranslationError{
		Message: message,
		Pos:     pos,
		Node:    node,
		Cause:   cause,
	}
}

// UnsupportedFeatureError indicates a SQL feature not supported by the translator.
type UnsupportedFeatureError struct {
	Feature string
	Dialect Dialect
	Pos     Position
}

// Error implements the error interface.
func (e *UnsupportedFeatureError) Error() string {
	dialect := "target dialect"
	if e.Dialect == DialectSQLite {
		dialect = "SQLite"
	} else if e.Dialect == DialectPostgreSQL {
		dialect = "PostgreSQL"
	}
	return fmt.Sprintf("unsupported feature for %s: %s", dialect, e.Feature)
}

// NewUnsupportedFeatureError creates a new unsupported feature error.
func NewUnsupportedFeatureError(feature string, dialect Dialect, pos Position) *UnsupportedFeatureError {
	return &UnsupportedFeatureError{
		Feature: feature,
		Dialect: dialect,
		Pos:     pos,
	}
}

// Errors is a collection of errors.
type Errors []error

// Error implements the error interface.
func (e Errors) Error() string {
	if len(e) == 0 {
		return "no errors"
	}
	if len(e) == 1 {
		return e[0].Error()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d errors:\n", len(e)))
	for i, err := range e {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// Add adds an error to the collection.
func (e *Errors) Add(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

// HasErrors returns true if there are any errors.
func (e Errors) HasErrors() bool {
	return len(e) > 0
}

// ToError returns nil if there are no errors, the single error if there is one,
// or the Errors collection if there are multiple.
func (e Errors) ToError() error {
	if len(e) == 0 {
		return nil
	}
	if len(e) == 1 {
		return e[0]
	}
	return e
}
