import { useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import { supabase } from '../lib/supabase'

function Orders() {
  const location = useLocation()
  const [orders, setOrders] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const newOrderId = location.state?.newOrder

  useEffect(() => {
    fetchOrders()
  }, [])

  async function fetchOrders() {
    try {
      // Fetch orders with their items and product details
      const { data: ordersData, error: ordersError } = await supabase
        .from('orders')
        .select('*')
        .order('created_at', { ascending: false })

      if (ordersError) throw ordersError

      // Fetch order items for each order
      const ordersWithItems = await Promise.all(
        (ordersData || []).map(async (order) => {
          const { data: items } = await supabase
            .from('order_items')
            .select(`
              id,
              quantity,
              price_at_purchase,
              products (
                id,
                name
              )
            `)
            .eq('order_id', order.id)

          return { ...order, items: items || [] }
        })
      )

      setOrders(ordersWithItems)
    } catch (err) {
      setError('Failed to load orders')
      console.error('Error fetching orders:', err)
    } finally {
      setLoading(false)
    }
  }

  function formatDate(dateString) {
    return new Date(dateString).toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit'
    })
  }

  if (loading) {
    return <div className="loading">Loading orders...</div>
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
    <main className="container orders-page">
      <div className="page-header">
        <h1 className="page-title">Your Orders</h1>
      </div>

      {newOrderId && (
        <div className="alert alert-success">
          Order placed successfully! Order ID: {newOrderId.slice(0, 8)}...
        </div>
      )}

      {orders.length === 0 ? (
        <div className="empty-state">
          <h3>No orders yet</h3>
          <p>Start shopping to see your orders here.</p>
          <Link to="/" className="btn btn-primary" style={{ marginTop: '1rem' }}>
            Browse Products
          </Link>
        </div>
      ) : (
        <div data-testid="orders-list">
          {orders.map((order) => (
            <div
              key={order.id}
              className="order-card"
              data-testid="order-card"
              data-order-id={order.id}
            >
              <div className="order-header">
                <div>
                  <span className="order-id">Order #{order.id.slice(0, 8)}</span>
                  <span style={{ marginLeft: '1rem', color: 'var(--text-light)', fontSize: '0.875rem' }}>
                    {formatDate(order.created_at)}
                  </span>
                </div>
                <span className={`order-status ${order.status}`}>
                  {order.status}
                </span>
              </div>

              <div className="order-items">
                {order.items.map((item) => (
                  <div key={item.id} className="order-item">
                    <span>
                      {item.products?.name || 'Unknown Product'} x {item.quantity}
                    </span>
                    <span>
                      ${(parseFloat(item.price_at_purchase) * item.quantity).toFixed(2)}
                    </span>
                  </div>
                ))}
              </div>

              <div className="order-footer">
                <span>{order.items.length} item(s)</span>
                <span className="order-total" data-testid="order-total">
                  Total: ${parseFloat(order.total).toFixed(2)}
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </main>
  )
}

export default Orders
