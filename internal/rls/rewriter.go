// internal/rls/rewriter.go
package rls

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// AuthContext holds JWT claims for auth function substitution
type AuthContext struct {
	UserID    string
	Email     string
	Role      string
	Claims    map[string]any
	BypassRLS bool // true for service_role API key
}

// SubstituteAuthFunctions replaces auth.uid(), auth.role(), etc. with actual values
func SubstituteAuthFunctions(expr string, ctx *AuthContext) string {
	if ctx == nil {
		return expr
	}

	// Replace auth.uid()
	expr = strings.ReplaceAll(expr, "auth.uid()", "'"+escapeSQLString(ctx.UserID)+"'")

	// Replace auth.role()
	expr = strings.ReplaceAll(expr, "auth.role()", "'"+escapeSQLString(ctx.Role)+"'")

	// Replace auth.email()
	expr = strings.ReplaceAll(expr, "auth.email()", "'"+escapeSQLString(ctx.Email)+"'")

	// Replace auth.jwt()->>'key' patterns
	jwtPattern := regexp.MustCompile(`auth\.jwt\(\)->>'\s*(\w+)\s*'`)
	expr = jwtPattern.ReplaceAllStringFunc(expr, func(match string) string {
		submatches := jwtPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		key := submatches[1]
		if val, ok := ctx.Claims[key]; ok {
			switch v := val.(type) {
			case string:
				return "'" + escapeSQLString(v) + "'"
			default:
				return "'" + escapeSQLString(toString(v)) + "'"
			}
		}
		return "NULL"
	})

	return expr
}

func escapeSQLString(s string) string {
	return strings.ReplaceAll(s, "'", "''")
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		return fmt.Sprintf("%v", val)
	}
}

// SubstituteStorageFunctions replaces storage.filename(col), storage.foldername(col), storage.extension(col)
// with SQL expressions that extract the relevant parts from the object name column.
// These functions are used in RLS policies on the storage_objects table.
func SubstituteStorageFunctions(expr string) string {
	// Replace storage.filename(name) - extracts filename from path
	// 'folder/subfolder/file.txt' -> 'file.txt'
	filenamePattern := regexp.MustCompile(`storage\.filename\(\s*(\w+)\s*\)`)
	expr = filenamePattern.ReplaceAllString(expr, "substr($1, length($1) - length(replace($1, '/', '')) + 1)")

	// Replace storage.foldername(name) - extracts folder path from path
	// 'folder/subfolder/file.txt' -> 'folder/subfolder'
	foldernamePattern := regexp.MustCompile(`storage\.foldername\(\s*(\w+)\s*\)`)
	expr = foldernamePattern.ReplaceAllStringFunc(expr, func(match string) string {
		submatches := foldernamePattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		col := submatches[1]
		// CASE for handling paths with/without folders
		return fmt.Sprintf("CASE WHEN instr(%s, '/') > 0 THEN substr(%s, 1, length(%s) - length(substr(%s, length(%s) - length(replace(%s, '/', '')) + 1)) - 1) ELSE '' END", col, col, col, col, col, col)
	})

	// Replace storage.extension(name) - extracts file extension
	// 'file.txt' -> 'txt', 'file' -> ''
	extensionPattern := regexp.MustCompile(`storage\.extension\(\s*(\w+)\s*\)`)
	expr = extensionPattern.ReplaceAllStringFunc(expr, func(match string) string {
		submatches := extensionPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		col := submatches[1]
		// Extract filename first, then get extension
		filename := fmt.Sprintf("substr(%s, length(%s) - length(replace(%s, '/', '')) + 1)", col, col, col)
		// CASE for handling files with/without extensions
		return fmt.Sprintf("CASE WHEN instr(%s, '.') > 0 THEN lower(substr(%s, length(%s) - length(replace(%s, '.', '')) + 1)) ELSE '' END", filename, filename, filename, filename)
	})

	return expr
}

// StorageFilename extracts the filename from a path (Go implementation for testing)
func StorageFilename(path string) string {
	return filepath.Base(path)
}

// StorageFoldername extracts the folder path from a path (Go implementation for testing)
func StorageFoldername(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return ""
	}
	return dir
}

// StorageExtension extracts the lowercase file extension from a path (Go implementation for testing)
func StorageExtension(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return ""
	}
	return strings.ToLower(ext[1:]) // Remove leading dot
}
