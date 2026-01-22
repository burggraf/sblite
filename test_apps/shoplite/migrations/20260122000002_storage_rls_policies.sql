-- Storage RLS policies for product-images bucket
-- Allow anyone to read from public buckets (SELECT)
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('storage_objects', 'storage_public_read', 'SELECT', 'bucket_id IN (SELECT id FROM storage_buckets WHERE public = 1)');

-- Admin can upload to product-images bucket (INSERT)
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('storage_objects', 'storage_admin_insert', 'INSERT', 'bucket_id = ''product-images'' AND EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- Admin can update objects in product-images bucket (UPDATE)
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('storage_objects', 'storage_admin_update', 'UPDATE', 'bucket_id = ''product-images'' AND EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')', 'bucket_id = ''product-images'' AND EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');

-- Admin can delete from product-images bucket (DELETE)
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('storage_objects', 'storage_admin_delete', 'DELETE', 'bucket_id = ''product-images'' AND EXISTS (SELECT 1 FROM user_roles WHERE user_id = auth.uid() AND role = ''admin'')');
