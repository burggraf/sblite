-- Admin product management policies
-- Enable RLS on products table
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('products', 1);

-- Admin can INSERT products
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('products', 'products_admin_insert', 'INSERT', 'EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- Admin can UPDATE products
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('products', 'products_admin_update', 'UPDATE', 'EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')', 'EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- Admin can DELETE products
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('products', 'products_admin_delete', 'DELETE', 'EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');
