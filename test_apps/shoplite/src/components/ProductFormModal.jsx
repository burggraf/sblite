import { useState, useEffect, useRef } from 'react'
import { getStorageUrl } from '../lib/supabase'

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
      setImagePreview(getStorageUrl(product.image_url) || '')
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
    setImagePreview(isEditing ? (getStorageUrl(product?.image_url) || '') : '')
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
