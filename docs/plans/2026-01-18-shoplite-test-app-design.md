# ShopLite - E-commerce Reference Test App

## Overview

ShopLite is a minimal e-commerce application designed to test sblite's core functionality through realistic usage patterns. Built with React + Vite, tested with Playwright.

## Data Model

### Tables

**products**
- `id` (uuid, PK)
- `name` (text)
- `description` (text)
- `price` (numeric)
- `image_url` (text)
- `stock` (integer)
- `created_at` (timestamptz)

**cart_items**
- `id` (uuid, PK)
- `user_id` (uuid, FK → auth_users)
- `product_id` (uuid, FK → products)
- `quantity` (integer)
- `created_at` (timestamptz)

**orders**
- `id` (uuid, PK)
- `user_id` (uuid, FK → auth_users)
- `status` (text) - pending, confirmed, shipped, delivered
- `total` (numeric)
- `created_at` (timestamptz)

**order_items**
- `id` (uuid, PK)
- `order_id` (uuid, FK → orders)
- `product_id` (uuid, FK → products)
- `quantity` (integer)
- `price_at_purchase` (numeric)

### Relationships

- cart_items → products (many-to-one)
- orders → order_items (one-to-many)
- order_items → products (many-to-one)

### RLS Policies

- **products**: Public read (anon key), no write
- **cart_items**: Users can only CRUD their own items (`user_id = auth.uid()`)
- **orders**: Users can only read/insert their own orders
- **order_items**: Users can only read items from their own orders

## User Flows

### Authentication
1. Sign up with email/password
2. Sign in
3. Sign out
4. Session auto-refresh

### Shopping
1. Browse products (public, no auth required)
2. Add to cart (requires auth)
3. View/edit cart
4. Checkout (creates order, clears cart)
5. View order history

## Pages

| Page | Route | Auth Required | Description |
|------|-------|---------------|-------------|
| Home | `/` | No | Product grid with add-to-cart |
| Cart | `/cart` | Yes | Cart items, quantities, checkout |
| Checkout | `/checkout` | Yes | Order review and placement |
| Orders | `/orders` | Yes | Order history list |
| Login | `/login` | No | Sign in form |
| Register | `/register` | No | Sign up form |

## Project Structure

```
test_apps/
└── shoplite/
    ├── src/
    │   ├── main.jsx
    │   ├── App.jsx
    │   ├── lib/
    │   │   └── supabase.js
    │   ├── components/
    │   │   ├── Navbar.jsx
    │   │   ├── ProductCard.jsx
    │   │   └── CartItem.jsx
    │   └── pages/
    │       ├── Home.jsx
    │       ├── Cart.jsx
    │       ├── Checkout.jsx
    │       ├── Orders.jsx
    │       ├── Login.jsx
    │       └── Register.jsx
    ├── migrations/
    │   └── 001_schema.sql
    ├── seed.sql
    ├── e2e/
    │   ├── playwright.config.js
    │   └── tests/
    │       ├── auth.spec.js
    │       ├── cart.spec.js
    │       ├── checkout.spec.js
    │       └── orders.spec.js
    ├── package.json
    ├── vite.config.js
    └── README.md
```

## E2E Test Suites

### auth.spec.js (~5 tests)
- Register new user
- Login with valid credentials
- Login with invalid credentials
- Logout clears session
- Session persists on reload

### cart.spec.js (~5 tests)
- Add product to cart
- Update cart item quantity
- Remove item from cart
- Cart persists across sessions
- Cart is user-isolated (RLS)

### checkout.spec.js (~4 tests)
- Checkout creates order
- Cart cleared after checkout
- Order total matches cart
- Order items match cart items

### orders.spec.js (~4 tests)
- View own orders
- Order shows correct items
- Cannot see other user's orders (RLS)
- Order status displayed correctly

## sblite Features Tested

### Auth API
- `auth.signUp()`
- `auth.signInWithPassword()`
- `auth.signOut()`
- `auth.getSession()`
- `auth.onAuthStateChange()`

### REST API
- `from().select()` with relationships
- `from().insert()`
- `from().update()`
- `from().delete()`
- Filters: eq, in
- Modifiers: order, limit, single

### RLS
- User isolation policies
- Policy enforcement on all operations

## Setup & Running

```bash
# 1. Start sblite
./sblite serve --db shoplite.db

# 2. Apply migrations
./sblite db push --db shoplite.db --migrations-dir test_apps/shoplite/migrations

# 3. Seed data
sqlite3 shoplite.db < test_apps/shoplite/seed.sql

# 4. Start dev server
cd test_apps/shoplite && npm run dev

# 5. Run tests
cd test_apps/shoplite && npx playwright test
```

## Dependencies

- `@supabase/supabase-js` - Supabase client
- `react-router-dom` - Client routing
- `@playwright/test` - E2E testing
