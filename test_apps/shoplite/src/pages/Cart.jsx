import { useState, useEffect } from 'react'
import { Link } from 'react-router-dom'
import { supabase } from '../lib/supabase'
import CartItem from '../components/CartItem'

function Cart() {
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    fetchCart()
  }, [])

  async function fetchCart() {
    try {
      const { data, error } = await supabase
        .from('cart_items')
        .select(`
          id,
          quantity,
          product_id,
          products (
            id,
            name,
            price,
            image_url,
            stock
          )
        `)
        .order('created_at')

      if (error) throw error
      setItems(data || [])
    } catch (err) {
      setError('Failed to load cart')
      console.error('Error fetching cart:', err)
    } finally {
      setLoading(false)
    }
  }

  function calculateTotal() {
    return items
      .reduce((sum, item) => {
        return sum + parseFloat(item.products.price) * item.quantity
      }, 0)
      .toFixed(2)
  }

  if (loading) {
    return <div className="loading">Loading cart...</div>
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
    <main className="container cart-page">
      <div className="page-header">
        <h1 className="page-title">Shopping Cart</h1>
      </div>

      {items.length === 0 ? (
        <div className="empty-state">
          <h3>Your cart is empty</h3>
          <p>Add some products to get started.</p>
          <Link to="/" className="btn btn-primary" style={{ marginTop: '1rem' }}>
            Browse Products
          </Link>
        </div>
      ) : (
        <div className="two-column">
          <div className="cart-items" data-testid="cart-items">
            {items.map((item) => (
              <CartItem key={item.id} item={item} onUpdate={fetchCart} />
            ))}
          </div>

          <div className="cart-summary" data-testid="cart-summary">
            <h3>Order Summary</h3>
            <div style={{ margin: '1rem 0', borderTop: '1px solid var(--border)', paddingTop: '1rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: '0.5rem' }}>
                <span>Items ({items.reduce((sum, i) => sum + i.quantity, 0)})</span>
                <span>${calculateTotal()}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', fontWeight: 600, fontSize: '1.25rem' }}>
                <span>Total</span>
                <span data-testid="cart-total">${calculateTotal()}</span>
              </div>
            </div>
            <Link
              to="/checkout"
              className="btn btn-primary"
              style={{ width: '100%', marginTop: '1rem' }}
              data-testid="checkout-button"
            >
              Proceed to Checkout
            </Link>
          </div>
        </div>
      )}
    </main>
  )
}

export default Cart
