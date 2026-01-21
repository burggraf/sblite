-- User roles table (only admins have entries, no entry = customer)
CREATE TABLE user_roles (
    user_id TEXT PRIMARY KEY,
    role TEXT NOT NULL DEFAULT 'admin',
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Register user_roles columns with types
INSERT INTO _columns (table_name, column_name, pg_type) VALUES
    ('user_roles', 'user_id', 'uuid'),
    ('user_roles', 'role', 'text'),
    ('user_roles', 'created_at', 'timestamptz');

-- Enable RLS on user_roles
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('user_roles', 1);

-- SELECT: Users can read their own role, admins can read all
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('user_roles', 'user_roles_select', 'SELECT',
     'user_id = auth.uid() OR EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- INSERT: Users can insert their own role entry
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('user_roles', 'user_roles_insert', 'INSERT', 'user_id = auth.uid()');
