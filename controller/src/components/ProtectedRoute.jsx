import { useAuth } from '../context/AuthContext'
import { Navigate } from 'react-router-dom'

export function ProtectedRoute({ children }) {
  const user = useAuth()

  // Still resolving Firebase session
  if (user === undefined) {
    return (
      <div className="auth-loading">
        <div className="auth-loading-ring" />
      </div>
    )
  }

  if (!user) return <Navigate to="/login" replace />
  return children
}
