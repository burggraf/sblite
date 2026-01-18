import { useState, useEffect } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { supabase } from '../lib/supabase'
import { useAuth, useCart } from '../App'

function Checkout() {
  const { user } = useAuth()
  const { refreshCart } = useCart()
  const navigate = useNavigate()
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
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
            image_url
          )
        `)
        .order('created_at')

      if (error) throw error

      if (!data || data.length === 0) {
        navigate('/cart')
        return
      }

      setItems(data)
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

  async function handlePlaceOrder() {
    setSubmitting(true)
    setError('')

    try {
      const total = calculateTotal()
      const orderId = crypto.randomUUID()

      // Create the order
      const { error: orderError } = await supabase.from('orders').insert({
        id: orderId,
        user_id: user.id,
        status: 'pending',
        total: total
      })

      if (orderError) throw orderError

      // Create order items
      const orderItems = items.map((item) => ({
        id: crypto.randomUUID(),
        order_id: orderId,
        product_id: item.product_id,
        quantity: item.quantity,
        price_at_purchase: item.products.price
      }))

      const { error: itemsError } = await supabase.from('order_items').insert(orderItems)

      if (itemsError) throw itemsError

      // Clear the cart
      const cartIds = items.map((item) => item.id)
      const { error: deleteError } = await supabase
        .from('cart_items')
        .delete()
        .in('id', cartIds)

      if (deleteError) throw deleteError

      // Refresh cart count and navigate to orders
      refreshCart()
      navigate('/orders', { state: { newOrder: orderId } })
    } catch (err) {
      setError('Failed to place order. Please try again.')
      console.error('Error placing order:', err)
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) {
    return <div className="loading">Loading checkout...</div>
  }

  return (
    <main className="container cart-page">
      <div className="page-header">
        <h1 className="page-title">Checkout</h1>
      </div>

      {error && <div className="alert alert-error">{error}</div>}

      <div className="two-column">
        <div>
          <h3 style={{ marginBottom: '1rem' }}>Order Items</h3>
          <div data-testid="checkout-items">
            {items.map((item) => (
              <div
                key={item.id}
                className="cart-item"
                style={{ padding: '0.75rem' }}
              >
                <img
                  src={item.products.image_url}
                  alt={item.products.name}
                  className="cart-item-image"
                  style={{ width: '60px', height: '60px' }}
                />
                <div className="cart-item-info">
                  <h4 className="cart-item-name">{item.products.name}</h4>
                  <p className="cart-item-price">
                    ${item.products.price} x {item.quantity}
                  </p>
                </div>
                <div style={{ fontWeight: 600 }}>
                  ${(parseFloat(item.products.price) * item.quantity).toFixed(2)}
                </div>
              </div>
            ))}
          </div>
        </div>

        <div className="cart-summary" data-testid="checkout-summary">
          <h3>Order Summary</h3>
          <div
            style={{
              margin: '1rem 0',
              borderTop: '1px solid var(--border)',
              paddingTop: '1rem'
            }}
          >
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                marginBottom: '0.5rem'
              }}
            >
              <span>Subtotal</span>
              <span>${calculateTotal()}</span>
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                marginBottom: '0.5rem',
                color: 'var(--text-light)'
              }}
            >
              <span>Shipping</span>
              <span>Free</span>
            </div>
            <div
              style={{
                display: 'flex',
                justifyContent: 'space-between',
                fontWeight: 600,
                fontSize: '1.25rem',
                borderTop: '1px solid var(--border)',
                paddingTop: '0.75rem',
                marginTop: '0.75rem'
              }}
            >
              <span>Total</span>
              <span data-testid="checkout-total">${calculateTotal()}</span>
            </div>
          </div>

          <button
            className="btn btn-primary"
            style={{ width: '100%', marginTop: '1rem' }}
            onClick={handlePlaceOrder}
            disabled={submitting}
            data-testid="place-order-button"
          >
            {submitting ? 'Placing Order...' : 'Place Order'}
          </button>

          <Link
            to="/cart"
            className="btn btn-secondary"
            style={{ width: '100%', marginTop: '0.5rem', display: 'block', textAlign: 'center' }}
          >
            Back to Cart
          </Link>
        </div>
      </div>
    </main>
  )
}

export default Checkout
