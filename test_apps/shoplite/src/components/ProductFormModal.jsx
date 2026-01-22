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
