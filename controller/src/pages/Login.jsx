import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  auth,
  googleProvider,
  signInWithEmailAndPassword,
  signInWithPopup,
} from '../firebase'
import './Auth.css'

/* ── Google Icon (official colours) ── */
function GoogleIcon() {
  return (
    <svg viewBox="0 0 24 24" xmlns="http://www.w3.org/2000/svg">
      <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4" />
      <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853" />
      <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z" fill="#FBBC05" />
      <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335" />
    </svg>
  )
}

/* ── Brand mark SVG ── */
function BrandMark() {
  return (
    <svg className="auth-brand-mark" viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
      <rect x="1.5" y="1.5" width="25" height="18" rx="2.5" stroke="#0a0a0a" strokeWidth="1.8"/>
      <line x1="1.5" y1="6" x2="26.5" y2="6" stroke="#0a0a0a" strokeWidth="1.4"/>
      <line x1="14" y1="19.5" x2="14" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
      <line x1="9" y1="24" x2="19" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
      <rect x="6" y="9.5" width="6" height="5" rx="1" stroke="#0a0a0a" strokeWidth="1.4"/>
      <line x1="15" y1="10.5" x2="22" y2="10.5" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
      <line x1="15" y1="13" x2="20" y2="13" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
    </svg>
  )
}

export function Login() {
  const navigate = useNavigate()
  const [email, setEmail]       = useState('')
  const [password, setPassword] = useState('')
  const [error, setError]       = useState('')
  const [loading, setLoading]   = useState(false)
  const [gLoading, setGLoading] = useState(false)

  const friendlyError = (code) => {
    switch (code) {
      case 'auth/user-not-found':
      case 'auth/wrong-password':
      case 'auth/invalid-credential':
        return 'Invalid email or password.'
      case 'auth/too-many-requests':
        return 'Too many attempts. Try again later or reset your password.'
      case 'auth/user-disabled':
        return 'This account has been disabled. Contact your administrator.'
      default:
        return 'An error occurred. Please try again.'
    }
  }

  const handleEmail = async (e) => {
    e.preventDefault()
    if (!email || !password) return
    setError('')
    setLoading(true)
    try {
      await signInWithEmailAndPassword(auth, email, password)
      navigate('/app', { replace: true })
    } catch (err) {
      setError(friendlyError(err.code))
    } finally {
      setLoading(false)
    }
  }

  const handleGoogle = async () => {
    setError('')
    setGLoading(true)
    try {
      await signInWithPopup(auth, googleProvider)
      navigate('/app', { replace: true })
    } catch (err) {
      if (err.code !== 'auth/popup-closed-by-user') {
        setError(friendlyError(err.code))
      }
    } finally {
      setGLoading(false)
    }
  }

  const busy = loading || gLoading

  return (
    <div className="auth-page">
      <div className="auth-card">
        {/* Brand */}
        <div className="auth-brand">
          <div className="auth-brand-mark"><BrandMark /></div>
          <div className="auth-brand-name">S<span>CON</span></div>
        </div>

        <h1 className="auth-heading">Sign in to your workspace</h1>
        <p className="auth-subheading">Authorized personnel only</p>

        {/* Error */}
        {error && (
          <div className="auth-error" style={{ marginBottom: 16 }}>
            <span className="auth-error-icon">⚠</span>
            <span>{error}</span>
          </div>
        )}

        {/* Google */}
        <button
          className="auth-btn-google"
          onClick={handleGoogle}
          disabled={busy}
          style={{ marginBottom: 16 }}
        >
          {gLoading ? <span className="auth-spinner" /> : <GoogleIcon />}
          {gLoading ? 'Authenticating…' : 'Continue with Google'}
        </button>

        <div className="auth-divider">or sign in with email</div>

        {/* Email form */}
        <form className="auth-form" onSubmit={handleEmail} style={{ marginTop: 14 }}>
          <div className="auth-field">
            <label className="auth-label" htmlFor="login-email">Email</label>
            <input
              id="login-email"
              className="auth-input"
              type="email"
              placeholder="admin@company.com"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              required
              disabled={busy}
            />
          </div>

          <div className="auth-field">
            <label className="auth-label" htmlFor="login-password">Password</label>
            <input
              id="login-password"
              className="auth-input"
              type="password"
              placeholder="••••••••••"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="current-password"
              required
              disabled={busy}
            />
          </div>

          <button
            className="auth-btn-primary"
            type="submit"
            disabled={busy || !email || !password}
          >
            {loading ? <span className="auth-spinner" /> : 'Sign In →'}
          </button>
        </form>

        {/* Footer */}
        <div className="auth-footer">
          Don't have an account?{' '}
          <button onClick={() => navigate('/signup')}>Create one</button>
        </div>

        <div className="auth-secure">
          <span className="auth-secure-dot" />
          Secured by Firebase Authentication
          <span className="auth-secure-dot" />
        </div>
      </div>
    </div>
  )
}
