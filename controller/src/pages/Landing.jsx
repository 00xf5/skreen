import { Link } from 'react-router-dom'
import './Landing.css'

export function Landing() {
  return (
    <div className="landing-page">
      {/* ── Navigation ── */}
      <nav className="landing-nav">
        <div className="landing-nav-inner">
          <Link to="/" className="landing-logo">
            <svg className="landing-logo-mark" viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="1.5" y="1.5" width="25" height="18" rx="2.5" stroke="#0a0a0a" strokeWidth="1.8"/>
              <line x1="1.5" y1="6" x2="26.5" y2="6" stroke="#0a0a0a" strokeWidth="1.4"/>
              <line x1="14" y1="19.5" x2="14" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
              <line x1="9" y1="24" x2="19" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
              <rect x="6" y="9.5" width="6" height="5" rx="1" stroke="#0a0a0a" strokeWidth="1.4"/>
              <line x1="15" y1="10.5" x2="22" y2="10.5" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
              <line x1="15" y1="13" x2="20" y2="13" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
            </svg>
            <span className="landing-logo-name">Skreen</span>
          </Link>
          <ul className="landing-nav-links">
            <li><a href="#what">Platform</a></li>
            <li><a href="#how">How it works</a></li>
            <li><a href="#capabilities">Capabilities</a></li>
            <li><a href="#who">Who it's for</a></li>
          </ul>
          <div className="landing-nav-cta">
            <Link to="/login" className="landing-btn-ghost">Sign in</Link>
            <Link to="/signup" className="landing-btn-solid">Get started</Link>
          </div>
        </div>
      </nav>

      {/* ── Hero ── */}
      <section className="landing-hero">
        <div className="landing-container">
          <div className="landing-hero-label">Remote access infrastructure</div>
          <h1>
            Your machines,<br/>
            under your <em>control.</em>
          </h1>
          <p className="landing-hero-body">
            Skreen is a self-hosted remote administration platform for teams that need reliable, 
            persistent access to their endpoints — without routing sensitive sessions through 
            someone else's cloud.
          </p>
          <div className="landing-hero-actions">
            <Link to="/signup" className="landing-btn-solid">Start for free</Link>
            <a href="#how" className="landing-link-arrow">See how it works &#x2192;</a>
          </div>
          <div className="landing-hero-meta">
            <div>
              <div className="landing-hero-stat-num">100%</div>
              <div className="landing-hero-stat-label">Self-hosted. Your servers.</div>
            </div>
            <div>
              <div className="landing-hero-stat-num">&lt;50ms</div>
              <div className="landing-hero-stat-label">Typical command latency</div>
            </div>
            <div>
              <div className="landing-hero-stat-num">No SaaS</div>
              <div className="landing-hero-stat-label">No vendor lock-in</div>
            </div>
          </div>
        </div>
      </section>

      {/* ── What is Skreen ── */}
      <section id="what" className="landing-section">
        <div className="landing-container">
          <div className="landing-section-label">The platform</div>
          <h2 className="landing-section-heading">Built differently from the start</h2>
          <div className="landing-what-grid">
            <div className="landing-what-item">
              <div className="landing-what-num">01</div>
              <div className="landing-what-title">Persistent agents, not sessions</div>
              <div className="landing-what-body">The Skreen agent installs silently and survives reboots. It registers itself to your controller on startup and stays ready — no user action required on the remote end.</div>
            </div>
            <div className="landing-what-item">
              <div className="landing-what-num">02</div>
              <div className="landing-what-title">Your infrastructure, your rules</div>
              <div className="landing-what-body">Deploy the Skreen server on any machine you own. No data leaves your network unless you decide otherwise. Compliance, data residency, and audit requirements stay within your control.</div>
            </div>
            <div className="landing-what-item">
              <div className="landing-what-num">03</div>
              <div className="landing-what-title">Real-time screen access</div>
              <div className="landing-what-body">WebRTC-based screen streaming delivers low-latency video of the remote desktop. No plugins, no viewer software — your operator sees the endpoint directly in their browser.</div>
            </div>
            <div className="landing-what-item">
              <div className="landing-what-num">04</div>
              <div className="landing-what-title">Full command execution</div>
              <div className="landing-what-body">Run shell commands, PowerShell scripts, or system queries against any connected agent from an in-browser terminal. Output is streamed back immediately over a persistent WebSocket connection.</div>
            </div>
          </div>
        </div>
      </section>

      {/* ── CTA ── */}
      <section className="landing-cta-section">
        <div className="landing-container">
          <h2>Ready to run it<br/><em>your way?</em></h2>
          <Link to="/signup" className="landing-btn-solid" style={{fontSize:'1rem', padding:'13px 28px'}}>Create an account</Link>
          <p className="landing-cta-note">Free to use. Self-hosted. No usage limits imposed by us.</p>
        </div>
      </section>

      {/* ── Footer ── */}
      <footer className="landing-footer">
        <div className="landing-container">
          <div className="landing-footer-inner">
            <div className="landing-footer-left">
              <div className="landing-logo">
                <svg className="landing-logo-mark" viewBox="0 0 28 28" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <rect x="1.5" y="1.5" width="25" height="18" rx="2.5" stroke="#0a0a0a" strokeWidth="1.8"/>
                  <line x1="1.5" y1="6" x2="26.5" y2="6" stroke="#0a0a0a" strokeWidth="1.4"/>
                  <line x1="14" y1="19.5" x2="14" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
                  <line x1="9" y1="24" x2="19" y2="24" stroke="#0a0a0a" strokeWidth="1.8" strokeLinecap="round"/>
                  <rect x="6" y="9.5" width="6" height="5" rx="1" stroke="#0a0a0a" strokeWidth="1.4"/>
                  <line x1="15" y1="10.5" x2="22" y2="10.5" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
                  <line x1="15" y1="13" x2="20" y2="13" stroke="#0a0a0a" strokeWidth="1.4" strokeLinecap="round"/>
                </svg>
                <span className="landing-logo-name">Skreen</span>
              </div>
              <div className="landing-footer-copy">Remote access infrastructure. Self-hosted.</div>
            </div>
            <ul className="landing-footer-links">
              <li><a href="#what">Platform</a></li>
              <li><a href="#how">How it works</a></li>
              <li><a href="#capabilities">Capabilities</a></li>
              <li><Link to="/login">Sign in</Link></li>
            </ul>
          </div>
        </div>
      </footer>
    </div>
  )
}
