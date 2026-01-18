// internal/rls/enforcer.go
package rls

import (
	"fmt"
	"strings"
)

// Enforcer applies RLS policies to queries
type Enforcer struct {
	policyService *Service
}

// NewEnforcer creates a new RLS enforcer
func NewEnforcer(policyService *Service) *Enforcer {
	return &Enforcer{policyService: policyService}
}

// substituteAllFunctions applies both auth and storage function substitutions
func substituteAllFunctions(expr string, ctx *AuthContext) string {
	expr = SubstituteAuthFunctions(expr, ctx)
	expr = SubstituteStorageFunctions(expr)
	return expr
}

// GetSelectConditions returns WHERE conditions for SELECT queries
func (e *Enforcer) GetSelectConditions(tableName string, ctx *AuthContext) (string, error) {
	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return "", nil
	}

	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", fmt.Errorf("failed to get policies for %s: %w", tableName, err)
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "SELECT" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := substituteAllFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil // No RLS policies, allow all
	}

	// AND all conditions together (all policies must pass)
	return strings.Join(conditions, " AND "), nil
}

// GetInsertConditions returns CHECK conditions for INSERT queries
func (e *Enforcer) GetInsertConditions(tableName string, ctx *AuthContext) (string, error) {
	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return "", nil
	}

	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", fmt.Errorf("failed to get policies for %s: %w", tableName, err)
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "INSERT" || p.Command == "ALL" {
			if p.CheckExpr != "" {
				substituted := substituteAllFunctions(p.CheckExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), nil
}

// GetUpdateConditions returns WHERE conditions for UPDATE queries
func (e *Enforcer) GetUpdateConditions(tableName string, ctx *AuthContext) (string, error) {
	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return "", nil
	}

	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", fmt.Errorf("failed to get policies for %s: %w", tableName, err)
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "UPDATE" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := substituteAllFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), nil
}

// GetDeleteConditions returns WHERE conditions for DELETE queries
func (e *Enforcer) GetDeleteConditions(tableName string, ctx *AuthContext) (string, error) {
	// service_role bypasses RLS
	if ctx != nil && ctx.BypassRLS {
		return "", nil
	}

	policies, err := e.policyService.GetPoliciesForTable(tableName)
	if err != nil {
		return "", fmt.Errorf("failed to get policies for %s: %w", tableName, err)
	}

	var conditions []string
	for _, p := range policies {
		if p.Command == "DELETE" || p.Command == "ALL" {
			if p.UsingExpr != "" {
				substituted := substituteAllFunctions(p.UsingExpr, ctx)
				conditions = append(conditions, "("+substituted+")")
			}
		}
	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), nil
}
