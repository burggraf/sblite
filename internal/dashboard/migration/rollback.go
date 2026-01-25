// Package migration provides tools for migrating sblite databases to Supabase.
package migration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Rollback rolls back a migration by undoing completed items in reverse order.
func (s *Service) Rollback(migrationID string) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Validate migration can be rolled back
	if m.Status != StatusCompleted && m.Status != StatusFailed {
		return fmt.Errorf("cannot rollback migration with status %s (must be completed or failed)", m.Status)
	}

	// Update migration status to in_progress
	m.Status = StatusInProgress
	m.ErrorMessage = ""
	if err := s.state.UpdateMigration(m); err != nil {
		return fmt.Errorf("update migration status: %w", err)
	}

	// Get completed items in reverse order
	items, err := s.state.GetCompletedItemsReverse(migrationID)
	if err != nil {
		return fmt.Errorf("get completed items: %w", err)
	}

	if len(items) == 0 {
		// No items to rollback, mark as rolled back
		m.Status = StatusRolledBack
		now := time.Now().UTC()
		m.CompletedAt = &now
		return s.state.UpdateMigration(m)
	}

	// Track rollback errors
	var rollbackErrors []string

	// Process each completed item in reverse order
	for _, item := range items {
		var rollbackErr error

		switch item.ItemType {
		case ItemSchema:
			rollbackErr = s.rollbackSchema(m, item)
		case ItemData:
			rollbackErr = s.rollbackData(m, item)
		case ItemUsers:
			rollbackErr = s.rollbackUsers(m, item)
		case ItemIdentities:
			rollbackErr = s.rollbackIdentities(m, item)
		case ItemRLS:
			rollbackErr = s.rollbackRLS(m, item)
		case ItemStorageBuckets:
			rollbackErr = s.rollbackStorageBuckets(m, item)
		case ItemStorageFiles:
			rollbackErr = s.rollbackStorageFiles(m, item)
		case ItemFunctions:
			rollbackErr = s.rollbackFunctions(m, item)
		case ItemSecrets:
			rollbackErr = s.rollbackSecrets(m, item)
		case ItemAuthConfig, ItemOAuthConfig, ItemEmailTemplates:
			// Config items don't have meaningful rollback, just mark as rolled back
			rollbackErr = s.markItemRolledBack(item)
		default:
			rollbackErr = fmt.Errorf("unknown item type: %s", item.ItemType)
		}

		if rollbackErr != nil {
			rollbackErrors = append(rollbackErrors, fmt.Sprintf("%s/%s: %v", item.ItemType, item.ItemName, rollbackErr))
			// Continue with other items even if one fails
		}
	}

	// Update migration status based on results
	now := time.Now().UTC()
	m.CompletedAt = &now

	if len(rollbackErrors) > 0 {
		m.Status = StatusFailed
		m.ErrorMessage = fmt.Sprintf("rollback errors: %s", strings.Join(rollbackErrors, "; "))
	} else {
		m.Status = StatusRolledBack
	}

	if err := s.state.UpdateMigration(m); err != nil {
		return fmt.Errorf("update migration status: %w", err)
	}

	if len(rollbackErrors) > 0 {
		return fmt.Errorf("rollback completed with errors: %s", strings.Join(rollbackErrors, "; "))
	}

	return nil
}

// markItemRolledBack marks an item as rolled back.
func (s *Service) markItemRolledBack(item *MigrationItem) error {
	item.Status = ItemRolledBack
	return s.state.UpdateItem(item)
}

// rollbackSchema drops tables that were created during schema migration.
func (s *Service) rollbackSchema(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info SchemaRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if len(info.Tables) == 0 {
		return s.markItemRolledBack(item)
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pgDB.Close()

	// Drop tables in reverse order (to handle potential foreign key dependencies)
	for i := len(info.Tables) - 1; i >= 0; i-- {
		tableName := info.Tables[i]

		quotedTable, err := quoteIdentifier(tableName)
		if err != nil {
			return fmt.Errorf("invalid table name %s: %w", tableName, err)
		}

		// Use CASCADE to drop dependent objects
		_, err = pgDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", quotedTable))
		if err != nil {
			return fmt.Errorf("drop table %s: %w", tableName, err)
		}
	}

	return s.markItemRolledBack(item)
}

// rollbackData deletes data that was migrated to a table.
func (s *Service) rollbackData(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info DataRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if info.TableName == "" || info.RowCount == 0 {
		return s.markItemRolledBack(item)
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pgDB.Close()

	quotedTable, err := quoteIdentifier(info.TableName)
	if err != nil {
		return fmt.Errorf("invalid table name %s: %w", info.TableName, err)
	}

	// TRUNCATE is faster and resets sequences
	_, err = pgDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", quotedTable))
	if err != nil {
		// If TRUNCATE fails (e.g., due to permissions), try DELETE
		_, err = pgDB.Exec(fmt.Sprintf("DELETE FROM %s", quotedTable))
		if err != nil {
			return fmt.Errorf("delete data from %s: %w", info.TableName, err)
		}
	}

	return s.markItemRolledBack(item)
}

// rollbackUsers deletes users that were migrated to Supabase.
func (s *Service) rollbackUsers(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info UsersRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if len(info.UserIDs) == 0 {
		return s.markItemRolledBack(item)
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pgDB.Close()

	// Build placeholders for IN clause
	placeholders := make([]string, len(info.UserIDs))
	args := make([]interface{}, len(info.UserIDs))
	for i, id := range info.UserIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	// Delete users (cascades to related tables like identities, sessions)
	query := fmt.Sprintf("DELETE FROM auth.users WHERE id IN (%s)", strings.Join(placeholders, ", "))
	_, err = pgDB.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete users: %w", err)
	}

	return s.markItemRolledBack(item)
}

// rollbackIdentities deletes OAuth identities that were migrated to Supabase.
func (s *Service) rollbackIdentities(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info IdentitiesRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if len(info.IdentityIDs) == 0 {
		return s.markItemRolledBack(item)
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pgDB.Close()

	// Build placeholders for IN clause
	placeholders := make([]string, len(info.IdentityIDs))
	args := make([]interface{}, len(info.IdentityIDs))
	for i, id := range info.IdentityIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	query := fmt.Sprintf("DELETE FROM auth.identities WHERE id IN (%s)", strings.Join(placeholders, ", "))
	_, err = pgDB.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("delete identities: %w", err)
	}

	return s.markItemRolledBack(item)
}

// rollbackRLS drops RLS policies that were created during migration.
func (s *Service) rollbackRLS(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info RLSRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if len(info.Policies) == 0 {
		return s.markItemRolledBack(item)
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pgDB.Close()

	// Drop each policy
	for _, policy := range info.Policies {
		quotedTable, err := quoteIdentifier(policy.TableName)
		if err != nil {
			return fmt.Errorf("invalid table name %s: %w", policy.TableName, err)
		}

		quotedPolicy, err := quoteIdentifier(policy.PolicyName)
		if err != nil {
			return fmt.Errorf("invalid policy name %s: %w", policy.PolicyName, err)
		}

		_, err = pgDB.Exec(fmt.Sprintf("DROP POLICY IF EXISTS %s ON %s", quotedPolicy, quotedTable))
		if err != nil {
			return fmt.Errorf("drop policy %s.%s: %w", policy.TableName, policy.PolicyName, err)
		}
	}

	return s.markItemRolledBack(item)
}

// rollbackStorageBuckets deletes storage buckets that were created during migration.
func (s *Service) rollbackStorageBuckets(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info BucketsRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if len(info.BucketIDs) == 0 {
		return s.markItemRolledBack(item)
	}

	// Connect to Supabase PostgreSQL
	pgDB, err := s.getPostgresConnection(m)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}
	defer pgDB.Close()

	// Build placeholders for IN clause
	placeholders := make([]string, len(info.BucketIDs))
	args := make([]interface{}, len(info.BucketIDs))
	for i, id := range info.BucketIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	// Delete objects first (foreign key constraint)
	objectsQuery := fmt.Sprintf("DELETE FROM storage.objects WHERE bucket_id IN (%s)", strings.Join(placeholders, ", "))
	_, err = pgDB.Exec(objectsQuery, args...)
	if err != nil {
		// Non-fatal, continue with bucket deletion
	}

	// Delete buckets
	bucketsQuery := fmt.Sprintf("DELETE FROM storage.buckets WHERE id IN (%s)", strings.Join(placeholders, ", "))
	_, err = pgDB.Exec(bucketsQuery, args...)
	if err != nil {
		return fmt.Errorf("delete buckets: %w", err)
	}

	return s.markItemRolledBack(item)
}

// rollbackStorageFiles deletes files that were uploaded to Supabase storage.
func (s *Service) rollbackStorageFiles(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info FilesRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if info.BucketID == "" || len(info.Paths) == 0 {
		return s.markItemRolledBack(item)
	}

	// Get Supabase client and API keys
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		return fmt.Errorf("get supabase client: %w", err)
	}

	apiKeys, err := client.GetAPIKeys(m.SupabaseProjectRef)
	if err != nil {
		return fmt.Errorf("get api keys: %w", err)
	}

	// Find service_role key
	var serviceKey string
	for _, key := range apiKeys {
		if key.Name == "service_role" {
			serviceKey = key.APIKey
			break
		}
	}
	if serviceKey == "" {
		return fmt.Errorf("service_role key not found")
	}

	// Delete each file via Supabase Storage API
	storageURL := fmt.Sprintf("https://%s.supabase.co/storage/v1/object/%s", m.SupabaseProjectRef, info.BucketID)

	for _, path := range info.Paths {
		deleteURL := fmt.Sprintf("%s/%s", storageURL, path)
		req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
		if err != nil {
			return fmt.Errorf("create delete request for %s: %w", path, err)
		}

		req.Header.Set("Authorization", "Bearer "+serviceKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("delete file %s: %w", path, err)
		}
		resp.Body.Close()

		// Accept 200, 204, or 404 (already deleted)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("delete file %s: status %d", path, resp.StatusCode)
		}
	}

	return s.markItemRolledBack(item)
}

// rollbackFunctions deletes edge functions that were deployed to Supabase.
func (s *Service) rollbackFunctions(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info FunctionsRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if info.FunctionName == "" {
		return s.markItemRolledBack(item)
	}

	// Get Supabase client
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		return fmt.Errorf("get supabase client: %w", err)
	}

	// Delete the function
	if err := client.DeleteFunction(m.SupabaseProjectRef, info.FunctionName); err != nil {
		return fmt.Errorf("delete function %s: %w", info.FunctionName, err)
	}

	return s.markItemRolledBack(item)
}

// rollbackSecrets deletes secrets that were created in Supabase.
func (s *Service) rollbackSecrets(m *Migration, item *MigrationItem) error {
	if item.RollbackInfo == "" {
		return s.markItemRolledBack(item)
	}

	var info SecretsRollbackInfo
	if err := json.Unmarshal([]byte(item.RollbackInfo), &info); err != nil {
		return fmt.Errorf("parse rollback info: %w", err)
	}

	if len(info.SecretNames) == 0 {
		return s.markItemRolledBack(item)
	}

	// Get Supabase client
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		return fmt.Errorf("get supabase client: %w", err)
	}

	// Delete secrets
	if err := client.DeleteSecrets(m.SupabaseProjectRef, info.SecretNames); err != nil {
		return fmt.Errorf("delete secrets: %w", err)
	}

	return s.markItemRolledBack(item)
}

// DeleteStorageObject deletes a single storage object via the Supabase Storage API.
// This is a helper method exposed for testing or manual cleanup.
func (s *Service) DeleteStorageObject(migrationID, bucketID, path string) error {
	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Get Supabase client and API keys
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		return fmt.Errorf("get supabase client: %w", err)
	}

	apiKeys, err := client.GetAPIKeys(m.SupabaseProjectRef)
	if err != nil {
		return fmt.Errorf("get api keys: %w", err)
	}

	// Find service_role key
	var serviceKey string
	for _, key := range apiKeys {
		if key.Name == "service_role" {
			serviceKey = key.APIKey
			break
		}
	}
	if serviceKey == "" {
		return fmt.Errorf("service_role key not found")
	}

	// Delete via Storage API
	deleteURL := fmt.Sprintf("https://%s.supabase.co/storage/v1/object/%s/%s", m.SupabaseProjectRef, bucketID, path)
	req, err := http.NewRequest(http.MethodDelete, deleteURL, nil)
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+serviceKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("delete file: status %d", resp.StatusCode)
	}

	return nil
}

// BulkDeleteStorageObjects deletes multiple storage objects from a bucket.
// This uses the Supabase bulk delete endpoint for efficiency.
func (s *Service) BulkDeleteStorageObjects(migrationID, bucketID string, paths []string) error {
	if len(paths) == 0 {
		return nil
	}

	m, err := s.GetMigration(migrationID)
	if err != nil {
		return err
	}

	// Get Supabase client and API keys
	client, err := s.getSupabaseClient(m.ID)
	if err != nil {
		return fmt.Errorf("get supabase client: %w", err)
	}

	apiKeys, err := client.GetAPIKeys(m.SupabaseProjectRef)
	if err != nil {
		return fmt.Errorf("get api keys: %w", err)
	}

	// Find service_role key
	var serviceKey string
	for _, key := range apiKeys {
		if key.Name == "service_role" {
			serviceKey = key.APIKey
			break
		}
	}
	if serviceKey == "" {
		return fmt.Errorf("service_role key not found")
	}

	// Supabase bulk delete endpoint
	deleteURL := fmt.Sprintf("https://%s.supabase.co/storage/v1/object/%s", m.SupabaseProjectRef, bucketID)

	// Build request body with prefixes array
	body := map[string]interface{}{
		"prefixes": paths,
	}
	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal delete body: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, deleteURL, bytes.NewReader(bodyJSON))
	if err != nil {
		return fmt.Errorf("create delete request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+serviceKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("bulk delete: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("bulk delete: status %d", resp.StatusCode)
	}

	return nil
}
