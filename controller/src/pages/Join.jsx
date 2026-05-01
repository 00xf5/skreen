import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import './Auth.css' // Reuse the beautiful B&W aesthetics!

const SERVER = import.meta.env.VITE_API_URL || 'http://localhost:8080' // Go backend

export function Join() {
  const { code: urlCode } = useParams()
  const navigate = useNavigate()

  const [step, setStep] = useState(1) // 1=Input, 2=Preview, 3=Download, 4=Waiting
  const [code, setCode] = useState(urlCode || '')
  const [session, setSession] = useState(null)
  
  const [valStatus, setValStatus] = useState('idle') // idle, checking, valid, invalid
  const [valMsg, setValMsg] = useState('')
  const [countdown, setCountdown] = useState('')

  // Step 1: Input & Validation
  useEffect(() => {
    // Format input
    let v = code.replace(/[^a-zA-Z0-9]/g, '').toUpperCase()
    if (v.length > 4) v = v.slice(0, 4) + '-' + v.slice(4, 8)
    if (code !== v) setCode(v)

    if (v.length === 9) {
      setValStatus('checking')
      setValMsg('Checking code...')
      const timer = setTimeout(() => validateCode(v), 600)
      return () => clearTimeout(timer)
    } else {
      setValStatus('idle')
      setValMsg('')
      setSession(null)
    }
  }, [code])

  const validateCode = async (c) => {
    try {
      const res = await fetch(`${SERVER}/api/invite/validate?code=${encodeURIComponent(c)}`)
      const data = await res.json()
      if (data.valid) {
        setSession(data)
        setValStatus('valid')
        setValMsg('✅ Code valid')
      } else {
        setValStatus('invalid')
        setValMsg(`❌ ${data.error || 'Invalid code'}`)
        setSession(null)
      }
    } catch {
      setValStatus('invalid')
      setValMsg('❌ Could not reach server')
      setSession(null)
    }
  }

  // Step 2: Countdown Timer
  useEffect(() => {
    if (step !== 2 || !session) return
    const expiry = session.expires_at * 1000
    const tick = () => {
      const rem = Math.max(0, Math.floor((expiry - Date.now()) / 1000))
      const m = Math.floor(rem / 60)
      const s = rem % 60
      setCountdown(`${m}:${s.toString().padStart(2, '0')}`)
      if (rem === 0) {
        setValMsg('❌ Code expired')
        setStep(1)
      }
    }
    tick()
    const int = setInterval(tick, 1000)
    return () => clearInterval(int)
  }, [step, session])

  // Step 4: Polling for connection
  const [connected, setConnected] = useState(false)
  useEffect(() => {
    if (step !== 4) return
    const int = setInterval(async () => {
      try {
        const res = await fetch(`${SERVER}/api/invite/validate?code=${encodeURIComponent(code)}`)
        const data = await res.json()
        if (data.error === 'code already used') {
          setConnected(true)
          clearInterval(int)
        }
      } catch {}
    }, 2000)
    return () => clearInterval(int)
  }, [step, code])

  const doDownload = () => {
    const url = `${SERVER}/download/agent?code=${encodeURIComponent(code)}`
    const a = document.createElement('a')
    a.href = url
    a.download = 'skreen-agent-setup.exe'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    
    // Auto advance after short delay
    setTimeout(() => setStep(4), 2500)
  }

  const isWin = navigator.userAgent.includes('Win')

  return (
    <div className="auth-page">
      <div className="auth-card" style={{ maxWidth: 460, border: '1px solid #e2e2e2', padding: 40 }}>
        
        {/* Brand Header */}
        <div className="auth-brand" style={{ marginBottom: 32 }}>
          <div className="auth-brand-mark">
            <svg viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="1.5" y="1.5" width="25" height="18" rx="2.5" stroke="#0a0a0a" strokeWidth="1.8"/>
              <line x1="1.5" y1="6" x2="26.5" y2="6" stroke="#0a0a0a" strokeWidth="1.4"/>
              <line x1="14" y1="19.5" x2="14" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
              <line x1="9" y1="24" x2="19" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
              <rect x="6" y="9.5" width="6" height="5" rx="1" stroke="#0a0a0a" strokeWidth="1.4"/>
              <line x1="15" y1="10.5" x2="22" y2="10.5" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
              <line x1="15" y1="13" x2="20" y2="13" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
            </svg>
          </div>
          <div className="auth-brand-name">Skreen</div>
        </div>

        {/* ── Screen 1: Input Code ── */}
        {step === 1 && (
          <div>
            <h1 className="auth-heading" style={{ fontSize: '1.6rem' }}>Join Remote Session</h1>
            <p className="auth-subheading">Enter the access code provided by your technician.</p>
            
            <div className="auth-field" style={{ marginTop: 24 }}>
              <label className="auth-label">Access Code</label>
              <input
                className="auth-input"
                style={{
                  fontSize: '1.8rem',
                  letterSpacing: '0.15em',
                  textAlign: 'center',
                  fontFamily: 'monospace',
                  padding: '16px',
                  borderColor: valStatus === 'valid' ? '#0a0a0a' : valStatus === 'invalid' ? '#cc0000' : '#e2e2e2'
                }}
                value={code}
                onChange={e => setCode(e.target.value)}
                placeholder="XXXX-XXXX"
                maxLength={9}
              />
              <div style={{
                fontSize: '0.85rem',
                marginTop: 8,
                color: valStatus === 'invalid' ? '#cc0000' : valStatus === 'valid' ? '#0a0a0a' : '#888'
              }}>
                {valStatus === 'checking' && <span className="auth-spinner" style={{width: 12, height: 12, marginRight: 6, borderColor: 'rgba(0,0,0,0.1)', borderTopColor: '#000'}} />}
                {valMsg}
              </div>
            </div>

            <button 
              className="auth-btn-primary" 
              style={{ marginTop: 24, padding: 14 }}
              disabled={valStatus !== 'valid'}
              onClick={() => setStep(2)}
            >
              Continue →
            </button>
          </div>
        )}

        {/* ── Screen 2: Review & Trust ── */}
        {step === 2 && session && (
          <div>
            <h1 className="auth-heading" style={{ fontSize: '1.6rem' }}>Review session</h1>
            <p className="auth-subheading">Please confirm the details below.</p>
            
            <div style={{ border: '1px solid #e2e2e2', borderRadius: 6, padding: 16, marginBottom: 20 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                <span style={{ color: '#888', fontSize: '0.85rem' }}>Company</span>
                <span style={{ fontWeight: 500 }}>{session.company}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 8 }}>
                <span style={{ color: '#888', fontSize: '0.85rem' }}>Technician</span>
                <span style={{ fontWeight: 500 }}>{session.technician}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ color: '#888', fontSize: '0.85rem' }}>Expires in</span>
                <span style={{ fontWeight: 500, color: '#cc0000' }}>{countdown}</span>
              </div>
            </div>

            <div style={{ background: '#f7f7f5', padding: 14, borderRadius: 6, fontSize: '0.85rem', color: '#4a4a4a', marginBottom: 24 }}>
              <strong>Security Notice:</strong> By continuing, you are granting this technician permission to view and control your computer.
            </div>

            <button className="auth-btn-primary" style={{ padding: 14 }} onClick={() => setStep(3)}>
              I understand, Continue
            </button>
            <button className="auth-btn-google" style={{ border: 'none', marginTop: 8 }} onClick={() => setStep(1)}>
              Cancel
            </button>
          </div>
        )}

        {/* ── Screen 3: Download ── */}
        {step === 3 && (
          <div style={{ textAlign: 'center' }}>
            <h1 className="auth-heading" style={{ fontSize: '1.6rem' }}>Download Agent</h1>
            <p className="auth-subheading">Run the file to connect your session.</p>
            
            <div style={{ border: '1px solid #e2e2e2', borderRadius: 6, padding: '32px 16px', marginBottom: 24, background: '#f7f7f5' }}>
              <div style={{ fontSize: '3rem', marginBottom: 12 }}>📦</div>
              <div style={{ fontWeight: 500, fontSize: '1.1rem' }}>skreen-agent-setup.exe</div>
              <div style={{ fontSize: '0.85rem', color: '#888', marginTop: 4 }}>
                Windows Setup Wizard
              </div>
            </div>

            <button className="auth-btn-primary" style={{ padding: 14 }} onClick={doDownload}>
              Download file
            </button>

            <div style={{ marginTop: 24, fontSize: '0.85rem', color: '#4a4a4a', textAlign: 'left' }}>
              <ol style={{ paddingLeft: 20 }}>
                <li style={{ marginBottom: 8 }}>Locate <strong>skreen-agent-setup.exe</strong> in your downloads.</li>
                <li style={{ marginBottom: 8 }}>Double-click it to launch the installer.</li>
                <li style={{ marginBottom: 8 }}>Click <strong>Next</strong> through the setup wizard and accept the agreement.</li>
                <li>Click <strong>Install</strong> — the agent will start automatically when done.</li>
              </ol>
            </div>
          </div>
        )}

        {/* ── Screen 4: Waiting ── */}
        {step === 4 && (
          <div style={{ textAlign: 'center', padding: '20px 0' }}>
            {connected ? (
              <>
                <div style={{ fontSize: '4rem', marginBottom: 16 }}>✅</div>
                <h1 className="auth-heading" style={{ fontSize: '1.6rem' }}>Connected!</h1>
                <p className="auth-subheading" style={{ marginBottom: 0 }}>Your technician now has access. You can close this window.</p>
              </>
            ) : (
              <>
                <div className="auth-loading-ring" style={{ width: 64, height: 64, margin: '0 auto 24px', borderWidth: 3 }} />
                <h1 className="auth-heading" style={{ fontSize: '1.6rem' }}>Waiting for connection...</h1>
                <p className="auth-subheading">Please run the downloaded file to complete the connection.</p>
                <button className="auth-btn-google" style={{ border: 'none', marginTop: 16 }} onClick={doDownload}>
                  Download again
                </button>
              </>
            )}
          </div>
        )}

      </div>
    </div>
  )
}
