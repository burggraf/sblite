-- ShopLite E-commerce Schema

-- Products table (public catalog)
CREATE TABLE products (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price TEXT NOT NULL,
    image_url TEXT,
    stock INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Register product columns with types
INSERT INTO _columns (table_name, column_name, pg_type) VALUES
    ('products', 'id', 'uuid'),
    ('products', 'name', 'text'),
    ('products', 'description', 'text'),
    ('products', 'price', 'numeric'),
    ('products', 'image_url', 'text'),
    ('products', 'stock', 'integer'),
    ('products', 'created_at', 'timestamptz');

-- Cart items table (user-owned)
CREATE TABLE cart_items (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    product_id TEXT NOT NULL REFERENCES products(id),
    quantity INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now')),
    UNIQUE(user_id, product_id)
);

-- Register cart_items columns with types
INSERT INTO _columns (table_name, column_name, pg_type) VALUES
    ('cart_items', 'id', 'uuid'),
    ('cart_items', 'user_id', 'uuid'),
    ('cart_items', 'product_id', 'uuid'),
    ('cart_items', 'quantity', 'integer'),
    ('cart_items', 'created_at', 'timestamptz');

-- Orders table (user-owned)
CREATE TABLE orders (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    total TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ', 'now'))
);

-- Register orders columns with types
INSERT INTO _columns (table_name, column_name, pg_type) VALUES
    ('orders', 'id', 'uuid'),
    ('orders', 'user_id', 'uuid'),
    ('orders', 'status', 'text'),
    ('orders', 'total', 'numeric'),
    ('orders', 'created_at', 'timestamptz');

-- Order items table
CREATE TABLE order_items (
    id TEXT PRIMARY KEY,
    order_id TEXT NOT NULL REFERENCES orders(id),
    product_id TEXT NOT NULL REFERENCES products(id),
    quantity INTEGER NOT NULL,
    price_at_purchase TEXT NOT NULL
);

-- Register order_items columns with types
INSERT INTO _columns (table_name, column_name, pg_type) VALUES
    ('order_items', 'id', 'uuid'),
    ('order_items', 'order_id', 'uuid'),
    ('order_items', 'product_id', 'uuid'),
    ('order_items', 'quantity', 'integer'),
    ('order_items', 'price_at_purchase', 'numeric');

-- Create indexes for performance
CREATE INDEX idx_cart_items_user_id ON cart_items(user_id);
CREATE INDEX idx_orders_user_id ON orders(user_id);
CREATE INDEX idx_order_items_order_id ON order_items(order_id);
