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
