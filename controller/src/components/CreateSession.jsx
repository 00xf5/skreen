import { useState } from 'react'
import './CreateSession.css'

export function CreateSession({ onClose }) {
  const [company, setCompany] = useState('SCON Support')
  const [tech, setTech] = useState('Technician')
  const [type, setType] = useState('Remote Assistance')
  const [result, setResult] = useState(null)
  const [loading, setLoading] = useState(false)
  const [copied, setCopied] = useState(null)

  const create = async () => {
    setLoading(true)
    // Use the same URL resolution priority as websocket.js
    const SERVER = (localStorage.getItem('scon_api_url') || import.meta.env.VITE_API_URL || 'http://localhost:8080').replace(/\/$/, '')
    try {
      const res = await fetch(`${SERVER}/api/invite/create`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ company, technician: tech, session_type: type })
      })
      if (!res.ok) throw new Error(`Server error: ${res.status}`)
      const data = await res.json()
      // Build the join URL pointing to the Vercel dashboard's /join route
      const joinUrl = `${window.location.origin}/join/${data.code}`
      setResult({ ...data, join_url: joinUrl })
    } catch (err) {
      alert(`Failed to reach server: ${err.message}`)
    } finally {
      setLoading(false)
    }
  }

  const copy = (text, key) => {
    navigator.clipboard.writeText(text)
    setCopied(key)
    setTimeout(() => setCopied(null), 2000)
  }

  return (
    <div className="modal-backdrop" onClick={onClose}>
      <div className="modal-card" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <h2>Create Session</h2>
          <button className="modal-close" onClick={onClose}>✕</button>
        </div>

        {!result ? (
          <div className="modal-body">
            <div className="field">
              <label>Company / Organization</label>
              <input value={company} onChange={(e) => setCompany(e.target.value)} placeholder="SCON Support" />
            </div>
            <div className="field">
              <label>Technician Name</label>
              <input value={tech} onChange={(e) => setTech(e.target.value)} placeholder="Your name" />
            </div>
            <div className="field">
              <label>Session Type</label>
              <select value={type} onChange={(e) => setType(e.target.value)}>
                <option>Remote Assistance</option>
                <option>Screen View Only</option>
                <option>File Transfer</option>
                <option>System Administration</option>
              </select>
            </div>
            <button className="create-btn" onClick={create} disabled={loading}>
              {loading ? '⏳ Generating...' : '⚡ Generate Invite Code'}
            </button>
          </div>
        ) : (
          <div className="modal-body">
            <div className="invite-result">
              <div className="invite-label">Invite Code</div>
              <div className="invite-code">{result.code}</div>
              <div className="invite-expiry">⏱ Expires in {result.expires_in}</div>
            </div>

            <div className="copy-row">
              <button className="copy-btn" onClick={() => copy(result.code, 'code')}>
                {copied === 'code' ? '✅ Copied!' : '📋 Copy Code'}
              </button>
              <button className="copy-btn" onClick={() => copy(result.join_url, 'link')}>
                {copied === 'link' ? '✅ Copied!' : '🔗 Copy Join Link'}
              </button>
            </div>

            <div className="link-preview">
              <span>Join URL</span>
              <code>{result.join_url}</code>
            </div>

            <div className="session-details">
              <div className="sd-row"><span>Company</span><strong>{result.company}</strong></div>
              <div className="sd-row"><span>Technician</span><strong>{result.technician}</strong></div>
              <div className="sd-row"><span>Type</span><strong>{result.session_type}</strong></div>
            </div>

            <button className="create-btn outline" onClick={() => setResult(null)}>
              ← Create Another
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
