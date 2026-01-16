// internal/rls/policy_test.go
package rls

import (
	"testing"

	"github.com/markb/sblite/internal/db"
)

func setupTestDB(t *testing.T) *db.DB {
	t.Helper()
	path := t.TempDir() + "/test.db"
	database, err := db.New(path)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	return database
}

func TestCreatePolicy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	policy, err := service.CreatePolicy("todos", "user_isolation", "ALL", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	if policy.TableName != "todos" {
		t.Errorf("expected table_name 'todos', got %s", policy.TableName)
	}
	if policy.PolicyName != "user_isolation" {
		t.Errorf("expected policy_name 'user_isolation', got %s", policy.PolicyName)
	}
	if policy.UsingExpr != "user_id = auth.uid()" {
		t.Errorf("expected using_expr 'user_id = auth.uid()', got %s", policy.UsingExpr)
	}
	if policy.Command != "ALL" {
		t.Errorf("expected command 'ALL', got %s", policy.Command)
	}
	if !policy.Enabled {
		t.Error("expected policy to be enabled by default")
	}
}

func TestGetPoliciesForTable(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	// Create two policies for same table
	_, err := service.CreatePolicy("todos", "policy1", "SELECT", "user_id = auth.uid()", "")
	if err != nil {
		t.Fatalf("failed to create policy1: %v", err)
	}
	_, err = service.CreatePolicy("todos", "policy2", "INSERT", "", "user_id = auth.uid()")
	if err != nil {
		t.Fatalf("failed to create policy2: %v", err)
	}
	_, err = service.CreatePolicy("other_table", "policy3", "ALL", "true", "")
	if err != nil {
		t.Fatalf("failed to create policy3: %v", err)
	}

	policies, err := service.GetPoliciesForTable("todos")
	if err != nil {
		t.Fatalf("failed to get policies: %v", err)
	}

	if len(policies) != 2 {
		t.Errorf("expected 2 policies for 'todos', got %d", len(policies))
	}
}

func TestListAllPolicies(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	// Create policies for different tables
	_, err := service.CreatePolicy("todos", "policy1", "SELECT", "true", "")
	if err != nil {
		t.Fatalf("failed to create policy1: %v", err)
	}
	_, err = service.CreatePolicy("users", "policy2", "ALL", "true", "")
	if err != nil {
		t.Fatalf("failed to create policy2: %v", err)
	}

	policies, err := service.ListAllPolicies()
	if err != nil {
		t.Fatalf("failed to list policies: %v", err)
	}

	if len(policies) != 2 {
		t.Errorf("expected 2 policies, got %d", len(policies))
	}

	// Should be ordered by table_name
	if policies[0].TableName != "todos" {
		t.Errorf("expected first policy table_name 'todos', got %s", policies[0].TableName)
	}
	if policies[1].TableName != "users" {
		t.Errorf("expected second policy table_name 'users', got %s", policies[1].TableName)
	}
}

func TestDeletePolicy(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	policy, err := service.CreatePolicy("todos", "to_delete", "SELECT", "true", "")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	err = service.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("failed to delete policy: %v", err)
	}

	_, err = service.GetPolicyByID(policy.ID)
	if err == nil {
		t.Error("expected error when getting deleted policy")
	}
}

func TestGetPolicyByID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	created, err := service.CreatePolicy("todos", "test_policy", "UPDATE", "user_id = auth.uid()", "user_id = auth.uid()")
	if err != nil {
		t.Fatalf("failed to create policy: %v", err)
	}

	retrieved, err := service.GetPolicyByID(created.ID)
	if err != nil {
		t.Fatalf("failed to get policy: %v", err)
	}

	if retrieved.ID != created.ID {
		t.Errorf("expected id %d, got %d", created.ID, retrieved.ID)
	}
	if retrieved.TableName != "todos" {
		t.Errorf("expected table_name 'todos', got %s", retrieved.TableName)
	}
	if retrieved.PolicyName != "test_policy" {
		t.Errorf("expected policy_name 'test_policy', got %s", retrieved.PolicyName)
	}
	if retrieved.Command != "UPDATE" {
		t.Errorf("expected command 'UPDATE', got %s", retrieved.Command)
	}
	if retrieved.UsingExpr != "user_id = auth.uid()" {
		t.Errorf("expected using_expr 'user_id = auth.uid()', got %s", retrieved.UsingExpr)
	}
	if retrieved.CheckExpr != "user_id = auth.uid()" {
		t.Errorf("expected check_expr 'user_id = auth.uid()', got %s", retrieved.CheckExpr)
	}
}

func TestGetPolicyByID_NotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	_, err := service.GetPolicyByID(999)
	if err == nil {
		t.Error("expected error when getting non-existent policy")
	}
}

func TestCreatePolicy_DuplicateName(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	service := NewService(database)

	_, err := service.CreatePolicy("todos", "same_name", "SELECT", "true", "")
	if err != nil {
		t.Fatalf("failed to create first policy: %v", err)
	}

	_, err = service.CreatePolicy("todos", "same_name", "INSERT", "true", "")
	if err == nil {
		t.Error("expected error when creating duplicate policy name for same table")
	}
}
