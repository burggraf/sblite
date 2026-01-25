import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { supabase, getStorageUrl } from '../lib/supabase'
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
        src={getStorageUrl(product.image_url)}
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
