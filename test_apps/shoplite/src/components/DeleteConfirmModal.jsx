import { useState } from 'react'

function DeleteConfirmModal({ isOpen, onClose, product, onConfirm }) {
  const [deleting, setDeleting] = useState(false)

  if (!isOpen || !product) return null

  async function handleDelete() {
    setDeleting(true)
    try {
      await onConfirm(product.id)
      onClose()
    } catch (err) {
      console.error('Delete failed:', err)
      setDeleting(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal delete-confirm-modal" onClick={e => e.stopPropagation()}>
        <h2 className="modal-title">Delete Product</h2>
        <div className="modal-body">
          <p>Are you sure you want to delete <strong>{product.name}</strong>?</p>
          <p className="text-muted">This action cannot be undone.</p>
        </div>
        <div className="modal-actions">
          <button className="btn btn-secondary" onClick={onClose} disabled={deleting}>
            Cancel
          </button>
          <button className="btn btn-danger" onClick={handleDelete} disabled={deleting}>
            {deleting ? 'Deleting...' : 'Delete'}
          </button>
        </div>
      </div>
    </div>
  )
}

export default DeleteConfirmModal
