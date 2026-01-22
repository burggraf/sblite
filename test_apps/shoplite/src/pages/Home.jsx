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
