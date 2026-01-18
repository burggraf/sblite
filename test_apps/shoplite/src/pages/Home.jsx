import { useState, useEffect } from 'react'
import { supabase } from '../lib/supabase'
import ProductCard from '../components/ProductCard'

function Home() {
  const [products, setProducts] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

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
      </div>

      {products.length === 0 ? (
        <div className="empty-state">
          <h3>No products available</h3>
          <p>Check back later for new items.</p>
        </div>
      ) : (
        <div className="product-grid" data-testid="product-grid">
          {products.map((product) => (
            <ProductCard key={product.id} product={product} />
          ))}
        </div>
      )}
    </main>
  )
}

export default Home
