import { useState, useEffect } from 'react'
import './Dashboard.css'

export function Dashboard({ agents, metrics, onSelectAgent }) {
  const onlineCount = agents.filter(a => a.online).length
  const offlineCount = agents.length - onlineCount

  // Greeting logic
  const hour = new Date().getHours()
  const greeting = hour < 12 ? 'Good morning' : hour < 18 ? 'Good afternoon' : 'Good evening'

  const memMB = Math.round((metrics?.memory_usage_bytes || 0) / (1024 * 1024))
  const wsLoad = Math.round(metrics?.websocket_load || 0)

  return (
    <div className="dashboard">
      <div className="dashboard-header">
        <h2 className="dashboard-title">{greeting}, Admin.</h2>
        <p className="dashboard-subtitle">Here is your SCON network overview.</p>
      </div>
      
      {/* ── Top Stats Grid ── */}
      <div className="stats-grid">
        <div className="stat-card online">
          <div className="stat-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20 6L9 17l-5-5"/></svg>
          </div>
          <div className="stat-info">
            <div className="stat-value">{onlineCount}</div>
            <div className="stat-label">Online Agents</div>
          </div>
          <div className="stat-glow"></div>
        </div>
        
        <div className="stat-card offline">
          <div className="stat-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M18 6L6 18M6 6l12 12"/></svg>
          </div>
          <div className="stat-info">
            <div className="stat-value">{offlineCount}</div>
            <div className="stat-label">Offline Agents</div>
          </div>
          <div className="stat-glow"></div>
        </div>
        
        <div className="stat-card total">
          <div className="stat-icon">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
          </div>
          <div className="stat-info">
            <div className="stat-value">{agents.length}</div>
            <div className="stat-label">Total Endpoints</div>
          </div>
          <div className="stat-glow"></div>
        </div>
      </div>

      <div className="dashboard-main">
        {/* ── Left Column ── */}
        <div className="dashboard-col">
          <div className="dash-card quick-actions">
            <div className="dash-card-header">
              <h3>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="15" height="15" style={{marginRight:'6px',verticalAlign:'middle',display:'inline-block'}}><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
                Quick Actions
              </h3>
            </div>
            <div className="action-cards">
              <button className="action-card primary" onClick={() => onSelectAgent(agents.find(a => a.online)?.id)} disabled={!onlineCount}>
                <div className="ac-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>
                </div>
                <div className="ac-text">
                  <strong>Control Agent</strong>
                  <span>Connect to the first available endpoint</span>
                </div>
              </button>

              <button className="action-card disabled">
                <div className="ac-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/></svg>
                </div>
                <div className="ac-text">
                  <strong>Mass Deploy</strong>
                  <span>Run scripts across multiple devices</span>
                </div>
                <div className="ac-badge">Soon</div>
              </button>

              <button className="action-card disabled">
                <div className="ac-icon">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/></svg>
                </div>
                <div className="ac-text">
                  <strong>Export Audit Log</strong>
                  <span>Download session compliance reports</span>
                </div>
                <div className="ac-badge">Soon</div>
              </button>
            </div>
          </div>

          <div className="dash-card metrics">
            <div className="dash-card-header">
              <h3>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="15" height="15" style={{marginRight:'6px',verticalAlign:'middle',display:'inline-block'}}><polyline points="22 12 18 12 15 21 9 3 6 12 2 12"/></svg>
                Server Telemetry
              </h3>
            </div>
            <div className="metrics-body">
              <div className="metric-row">
                <div className="metric-label">WebSocket Load</div>
                <div className="metric-bar-wrap">
                  <div className="metric-bar" style={{ width: `${wsLoad}%` }}></div>
                </div>
                <div className="metric-val">{wsLoad}%</div>
              </div>
              <div className="metric-row">
                <div className="metric-label">Active Controllers</div>
                <div className="metric-bar-wrap">
                  <div className="metric-bar" style={{ width: '2%', background: 'var(--brand)' }}></div>
                </div>
                <div className="metric-val">{metrics?.active_controllers || 0}</div>
              </div>
              <div className="metric-row">
                <div className="metric-label">Memory Usage</div>
                <div className="metric-bar-wrap">
                  <div className="metric-bar" style={{ width: `${Math.min(100, memMB / 10)}%`, background: '#ffb84d' }}></div>
                </div>
                <div className="metric-val">{memMB}MB</div>
              </div>
            </div>
          </div>
        </div>

        {/* ── Right Column ── */}
        <div className="dashboard-col">
          <div className="dash-card audit-log">
            <div className="dash-card-header">
              <h3>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="15" height="15" style={{marginRight:'6px',verticalAlign:'middle',display:'inline-block'}}><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/></svg>
                Security &amp; Audit Log
              </h3>
              <button className="text-btn">View All</button>
            </div>
            <div className="audit-list">
              <div className="audit-item success">
                <div className="audit-icon"></div>
                <div className="audit-content">
                  <strong>Server online</strong>
                  <span>SCON Controller initialized successfully</span>
                </div>
                <div className="audit-time">Just now</div>
              </div>
              <div className="audit-item info">
                <div className="audit-icon"></div>
                <div className="audit-content">
                  <strong>WebSocket listening</strong>
                  <span>WSS protocol active on port 8080</span>
                </div>
                <div className="audit-time">2m ago</div>
              </div>
              <div className="audit-item warning">
                <div className="audit-icon"></div>
                <div className="audit-content">
                  <strong>TURN Relay inactive</strong>
                  <span>Using STUN fallback for WebRTC streams</span>
                </div>
                <div className="audit-time">5m ago</div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
