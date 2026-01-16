// internal/rls/rewriter.go
package rls

import (
	"fmt"
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
