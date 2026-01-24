import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useAuth } from '../App'

function Login() {
  const { signIn, resendConfirmation, resetPasswordForEmail, signInWithMagicLink } = useAuth()
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [needsConfirmation, setNeedsConfirmation] = useState(false)
  const [resendLoading, setResendLoading] = useState(false)
  const [resendSuccess, setResendSuccess] = useState(false)

  // View state: 'login' | 'forgot' | 'magic'
  const [view, setView] = useState('login')
  const [forgotSuccess, setForgotSuccess] = useState(false)
  const [magicLinkSuccess, setMagicLinkSuccess] = useState(false)

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

  async function handleForgotPassword(e) {
    e.preventDefault()
    setError('')
    setForgotSuccess(false)
    setLoading(true)

    const { error } = await resetPasswordForEmail(email)

    setLoading(false)

    if (error) {
      setError(error.message)
    } else {
      setForgotSuccess(true)
    }
  }

  async function handleMagicLink(e) {
    e.preventDefault()
    setError('')
    setMagicLinkSuccess(false)
    setLoading(true)

    const { error } = await signInWithMagicLink(email)

    setLoading(false)

    if (error) {
      setError(error.message)
    } else {
      setMagicLinkSuccess(true)
    }
  }

  function switchView(newView) {
    setView(newView)
    setError('')
    setForgotSuccess(false)
    setMagicLinkSuccess(false)
    setNeedsConfirmation(false)
    setResendSuccess(false)
  }

  // Forgot Password View
  if (view === 'forgot') {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Reset Password</h1>

          {error && <div className="alert alert-error">{error}</div>}

          {forgotSuccess ? (
            <>
              <div className="alert alert-success">
                If an account exists for {email}, you will receive a password reset link shortly.
              </div>
              <button
                type="button"
                className="btn btn-secondary"
                style={{ width: '100%', marginTop: '1rem' }}
                onClick={() => switchView('login')}
              >
                Back to Sign In
              </button>
            </>
          ) : (
            <form onSubmit={handleForgotPassword}>
              <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)' }}>
                Enter your email address and we'll send you a link to reset your password.
              </p>

              <div className="form-group">
                <label className="form-label" htmlFor="forgot-email">
                  Email
                </label>
                <input
                  id="forgot-email"
                  type="email"
                  className="form-input"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  required
                  data-testid="forgot-email-input"
                />
              </div>

              <button
                type="submit"
                className="btn btn-primary"
                style={{ width: '100%' }}
                disabled={loading}
                data-testid="forgot-submit-button"
              >
                {loading ? 'Sending...' : 'Send Reset Link'}
              </button>

              <button
                type="button"
                className="btn btn-secondary"
                style={{ width: '100%', marginTop: '0.5rem' }}
                onClick={() => switchView('login')}
              >
                Back to Sign In
              </button>
            </form>
          )}
        </div>
      </div>
    )
  }

  // Magic Link View
  if (view === 'magic') {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <h1 className="auth-title">Sign In with Magic Link</h1>

          {error && <div className="alert alert-error">{error}</div>}

          {magicLinkSuccess ? (
            <>
              <div className="alert alert-success">
                Check your email! We've sent a magic link to {email}. Click the link to sign in.
              </div>
              <button
                type="button"
                className="btn btn-secondary"
                style={{ width: '100%', marginTop: '1rem' }}
                onClick={() => switchView('login')}
              >
                Back to Sign In
              </button>
            </>
          ) : (
            <form onSubmit={handleMagicLink}>
              <p style={{ marginBottom: '1rem', color: 'var(--text-secondary)' }}>
                Enter your email and we'll send you a magic link to sign in instantly - no password needed.
              </p>

              <div className="form-group">
                <label className="form-label" htmlFor="magic-email">
                  Email
                </label>
                <input
                  id="magic-email"
                  type="email"
                  className="form-input"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="you@example.com"
                  required
                  data-testid="magic-email-input"
                />
              </div>

              <button
                type="submit"
                className="btn btn-primary"
                style={{ width: '100%' }}
                disabled={loading}
                data-testid="magic-submit-button"
              >
                {loading ? 'Sending...' : 'Send Magic Link'}
              </button>

              <button
                type="button"
                className="btn btn-secondary"
                style={{ width: '100%', marginTop: '0.5rem' }}
                onClick={() => switchView('login')}
              >
                Back to Sign In
              </button>
            </form>
          )}
        </div>
      </div>
    )
  }

  // Default Login View
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
            <div style={{ textAlign: 'right', marginTop: '0.25rem' }}>
              <button
                type="button"
                className="link-button"
                onClick={() => switchView('forgot')}
                data-testid="forgot-password-link"
              >
                Forgot password?
              </button>
            </div>
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

        <div style={{ textAlign: 'center', marginTop: '1rem' }}>
          <button
            type="button"
            className="link-button"
            onClick={() => switchView('magic')}
            data-testid="magic-link-button"
          >
            Sign in with Magic Link instead
          </button>
        </div>

        <div className="auth-footer">
          Don't have an account? <Link to="/register">Sign up</Link>
        </div>
      </div>
    </div>
  )
}

export default Login
