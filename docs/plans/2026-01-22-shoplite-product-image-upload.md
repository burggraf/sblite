# ShopLite Product Image Upload Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Enable admins to upload product images to sblite storage, with fallback to external URLs.

**Architecture:** Public storage bucket for product images. File upload in ProductFormModal with preview. Home.jsx handles upload to storage and cleanup on delete. Images named by product ID for easy management.

**Tech Stack:** React, @supabase/supabase-js storage API, sblite storage backend

---

### Task 1: Create Product Images Bucket Migration

**Files:**
- Create: `test_apps/shoplite/migrations/20260122000001_product_images_bucket.sql`

**Step 1: Create migration file**

```sql
-- Create product-images bucket for storing product photos
-- Public bucket: anyone can view images
-- 2MB limit, images only
INSERT INTO storage_buckets (id, name, public, file_size_limit, allowed_mime_types)
VALUES (
  'product-images',
  'product-images',
  1,
  2097152,
  '["image/jpeg","image/png","image/gif","image/webp"]'
);
```

**Step 2: Apply migration**

Run: `cd /Users/markb/dev/sblite && ./sblite db push --db test_apps/shoplite/shoplite.db --migrations-dir test_apps/shoplite/migrations`

Expected: Migration applied successfully

**Step 3: Commit**

```bash
git add test_apps/shoplite/migrations/20260122000001_product_images_bucket.sql
git commit -m "feat(shoplite): add product-images storage bucket migration"
```

---

### Task 2: Add Image Upload Styles to CSS

**Files:**
- Modify: `test_apps/shoplite/src/index.css`

**Step 1: Add styles at end of file**

Add after the existing `.admin-controls .btn` styles (around line 655):

```css

/* Image Upload Section */
.image-upload-section {
  margin-bottom: 1rem;
}

.image-upload-section .form-label {
  margin-bottom: 0.5rem;
}

.image-upload-preview {
  position: relative;
  width: 100%;
  max-width: 200px;
  margin-bottom: 0.75rem;
}

.image-upload-preview img {
  width: 100%;
  height: 150px;
  object-fit: cover;
  border-radius: var(--radius);
  border: 1px solid var(--border);
}

.image-upload-preview .remove-image {
  position: absolute;
  top: 0.25rem;
  right: 0.25rem;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: var(--danger);
  color: white;
  border: none;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  line-height: 1;
}

.image-upload-preview .remove-image:hover {
  background: #dc2626;
}

.image-upload-input {
  display: none;
}

.image-upload-button {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.image-upload-info {
  font-size: 0.75rem;
  color: var(--text-light);
  margin-top: 0.25rem;
}

.image-upload-error {
  font-size: 0.75rem;
  color: var(--danger);
  margin-top: 0.25rem;
}

.form-group.disabled .form-input {
  background: var(--background);
  color: var(--text-light);
  cursor: not-allowed;
}
```

**Step 2: Commit**

```bash
git add test_apps/shoplite/src/index.css
git commit -m "feat(shoplite): add image upload styles"
```

---

### Task 3: Update ProductFormModal with File Upload

**Files:**
- Modify: `test_apps/shoplite/src/components/ProductFormModal.jsx`

**Step 1: Replace the entire file with updated component**

```jsx
import { useState, useEffect, useRef } from 'react'

const MAX_FILE_SIZE = 2 * 1024 * 1024 // 2MB
const ALLOWED_TYPES = ['image/jpeg', 'image/png', 'image/gif', 'image/webp']

function ProductFormModal({ isOpen, onClose, product, onSave }) {
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [price, setPrice] = useState('')
  const [imageUrl, setImageUrl] = useState('')
  const [stock, setStock] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  // Image upload state
  const [imageFile, setImageFile] = useState(null)
  const [imagePreview, setImagePreview] = useState('')
  const [imageError, setImageError] = useState('')
  const fileInputRef = useRef(null)

  const isEditing = !!product

  useEffect(() => {
    if (product) {
      setName(product.name || '')
      setDescription(product.description || '')
      setPrice(product.price || '')
      setImageUrl(product.image_url || '')
      setStock(product.stock?.toString() || '0')
      // Set preview to existing image if it exists
      setImagePreview(product.image_url || '')
    } else {
      setName('')
      setDescription('')
      setPrice('')
      setImageUrl('')
      setStock('0')
      setImagePreview('')
    }
    setError('')
    setImageFile(null)
    setImageError('')
  }, [product, isOpen])

  // Cleanup blob URL on unmount or when preview changes
  useEffect(() => {
    return () => {
      if (imagePreview && imagePreview.startsWith('blob:')) {
        URL.revokeObjectURL(imagePreview)
      }
    }
  }, [imagePreview])

  if (!isOpen) return null

  function handleFileSelect(e) {
    const file = e.target.files?.[0]
    if (!file) return

    setImageError('')

    // Validate file type
    if (!ALLOWED_TYPES.includes(file.type)) {
      setImageError('Please select a valid image (JPEG, PNG, GIF, or WebP)')
      return
    }

    // Validate file size
    if (file.size > MAX_FILE_SIZE) {
      setImageError('Image must be less than 2MB')
      return
    }

    // Create preview
    const previewUrl = URL.createObjectURL(file)
    setImageFile(file)
    setImagePreview(previewUrl)
    setImageUrl('') // Clear URL field when file is selected
  }

  function handleRemoveImage() {
    if (imagePreview && imagePreview.startsWith('blob:')) {
      URL.revokeObjectURL(imagePreview)
    }
    setImageFile(null)
    setImagePreview(isEditing ? (product?.image_url || '') : '')
    setImageError('')
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
    }
  }

  function handleChooseFile() {
    fileInputRef.current?.click()
  }

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
        image_url: imageUrl.trim() || null, // Will be overwritten if file uploaded
        stock: parseInt(stock),
        imageFile: imageFile // Pass file to parent for upload
      })
      onClose()
    } catch (err) {
      setError(err.message || 'Failed to save product')
    } finally {
      setSaving(false)
    }
  }

  const hasFile = !!imageFile
  const showPreview = imagePreview && !imagePreview.includes('placehold.co')

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

          {/* Image Upload Section */}
          <div className="image-upload-section">
            <label className="form-label">Product Image</label>

            {showPreview && (
              <div className="image-upload-preview">
                <img src={imagePreview} alt="Preview" />
                <button
                  type="button"
                  className="remove-image"
                  onClick={handleRemoveImage}
                  title="Remove image"
                >
                  ×
                </button>
              </div>
            )}

            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/gif,image/webp"
              onChange={handleFileSelect}
              className="image-upload-input"
            />

            <button
              type="button"
              className="btn btn-secondary btn-sm image-upload-button"
              onClick={handleChooseFile}
            >
              {hasFile ? 'Change Image' : 'Choose Image'}
            </button>

            {hasFile && (
              <div className="image-upload-info">
                {imageFile.name} ({(imageFile.size / 1024).toFixed(1)} KB)
              </div>
            )}

            {imageError && (
              <div className="image-upload-error">{imageError}</div>
            )}
          </div>

          {/* Image URL fallback */}
          <div className={`form-group ${hasFile ? 'disabled' : ''}`}>
            <label className="form-label" htmlFor="imageUrl">Or enter Image URL</label>
            <input
              id="imageUrl"
              type="text"
              className="form-input"
              value={imageUrl}
              onChange={e => setImageUrl(e.target.value)}
              placeholder="https://example.com/image.jpg"
              disabled={hasFile}
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
git commit -m "feat(shoplite): add image file upload to ProductFormModal"
```

---

### Task 4: Update Home.jsx to Handle Image Upload and Delete

**Files:**
- Modify: `test_apps/shoplite/src/pages/Home.jsx`

**Step 1: Replace the entire file with updated component**

```jsx
import { useState, useEffect } from 'react'
import { supabase } from '../lib/supabase'
import { useAuth } from '../App'
import ProductCard from '../components/ProductCard'
import ProductFormModal from '../components/ProductFormModal'
import DeleteConfirmModal from '../components/DeleteConfirmModal'

const STORAGE_BUCKET = 'product-images'
const STORAGE_URL_PREFIX = `/storage/v1/object/public/${STORAGE_BUCKET}/`

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

  // Check if a URL is a storage URL (uploaded image)
  function isStorageUrl(url) {
    return url && url.includes(STORAGE_URL_PREFIX)
  }

  // Extract storage path from URL
  function getStoragePath(url) {
    if (!isStorageUrl(url)) return null
    const idx = url.indexOf(STORAGE_URL_PREFIX)
    return url.substring(idx + STORAGE_URL_PREFIX.length)
  }

  // Get file extension from file object
  function getFileExtension(file) {
    const name = file.name
    const lastDot = name.lastIndexOf('.')
    if (lastDot === -1) {
      // Fallback based on MIME type
      const mimeToExt = {
        'image/jpeg': 'jpg',
        'image/png': 'png',
        'image/gif': 'gif',
        'image/webp': 'webp'
      }
      return mimeToExt[file.type] || 'jpg'
    }
    return name.substring(lastDot + 1).toLowerCase()
  }

  async function handleSaveProduct(productData) {
    const productId = productData.id || crypto.randomUUID()
    let finalImageUrl = productData.image_url

    // Handle image upload if file provided
    if (productData.imageFile) {
      const ext = getFileExtension(productData.imageFile)
      const storagePath = `${productId}.${ext}`

      // Delete old image if it exists and is different
      if (productData.id && isStorageUrl(editingProduct?.image_url)) {
        const oldPath = getStoragePath(editingProduct.image_url)
        if (oldPath && oldPath !== storagePath) {
          await supabase.storage.from(STORAGE_BUCKET).remove([oldPath])
        }
      }

      // Upload new image
      const { error: uploadError } = await supabase.storage
        .from(STORAGE_BUCKET)
        .upload(storagePath, productData.imageFile, { upsert: true })

      if (uploadError) {
        throw new Error(`Failed to upload image: ${uploadError.message}`)
      }

      // Get public URL
      const { data: urlData } = supabase.storage
        .from(STORAGE_BUCKET)
        .getPublicUrl(storagePath)

      finalImageUrl = urlData.publicUrl
    }

    // Default to placeholder if no image
    if (!finalImageUrl) {
      finalImageUrl = 'https://placehold.co/400x300?text=Product'
    }

    if (productData.id) {
      // Update existing product
      const { error } = await supabase
        .from('products')
        .update({
          name: productData.name,
          description: productData.description,
          price: productData.price,
          image_url: finalImageUrl,
          stock: productData.stock
        })
        .eq('id', productData.id)

      if (error) throw error
    } else {
      // Insert new product
      const { error } = await supabase
        .from('products')
        .insert({
          id: productId,
          name: productData.name,
          description: productData.description,
          price: productData.price,
          image_url: finalImageUrl,
          stock: productData.stock
        })

      if (error) throw error
    }

    await fetchProducts()
  }

  async function handleDeleteProduct(productId) {
    // Find the product to get its image URL
    const product = products.find(p => p.id === productId)

    // Delete image from storage if it's an uploaded image
    if (product && isStorageUrl(product.image_url)) {
      const storagePath = getStoragePath(product.image_url)
      if (storagePath) {
        const { error: storageError } = await supabase.storage
          .from(STORAGE_BUCKET)
          .remove([storagePath])

        if (storageError) {
          console.warn('Failed to delete product image:', storageError)
          // Continue with product deletion even if image delete fails
        }
      }
    }

    // Delete the product
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
git commit -m "feat(shoplite): handle image upload and delete in Home page"
```

---

### Task 5: Manual Testing

**Step 1: Rebuild and start sblite server**

Run: `cd /Users/markb/dev/sblite && go build -o sblite . && ./sblite serve --db test_apps/shoplite/shoplite.db`

**Step 2: Start shoplite frontend**

In a new terminal:
Run: `cd /Users/markb/dev/sblite/test_apps/shoplite && npm run dev`

**Step 3: Test image upload**

1. Sign in as admin user
2. Click "Add Product"
3. Fill in name, price, stock
4. Click "Choose Image" and select a JPG/PNG file under 2MB
5. Verify preview appears
6. Save product - verify image displays on card
7. Edit the same product, change the image
8. Verify old image replaced with new one

**Step 4: Test image URL fallback**

1. Add another product
2. Don't upload a file, instead enter an external image URL
3. Save and verify external image displays

**Step 5: Test delete with image cleanup**

1. Delete the product with uploaded image
2. Verify product is removed
3. Check storage bucket - image file should be deleted

**Step 6: Test validation**

1. Try uploading a file > 2MB - should show error
2. Try uploading a non-image file - should show error

---

### Task 6: Build and Final Verification

**Step 1: Build frontend**

Run: `cd /Users/markb/dev/sblite/test_apps/shoplite && npm run build`

Expected: Build succeeds with no errors

**Step 2: Verify git status**

Run: `git status`

Expected: Clean working tree

**Step 3: View commit log**

Run: `git log --oneline -5`

Expected: 4 new commits for image upload feature
