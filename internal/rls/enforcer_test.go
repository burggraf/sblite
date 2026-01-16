// internal/rls/enforcer_test.go
package rls

import (
	"strings"
	"testing"
)

func TestEnforcerSelectConditions(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create a SELECT policy
	_, err := policyService.CreatePolicy("todos", "user_isolation", "SELECT", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{
		UserID: "user-123",
		Role:   "authenticated",
	}

	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	expected := "(user_id = 'user-123')"
	if conditions != expected {
		t.Errorf("expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerNoPolicy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	ctx := &AuthContext{UserID: "user-123"}

	conditions, err := enforcer.GetSelectConditions("no_policy_table", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	if conditions != "" {
		t.Errorf("expected empty conditions for table without policy, got %q", conditions)
	}
}

func TestEnforcerMultiplePolicies(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create multiple policies - both must pass
	_, err := policyService.CreatePolicy("todos", "user_isolation", "SELECT", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy1: %v", err)
	}
	_, err = policyService.CreatePolicy("todos", "not_deleted", "SELECT", "deleted = 0", "")
	if err != nil {
		t.Fatalf("failed to create policy2: %v", err)
	}

	ctx := &AuthContext{UserID: "user-123"}

	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	// Both conditions should be ANDed
	if !strings.Contains(conditions, "user_id = 'user-123'") {
		t.Errorf("expected user condition in %q", conditions)
	}
	if !strings.Contains(conditions, "deleted = 0") {
		t.Errorf("expected deleted condition in %q", conditions)
	}
	if !strings.Contains(conditions, " AND ") {
		t.Errorf("expected AND between conditions in %q", conditions)
	}
}

func TestEnforcerInsertConditions(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create an INSERT policy with check_expr
	_, err := policyService.CreatePolicy("todos", "user_insert", "INSERT", "", "user_id = auth.uid()")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{
		UserID: "user-456",
		Role:   "authenticated",
	}

	conditions, err := enforcer.GetInsertConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	expected := "(user_id = 'user-456')"
	if conditions != expected {
		t.Errorf("expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerUpdateConditions(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create an UPDATE policy
	_, err := policyService.CreatePolicy("todos", "user_update", "UPDATE", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{
		UserID: "user-789",
		Role:   "authenticated",
	}

	conditions, err := enforcer.GetUpdateConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	expected := "(user_id = 'user-789')"
	if conditions != expected {
		t.Errorf("expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerDeleteConditions(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create a DELETE policy
	_, err := policyService.CreatePolicy("todos", "user_delete", "DELETE", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{
		UserID: "user-delete",
		Role:   "authenticated",
	}

	conditions, err := enforcer.GetDeleteConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	expected := "(user_id = 'user-delete')"
	if conditions != expected {
		t.Errorf("expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerAllCommand(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create an ALL policy (should apply to all operations)
	_, err := policyService.CreatePolicy("todos", "all_ops", "ALL", "user_id = auth.uid()", "user_id = auth.uid()")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{
		UserID: "user-all",
		Role:   "authenticated",
	}

	expected := "(user_id = 'user-all')"

	// Test SELECT
	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get SELECT conditions: %v", err)
	}
	if conditions != expected {
		t.Errorf("SELECT: expected %q, got %q", expected, conditions)
	}

	// Test UPDATE
	conditions, err = enforcer.GetUpdateConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get UPDATE conditions: %v", err)
	}
	if conditions != expected {
		t.Errorf("UPDATE: expected %q, got %q", expected, conditions)
	}

	// Test DELETE
	conditions, err = enforcer.GetDeleteConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get DELETE conditions: %v", err)
	}
	if conditions != expected {
		t.Errorf("DELETE: expected %q, got %q", expected, conditions)
	}

	// Test INSERT (uses check_expr)
	conditions, err = enforcer.GetInsertConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get INSERT conditions: %v", err)
	}
	if conditions != expected {
		t.Errorf("INSERT: expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerPolicyWithRoleCheck(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create a policy that checks role
	_, err := policyService.CreatePolicy("admin_data", "admin_only", "SELECT", "auth.role() = 'admin'", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{
		UserID: "user-123",
		Role:   "admin",
	}

	conditions, err := enforcer.GetSelectConditions("admin_data", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	expected := "('admin' = 'admin')"
	if conditions != expected {
		t.Errorf("expected %q, got %q", expected, conditions)
	}
}

func TestEnforcerIgnoresOtherCommands(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create INSERT-only policy
	_, err := policyService.CreatePolicy("todos", "insert_only", "INSERT", "", "user_id = auth.uid()")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{UserID: "user-123"}

	// SELECT should not use INSERT policy
	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}
	if conditions != "" {
		t.Errorf("expected empty conditions for SELECT when only INSERT policy exists, got %q", conditions)
	}
}

func TestEnforcerEmptyUsingExpr(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	policyService := NewService(database)
	enforcer := NewEnforcer(policyService)

	// Create a policy with empty using_expr
	_, err := policyService.CreatePolicy("todos", "no_using", "SELECT", "", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	ctx := &AuthContext{UserID: "user-123"}

	conditions, err := enforcer.GetSelectConditions("todos", ctx)
	if err != nil {
		t.Fatalf("failed to get conditions: %v", err)
	}

	// Empty using_expr should result in no conditions
	if conditions != "" {
		t.Errorf("expected empty conditions for policy with empty using_expr, got %q", conditions)
	}
}
