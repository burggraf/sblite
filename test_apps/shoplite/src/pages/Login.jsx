import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../App'

function Login() {
  const { signIn, resendConfirmation } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [needsConfirmation, setNeedsConfirmation] = useState(false)
  const [resendLoading, setResendLoading] = useState(false)
  const [resendSuccess, setResendSuccess] = useState(false)

  async function handleSubmit(e) {
    e.preventDefault()
    setError('')
    setNeedsConfirmation(false)
    setResendSuccess(false)
    setLoading(true)

    const { error } = await signIn(email, password)

    if (error) {
      // Check if the error is due to unconfirmed email
      if (error.message?.toLowerCase().includes('not confirmed') ||
          error.message?.toLowerCase().includes('email_not_confirmed')) {
        setNeedsConfirmation(true)
        setError('Your email address has not been confirmed. Please check your inbox for a confirmation link.')
      } else {
        setError(error.message)
      }
      setLoading(false)
    } else {
      navigate('/')
    }
  }

  async function handleResendConfirmation() {
    setResendLoading(true)
    setResendSuccess(false)

    const { error } = await resendConfirmation(email)

    setResendLoading(false)

    if (error) {
      setError(`Failed to resend: ${error.message}`)
    } else {
      setResendSuccess(true)
      setError('')
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <h1 className="auth-title">Sign In</h1>

        {error && <div className="alert alert-error">{error}</div>}

        {resendSuccess && (
          <div className="alert alert-success">
            Confirmation email sent! Please check your inbox.
          </div>
        )}

        {needsConfirmation && !resendSuccess && (
          <div style={{ marginBottom: '1rem', textAlign: 'center' }}>
            <button
              type="button"
              className="btn btn-secondary"
              onClick={handleResendConfirmation}
              disabled={resendLoading}
              style={{ marginTop: '0.5rem' }}
            >
              {resendLoading ? 'Sending...' : 'Resend Confirmation Email'}
            </button>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label className="form-label" htmlFor="email">
              Email
            </label>
            <input
              id="email"
              type="email"
              className="form-input"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              placeholder="you@example.com"
              required
              data-testid="email-input"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="password">
              Password
            </label>
            <input
              id="password"
              type="password"
              className="form-input"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="Your password"
              required
              data-testid="password-input"
            />
          </div>

          <button
            type="submit"
            className="btn btn-primary"
            style={{ width: '100%' }}
            disabled={loading}
            data-testid="submit-button"
          >
            {loading ? 'Signing in...' : 'Sign In'}
          </button>
        </form>

        <div className="auth-footer">
          Don't have an account? <Link to="/register">Sign up</Link>
        </div>
      </div>
    </div>
  )
}

export default Login
