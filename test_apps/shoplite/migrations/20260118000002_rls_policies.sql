-- ShopLite RLS Policies

-- Enable RLS on user-owned tables
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('cart_items', 1);
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('orders', 1);
INSERT OR REPLACE INTO _rls_tables (table_name, enabled) VALUES ('order_items', 1);

-- Products: public read for everyone (no RLS needed, but add policy for clarity)
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('products', 'products_public_read', 'SELECT', '1=1');

-- Cart items: users can only access their own cart
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('cart_items', 'cart_items_user_select', 'SELECT', 'user_id = auth.uid()');
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('cart_items', 'cart_items_user_insert', 'INSERT', 'user_id = auth.uid()');
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr, check_expr) VALUES
    ('cart_items', 'cart_items_user_update', 'UPDATE', 'user_id = auth.uid()', 'user_id = auth.uid()');
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('cart_items', 'cart_items_user_delete', 'DELETE', 'user_id = auth.uid()');

-- Orders: users can only access their own orders
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('orders', 'orders_user_select', 'SELECT', 'user_id = auth.uid()');
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('orders', 'orders_user_insert', 'INSERT', 'user_id = auth.uid()');

-- Order items: users can access items from their own orders
INSERT INTO _rls_policies (table_name, policy_name, command, using_expr) VALUES
    ('order_items', 'order_items_user_select', 'SELECT', 'order_id IN (SELECT id FROM orders WHERE user_id = auth.uid())');
INSERT INTO _rls_policies (table_name, policy_name, command, check_expr) VALUES
    ('order_items', 'order_items_user_insert', 'INSERT', 'order_id IN (SELECT id FROM orders WHERE user_id = auth.uid())');
