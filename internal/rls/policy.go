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

// UpdatePolicy updates an existing policy
func (s *Service) UpdatePolicy(id int64, policyName, command, usingExpr, checkExpr string, enabled bool) (*Policy, error) {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := s.db.Exec(`
		UPDATE _rls_policies
		SET policy_name = ?, command = ?, using_expr = ?, check_expr = ?, enabled = ?
		WHERE id = ?
	`, policyName, command, usingExpr, checkExpr, enabledInt, id)
	if err != nil {
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}
	return s.GetPolicyByID(id)
}

// SetPolicyEnabled enables or disables a policy
func (s *Service) SetPolicyEnabled(id int64, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := s.db.Exec("UPDATE _rls_policies SET enabled = ? WHERE id = ?", enabledInt, id)
	if err != nil {
		return fmt.Errorf("failed to update policy enabled state: %w", err)
	}
	return nil
}

// GetAllPoliciesForTable retrieves all policies for a table (including disabled)
func (s *Service) GetAllPoliciesForTable(tableName string) ([]*Policy, error) {
	rows, err := s.db.Query(`
		SELECT id, table_name, policy_name, command, using_expr, check_expr, enabled, created_at
		FROM _rls_policies WHERE table_name = ? ORDER BY policy_name
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

// DeletePolicy deletes a policy by its ID
func (s *Service) DeletePolicy(id int64) error {
	_, err := s.db.Exec("DELETE FROM _rls_policies WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete policy: %w", err)
	}
	return nil
}

// IsRLSEnabled checks if RLS is enabled for a table
func (s *Service) IsRLSEnabled(tableName string) (bool, error) {
	var enabled int
	err := s.db.QueryRow("SELECT enabled FROM _rls_tables WHERE table_name = ?", tableName).Scan(&enabled)
	if err == sql.ErrNoRows {
		return false, nil // Default to disabled if not set
	}
	if err != nil {
		return false, fmt.Errorf("failed to check RLS state: %w", err)
	}
	return enabled == 1, nil
}

// SetRLSEnabled enables or disables RLS for a table
func (s *Service) SetRLSEnabled(tableName string, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := s.db.Exec(`
		INSERT INTO _rls_tables (table_name, enabled) VALUES (?, ?)
		ON CONFLICT(table_name) DO UPDATE SET enabled = excluded.enabled
	`, tableName, enabledInt)
	if err != nil {
		return fmt.Errorf("failed to set RLS state: %w", err)
	}
	return nil
}

// GetTablesWithRLSState returns all tables with their RLS enabled state
func (s *Service) GetTablesWithRLSState() (map[string]bool, error) {
	rows, err := s.db.Query("SELECT table_name, enabled FROM _rls_tables")
	if err != nil {
		return nil, fmt.Errorf("failed to query RLS states: %w", err)
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var tableName string
		var enabled int
		if err := rows.Scan(&tableName, &enabled); err != nil {
			return nil, fmt.Errorf("failed to scan RLS state: %w", err)
		}
		result[tableName] = enabled == 1
	}
	return result, nil
}
