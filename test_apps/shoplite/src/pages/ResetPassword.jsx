import { useState, useEffect } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../App'

function ResetPassword() {
  const { updatePassword, user } = useAuth()
  const navigate = useNavigate()
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)
  const [ready, setReady] = useState(false)

  useEffect(() => {
    // The user should be set via the hash fragment from the recovery link
    // supabase-js handles parsing the token and creating a session
    // We just need to wait for the auth state to be ready
    const timer = setTimeout(() => {
      setReady(true)
    }, 1000)

    return () => clearTimeout(timer)
  }, [])

  useEffect(() => {
    // If user becomes available after page load, we're ready
    if (user) {
      setReady(true)
    }
  }, [user])

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

    setLoading(true)

    const { error } = await updatePassword(password)

    setLoading(false)

    if (error) {
      setError(error.message)
    } else {
      setSuccess(true)
    }
  }

  // Show message if no valid session
  if (ready && !user) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Invalid or Expired Link</h1>
          <div className="alert alert-error">
            This password reset link is invalid or has expired. Please request a new one.
          </div>
          <Link to="/login" className="btn btn-primary" style={{ width: '100%', display: 'block', textAlign: 'center' }}>
            Back to Sign In
          </Link>
        </div>
      </div>
    )
  }

  // Show loading while checking session
  if (!ready) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Reset Password</h1>
          <div className="loading">Verifying your link...</div>
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
