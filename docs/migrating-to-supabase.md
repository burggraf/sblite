# Migrating to Supabase

This guide explains how to migrate your sblite project to full Supabase when you're ready to scale. sblite provides both automated one-click migration and manual export options.

## Overview

### Why Migrate?

sblite is designed as a development and prototyping environment that's 100% compatible with Supabase. When your project grows beyond sblite's capabilities, you can migrate to full Supabase with minimal friction:

- **More resources** - Dedicated PostgreSQL, unlimited storage, global CDN
- **Advanced features** - Database branching, read replicas, point-in-time recovery
- **Team collaboration** - Multiple users, role-based access, audit logs
- **Production support** - SLAs, priority support, enterprise compliance

### What Gets Migrated

| Component | Automated | Manual Export |
|-----------|-----------|---------------|
| Database schema | Yes | `.sql` file |
| Table data | Yes | `.json` or `.csv` per table |
| Auth users | Yes | `.json` file |
| OAuth identities | Yes | Included with users |
| RLS policies | Yes | `.sql` file |
| Storage buckets | Yes | `.json` config |
| Storage files | Yes | `.zip` per bucket |
| Edge functions | Yes | `.zip` with config |
| Secrets | Yes (values transferred) | Names only (`.txt`) |
| Auth configuration | Yes | `.json` file |
| OAuth provider config | Yes | `.json` (secrets redacted) |
| Email templates | Yes | `.json` file |

### Migration Paths

**Automated Migration** - Recommended for most users. Connect your Supabase account and migrate everything with a guided wizard. Includes verification and rollback capabilities.

**Manual Migration** - For users who prefer full control or need to migrate to a self-hosted Supabase instance. Download export packages and import them yourself.

## Prerequisites

### Supabase Account and Project

1. Create a Supabase account at [supabase.com](https://supabase.com)
2. Create a new project or select an existing empty project
3. Note your project's reference ID (visible in the URL: `https://supabase.com/dashboard/project/{ref}`)

### Management API Token (Automated Migration)

For automated migration, you need a Supabase Management API personal access token:

1. Go to [supabase.com/dashboard/account/tokens](https://supabase.com/dashboard/account/tokens)
2. Click **Generate new token**
3. Give it a descriptive name (e.g., "sblite migration")
4. Copy the token immediately - it won't be shown again
5. Store it securely - this token has full access to your account

**Token permissions**: The Management API token can create/delete projects, deploy functions, manage secrets, and modify all project settings. Keep it secure and delete it after migration.

### Database Password (Manual Migration)

For manual data import, you need your Supabase database password:

1. Go to your project's **Settings** > **Database**
2. Under **Connection string**, you'll find the password (or reset it if needed)
3. Use this with `psql` or other PostgreSQL clients

## Automated Migration

The automated migration wizard handles the entire process through the sblite dashboard.

### Accessing the Migration Wizard

1. Open the sblite dashboard at `http://localhost:8080/_`
2. Navigate to **Export & Migration** in the sidebar
3. Select the **Migrate** tab

### Step 1: Connect to Supabase

1. Enter your Management API token
2. Click **Connect**
3. sblite validates the token and fetches your projects

If connection fails:
- Verify the token was copied correctly
- Check that the token hasn't expired
- Ensure you have network connectivity to `api.supabase.com`

### Step 2: Select Target Project

Choose where to migrate:

**Use existing project:**
- Select a project from the dropdown
- Warning: Existing data may conflict with migration

**Create new project:**
- Enter a project name
- Select a region (choose closest to your users)
- Wait for project provisioning (1-2 minutes)

### Step 3: Select Items to Migrate

Review the checklist of all items that can be migrated:

- **Database schema** - All user tables with types preserved
- **Database data** - Row data for each table (individually selectable)
- **Auth users** - All user accounts with password hashes
- **RLS policies** - Row Level Security rules
- **Storage buckets** - Bucket configurations
- **Storage files** - All files in each bucket
- **Edge functions** - Function code and configuration
- **Secrets** - Environment variables for functions
- **Auth configuration** - JWT settings, signup rules
- **OAuth provider config** - Google, GitHub settings
- **Email templates** - Custom email templates

Click items to toggle selection. Dependencies are handled automatically (e.g., selecting "Storage files" automatically selects "Storage buckets").

### Step 4: Review and Confirm

The review screen shows:

- Summary of selected items
- Estimated migration time
- Warnings about potential issues:
  - Tables with the same name already exist
  - Functions that may conflict
  - OAuth providers requiring manual setup

Click **Start Migration** to begin.

### Step 5: Monitor Progress

The progress view shows real-time status for each item:

| Status | Meaning |
|--------|---------|
| Pending | Waiting to start |
| In Progress | Currently migrating |
| Completed | Successfully migrated |
| Failed | Error occurred (click for details) |
| Skipped | Intentionally not migrated |

**If an item fails:**
1. Click the item to view the error details
2. Note the suggested fix
3. After the migration completes, you can retry failed items

The migration continues even if individual items fail. You can retry failed items afterward.

### Step 6: Verify Migration

After migration completes, run verification checks:

**Basic Checks** (automatic):
- Tables exist with correct columns
- Functions are deployed and callable
- Storage buckets created with correct settings
- RLS enabled on correct tables
- Secrets exist (names verified)

**Data Integrity** (click to run):
- Row counts match per table
- Sample data comparison
- Foreign key validation
- Storage file counts

**Functional Tests** (click to run):
- Execute test queries
- Upload/download test files
- Invoke functions with test data

Green checkmarks indicate successful verification. Click any failed check for details.

### Rollback

If migration didn't go as expected, you can roll back:

1. Go to the **History** tab
2. Find your migration session
3. Click **Rollback**

**Rollback actions:**
- Drops migrated tables
- Deletes deployed functions
- Removes storage buckets (empties first)
- Deletes created secrets
- Removes migrated users

**Rollback limitations:**
- Data created after migration is not preserved
- Users who signed in after migration may have new sessions
- Files uploaded after migration remain
- Rollback is best-effort, not transactional

## Manual Migration

For full control over the migration process, use manual exports.

### Accessing Exports

1. Open the sblite dashboard at `http://localhost:8080/_`
2. Navigate to **Export & Migration**
3. Select the **Export** tab

### Export Types

#### Schema Export (`.sql`)

PostgreSQL-compatible DDL for all tables:

```bash
# Download via dashboard or CLI
./sblite migrate export --db data.db -o schema.sql
```

**Import to Supabase:**
1. Go to **SQL Editor** in Supabase dashboard
2. Paste the schema SQL
3. Click **Run**

Or via `psql`:
```bash
psql "postgresql://postgres:[PASSWORD]@db.[PROJECT-REF].supabase.co:5432/postgres" -f schema.sql
```

#### Data Export (`.json` or `.csv`)

Per-table data exports:

1. Click **Data** in the Export tab
2. Select tables to export
3. Choose format (JSON or CSV)
4. Download

**Import to Supabase:**

Using the SQL Editor with JSON:
```sql
-- For each table
INSERT INTO my_table
SELECT * FROM json_populate_recordset(null::my_table,
  '[{"id": 1, "name": "test"}, ...]'::json
);
```

Using CSV import:
1. Go to **Table Editor** in Supabase
2. Select your table
3. Click **Import data from CSV**
4. Upload the CSV file

#### RLS Policies Export (`.sql`)

Row Level Security policies:

```sql
-- Example exported policy
CREATE POLICY "Users can view own data" ON public.profiles
FOR SELECT USING (auth.uid() = user_id);
```

**Import:** Run in Supabase SQL Editor after schema and data import.

#### Auth Users Export (`.json`)

User accounts with metadata:

```json
{
  "users": [
    {
      "id": "uuid",
      "email": "user@example.com",
      "encrypted_password": "$2a$...",
      "email_confirmed_at": "2025-01-20T...",
      "user_metadata": {},
      "app_metadata": {}
    }
  ]
}
```

**Import:**
1. Use Supabase Admin API or direct database insert
2. Note: Password hashes are bcrypt and transfer directly

```sql
INSERT INTO auth.users (id, email, encrypted_password, ...)
VALUES ('uuid', 'user@example.com', '$2a$...', ...);
```

#### Storage Files Export (`.zip`)

Per-bucket ZIP archives with folder structure preserved:

```
bucket-name.zip
├── folder/
│   ├── file1.txt
│   └── file2.txt
└── root-file.txt
```

**Import:**
1. Create the bucket in Supabase Storage (or via API)
2. Extract the ZIP locally
3. Upload files using Supabase dashboard, CLI, or API

Via CLI:
```bash
# Install Supabase CLI
npm install -g supabase

# Upload files
supabase storage cp -r ./extracted-bucket/* storage://bucket-name/
```

#### Edge Functions Export (`.zip`)

Function source code with configuration:

```
functions.zip
├── hello-world/
│   └── index.ts
├── api-handler/
│   └── index.ts
└── _metadata.json
```

**Import:**
```bash
# Copy to your Supabase project
cp -r functions/* ~/my-supabase-project/supabase/functions/

# Deploy
cd ~/my-supabase-project
supabase functions deploy
```

#### Secrets Export (`.txt`)

**Security note:** Only secret names are exported, never values. You must re-enter secret values in Supabase.

```
API_KEY
DATABASE_URL
STRIPE_SECRET
```

**Import:**
```bash
# Set each secret
supabase secrets set API_KEY=your-actual-value
supabase secrets set DATABASE_URL=your-actual-value
```

Or via Supabase dashboard: **Project Settings** > **Edge Functions** > **Secrets**

#### Auth Configuration Export (`.json`)

JWT settings, signup configuration, SMTP settings:

```json
{
  "jwt_expiry": 3600,
  "signup_enabled": true,
  "email_confirmation_required": true,
  "smtp_host": "smtp.example.com",
  "smtp_port": 587
}
```

**Import:** Configure manually in Supabase dashboard under **Authentication** > **Settings**

#### Email Templates Export (`.json`)

Custom email templates:

```json
{
  "confirmation": {
    "subject": "Confirm your email",
    "content_html": "<h1>Welcome!</h1>...",
    "content_text": "Welcome!..."
  }
}
```

**Import:** Copy content to Supabase **Authentication** > **Email Templates**

## Manual Setup Steps

Some items cannot be fully automated and require manual configuration in external services.

### OAuth Provider Apps

OAuth requires creating new app credentials in each provider's console because redirect URLs change.

#### Google OAuth

1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Select or create a project
3. Go to **Credentials** > **Create Credentials** > **OAuth Client ID**
4. Application type: **Web application**
5. Add authorized redirect URI:
   ```
   https://<your-project-ref>.supabase.co/auth/v1/callback
   ```
6. Copy **Client ID** and **Client Secret**
7. In Supabase dashboard: **Authentication** > **Providers** > **Google**
8. Enter Client ID and Client Secret
9. Enable the provider

#### GitHub OAuth

1. Go to [GitHub Developer Settings](https://github.com/settings/developers)
2. Click **New OAuth App**
3. Set **Authorization callback URL**:
   ```
   https://<your-project-ref>.supabase.co/auth/v1/callback
   ```
4. Register the application
5. Generate a new client secret
6. Copy **Client ID** and **Client Secret**
7. In Supabase dashboard: **Authentication** > **Providers** > **GitHub**
8. Enter Client ID and Client Secret
9. Enable the provider

### Custom Domain DNS

If you were using a custom domain with sblite:

1. Go to Supabase **Project Settings** > **Custom Domains**
2. Add your domain
3. Update DNS records:
   - Add CNAME record pointing to Supabase
   - Follow the verification steps shown

### SMTP Configuration

If using external SMTP (not the default Supabase email):

1. Go to **Project Settings** > **Auth** > **SMTP Settings**
2. Enable custom SMTP
3. Enter your SMTP credentials:
   - Host
   - Port
   - Username
   - Password
4. Save and test with a verification email

## Troubleshooting

### Connection Errors

**"Invalid API token"**
- Verify the token was copied completely
- Check token hasn't been revoked
- Generate a new token if needed

**"Network error"**
- Check internet connectivity
- Verify firewall allows HTTPS to `api.supabase.com`
- Try again in a few minutes

**"Project not found"**
- Verify project reference ID
- Ensure the token has access to this project

### Schema Conflicts

**"Table already exists"**
- The target project has existing tables
- Options:
  - Use a fresh project
  - Manually drop conflicting tables
  - Skip schema migration and import data only

**"Column type mismatch"**
- Type mappings differ between SQLite and PostgreSQL
- Review the schema export and adjust if needed
- Common fixes:
  - `TEXT` to `text` (usually automatic)
  - `INTEGER` to `integer` or `bigint`
  - `REAL` to `numeric` or `double precision`

### Data Import Issues

**"Foreign key constraint violation"**
- Import tables in dependency order
- Temporarily disable constraints:
  ```sql
  SET session_replication_role = 'replica';
  -- import data
  SET session_replication_role = 'origin';
  ```

**"Duplicate key"**
- Data already exists in target
- Use `ON CONFLICT` clause or truncate target table

**"Value too long"**
- PostgreSQL column has smaller limit than SQLite
- Alter column or truncate data

### Function Deployment Failures

**"Function already exists"**
- Delete existing function in Supabase first
- Or use a different function name

**"Import error in function"**
- Check that all Deno imports are valid
- Update import URLs if using old versions

### Storage Issues

**"Bucket already exists"**
- Delete or rename existing bucket
- Or skip bucket creation and upload to existing

**"File upload failed"**
- Check file size limits
- Verify MIME type is allowed
- Ensure storage quota not exceeded

## Post-Migration Checklist

After migration, verify everything works correctly:

### Update Application URLs

Replace sblite URLs with Supabase URLs in your application:

```javascript
// Before
const supabase = createClient('http://localhost:8080', 'your-anon-key')

// After
const supabase = createClient(
  'https://<project-ref>.supabase.co',
  '<supabase-anon-key>'
)
```

### Verify API Keys

1. Go to Supabase **Project Settings** > **API**
2. Copy the new `anon` and `service_role` keys
3. Update all applications using these keys

### Test Authentication

1. **Sign up** - Create a new test user
2. **Sign in** - Log in with existing user (password hashes transferred)
3. **OAuth** - Test each configured provider
4. **Password reset** - Verify email delivery

### Validate Data

1. **Row counts** - Compare counts between sblite and Supabase
2. **Sample queries** - Run typical application queries
3. **RLS policies** - Test access control with different user roles

### Test Storage

1. **Upload** - Upload a new file
2. **Download** - Download an existing file
3. **Public access** - Verify public bucket URLs work
4. **RLS** - Test storage policies

### Test Functions

1. **Invoke** - Call each edge function
2. **Secrets** - Verify functions can access secrets
3. **Database access** - Test functions that query the database

### Remove/Archive sblite Instance

Once migration is verified:

1. **Backup** - Keep a final backup of `data.db`
2. **Stop server** - Stop the sblite process
3. **Archive** - Move database file to archive storage
4. **Update documentation** - Update any internal docs pointing to sblite

### DNS and Domain Updates

If applicable:

1. Update DNS records to point to Supabase
2. Update SSL certificates if using custom domain
3. Allow DNS propagation time (up to 48 hours)

---

## Summary

| Migration Type | Best For | Time Required |
|---------------|----------|---------------|
| Automated | Most users, full projects | 5-30 minutes |
| Manual | Custom requirements, self-hosted | 1-4 hours |

The automated migration handles most scenarios. Use manual migration when you need precise control over each step or are migrating to a self-hosted Supabase instance.

For issues not covered here, check the [sblite GitHub repository](https://github.com/your-repo/sblite) or open an issue.
