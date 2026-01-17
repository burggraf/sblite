# Test Inventory

Complete list of all E2E test cases for sblite Supabase compatibility.

## Legend

- ✅ Passing
- ❌ Failing (should be working but isn't)
- ⏭️ Skipped (feature not yet implemented)

## Summary

| Category | Total | Passing | Failing | Skipped |
|----------|-------|---------|---------|---------|
| REST - SELECT | 16 | 10 | 0 | 6 |
| REST - INSERT | 9 | 9 | 0 | 0 |
| REST - UPDATE | 10 | 8 | 0 | 2 |
| REST - UPSERT | 7 | 5 | 0 | 2 |
| REST - DELETE | 10 | 10 | 0 | 0 |
| Filters - Basic | 24 | 22 | 0 | 2 |
| Filters - Advanced | 15 | 0 | 0 | 15 |
| Filters - Logical | 11 | 3 | 0 | 8 |
| Modifiers | 18 | 18 | 0 | 0 |
| Modifiers - Count | 12 | 12 | 0 | 0 |
| Modifiers - CSV | 6 | 6 | 0 | 0 |
| Auth - Sign Up | 11 | 8 | 0 | 3 |
| Auth - Sign In/Out | 12 | 8 | 0 | 4 |
| Auth - Session | 11 | 8 | 0 | 3 |
| Auth - User | 12 | 7 | 0 | 5 |
| Auth - State Change | 11 | 6 | 0 | 5 |
| Auth - Password Reset | 6 | 3 | 0 | 3 |
| Email - Mail API | 13 | 13 | 0 | 0 |
| Email - Flows | 11 | 8 | 0 | 3 |
| Email - Verification | 11 | 9 | 0 | 2 |
| Email - SMTP | 4 | 3 | 0 | 1 |
| Relations | 10 | 10 | 0 | 0 |
| RLS | 9 | 9 | 0 | 0 |
| API Key | 12 | 12 | 0 | 0 |
| **TOTAL** | **288** | **222** | **0** | **65** |

*Last tested: 2026-01-17*

**Note:** Server must be started in `catch` mode (`--mail-mode catch`) for email tests to pass.

---

## REST API Tests

### `tests/rest/select.test.ts`

**1. Getting your data**
- ✅ should retrieve all rows and columns from a table

**2. Selecting specific columns**
- ✅ should return only the specified columns
- ✅ should return multiple specific columns

**3. Query referenced tables**
- ⏭️ should fetch related data from referenced tables

**4. Query referenced tables with spaces**
- ⏭️ should handle table names with spaces using quotes

**5. Query referenced tables through join table**
- ⏭️ should query through many-to-many join tables

**6. Query same referenced table multiple times**
- ⏭️ should allow aliasing for multiple references to same table

**7. Query nested foreign tables through join**
- ⏭️ should handle deeply nested relationships

**8. Filtering through referenced tables**
- ⏭️ should filter on fields from referenced tables

**9. Querying referenced table with count**
- ⏭️ should return count of related records

**10. Querying with count option**
- ⏭️ should return only count without data when head: true
- ⏭️ should return both count and data

**11. Querying JSON data**
- ⏭️ should extract fields from JSON columns using arrow notation

**12. Querying with inner join**
- ⏭️ should perform inner join on referenced tables

**13. Switching schemas per query**
- ⏭️ should switch to specified schema

**Additional SELECT functionality**
- ✅ should handle empty table gracefully
- ✅ should handle non-existent table with error
- ✅ should handle selecting all columns with *

---

### `tests/rest/insert.test.ts`

**1. Create a record**
- ✅ should insert a single record into the table
- ✅ should handle insert with only required fields

**2. Create a record and return it**
- ✅ should insert and return the inserted record
- ✅ should return only selected columns when specified

**3. Bulk create**
- ✅ should insert multiple records in a single operation
- ✅ should insert multiple records and return them

**Additional INSERT functionality**
- ✅ should handle duplicate primary key with error
- ✅ should handle insert to non-existent table with error
- ✅ should handle insert with null values where allowed

---

### `tests/rest/update.test.ts`

**1. Updating your data**
- ✅ should update a record matching the filter
- ✅ should update multiple fields at once

**2. Update a record and return it**
- ✅ should update and return the updated record
- ✅ should return only selected columns

**3. Updating JSON data**
- ⏭️ should update nested JSON fields
- ✅ should update JSON stored as TEXT column

**Additional UPDATE functionality**
- ✅ should not update any records if filter matches none
- ✅ should update multiple records matching filter
- ✅ should handle update to non-existent table with error

---

### `tests/rest/upsert.test.ts`

**1. Upsert your data**
- ✅ should insert a new record if it does not exist
- ✅ should update an existing record if it exists

**2. Bulk Upsert your data**
- ✅ should upsert multiple records

**3. Upserting into tables with constraints**
- ⏭️ should use specified column for conflict resolution

**Additional UPSERT functionality**
- ✅ should handle upsert without select (no return)
- ✅ should return selected columns only
- ✅ should handle upsert to non-existent table with error

---

### `tests/rest/delete.test.ts`

**1. Delete a single record**
- ✅ should delete a single record matching the filter
- ✅ should not affect other records

**2. Delete a record and return it**
- ✅ should delete and return the deleted record
- ✅ should return only selected columns

**3. Delete multiple records**
- ✅ should delete multiple records using in() filter
- ✅ should delete and return multiple records

**Additional DELETE functionality**
- ✅ should not delete any records if filter matches none
- ✅ should delete records using other filters
- ✅ should handle delete on non-existent table with error
- ✅ should handle delete with combined filters

---

## Filter Tests

### `tests/filters/basic-filters.test.ts`

**Using Filters - General**
- ✅ should apply filters after select
- ✅ should chain multiple filters (AND logic)
- ⏭️ should filter on JSON columns using arrow notation

**eq() - Equals**
- ✅ should match rows where column equals value
- ✅ should return empty array when no match
- ✅ should work with numeric values

**neq() - Not Equals**
- ✅ should match rows where column does not equal value

**gt() - Greater Than**
- ✅ should match rows where column is greater than value

**gte() - Greater Than or Equal**
- ✅ should match rows where column is greater than or equal to value

**lt() - Less Than**
- ✅ should match rows where column is less than value

**lte() - Less Than or Equal**
- ✅ should match rows where column is less than or equal to value

**like() - Pattern Matching**
- ✅ should match rows using LIKE pattern with wildcards
- ✅ should match with prefix wildcard
- ✅ should match with suffix wildcard

**ilike() - Case-Insensitive Pattern Matching**
- ✅ should match rows case-insensitively
- ✅ should match regardless of case

**is() - Null/Boolean Check**
- ✅ should match rows where column is null
- ⏭️ should match rows where boolean column is true
- ⏭️ should match rows where boolean column is false

**in() - Match Any in Array**
- ✅ should match rows where column is in array of values
- ✅ should work with numeric arrays
- ✅ should return empty array when no matches

**Combined Filters**
- ✅ should combine eq and gt filters
- ✅ should combine gte and lte for range query

---

### `tests/filters/advanced-filters.test.ts`

**contains() - Contains All Elements**
- ⏭️ should match rows where array column contains all elements
- ⏭️ should match rows where range column contains value
- ⏭️ should match rows where JSONB column contains object

**containedBy() - Contained By**
- ⏭️ should match rows where array is contained by given array
- ⏭️ should match rows where range is contained by given range
- ⏭️ should match rows where JSONB is contained by given object

**rangeGt() - Greater Than Range**
- ⏭️ should match rows where range is greater than given range

**rangeGte() - Greater Than or Equal Range**
- ⏭️ should match rows where range is >= given range

**rangeLt() - Less Than Range**
- ⏭️ should match rows where range is less than given range

**rangeLte() - Less Than or Equal Range**
- ⏭️ should match rows where range is <= given range

**rangeAdjacent() - Adjacent Range**
- ⏭️ should match rows where range is adjacent to given range

**overlaps() - Has Common Element**
- ⏭️ should match rows where array has common elements
- ⏭️ should match rows where range overlaps

**textSearch() - Full Text Search**
- ⏭️ should perform text search with AND logic
- ⏭️ should perform text search with plain type
- ⏭️ should perform text search with phrase type
- ⏭️ should perform websearch-style text search

---

### `tests/filters/logical-filters.test.ts`

**match() - Match Multiple Columns**
- ⏭️ should match rows where all columns match their values
- ✅ should work with multiple eq() calls (equivalent)

**not() - Negate Filter**
- ⏭️ should negate is null filter
- ⏭️ should negate in filter

**or() - OR Logic**
- ⏭️ should match rows satisfying any condition
- ⏭️ should combine or with and logic
- ⏭️ should apply or filter to referenced tables

**filter() - Generic Filter**
- ⏭️ should apply raw PostgREST filter
- ⏭️ should apply filter on referenced tables

**Workarounds for OR logic**
- ✅ should use multiple queries for OR logic
- ✅ should use in() for simple OR on same column

---

## Modifier Tests

### `tests/modifiers/modifiers.test.ts`

**select() - Return Data After Insert/Update/Delete**
- ✅ should return data after upsert
- ✅ should return specific columns after insert

**order() - Sort Results**
- ✅ should order results in descending order
- ✅ should order results in ascending order
- ✅ should order by string column
- ⏭️ should order on referenced table
- ⏭️ should order parent table by referenced table column

**limit() - Limit Results**
- ✅ should limit the number of returned rows
- ✅ should return all rows when limit exceeds total
- ⏭️ should limit results from referenced table

**range() - Pagination**
- ✅ should return rows in the specified range
- ✅ should work for pagination

**single() - Return Single Object**
- ✅ should return data as single object instead of array
- ✅ should error when multiple rows returned
- ✅ should error when no rows returned

**maybeSingle() - Return Zero or One Row**
- ✅ should return single object when one row matches
- ✅ should return null when no rows match
- ✅ should error when multiple rows returned

**csv() - Return as CSV**
- ⏭️ should return data as CSV string

**explain() - Query Execution Plan**
- ⏭️ should return query execution plan
- ⏭️ should return detailed execution plan

**Combined Modifiers**
- ✅ should combine order and limit
- ✅ should combine order, limit, and range for pagination

---

## Auth Tests

### `tests/auth/signup.test.ts`

**1. Sign up with email and password**
- ✅ should create a new user account
- ✅ should return user object with expected properties
- ❌ should return session when email confirmation is disabled (session not returned)
- ✅ should reject duplicate email addresses
- ✅ should reject weak passwords

**2. Sign up with phone (SMS)**
- ⏭️ should create account with phone number via SMS

**3. Sign up with phone (WhatsApp)**
- ⏭️ should create account with phone via WhatsApp

**4. Sign up with user metadata**
- ❌ should store custom user metadata during signup
- ✅ should handle empty metadata

**5. Sign up with redirect URL**
- ⏭️ should accept emailRedirectTo option

**Edge Cases**
- ✅ should handle missing email
- ✅ should handle missing password
- ✅ should handle invalid email format

---

### `tests/auth/signin-signout.test.ts`

**signInWithPassword() - Sign in with email**
- ✅ should authenticate user with valid credentials
- ✅ should return session with access token and refresh token
- ✅ should reject invalid password
- ✅ should reject non-existent user

**signInWithPassword() - Sign in with phone**
- ⏭️ should authenticate with phone number

**signOut() - Sign out all sessions**
- ✅ should sign out the current user

**signOut() - Sign out current session**
- ⏭️ should sign out only the current session

**signOut() - Sign out other sessions**
- ⏭️ should sign out all other sessions except current

**Additional Sign In Tests**
- ✅ should handle empty email
- ✅ should handle empty password
- ✅ should allow multiple sign ins (refresh session)

---

### `tests/auth/session.test.ts`

**getSession()**
- ✅ should return null when not signed in
- ❌ should return session when signed in (client-side session not persisted)
- ❌ should return session with user object

**refreshSession() - Using current session**
- ❌ should refresh the current session (session not available)
- ❌ should return new access token

**refreshSession() - Using refresh token**
- ❌ should refresh session using provided refresh token
- ✅ should reject invalid refresh token

**setSession()**
- ⏭️ should set session from tokens
- ⏭️ should reject invalid tokens

**Session Lifecycle**
- ❌ should clear session on sign out

---

### `tests/auth/user.test.ts`

**getUser() - Get user with current session**
- ❌ should retrieve the logged in user (needs session)
- ❌ should return user with all expected properties

**getUser() - Get user with custom JWT**
- ⏭️ should retrieve user using provided JWT

**getUser() - Not authenticated**
- ✅ should return null when not authenticated

**updateUser() - Update email**
- ⏭️ should update user email

**updateUser() - Update phone**
- ⏭️ should update user phone

**updateUser() - Update password**
- ❌ should update user password (session needed)
- ❌ should reject sign in with old password after update

**updateUser() - Update metadata**
- ❌ should update user metadata (session needed)
- ❌ should merge metadata with existing values

**updateUser() - Update with nonce**
- ⏭️ should update password with reauthentication nonce

**getClaims()**
- ⏭️ should retrieve JWT claims

---

### `tests/auth/auth-state-change.test.ts`

**onAuthStateChange() - Listen to auth changes**
- ✅ should provide subscription object
- ✅ should be able to unsubscribe

**onAuthStateChange() - SIGNED_OUT**
- ❌ should fire SIGNED_OUT event on sign out

**onAuthStateChange() - OAuth tokens**
- ⏭️ should receive provider tokens on OAuth sign in

**onAuthStateChange() - React context pattern**
- ❌ should work with callback-based state management

**onAuthStateChange() - PASSWORD_RECOVERY**
- ⏭️ should fire PASSWORD_RECOVERY event

**onAuthStateChange() - SIGNED_IN**
- ❌ should fire SIGNED_IN event on sign in

**onAuthStateChange() - TOKEN_REFRESHED**
- ⏭️ should fire TOKEN_REFRESHED on session refresh

**onAuthStateChange() - USER_UPDATED**
- ⏭️ should fire USER_UPDATED on user update

**Event Data**
- ❌ should include session in event callback

---

### `tests/auth/password-reset.test.ts`

**resetPasswordForEmail() - Reset password**
- ⏭️ should send password reset email
- ⏭️ should accept valid email addresses

**resetPasswordForEmail() - React flow**
- ⏭️ should complete full password reset flow

**Edge Cases**
- ⏭️ should handle non-existent email gracefully
- ⏭️ should handle invalid email format
- ⏭️ should rate limit password reset requests

---

## Email Tests

### `tests/email/mail-api.test.ts`

**GET /mail/api/emails**
- ✅ should return empty array when no emails
- ✅ should return emails after triggering email send
- ✅ should support limit parameter
- ✅ should support offset parameter for pagination
- ✅ should return emails in descending order by default (newest first)

**GET /mail/api/emails/:id**
- ✅ should return single email by ID
- ✅ should return null for non-existent ID

**DELETE /mail/api/emails/:id**
- ✅ should delete single email by ID
- ✅ should not error when deleting non-existent email

**DELETE /mail/api/emails**
- ✅ should clear all emails

**Email Content Structure**
- ✅ should include all required fields
- ✅ should have correct type for recovery email
- ✅ should contain verification URL in body

---

### `tests/email/email-flows.test.ts`

**Password Recovery (resetPasswordForEmail)**
- ✅ should send recovery email for existing user
- ✅ should include verification token in recovery email
- ✅ should send recovery email even for non-existent user (no information leak)
- ✅ should accept redirectTo option

**Magic Link (signInWithOtp)**
- ✅ should send magic link email
- ✅ should include verification token in magic link email
- ✅ should work for existing users

**User Invite (Admin Only)**
- ✅ should send invite email via service role
- ✅ should include verification token in invite email
- ✅ should reject invite without service role

**Resend Email**
- ✅ should resend confirmation email
- ✅ should resend recovery email

**Security**
- ✅ should not reveal user existence via API response
- ✅ should not send email to non-existent user for recovery

---

### `tests/email/verification.test.ts`

**Password Reset Flow**
- ✅ should complete full password reset flow
- ✅ should reject invalid recovery token
- ✅ should reject already-used recovery token

**Magic Link Flow**
- ✅ should complete full magic link sign-in flow
- ✅ should reject invalid magic link token
- ✅ should reject already-used magic link token

**Invite Flow**
- ✅ should complete full invite acceptance flow
- ✅ should reject invalid invite token

**Token Validation**
- ✅ should reject token with wrong type
- ✅ should reject empty token
- ✅ should reject missing type

---

### `tests/email/smtp.test.ts`

*Note: These tests are skipped by default. See setup instructions below.*

#### SMTP Test Setup

1. **Start Mailpit** (local SMTP test server with auth support):
   ```bash
   docker run -d --name mailpit -p 8025:8025 -p 1025:1025 axllent/mailpit --smtp-auth-accept-any --smtp-auth-allow-insecure
   ```
   Note: The `--smtp-auth-*` flags are required because sblite's SMTP client uses authentication.

2. **Start sblite in SMTP mode**:
   ```bash
   SBLITE_SMTP_HOST=localhost SBLITE_SMTP_PORT=1025 SBLITE_SMTP_USER=test SBLITE_SMTP_PASS=test ./sblite serve --mail-mode=smtp --db test.db
   ```
   Note: SMTP_USER and SMTP_PASS are required even for Mailpit (use any dummy values).

3. **Run SMTP tests**:
   ```bash
   cd e2e
   SBLITE_TEST_SMTP=true npm run test:email:smtp
   ```

4. **View emails** (optional): Open http://localhost:8025

5. **Cleanup**:
   ```bash
   docker stop mailpit && docker rm mailpit
   ```

**Password Recovery via SMTP**
- ✅ should send recovery email through SMTP

**Magic Link via SMTP**
- ✅ should send magic link email through SMTP

**User Invite via SMTP**
- ⏭️ should send invite email through SMTP (skipped - requires service_role auth fix)

**SMTP Configuration**
- ✅ should include correct sender address

---

## Known Issues Summary

### Fixed Issues (as of 2026-01-16)

The following issues have been resolved:
- ✅ `in()` filter - Now working
- ✅ `Prefer: return=representation` - Implemented for INSERT/UPDATE/DELETE
- ✅ `single()` modifier - Returns object and errors properly
- ✅ `like()`/`ilike()` wildcards - Working correctly
- ✅ Bulk INSERT - Supported via JSON arrays
- ✅ UPSERT - Implemented via `resolution=merge-duplicates` header

### Remaining Issues (Auth-Related)

These issues are related to client-side session management in tests:

1. **Session persistence** - Supabase-js client not maintaining session between operations
2. **User metadata** - Not being stored during signup (server-side fix needed)
3. **`getSession()` fails** - Depends on session persistence
4. **`refreshSession()` fails** - Same root cause
5. **Auth state change events** - Not firing for SIGNED_IN/SIGNED_OUT

---

## Documentation References

Each test maps to examples from the Supabase JavaScript documentation:

| Test File | Documentation URL |
|-----------|-------------------|
| select.test.ts | https://supabase.com/docs/reference/javascript/select |
| insert.test.ts | https://supabase.com/docs/reference/javascript/insert |
| update.test.ts | https://supabase.com/docs/reference/javascript/update |
| upsert.test.ts | https://supabase.com/docs/reference/javascript/upsert |
| delete.test.ts | https://supabase.com/docs/reference/javascript/delete |
| basic-filters.test.ts | https://supabase.com/docs/reference/javascript/using-filters |
| advanced-filters.test.ts | https://supabase.com/docs/reference/javascript/contains |
| logical-filters.test.ts | https://supabase.com/docs/reference/javascript/or |
| modifiers.test.ts | https://supabase.com/docs/reference/javascript/using-modifiers |
| signup.test.ts | https://supabase.com/docs/reference/javascript/auth-signup |
| signin-signout.test.ts | https://supabase.com/docs/reference/javascript/auth-signinwithpassword |
| session.test.ts | https://supabase.com/docs/reference/javascript/auth-getsession |
| user.test.ts | https://supabase.com/docs/reference/javascript/auth-getuser |
| auth-state-change.test.ts | https://supabase.com/docs/reference/javascript/auth-onauthstatechange |
| password-reset.test.ts | https://supabase.com/docs/reference/javascript/auth-resetpasswordforemail |
| mail-api.test.ts | (sblite-specific mail viewer API) |
| email-flows.test.ts | https://supabase.com/docs/reference/javascript/auth-resetpasswordforemail |
| verification.test.ts | https://supabase.com/docs/reference/javascript/auth-verifyotp |
| smtp.test.ts | (sblite-specific SMTP mode testing) |
