import { useState, useEffect } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'

function ResetPassword() {
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  // Get token from URL query params
  const token = searchParams.get('token')
  const type = searchParams.get('type')

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (password.length < 6) {
      setError('Password must be at least 6 characters')
      return
    }

    if (!token) {
      setError('Invalid reset link - no token found')
      return
    }

    setLoading(true)

    try {
      // Call the verify endpoint with the token and new password
      const response = await fetch(`${import.meta.env.VITE_SUPABASE_URL || 'http://localhost:8080'}/auth/v1/verify`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          type: 'recovery',
          token: token,
          password: password
        })
      })

      const data = await response.json()

      if (!response.ok) {
        setError(data.message || data.error || 'Failed to reset password')
        setLoading(false)
        return
      }

      setSuccess(true)
    } catch (err) {
      setError('Network error. Please try again.')
    }

    setLoading(false)
  }

  // Show message if no token in URL
  if (!token || type !== 'recovery') {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Invalid Link</h1>
          <div className="alert alert-error">
            This password reset link is invalid. Please request a new one.
          </div>
          <Link to="/login" className="btn btn-primary" style={{ width: '100%', display: 'block', textAlign: 'center' }}>
            Back to Sign In
          </Link>
        </div>
      </div>
    )
  }

  // Success state
  if (success) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Password Updated</h1>
          <div className="alert alert-success">
            Your password has been successfully updated.
          </div>
          <button
            onClick={() => navigate('/')}
            className="btn btn-primary"
            style={{ width: '100%' }}
          >
            Continue to Shop
          </button>
        </div>
      </div>
    )
  }

  // Password reset form
  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1 className="auth-title">Reset Password</h1>

        {error && <div className="alert alert-error">{error}</div>}

        <form onSubmit={handleSubmit}>
          <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)' }}>
            Enter your new password below.
          </p>

          <div className="form-group">
            <label className="form-label" htmlFor="password">
              New Password
            </label>
            <input
              id="password"
              type="password"
              className="form-input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Enter new password"
              required
              minLength={6}
              data-testid="new-password-input"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="confirm-password">
              Confirm Password
            </label>
            <input
              id="confirm-password"
              type="password"
              className="form-input"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              placeholder="Confirm new password"
              required
              minLength={6}
              data-testid="confirm-password-input"
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%' }}
            disabled={loading}
            data-testid="reset-submit-button"
          >
            {loading ? 'Updating...' : 'Update Password'}
          </button>
        </form>

        <div className="auth-footer">
          <Link to="/login">Back to Sign In</Link>
        </div>
      </div>
    </div>
  )
}

export default ResetPassword
