# ShopLite Admin Product Management Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable admin users to add, edit, and delete products from the Home page.

**Architecture:** Inline admin controls on existing Home page. Modal forms for add/edit. Confirmation dialog for delete. RLS policies enforce admin-only write access at database level.

**Tech Stack:** React, @supabase/supabase-js, vanilla CSS (existing style system)

---

### Task 1: Add Admin Write Policies for Products

**Files:**
- Create: `test_apps/shoplite/migrations/20260121000002_products_admin_policies.sql`

**Step 1: Create migration file**

```sql
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
```

**Step 2: Apply migration**

Run: `cd /Users/markb/dev/sblite && ./sblite db push --db test_apps/shoplite/shoplite.db --migrations-dir test_apps/shoplite/migrations`

Expected: Migration applied successfully

**Step 3: Commit**

```bash
git add test_apps/shoplite/migrations/20260121000002_products_admin_policies.sql
git commit -m "feat(shoplite): add admin RLS policies for products"
```

---

### Task 2: Create ProductFormModal Component

**Files:**
- Create: `test_apps/shoplite/src/components/ProductFormModal.jsx`

**Step 1: Create the component**

```jsx
import { useState, useEffect } from 'react'

function ProductFormModal({ isOpen, onClose, product, onSave }) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [price, setPrice] = useState('')
  const [imageUrl, setImageUrl] = useState('')
  const [stock, setStock] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const isEditing = !!product

  useEffect(() => {
    if (product) {
      setName(product.name || '')
      setDescription(product.description || '')
      setPrice(product.price || '')
      setImageUrl(product.image_url || '')
      setStock(product.stock?.toString() || '0')
    } else {
      setName('')
      setDescription('')
      setPrice('')
      setImageUrl('')
      setStock('0')
    }
    setError('')
  }, [product, isOpen])

  if (!isOpen) return null

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')

    // Validation
    if (!name.trim()) {
      setError('Name is required')
      return
    }
    if (!price || parseFloat(price) < 0) {
      setError('Price must be a valid positive number')
      return
    }
    if (stock === '' || parseInt(stock) < 0) {
      setError('Stock must be a non-negative integer')
      return
    }

    setSaving(true)
    try {
      await onSave({
        id: product?.id,
        name: name.trim(),
        description: description.trim(),
        price: price,
        image_url: imageUrl.trim() || 'https://placehold.co/400x300?text=Product',
        stock: parseInt(stock)
      })
      onClose()
    } catch (err) {
      setError(err.message || 'Failed to save product')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal product-form-modal" onClick={e => e.stopPropagation()}>
        <h2 className="modal-title">{isEditing ? 'Edit Product' : 'Add Product'}</h2>

        {error && <div className="alert alert-error">{error}</div>}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label" htmlFor="name">Name *</label>
            <input
              id="name"
              type="text"
              className="form-input"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Product name"
              autoFocus
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="description">Description</label>
            <textarea
              id="description"
              className="form-input form-textarea"
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="Product description"
              rows={3}
            />
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="form-label" htmlFor="price">Price *</label>
              <input
                id="price"
                type="number"
                step="0.01"
                min="0"
                className="form-input"
                value={price}
                onChange={e => setPrice(e.target.value)}
                placeholder="0.00"
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="stock">Stock *</label>
              <input
                id="stock"
                type="number"
                step="1"
                min="0"
                className="form-input"
                value={stock}
                onChange={e => setStock(e.target.value)}
                placeholder="0"
              />
            </div>
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="imageUrl">Image URL</label>
            <input
              id="imageUrl"
              type="text"
              className="form-input"
              value={imageUrl}
              onChange={e => setImageUrl(e.target.value)}
              placeholder="https://example.com/image.jpg"
            />
          </div>

          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose} disabled={saving}>
              Cancel
            </button>
            <button type="submit" className="btn btn-primary" disabled={saving}>
              {saving ? 'Saving...' : isEditing ? 'Save Changes' : 'Add Product'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

export default ProductFormModal
```

**Step 2: Commit**

```bash
git add test_apps/shoplite/src/components/ProductFormModal.jsx
git commit -m "feat(shoplite): add ProductFormModal component"
```

---

### Task 3: Create DeleteConfirmModal Component

**Files:**
- Create: `test_apps/shoplite/src/components/DeleteConfirmModal.jsx`

**Step 1: Create the component**

```jsx
import { useState } from 'react'

function DeleteConfirmModal({ isOpen, onClose, product, onConfirm }) {
  const [deleting, setDeleting] = useState(false)

  if (!isOpen || !product) return null

  async function handleDelete() {
    setDeleting(true)
    try {
      await onConfirm(product.id)
      onClose()
    } catch (err) {
      console.error('Delete failed:', err)
      setDeleting(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal delete-confirm-modal" onClick={e => e.stopPropagation()}>
        <h2 className="modal-title">Delete Product</h2>
        <div className="modal-body">
          <p>Are you sure you want to delete <strong>{product.name}</strong>?</p>
          <p className="text-muted">This action cannot be undone.</p>
        </div>
        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onClose} disabled={deleting}>
            Cancel
          </button>
          <button className="btn btn-danger" onClick={handleDelete} disabled={deleting}>
            {deleting ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default DeleteConfirmModal
```

**Step 2: Commit**

```bash
git add test_apps/shoplite/src/components/DeleteConfirmModal.jsx
git commit -m "feat(shoplite): add DeleteConfirmModal component"
```

---

### Task 4: Add Modal Styles to CSS

**Files:**
- Modify: `test_apps/shoplite/src/index.css`

**Step 1: Add styles at end of file**

Add after the existing `.modal-footer` styles (around line 589):

```css

/* Product Form Modal */
.product-form-modal {
  max-width: 500px;
}

.product-form-modal .modal-title {
  text-align: left;
  margin-bottom: 1.5rem;
}

.form-textarea {
  resize: vertical;
  min-height: 80px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 1.5rem;
}

/* Delete Confirm Modal */
.delete-confirm-modal {
  max-width: 400px;
}

.delete-confirm-modal .modal-title {
  text-align: left;
  margin-bottom: 1rem;
}

.delete-confirm-modal .modal-body {
  text-align: left;
  margin-bottom: 0;
}

.text-muted {
  color: var(--text-light);
  font-size: 0.875rem;
  margin-top: 0.5rem;
}

/* Admin Controls on Product Card */
.admin-controls {
  display: flex;
  gap: 0.5rem;
  margin-top: 0.5rem;
}

.admin-controls .btn {
  flex: 1;
}

/* Page Header with Actions */
.page-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 2rem 0 1rem;
}

.page-header .page-title {
  margin: 0;
}
```

**Step 2: Commit**

```bash
git add test_apps/shoplite/src/index.css
git commit -m "feat(shoplite): add admin modal and control styles"
```

---

### Task 5: Update ProductCard for Admin Controls

**Files:**
- Modify: `test_apps/shoplite/src/components/ProductCard.jsx`

**Step 1: Update the component**

Replace the entire file with:

```jsx
import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { supabase } from '../lib/supabase'
import { useAuth, useCart } from '../App'

function ProductCard({ product, isAdmin, onEdit, onDelete }) {
  const { user } = useAuth()
  const { refreshCart } = useCart()
  const navigate = useNavigate()
  const [adding, setAdding] = useState(false)
  const [added, setAdded] = useState(false)

  async function handleAddToCart() {
    if (!user) {
      navigate('/login')
      return
    }

    setAdding(true)
    try {
      // Check if item already in cart
      const { data: existing } = await supabase
        .from('cart_items')
        .select('id, quantity')
        .eq('product_id', product.id)
        .maybeSingle()

      if (existing) {
        // Update quantity
        await supabase
          .from('cart_items')
          .update({ quantity: existing.quantity + 1 })
          .eq('id', existing.id)
      } else {
        // Insert new cart item
        await supabase.from('cart_items').insert({
          id: crypto.randomUUID(),
          user_id: user.id,
          product_id: product.id,
          quantity: 1
        })
      }

      refreshCart()
      setAdded(true)
      setTimeout(() => setAdded(false), 2000)
    } catch (error) {
      console.error('Error adding to cart:', error)
    } finally {
      setAdding(false)
    }
  }

  const stockClass = product.stock < 10 ? 'product-stock low' : 'product-stock'

  return (
    <div className="product-card" data-testid="product-card">
      <img
        src={product.image_url}
        alt={product.name}
        className="product-image"
        loading="lazy"
      />
      <div className="product-info">
        <h3 className="product-name">{product.name}</h3>
        <p className="product-description">{product.description}</p>
        <div className="product-price">${product.price}</div>
        <div className={stockClass}>
          {product.stock > 0 ? `${product.stock} in stock` : 'Out of stock'}
        </div>
        <button
          className="btn btn-primary"
          style={{ marginTop: '0.75rem', width: '100%' }}
          onClick={handleAddToCart}
          disabled={adding || product.stock === 0}
          data-testid="add-to-cart"
        >
          {adding ? 'Adding...' : added ? 'Added!' : 'Add to Cart'}
        </button>

        {isAdmin && (
          <div className="admin-controls">
            <button
              className="btn btn-secondary btn-sm"
              onClick={() => onEdit(product)}
            >
              Edit
            </button>
            <button
              className="btn btn-danger btn-sm"
              onClick={() => onDelete(product)}
            >
              Delete
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

export default ProductCard
```

**Step 2: Commit**

```bash
git add test_apps/shoplite/src/components/ProductCard.jsx
git commit -m "feat(shoplite): add admin edit/delete controls to ProductCard"
```

---

### Task 6: Update Home Page with Admin Functionality

**Files:**
- Modify: `test_apps/shoplite/src/pages/Home.jsx`

**Step 1: Update the component**

Replace the entire file with:

```jsx
import { useState, useEffect } from 'react'
import { supabase } from '../lib/supabase'
import { useAuth } from '../App'
import ProductCard from '../components/ProductCard'
import ProductFormModal from '../components/ProductFormModal'
import DeleteConfirmModal from '../components/DeleteConfirmModal'

function Home() {
  const { role } = useAuth()
  const [products, setProducts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  // Modal state
  const [showProductForm, setShowProductForm] = useState(false)
  const [editingProduct, setEditingProduct] = useState(null)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [deletingProduct, setDeletingProduct] = useState(null)

  const isAdmin = role === 'admin'

  useEffect(() => {
    fetchProducts()
  }, [])

  async function fetchProducts() {
    try {
      const { data, error } = await supabase
        .from('products')
        .select('*')
        .order('name')

      if (error) throw error
      setProducts(data || [])
    } catch (err) {
      setError('Failed to load products')
      console.error('Error fetching products:', err)
    } finally {
      setLoading(false)
    }
  }

  function handleAddClick() {
    setEditingProduct(null)
    setShowProductForm(true)
  }

  function handleEditClick(product) {
    setEditingProduct(product)
    setShowProductForm(true)
  }

  function handleDeleteClick(product) {
    setDeletingProduct(product)
    setShowDeleteConfirm(true)
  }

  async function handleSaveProduct(productData) {
    if (productData.id) {
      // Update existing product
      const { error } = await supabase
        .from('products')
        .update({
          name: productData.name,
          description: productData.description,
          price: productData.price,
          image_url: productData.image_url,
          stock: productData.stock
        })
        .eq('id', productData.id)

      if (error) throw error
    } else {
      // Insert new product
      const { error } = await supabase
        .from('products')
        .insert({
          id: crypto.randomUUID(),
          name: productData.name,
          description: productData.description,
          price: productData.price,
          image_url: productData.image_url,
          stock: productData.stock
        })

      if (error) throw error
    }

    await fetchProducts()
  }

  async function handleDeleteProduct(productId) {
    const { error } = await supabase
      .from('products')
      .delete()
      .eq('id', productId)

    if (error) throw error
    await fetchProducts()
  }

  if (loading) {
    return <div className="loading">Loading products...</div>
  }

  if (error) {
    return (
      <div className="container">
        <div className="alert alert-error" style={{ marginTop: '2rem' }}>
          {error}
        </div>
      </div>
    )
  }

  return (
    <main className="container">
      <div className="page-header">
        <h1 className="page-title">Products</h1>
        {isAdmin && (
          <button className="btn btn-primary" onClick={handleAddClick}>
            Add Product
          </button>
        )}
      </div>

      {products.length === 0 ? (
        <div className="empty-state">
          <h3>No products available</h3>
          <p>Check back later for new items.</p>
        </div>
      ) : (
        <div className="product-grid" data-testid="product-grid">
          {products.map((product) => (
            <ProductCard
              key={product.id}
              product={product}
              isAdmin={isAdmin}
              onEdit={handleEditClick}
              onDelete={handleDeleteClick}
            />
          ))}
        </div>
      )}

      <ProductFormModal
        isOpen={showProductForm}
        onClose={() => setShowProductForm(false)}
        product={editingProduct}
        onSave={handleSaveProduct}
      />

      <DeleteConfirmModal
        isOpen={showDeleteConfirm}
        onClose={() => setShowDeleteConfirm(false)}
        product={deletingProduct}
        onConfirm={handleDeleteProduct}
      />
    </main>
  )
}

export default Home
```

**Step 2: Commit**

```bash
git add test_apps/shoplite/src/pages/Home.jsx
git commit -m "feat(shoplite): add admin product management to Home page"
```

---

### Task 7: Manual Testing

**Step 1: Rebuild and start sblite server**

Run: `cd /Users/markb/dev/sblite && go build -o sblite . && ./sblite serve --db test_apps/shoplite/shoplite.db`

**Step 2: Start shoplite frontend**

In a new terminal:
Run: `cd /Users/markb/dev/sblite/test_apps/shoplite && npm run dev`

**Step 3: Test as admin user**

1. Open browser to http://localhost:5173
2. Sign in as admin user (first registered user)
3. Verify "Add Product" button appears in header
4. Verify Edit/Delete buttons appear on product cards
5. Test adding a new product via modal
6. Test editing an existing product
7. Test deleting a product (with confirmation)

**Step 4: Test as non-admin user**

1. Sign out
2. Sign in as different (non-admin) user
3. Verify "Add Product" button is NOT visible
4. Verify Edit/Delete buttons are NOT visible on cards
5. Verify Add to Cart still works

---

### Task 8: Final Commit

**Step 1: Verify all changes committed**

Run: `git status`
Expected: Clean working tree

**Step 2: If any uncommitted changes, commit them**

```bash
git add -A
git commit -m "feat(shoplite): complete admin product management feature"
```
