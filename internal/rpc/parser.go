// internal/rpc/parser.go
package rpc

import (
	"fmt"
	"regexp"
	"strings"
)

// IsCreateFunction checks if SQL is a CREATE FUNCTION statement.
func IsCreateFunction(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "CREATE FUNCTION") ||
		strings.HasPrefix(normalized, "CREATE OR REPLACE FUNCTION")
}

// IsDropFunction checks if SQL is a DROP FUNCTION statement.
func IsDropFunction(sql string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(normalized, "DROP FUNCTION")
}

// ParseCreateFunction parses a CREATE FUNCTION statement.
func ParseCreateFunction(sql string) (*ParsedFunction, error) {
	// Normalize whitespace
	sql = strings.TrimSpace(sql)
	sql = strings.TrimSuffix(sql, ";")

	fn := &ParsedFunction{
		Language:   "sql",
		Volatility: "VOLATILE",
		Security:   "INVOKER",
	}

	// Check for OR REPLACE
	upperSQL := strings.ToUpper(sql)
	if strings.HasPrefix(upperSQL, "CREATE OR REPLACE FUNCTION") {
		fn.OrReplace = true
	}

	// Extract function name and arguments
	// Pattern: CREATE [OR REPLACE] FUNCTION name(args) RETURNS ...
	nameArgsPattern := regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?FUNCTION\s+(\w+)\s*\(([^)]*)\)`)
	matches := nameArgsPattern.FindStringSubmatch(sql)
	if matches == nil {
		return nil, fmt.Errorf("invalid CREATE FUNCTION syntax")
	}
	fn.Name = matches[1]
	argsStr := strings.TrimSpace(matches[2])

	// Parse arguments
	if argsStr != "" {
		args, err := parseArguments(argsStr)
		if err != nil {
			return nil, fmt.Errorf("parse arguments: %w", err)
		}
		fn.Args = args
	}

	// Extract RETURNS clause
	returnsPattern := regexp.MustCompile(`(?i)RETURNS\s+(TABLE\s*\([^)]+\)|SETOF\s+\w+|\w+)`)
	returnsMatch := returnsPattern.FindStringSubmatch(sql)
	if returnsMatch == nil {
		return nil, fmt.Errorf("missing RETURNS clause")
	}
	returnType := strings.TrimSpace(returnsMatch[1])

	// Check for SETOF or TABLE
	upperReturn := strings.ToUpper(returnType)
	if strings.HasPrefix(upperReturn, "SETOF ") {
		fn.ReturnsSet = true
		fn.ReturnType = strings.TrimSpace(returnType[6:]) // Remove "SETOF "
	} else if strings.HasPrefix(upperReturn, "TABLE") {
		fn.ReturnsSet = true
		fn.ReturnType = returnType
	} else {
		fn.ReturnType = returnType
	}

	// Extract LANGUAGE
	langPattern := regexp.MustCompile(`(?i)LANGUAGE\s+(\w+)`)
	langMatch := langPattern.FindStringSubmatch(sql)
	if langMatch != nil {
		fn.Language = strings.ToLower(langMatch[1])
	}

	// Reject non-SQL languages
	if fn.Language != "sql" {
		return nil, fmt.Errorf("only LANGUAGE sql is supported, got %q", fn.Language)
	}

	// Extract volatility
	if strings.Contains(upperSQL, "IMMUTABLE") {
		fn.Volatility = "IMMUTABLE"
	} else if strings.Contains(upperSQL, "STABLE") {
		fn.Volatility = "STABLE"
	}

	// Extract security
	if strings.Contains(upperSQL, "SECURITY DEFINER") {
		fn.Security = "DEFINER"
	}

	// Extract body from dollar-quoted string
	body, err := extractDollarQuotedBody(sql)
	if err != nil {
		return nil, fmt.Errorf("extract body: %w", err)
	}
	fn.Body = strings.TrimSpace(body)

	return fn, nil
}

// parseArguments parses function arguments from "arg1 type1, arg2 type2 DEFAULT val" format.
func parseArguments(argsStr string) ([]FunctionArg, error) {
	if strings.TrimSpace(argsStr) == "" {
		return nil, nil
	}

	var args []FunctionArg
	parts := splitArgs(argsStr)

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		arg := FunctionArg{Position: i}

		// Check for DEFAULT
		upperPart := strings.ToUpper(part)
		defaultIdx := strings.Index(upperPart, " DEFAULT ")
		if defaultIdx != -1 {
			defaultVal := strings.TrimSpace(part[defaultIdx+9:])
			arg.DefaultValue = &defaultVal
			part = strings.TrimSpace(part[:defaultIdx])
		}

		// Split into name and type
		tokens := strings.Fields(part)
		if len(tokens) < 2 {
			return nil, fmt.Errorf("invalid argument: %q", part)
		}

		arg.Name = tokens[0]
		arg.Type = strings.Join(tokens[1:], " ")

		args = append(args, arg)
	}

	return args, nil
}

// splitArgs splits arguments respecting parentheses (for types like TABLE(...)).
func splitArgs(s string) []string {
	var result []string
	var current strings.Builder
	depth := 0

	for _, ch := range s {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				result = append(result, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// extractDollarQuotedBody extracts the body from $$ ... $$ or $tag$ ... $tag$.
func extractDollarQuotedBody(sql string) (string, error) {
	// Find opening dollar quote
	dollarPattern := regexp.MustCompile(`\$(\w*)\$`)
	matches := dollarPattern.FindAllStringIndex(sql, -1)
	if len(matches) < 2 {
		return "", fmt.Errorf("missing dollar-quoted body")
	}

	// Get the tag
	openMatch := dollarPattern.FindStringSubmatch(sql[matches[0][0]:matches[0][1]])
	tag := openMatch[1]
	openDelim := "$" + tag + "$"

	// Find matching close
	openIdx := matches[0][1]
	closePattern := regexp.MustCompile(regexp.QuoteMeta(openDelim))
	closeMatches := closePattern.FindStringIndex(sql[openIdx:])
	if closeMatches == nil {
		return "", fmt.Errorf("unclosed dollar quote")
	}

	body := sql[openIdx : openIdx+closeMatches[0]]
	return body, nil
}

// ParseDropFunction parses DROP FUNCTION [IF EXISTS] name[(args)].
func ParseDropFunction(sql string) (name string, ifExists bool, err error) {
	sql = strings.TrimSpace(sql)
	sql = strings.TrimSuffix(sql, ";")

	upperSQL := strings.ToUpper(sql)
	if strings.Contains(upperSQL, "IF EXISTS") {
		ifExists = true
	}

	// Extract function name
	pattern := regexp.MustCompile(`(?i)DROP\s+FUNCTION\s+(?:IF\s+EXISTS\s+)?(\w+)`)
	matches := pattern.FindStringSubmatch(sql)
	if matches == nil {
		return "", false, fmt.Errorf("invalid DROP FUNCTION syntax")
	}

	return matches[1], ifExists, nil
}
