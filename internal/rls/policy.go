// internal/rls/policy.go
package rls

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/markb/sblite/internal/db"
)

// Policy represents a row-level security policy
type Policy struct {
	ID         int64     `json:"id"`
	TableName  string    `json:"table_name"`
	PolicyName string    `json:"policy_name"`
	Command    string    `json:"command"`
	UsingExpr  string    `json:"using_expr,omitempty"`
	CheckExpr  string    `json:"check_expr,omitempty"`
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
}

// Service provides CRUD operations for RLS policies
type Service struct {
	db *db.DB
}

// NewService creates a new RLS policy service
func NewService(database *db.DB) *Service {
	return &Service{db: database}
}

// CreatePolicy creates a new RLS policy
func (s *Service) CreatePolicy(tableName, policyName, command, usingExpr, checkExpr string) (*Policy, error) {
	result, err := s.db.Exec(`
		INSERT INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr)
		VALUES (?, ?, ?, ?, ?)
	`, tableName, policyName, command, usingExpr, checkExpr)
	if err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get inserted policy ID: %w", err)
	}
	return s.GetPolicyByID(id)
}

// GetPolicyByID retrieves a policy by its ID
func (s *Service) GetPolicyByID(id int64) (*Policy, error) {
	var p Policy
	var createdAt string
	var usingExpr, checkExpr sql.NullString

	err := s.db.QueryRow(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE id = ?
	`, id).Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &p.Enabled, &createdAt)

	if err != nil {
		return nil, fmt.Errorf("policy not found: %w", err)
	}

	p.UsingExpr = usingExpr.String
	p.CheckExpr = checkExpr.String
	parsedTime, err := time.Parse(time.DateTime, createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse created_at timestamp: %w", err)
	}
	p.CreatedAt = parsedTime
	return &p, nil
}

// GetPoliciesForTable retrieves all enabled policies for a specific table
func (s *Service) GetPoliciesForTable(tableName string) ([]*Policy, error) {
	rows, err := s.db.Query(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE table_name = ? AND enabled = 1
	`, tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to query policies for table: %w", err)
	}
	defer rows.Close()

	var policies []*Policy
	for rows.Next() {
		var p Policy
		var createdAt string
		var usingExpr, checkExpr sql.NullString

		if err := rows.Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &p.Enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}
		p.UsingExpr = usingExpr.String
		p.CheckExpr = checkExpr.String
		parsedTime, err := time.Parse(time.DateTime, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at timestamp: %w", err)
		}
		p.CreatedAt = parsedTime
		policies = append(policies, &p)
	}
	return policies, nil
}

// ListAllPolicies retrieves all policies ordered by table name and policy name
func (s *Service) ListAllPolicies() ([]*Policy, error) {
	rows, err := s.db.Query(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies ORDER BY table_name, policy_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all policies: %w", err)
	}
	defer rows.Close()

	var policies []*Policy
	for rows.Next() {
		var p Policy
		var createdAt string
		var usingExpr, checkExpr sql.NullString

		if err := rows.Scan(&p.ID, &p.TableName, &p.PolicyName, &p.Command, &usingExpr, &checkExpr, &p.Enabled, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}
		p.UsingExpr = usingExpr.String
		p.CheckExpr = checkExpr.String
		parsedTime, err := time.Parse(time.DateTime, createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to parse created_at timestamp: %w", err)
		}
		p.CreatedAt = parsedTime
		policies = append(policies, &p)
	}
	return policies, nil
}

// DeletePolicy deletes a policy by its ID
func (s *Service) DeletePolicy(id int64) error {
	_, err := s.db.Exec("DELETE FROM _rls_policies WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}
	return nil
}
