// Package verification provides post-migration verification checks.
package verification

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// VerificationLayer represents the layer of verification.
type VerificationLayer string

const (
	LayerBasic      VerificationLayer = "basic"
	LayerIntegrity  VerificationLayer = "integrity"
	LayerFunctional VerificationLayer = "functional"
)

// VerificationStatus represents the status of a verification.
type VerificationStatus string

const (
	VerifyPending VerificationStatus = "pending"
	VerifyRunning VerificationStatus = "running"
	VerifyPassed  VerificationStatus = "passed"
	VerifyFailed  VerificationStatus = "failed"
)

// CheckResult represents the result of a single verification check.
type CheckResult struct {
	Name    string      `json:"name"`
	Passed  bool        `json:"passed"`
	Message string      `json:"message,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

// VerificationResult represents the overall result of a verification layer.
type VerificationResult struct {
	Layer   VerificationLayer  `json:"layer"`
	Status  VerificationStatus `json:"status"`
	Checks  []CheckResult      `json:"checks"`
	Summary struct {
		Total  int `json:"total"`
		Passed int `json:"passed"`
		Failed int `json:"failed"`
	} `json:"summary"`
}

// MigrationItem represents a single item within a migration.
type MigrationItem struct {
	ID          string          `json:"id"`
	MigrationID string          `json:"migration_id"`
	ItemType    string          `json:"item_type"`
	ItemName    string          `json:"item_name"`
	Status      string          `json:"status"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// Migration represents a migration operation to Supabase.
type Migration struct {
	ID                  string `json:"id"`
	SupabaseProjectRef  string `json:"supabase_project_ref"`
	SupabaseProjectName string `json:"supabase_project_name"`
}

// SupabaseClient interface for Supabase Management API operations.
type SupabaseClient interface {
	ListFunctions(projectRef string) ([]FunctionInfo, error)
	ListSecrets(projectRef string) ([]SecretInfo, error)
	GetAuthConfig(projectRef string) (map[string]interface{}, error)
}

// FunctionInfo represents information about a deployed edge function.
type FunctionInfo struct {
	Slug      string `json:"slug"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	VerifyJWT bool   `json:"verify_jwt"`
}

// SecretInfo represents information about a secret (name only).
type SecretInfo struct {
	Name string `json:"name"`
}

// BasicVerifier performs basic verification checks after migration.
type BasicVerifier struct {
	sbliteDB       *sql.DB
	supabaseDB     *sql.DB
	supabaseClient SupabaseClient
	migration      *Migration
	items          []*MigrationItem
}

// NewBasicVerifier creates a new BasicVerifier instance.
func NewBasicVerifier(
	sbliteDB *sql.DB,
	supabaseDB *sql.DB,
	supabaseClient SupabaseClient,
	migration *Migration,
	items []*MigrationItem,
) *BasicVerifier {
	return &BasicVerifier{
		sbliteDB:       sbliteDB,
		supabaseDB:     supabaseDB,
		supabaseClient: supabaseClient,
		migration:      migration,
		items:          items,
	}
}

// RunBasicChecks executes all basic verification checks based on migrated items.
func (v *BasicVerifier) RunBasicChecks() (*VerificationResult, error) {
	result := &VerificationResult{
		Layer:  LayerBasic,
		Status: VerifyRunning,
		Checks: []CheckResult{},
	}

	// Group items by type for efficient checking
	itemsByType := make(map[string][]*MigrationItem)
	for _, item := range v.items {
		if item.Status == "completed" {
			itemsByType[item.ItemType] = append(itemsByType[item.ItemType], item)
		}
	}

	// Run checks based on what was migrated
	if _, ok := itemsByType["schema"]; ok {
		checks := v.checkTablesExist()
		result.Checks = append(result.Checks, checks...)
	}

	if dataItems, ok := itemsByType["data"]; ok {
		checks := v.checkColumnsMatch(dataItems)
		result.Checks = append(result.Checks, checks...)
	}

	if funcItems, ok := itemsByType["functions"]; ok {
		checks := v.checkFunctionsDeployed(funcItems)
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["storage_buckets"]; ok {
		checks := v.checkBucketsExist()
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["rls"]; ok {
		checks := v.checkRLSEnabled()
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["secrets"]; ok {
		checks := v.checkSecretsExist()
		result.Checks = append(result.Checks, checks...)
	}

	if _, ok := itemsByType["auth_config"]; ok {
		checks := v.checkAuthConfig()
		result.Checks = append(result.Checks, checks...)
	}

	// Calculate summary
	for _, check := range result.Checks {
		result.Summary.Total++
		if check.Passed {
			result.Summary.Passed++
		} else {
			result.Summary.Failed++
		}
	}

	// Set final status
	if result.Summary.Failed > 0 {
		result.Status = VerifyFailed
	} else {
		result.Status = VerifyPassed
	}

	return result, nil
}

// checkTablesExist verifies that all expected tables exist in Supabase.
func (v *BasicVerifier) checkTablesExist() []CheckResult {
	var results []CheckResult

	// Get tables from sblite _columns
	sbliteTables, err := v.getSbliteTables()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "tables_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite tables: %v", err),
		})
		return results
	}

	if len(sbliteTables) == 0 {
		results = append(results, CheckResult{
			Name:    "tables_exist",
			Passed:  true,
			Message: "No tables to verify",
		})
		return results
	}

	// Get tables from Supabase
	supabaseTables, err := v.getSupabaseTables()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "tables_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase tables: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseTableSet := make(map[string]bool)
	for _, t := range supabaseTables {
		supabaseTableSet[t] = true
	}

	// Check each sblite table exists in Supabase
	var missing []string
	var found []string
	for _, table := range sbliteTables {
		if supabaseTableSet[table] {
			found = append(found, table)
		} else {
			missing = append(missing, table)
		}
	}

	if len(missing) > 0 {
		results = append(results, CheckResult{
			Name:    "tables_exist",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d tables missing in Supabase", len(missing), len(sbliteTables)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else {
		results = append(results, CheckResult{
			Name:    "tables_exist",
			Passed:  true,
			Message: fmt.Sprintf("All %d tables exist in Supabase", len(sbliteTables)),
			Details: map[string]interface{}{
				"tables": found,
			},
		})
	}

	return results
}

// checkColumnsMatch verifies that table columns match between sblite and Supabase.
func (v *BasicVerifier) checkColumnsMatch(dataItems []*MigrationItem) []CheckResult {
	var results []CheckResult

	for _, item := range dataItems {
		tableName := item.ItemName

		// Get column count from sblite
		sbliteCount, err := v.getSbliteColumnCount(tableName)
		if err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get sblite columns for %s: %v", tableName, err),
			})
			continue
		}

		// Get column count from Supabase
		supabaseCount, err := v.getSupabaseColumnCount(tableName)
		if err != nil {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Failed to get Supabase columns for %s: %v", tableName, err),
			})
			continue
		}

		if sbliteCount == supabaseCount {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  true,
				Message: fmt.Sprintf("Table %s has %d columns (matching)", tableName, sbliteCount),
			})
		} else {
			results = append(results, CheckResult{
				Name:    fmt.Sprintf("columns_match_%s", tableName),
				Passed:  false,
				Message: fmt.Sprintf("Table %s column count mismatch: sblite=%d, Supabase=%d", tableName, sbliteCount, supabaseCount),
				Details: map[string]interface{}{
					"sblite_columns":   sbliteCount,
					"supabase_columns": supabaseCount,
				},
			})
		}
	}

	return results
}

// checkFunctionsDeployed verifies that edge functions are deployed in Supabase.
func (v *BasicVerifier) checkFunctionsDeployed(funcItems []*MigrationItem) []CheckResult {
	var results []CheckResult

	// Get functions from Supabase
	functions, err := v.supabaseClient.ListFunctions(v.migration.SupabaseProjectRef)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "functions_deployed",
			Passed:  false,
			Message: fmt.Sprintf("Failed to list Supabase functions: %v", err),
		})
		return results
	}

	// Build lookup set
	functionSet := make(map[string]FunctionInfo)
	for _, f := range functions {
		functionSet[f.Slug] = f
	}

	// Check each expected function
	var missing []string
	var found []string
	for _, item := range funcItems {
		funcName := item.ItemName
		if _, ok := functionSet[funcName]; ok {
			found = append(found, funcName)
		} else {
			missing = append(missing, funcName)
		}
	}

	if len(missing) > 0 {
		results = append(results, CheckResult{
			Name:    "functions_deployed",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d functions missing in Supabase", len(missing), len(funcItems)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else if len(found) > 0 {
		results = append(results, CheckResult{
			Name:    "functions_deployed",
			Passed:  true,
			Message: fmt.Sprintf("All %d functions deployed in Supabase", len(found)),
			Details: map[string]interface{}{
				"functions": found,
			},
		})
	}

	return results
}

// checkBucketsExist verifies that storage buckets exist in Supabase.
func (v *BasicVerifier) checkBucketsExist() []CheckResult {
	var results []CheckResult

	// Get buckets from sblite
	sbliteBuckets, err := v.getSbliteBuckets()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "buckets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite buckets: %v", err),
		})
		return results
	}

	if len(sbliteBuckets) == 0 {
		results = append(results, CheckResult{
			Name:    "buckets_exist",
			Passed:  true,
			Message: "No buckets to verify",
		})
		return results
	}

	// Get buckets from Supabase
	supabaseBuckets, err := v.getSupabaseBuckets()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "buckets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase buckets: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseBucketSet := make(map[string]bool)
	for _, b := range supabaseBuckets {
		supabaseBucketSet[b] = true
	}

	// Check each sblite bucket exists in Supabase
	var missing []string
	var found []string
	for _, bucket := range sbliteBuckets {
		if supabaseBucketSet[bucket] {
			found = append(found, bucket)
		} else {
			missing = append(missing, bucket)
		}
	}

	if len(missing) > 0 {
		results = append(results, CheckResult{
			Name:    "buckets_exist",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d buckets missing in Supabase", len(missing), len(sbliteBuckets)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else {
		results = append(results, CheckResult{
			Name:    "buckets_exist",
			Passed:  true,
			Message: fmt.Sprintf("All %d buckets exist in Supabase", len(sbliteBuckets)),
			Details: map[string]interface{}{
				"buckets": found,
			},
		})
	}

	return results
}

// checkRLSEnabled verifies that RLS is enabled on the correct tables.
func (v *BasicVerifier) checkRLSEnabled() []CheckResult {
	var results []CheckResult

	// Get tables with RLS from sblite
	sbliteRLSTables, err := v.getSbliteRLSTables()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "rls_enabled",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite RLS tables: %v", err),
		})
		return results
	}

	if len(sbliteRLSTables) == 0 {
		results = append(results, CheckResult{
			Name:    "rls_enabled",
			Passed:  true,
			Message: "No RLS tables to verify",
		})
		return results
	}

	// Get RLS-enabled tables from Supabase
	supabaseRLSTables, err := v.getSupabaseRLSTables()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "rls_enabled",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase RLS tables: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseRLSSet := make(map[string]bool)
	for _, t := range supabaseRLSTables {
		supabaseRLSSet[t] = true
	}

	// Check each sblite RLS table has RLS enabled in Supabase
	var missingRLS []string
	var hasRLS []string
	for _, table := range sbliteRLSTables {
		if supabaseRLSSet[table] {
			hasRLS = append(hasRLS, table)
		} else {
			missingRLS = append(missingRLS, table)
		}
	}

	if len(missingRLS) > 0 {
		results = append(results, CheckResult{
			Name:    "rls_enabled",
			Passed:  false,
			Message: fmt.Sprintf("RLS not enabled on %d of %d tables", len(missingRLS), len(sbliteRLSTables)),
			Details: map[string]interface{}{
				"missing_rls": missingRLS,
				"has_rls":     hasRLS,
			},
		})
	} else {
		results = append(results, CheckResult{
			Name:    "rls_enabled",
			Passed:  true,
			Message: fmt.Sprintf("RLS enabled on all %d expected tables", len(sbliteRLSTables)),
			Details: map[string]interface{}{
				"tables": hasRLS,
			},
		})
	}

	return results
}

// checkSecretsExist verifies that secrets exist in Supabase (names only).
func (v *BasicVerifier) checkSecretsExist() []CheckResult {
	var results []CheckResult

	// Get secret names from sblite
	sbliteSecrets, err := v.getSbliteSecretNames()
	if err != nil {
		results = append(results, CheckResult{
			Name:    "secrets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite secrets: %v", err),
		})
		return results
	}

	if len(sbliteSecrets) == 0 {
		results = append(results, CheckResult{
			Name:    "secrets_exist",
			Passed:  true,
			Message: "No secrets to verify",
		})
		return results
	}

	// Get secret names from Supabase
	supabaseSecrets, err := v.supabaseClient.ListSecrets(v.migration.SupabaseProjectRef)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "secrets_exist",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase secrets: %v", err),
		})
		return results
	}

	// Build lookup set
	supabaseSecretSet := make(map[string]bool)
	for _, s := range supabaseSecrets {
		supabaseSecretSet[s.Name] = true
	}

	// Check each sblite secret exists in Supabase
	var missing []string
	var found []string
	for _, secret := range sbliteSecrets {
		if supabaseSecretSet[secret] {
			found = append(found, secret)
		} else {
			missing = append(missing, secret)
		}
	}

	if len(missing) > 0 {
		results = append(results, CheckResult{
			Name:    "secrets_exist",
			Passed:  false,
			Message: fmt.Sprintf("%d of %d secrets missing in Supabase", len(missing), len(sbliteSecrets)),
			Details: map[string]interface{}{
				"missing": missing,
				"found":   found,
			},
		})
	} else {
		results = append(results, CheckResult{
			Name:    "secrets_exist",
			Passed:  true,
			Message: fmt.Sprintf("All %d secrets exist in Supabase", len(sbliteSecrets)),
			Details: map[string]interface{}{
				"secrets": found,
			},
		})
	}

	return results
}

// checkAuthConfig verifies that auth configuration values match expected.
func (v *BasicVerifier) checkAuthConfig() []CheckResult {
	var results []CheckResult

	// Get expected auth config from sblite
	var allowAnonymous string
	err := v.sbliteDB.QueryRow("SELECT value FROM _dashboard WHERE key = 'allow_anonymous'").Scan(&allowAnonymous)
	if err != nil && err != sql.ErrNoRows {
		results = append(results, CheckResult{
			Name:    "auth_config",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get sblite auth config: %v", err),
		})
		return results
	}

	// Get auth config from Supabase
	config, err := v.supabaseClient.GetAuthConfig(v.migration.SupabaseProjectRef)
	if err != nil {
		results = append(results, CheckResult{
			Name:    "auth_config",
			Passed:  false,
			Message: fmt.Sprintf("Failed to get Supabase auth config: %v", err),
		})
		return results
	}

	// Check anonymous users setting
	if allowAnonymous == "true" {
		if val, ok := config["EXTERNAL_ANONYMOUS_USERS_ENABLED"].(bool); ok && val {
			results = append(results, CheckResult{
				Name:    "auth_config_anonymous_users",
				Passed:  true,
				Message: "Anonymous users setting matches (enabled)",
			})
		} else {
			results = append(results, CheckResult{
				Name:    "auth_config_anonymous_users",
				Passed:  false,
				Message: "Anonymous users should be enabled but is not",
				Details: map[string]interface{}{
					"expected": true,
					"actual":   config["EXTERNAL_ANONYMOUS_USERS_ENABLED"],
				},
			})
		}
	} else {
		results = append(results, CheckResult{
			Name:    "auth_config",
			Passed:  true,
			Message: "Auth configuration verified",
		})
	}

	return results
}

// Helper methods for querying sblite

func (v *BasicVerifier) getSbliteTables() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT DISTINCT table_name FROM _columns ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (v *BasicVerifier) getSbliteColumnCount(tableName string) (int, error) {
	var count int
	err := v.sbliteDB.QueryRow("SELECT COUNT(*) FROM _columns WHERE table_name = ?", tableName).Scan(&count)
	return count, err
}

func (v *BasicVerifier) getSbliteBuckets() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT id FROM storage_buckets ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		buckets = append(buckets, id)
	}
	return buckets, rows.Err()
}

func (v *BasicVerifier) getSbliteRLSTables() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT DISTINCT table_name FROM _rls_policies WHERE enabled = 1 ORDER BY table_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (v *BasicVerifier) getSbliteSecretNames() ([]string, error) {
	rows, err := v.sbliteDB.Query("SELECT name FROM _functions_secrets ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var secrets []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		secrets = append(secrets, name)
	}
	return secrets, rows.Err()
}

// Helper methods for querying Supabase PostgreSQL

func (v *BasicVerifier) getSupabaseTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := v.supabaseDB.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		AND table_type = 'BASE TABLE'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}

func (v *BasicVerifier) getSupabaseColumnCount(tableName string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	err := v.supabaseDB.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
	`, tableName).Scan(&count)
	return count, err
}

func (v *BasicVerifier) getSupabaseBuckets() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := v.supabaseDB.QueryContext(ctx, `
		SELECT id FROM storage.buckets ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var buckets []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		buckets = append(buckets, id)
	}
	return buckets, rows.Err()
}

func (v *BasicVerifier) getSupabaseRLSTables() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := v.supabaseDB.QueryContext(ctx, `
		SELECT tablename
		FROM pg_tables
		WHERE schemaname = 'public'
		AND rowsecurity = true
		ORDER BY tablename
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}
	return tables, rows.Err()
}
