import { Link, useNavigate } from 'react-router-dom'
import { useAuth, useCart } from '../App'

function Navbar() {
  const { user, role, signOut } = useAuth()
  const { cartCount } = useCart()
  const navigate = useNavigate()

  async function handleSignOut() {
    await signOut()
    navigate('/')
  }

  return (
    <nav className="navbar">
      <div className="container">
        <Link to="/" className="navbar-brand">
          ShopLite
        </Link>
        <div className="navbar-nav">
          <Link to="/">Products</Link>
          {user ? (
            <>
              <Link to="/cart">
                Cart
                {cartCount > 0 && (
                  <span className="cart-badge" data-testid="cart-badge">
                    {cartCount}
                  </span>
                )}
              </Link>
              <Link to="/orders">Orders</Link>
              <span className="user-info">
                {user.user_metadata?.name || user.email}
                <span className="user-role">{role === 'admin' ? 'Admin' : 'Customer'}</span>
              </span>
              <button className="btn btn-secondary btn-sm" onClick={handleSignOut}>
                Sign Out
              </button>
            </>
          ) : (
            <>
              <Link to="/login">Sign In</Link>
              <Link to="/register">
                <button className="btn btn-primary btn-sm">Sign Up</button>
              </Link>
            </>
          )}
        </div>
      </div>
    </nav>
  )
}

export default Navbar
