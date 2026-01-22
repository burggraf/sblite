import { useState, useEffect, createContext, useContext, useRef } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { supabase } from './lib/supabase'
import Navbar from './components/Navbar'
import Home from './pages/Home'
import Cart from './pages/Cart'
import Checkout from './pages/Checkout'
import Orders from './pages/Orders'
import Login from './pages/Login'
import Register from './pages/Register'

// Auth Context
const AuthContext = createContext(null)

export function useAuth() {
  return useContext(AuthContext)
}

// Protected Route wrapper
function ProtectedRoute({ children }) {
  const { user, loading } = useAuth()

  if (loading) {
    return <div className="loading">Loading...</div>
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return children
}

// Cart Context
const CartContext = createContext(null)

export function useCart() {
  return useContext(CartContext)
}

function App() {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)
  const [cartCount, setCartCount] = useState(0)
  const [role, setRole] = useState(null)
  const [showAdminDialog, setShowAdminDialog] = useState(false)
  // Track users we've already checked for admin promotion
  const checkedUsersRef = useRef(new Set())

  // Check if user should be promoted to admin (first user only)
  async function checkAndPromoteAdmin(userId) {
    // Only check once per user per session
    if (checkedUsersRef.current.has(userId)) {
      return
    }
    checkedUsersRef.current.add(userId)

    try {
      // Check if user already has a role
      const { data: existingRole } = await supabase
        .from('user_roles')
        .select('role')
        .eq('user_id', userId)
        .maybeSingle()

      if (existingRole) {
        // User already has a role, don't change it
        return
      }

      // Check if this is the first user
      const { count, error: countError } = await supabase
        .from('auth_users')
        .select('*', { count: 'exact', head: true })

      if (countError) {
        console.warn('Failed to check user count:', countError.message)
        return
      }

      if (count === 1) {
        // First user becomes admin
        const { error: insertError } = await supabase
          .from('user_roles')
          .insert({ user_id: userId, role: 'admin' })

        if (insertError) {
          console.warn('Failed to insert admin role:', insertError.message)
        } else {
          console.log('First user promoted to admin')
          setRole('admin')
          setShowAdminDialog(true)
        }
      }
    } catch (err) {
      console.warn('Error checking admin status:', err)
    }
  }

  useEffect(() => {
    // Get initial session
    supabase.auth.getSession().then(async ({ data: { session } }) => {
      setUser(session?.user ?? null)
      if (session?.user) {
        // Check for admin promotion on initial load
        await checkAndPromoteAdmin(session.user.id)
        const userRole = await fetchUserRole(session.user.id)
        setRole(userRole)
      }
      setLoading(false)
    })

    // Listen for auth changes
    // Note: Don't use async/await in onAuthStateChange callback to avoid race conditions
    const { data: { subscription } } = supabase.auth.onAuthStateChange(
      (event, session) => {
        setUser(session?.user ?? null)
        if (session?.user) {
          // Check for admin promotion when user signs in
          const userId = session.user.id
          if (event === 'SIGNED_IN') {
            // Fire and forget - don't await to avoid blocking
            checkAndPromoteAdmin(userId).then(() => {
              return fetchUserRole(userId)
            }).then(userRole => {
              setRole(userRole)
            }).catch(err => {
              console.error('Error in auth state change handler:', err)
            })
          } else {
            // For other events (like TOKEN_REFRESHED), just fetch role
            fetchUserRole(userId).then(userRole => {
              setRole(userRole)
            }).catch(err => {
              console.error('Error fetching role:', err)
            })
          }
        } else {
          setRole(null)
        }
      }
    )

    return () => subscription.unsubscribe()
  }, [])

  // Fetch cart count when user changes
  useEffect(() => {
    if (user) {
      fetchCartCount()
    } else {
      setCartCount(0)
    }
  }, [user])

  async function fetchCartCount() {
    const { data, error } = await supabase
      .from('cart_items')
      .select('quantity')

    if (!error && data) {
      const total = data.reduce((sum, item) => sum + item.quantity, 0)
      setCartCount(total)
    }
  }

  async function fetchUserRole(userId) {
    const { data } = await supabase
      .from('user_roles')
      .select('role')
      .eq('user_id', userId)
      .maybeSingle()

    return data?.role || 'customer'
  }

  const authValue = {
    user,
    role,
    loading,
    signIn: async (email, password) => {
      const { data, error } = await supabase.auth.signInWithPassword({
        email,
        password
      })
      return { data, error }
    },
    signUp: async (email, password) => {
      const { data, error } = await supabase.auth.signUp({
        email,
        password
      })
      // Note: Admin promotion now happens in onAuthStateChange when user signs in
      // after confirming email, because we need a valid session to query the database
      return { data, error }
    },
    signOut: async () => {
      const { error } = await supabase.auth.signOut({ scope: 'local' })
      if (error) {
        console.error('Sign out error:', error)
      }
      // Explicitly clear user state to ensure UI updates
      setUser(null)
      setRole(null)
      return { error }
    },
    resendConfirmation: async (email) => {
      const { data, error } = await supabase.auth.resend({
        type: 'signup',
        email
      })
      return { data, error }
    }
  }

  const cartValue = {
    cartCount,
    refreshCart: fetchCartCount
  }

  return (
    <AuthContext.Provider value={authValue}>
      <CartContext.Provider value={cartValue}>
        <BrowserRouter>
          <Navbar />
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/login" element={<Login />} />
            <Route path="/register" element={<Register />} />
            <Route
              path="/cart"
              element={
                <ProtectedRoute>
                  <Cart />
                </ProtectedRoute>
              }
            />
            <Route
              path="/checkout"
              element={
                <ProtectedRoute>
                  <Checkout />
                </ProtectedRoute>
              }
            />
            <Route
              path="/orders"
              element={
                <ProtectedRoute>
                  <Orders />
                </ProtectedRoute>
              }
            />
          </Routes>

          {/* Admin Promotion Dialog */}
          {showAdminDialog && (
            <div className="modal-overlay" onClick={() => setShowAdminDialog(false)}>
              <div className="modal" onClick={e => e.stopPropagation()}>
                <div className="modal-icon">
                  <span style={{ color: 'white', fontSize: '2rem' }}>&#9733;</span>
                </div>
                <h2 className="modal-title">Welcome, Admin!</h2>
                <div className="modal-body">
                  <p>You are the first user to register on ShopLite.</p>
                  <p>You have been automatically promoted to <strong>Administrator</strong> and can now manage products, orders, and other users.</p>
                </div>
                <div className="modal-footer">
                  <button className="btn btn-primary" onClick={() => setShowAdminDialog(false)}>
                    Got it!
                  </button>
                </div>
              </div>
            </div>
          )}
        </BrowserRouter>
      </CartContext.Provider>
    </AuthContext.Provider>
  )
}

export default App
