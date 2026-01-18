import { useState } from 'react'
import { supabase } from '../lib/supabase'
import { useCart } from '../App'

function CartItem({ item, onUpdate }) {
  const { refreshCart } = useCart()
  const [updating, setUpdating] = useState(false)

  async function handleQuantityChange(newQuantity) {
    if (newQuantity < 1) return

    setUpdating(true)
    try {
      await supabase
        .from('cart_items')
        .update({ quantity: newQuantity })
        .eq('id', item.id)

      refreshCart()
      onUpdate()
    } catch (error) {
      console.error('Error updating quantity:', error)
    } finally {
      setUpdating(false)
    }
  }

  async function handleRemove() {
    setUpdating(true)
    try {
      await supabase.from('cart_items').delete().eq('id', item.id)
      refreshCart()
      onUpdate()
    } catch (error) {
      console.error('Error removing item:', error)
    } finally {
      setUpdating(false)
    }
  }

  const product = item.products

  return (
    <div className="cart-item" data-testid="cart-item">
      <img
        src={product.image_url}
        alt={product.name}
        className="cart-item-image"
      />
      <div className="cart-item-info">
        <h4 className="cart-item-name">{product.name}</h4>
        <p className="cart-item-price">${product.price} each</p>
        <div className="cart-item-actions">
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => handleQuantityChange(item.quantity - 1)}
            disabled={updating || item.quantity <= 1}
          >
            -
          </button>
          <input
            type="number"
            className="quantity-input"
            value={item.quantity}
            onChange={(e) => handleQuantityChange(parseInt(e.target.value) || 1)}
            min="1"
            disabled={updating}
            data-testid="quantity-input"
          />
          <button
            className="btn btn-secondary btn-sm"
            onClick={() => handleQuantityChange(item.quantity + 1)}
            disabled={updating}
          >
            +
          </button>
          <button
            className="btn btn-danger btn-sm"
            onClick={handleRemove}
            disabled={updating}
            data-testid="remove-item"
          >
            Remove
          </button>
        </div>
      </div>
      <div style={{ fontWeight: 600 }}>
        ${(parseFloat(product.price) * item.quantity).toFixed(2)}
      </div>
    </div>
  )
}

export default CartItem
