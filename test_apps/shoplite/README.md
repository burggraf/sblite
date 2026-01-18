# ShopLite - E-commerce Reference App

A minimal e-commerce application built to test sblite's core functionality using `@supabase/supabase-js`.

## Features Tested

- **Authentication:** signup, signin, signout, session management
- **REST API:** select, insert, update, delete with relationships
- **Filters:** eq, in, order
- **RLS:** user isolation for cart and orders

## Quick Start

### 1. Start sblite server

```bash
# From sblite root directory
./sblite init --db shoplite.db
./sblite serve --db shoplite.db
```

### 2. Apply migrations and seed data

```bash
# Apply schema migrations
./sblite db push --db shoplite.db --migrations-dir test_apps/shoplite/migrations

# Seed products
sqlite3 shoplite.db < test_apps/shoplite/seed.sql
```

### 3. Get API keys

Visit `http://localhost:8080/_` to access the dashboard (first-time setup required).
Go to **Settings** and expand the **API Keys** section to get your `anon` key.

### 4. Configure environment

```bash
cd test_apps/shoplite
cp .env.example .env
# Edit .env with your API keys
```

### 5. Install dependencies and run

```bash
npm install
npm run dev
```

Visit `http://localhost:3000` to use the app.

## Running Tests

```bash
# Install Playwright browsers (first time only)
npx playwright install chromium

# Run all tests
npm test

# Run tests with UI
npm run test:ui
```

## Test Suites

| Suite | Tests | Description |
|-------|-------|-------------|
| auth.spec.js | 6 | Registration, login, logout, session persistence |
| cart.spec.js | 5 | Add to cart, update quantity, remove, RLS isolation |
| checkout.spec.js | 5 | Order creation, cart clearing, totals validation |
| orders.spec.js | 5 | View orders, RLS isolation, order details |

## Project Structure

```
shoplite/
├── src/
│   ├── main.jsx          # Entry point
│   ├── App.jsx           # Router, auth context
│   ├── index.css         # Styles
│   ├── lib/
│   │   └── supabase.js   # Supabase client
│   ├── components/
│   │   ├── Navbar.jsx
│   │   ├── ProductCard.jsx
│   │   └── CartItem.jsx
│   └── pages/
│       ├── Home.jsx      # Product listing
│       ├── Cart.jsx      # Shopping cart
│       ├── Checkout.jsx  # Order placement
│       ├── Orders.jsx    # Order history
│       ├── Login.jsx
│       └── Register.jsx
├── migrations/           # SQL schema files
├── seed.sql              # Sample products
├── e2e/
│   ├── helpers.js        # Test utilities
│   └── tests/            # Playwright tests
├── package.json
├── vite.config.js
└── playwright.config.js
```

## Database Schema

### products
Public catalog (no RLS)

### cart_items
User's shopping cart items. RLS: users can only access their own cart.

### orders
User's orders. RLS: users can only access their own orders.

### order_items
Items within an order. RLS: users can access items from their own orders.
