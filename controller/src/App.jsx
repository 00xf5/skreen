import { useState, useEffect } from 'react'
import { Routes, Route } from 'react-router-dom'
import { useWebSocket } from './hooks/useWebSocket'
import { Dashboard } from './components/Dashboard'
import { AgentDetails } from './components/AgentDetails'
import { ScreenView } from './components/ScreenView'
import { FileManager } from './components/FileManager'
import { Terminal } from './components/Terminal'
import { CreateSession } from './components/CreateSession'
import { ProcessManager } from './components/ProcessManager'
import { ProtectedRoute } from './components/ProtectedRoute'
import { Login } from './pages/Login'
import { Signup } from './pages/Signup'
import { useAuth } from './context/AuthContext'
import { auth, signOut } from './firebase'
import { Landing } from './pages/Landing'
import { Join } from './pages/Join'
import './App.css'

/* ── Main authenticated shell ── */
function Shell() {
  const user = useAuth()
  const { connected, agents, metrics, results, sendCommand, togglePersistence, refreshAgents, uninstallAgent } = useWebSocket()
  const [showConnSettings, setShowConnSettings] = useState(false)
  const [newApiUrl, setNewApiUrl] = useState(localStorage.getItem('scon_api_url') || '')

  useEffect(() => {
    // Auto-discovery from query params
    const params = new URLSearchParams(window.location.search)
    const apiParam = params.get('api')
    if (apiParam && apiParam !== localStorage.getItem('scon_api_url')) {
      localStorage.setItem('scon_api_url', apiParam)
      window.location.reload()
    }
  }, [])

  const saveApiUrl = () => {
    localStorage.setItem('scon_api_url', newApiUrl)
    window.location.reload()
  }

  const [tabs, setTabs] = useState([
    { id: 'dashboard', type: 'dashboard', title: 'Access', icon: '🖥' }
  ])
  const [activeTabId, setActiveTabId] = useState('dashboard')
  const [activeRail, setActiveRail] = useState('access')
  const [showCreateSession, setShowCreateSession] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [showLogoutMenu, setShowLogoutMenu] = useState(false)
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  const openTab = (agentId, type, title, icon) => {
    const tabId = `${agentId}-${type}`
    if (!tabs.find(t => t.id === tabId)) {
      setTabs(prev => [...prev, { id: tabId, agentId, type, title, icon }])
    }
    setActiveTabId(tabId)
  }

  const closeTab = (e, tabId) => {
    e.stopPropagation()
    if (tabId === 'dashboard') return
    setTabs(prev => {
      const next = prev.filter(t => t.id !== tabId)
      if (activeTabId === tabId) setActiveTabId(next[next.length - 1]?.id || 'dashboard')
      return next
    })
  }

  const handleSelectAgent = (id) => {
    const agent = agents.find(a => a.id === id)
    const label = agent?.hostname || id.slice(0, 8)
    openTab(id, 'details', label, '⚙')
  }

  const handleLogout = async () => {
    await signOut(auth)
  }

  const onlineCount = agents.filter(a => a.online).length
  const filtered = agents.filter(a =>
    !searchQuery ||
    a.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
    (a.hostname || '').toLowerCase().includes(searchQuery.toLowerCase()) ||
    (a.username || '').toLowerCase().includes(searchQuery.toLowerCase())
  )
  const onlineAgents = filtered.filter(a => a.online)
  const offlineAgents = filtered.filter(a => !a.online)

  // Format idle seconds into a human-readable label matching ScreenConnect style
  const formatIdle = (seconds) => {
    if (!seconds || seconds < 60) return 'Active'
    if (seconds < 3600) return `Idle ${Math.floor(seconds / 60)}m`
    if (seconds < 86400) return `Idle ${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
    return `Idle ${Math.floor(seconds / 86400)}d`
  }

  // OS icon SVG paths (Windows / Linux / macOS fallback)
  const OsIcon = ({ os }) => {
    if (os === 'windows') return (
      <svg className="os-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M0 3.449L9.75 2.1v9.451H0m10.949-9.602L24 0v11.4H10.949M0 12.6h9.75v9.451L0 20.699M10.949 12.6H24V24l-12.9-1.801"/>
      </svg>
    )
    if (os === 'linux') return (
      <svg className="os-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12.504 0C6 0 3.252 5.4 3.252 9.6c0 2.04.576 3.84 1.56 5.232-.336.576-.576 1.2-.576 1.824 0 1.968 1.584 3.552 3.6 3.576-.48.576-.744 1.32-.744 2.112 0 1.776 1.344 3.024 3.024 3.024.48 0 .936-.12 1.344-.312.24.936 1.08 1.632 2.088 1.632.96 0 1.776-.624 2.064-1.488.384.144.816.24 1.272.24 1.68 0 3.024-1.248 3.024-3.024 0-.792-.264-1.536-.744-2.112 2.016-.024 3.6-1.608 3.6-3.576 0-.624-.24-1.248-.576-1.824.984-1.392 1.56-3.192 1.56-5.232C20.748 5.4 18 0 12.504 0z"/>
      </svg>
    )
    return (
      <svg className="os-icon" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z"/>
      </svg>
    )
  }

  const initials = user?.displayName
    ? user.displayName.split(' ').map(n => n[0]).join('').slice(0, 2).toUpperCase()
    : user?.email?.[0]?.toUpperCase() ?? 'U'

  return (
    <div className="app">
      {/* ── Icon Sidebar Rail ── */}
      <nav className="sidebar-rail">
        <div className="rail-logo">
          <svg viewBox="0 0 24 24"><path d="M12 2L2 7v10l10 5 10-5V7L12 2zm0 2.18l6.83 3.41L12 10.96 5.17 7.59 12 4.18zM4 8.74l7 3.5v7.02l-7-3.5V8.74zm9 10.52V12.24l7-3.5v7.02l-7 3.5z"/></svg>
        </div>

        <button className={`rail-btn ${activeRail === 'access' ? 'active' : ''}`} onClick={() => setActiveRail('access')} title="Access">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>
        </button>
        <button className={`rail-btn ${activeRail === 'support' ? 'active' : ''}`} onClick={() => setActiveRail('support')} title="Support">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M3 18v-6a9 9 0 0 1 18 0v6"/><path d="M21 19a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-3a2 2 0 0 1 2-2h3zM3 19a2 2 0 0 0 2 2h1a2 2 0 0 0 2-2v-3a2 2 0 0 0-2-2H3z"/></svg>
        </button>
        <button className={`rail-btn ${activeRail === 'meeting' ? 'active' : ''}`} onClick={() => setActiveRail('meeting')} title="Meeting">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/></svg>
        </button>

        <div className="rail-spacer" />

        <button className="rail-btn" title="Notifications">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
        </button>
        <button className="rail-btn" title="Settings">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round"><path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/><circle cx="12" cy="12" r="3"/></svg>
        </button>

        {/* Avatar / logout menu */}
        <div style={{ position: 'relative' }}>
          <button
            className="rail-btn"
            title={user?.email ?? 'Profile'}
            onClick={() => setShowLogoutMenu(v => !v)}
            style={{
              color: 'var(--brand)',
              fontSize: '0.8rem',
              fontWeight: 700,
              background: showLogoutMenu ? 'var(--brand-dim)' : undefined,
              border: showLogoutMenu ? '1px solid var(--border-active)' : undefined,
            }}
          >
            {initials}
          </button>

          {showLogoutMenu && (
            <div className="logout-popup">
              <div className="logout-popup-user">
                <span className="logout-popup-avatar">{initials}</span>
                <div>
                  <div className="logout-popup-name">{user?.displayName ?? 'Operator'}</div>
                  <div className="logout-popup-email">{user?.email}</div>
                </div>
              </div>
              <div className="logout-popup-divider" />
              <button className="logout-popup-btn" onClick={handleLogout}>
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="15" height="15" style={{flexShrink:0}}><path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/></svg>
                Sign out
              </button>
            </div>
          )}
        </div>
      </nav>

      {/* ── Agent Panel ── */}
      <aside className={`agent-panel ${mobileMenuOpen ? 'mobile-open' : ''}`}>
        <div className="panel-header">
          <h2>Access</h2>
          <span className="sub">Install an agent and connect to unattended devices.</span>
          <div className="panel-search">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>
            <input
              type="text"
              placeholder="Search All Sessions..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
        </div>

        <div className="agent-groups">
          <div className="group-label">
            <span>All Machines</span>
            <span className="count">{agents.length}</span>
          </div>

          {onlineAgents.map(agent => (
            <div
              key={agent.id}
              className={`agent-item ${tabs.find(t => t.id === activeTabId)?.agentId === agent.id ? 'selected' : ''}`}
              onClick={() => handleSelectAgent(agent.id)}
            >
              <OsIcon os={agent.os} />
              <div className="agent-item-info">
                <div className="agent-item-name">{agent.hostname || agent.id.slice(0, 12)}</div>
                <div className="agent-item-meta">
                  {agent.username ? `User: ${agent.username}` : 'Unknown'}
                  {' • '}
                  <span className={agent.idle_seconds > 60 ? 'idle-label' : 'active-label'}>
                    {formatIdle(agent.idle_seconds)}
                  </span>
                </div>
              </div>
              <span className="agent-dot online" />
            </div>
          ))}

          {offlineAgents.length > 0 && (
            <>
              <div className="group-label">
                <span>Offline</span>
                <span className="count">{offlineAgents.length}</span>
              </div>
              {offlineAgents.map(agent => (
                <div key={agent.id} className="agent-item offline-item" onClick={() => handleSelectAgent(agent.id)}>
                  <OsIcon os={agent.os} />
                  <div className="agent-item-info">
                    <div className="agent-item-name">{agent.hostname || agent.id.slice(0, 12)}</div>
                    <div className="agent-item-meta">Offline</div>
                  </div>
                  <span className="agent-dot offline" />
                </div>
              ))}
            </>
          )}

          {agents.length === 0 && (
            <div style={{padding: '24px 10px', textAlign: 'center', color: 'var(--text-muted)', fontSize: '0.82rem'}}>
              No agents connected.<br/>Waiting for connections...
            </div>
          )}
        </div>

        <div className="panel-footer">
            <button className="build-btn" onClick={() => setShowCreateSession(true)}>+ New Session</button>
        </div>
      </aside>

      {/* ── Workspace ── */}
      <div className="workspace" onClick={() => setMobileMenuOpen(false)}>
        <div className="topbar">
          <button className="mobile-toggle" onClick={(e) => { e.stopPropagation(); setMobileMenuOpen(!mobileMenuOpen) }}>☰</button>
          <span className="topbar-title">
            {tabs.find(t => t.id === activeTabId)?.title || 'Access'}
          </span>
          <span className="topbar-sub">
            {tabs.find(t => t.id === activeTabId)?.agentId
              ? `Agent ${tabs.find(t => t.id === activeTabId).agentId.slice(0,8)}`
              : `${onlineCount} online`}
          </span>
          <div className="topbar-spacer" />
          <div className="topbar-actions">
            <button className="topbar-btn" onClick={() => {
              const a = tabs.find(t => t.id === activeTabId)
              if (a?.agentId) openTab(a.agentId, 'screen', `${a.agentId.slice(0,8)} Screen`, 'screen')
            }}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="14" height="14"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>
              Join
            </button>
            <button className="topbar-btn" onClick={refreshAgents}>
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="14" height="14"><path d="M23 4v6h-6"/><path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10"/></svg>
              Refresh
            </button>
            <button className="topbar-btn">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" width="14" height="14"><circle cx="12" cy="5" r="1.2" fill="currentColor"/><circle cx="12" cy="12" r="1.2" fill="currentColor"/><circle cx="12" cy="19" r="1.2" fill="currentColor"/></svg>
              More
            </button>
          </div>
          <div className="topbar-search">
            <input type="text" placeholder="🔍 Search All Sessions..." />
          </div>
          <div 
            className={`status-pill ${connected ? 'online' : 'offline'}`}
            onClick={() => !connected && setShowConnSettings(true)}
            style={{ cursor: connected ? 'default' : 'pointer' }}
          >
            <span className="status-dot-sm" />
            {connected ? 'Connected' : 'Disconnected (Fix)'}
          </div>
        </div>

        {!connected && (
          <div className="conn-warning">
            ⚠️ Dashboard disconnected from Backend. 
            <button onClick={() => setShowConnSettings(true)}>Update Backend URL</button>
          </div>
        )}

        {/* ── Tabs ── */}
        <div className="tabs-bar">
          {tabs.map(tab => (
            <div
              key={tab.id}
              className={`tab ${activeTabId === tab.id ? 'active' : ''}`}
              onClick={() => setActiveTabId(tab.id)}
            >
              <span>{tab.icon}</span>
              <span>{tab.title}</span>
              {tab.id !== 'dashboard' && (
                <button className="tab-close" onClick={(e) => closeTab(e, tab.id)}>✕</button>
              )}
            </div>
          ))}
        </div>
        {/* ── Content ── */}
        <div className="tab-content">
          {tabs.map(tab => {
            if (tab.id !== activeTabId) return null
            switch (tab.type) {
              case 'dashboard':
                return <Dashboard key="dash" agents={agents} metrics={metrics} onSelectAgent={handleSelectAgent} />
              case 'details': {
                const agent = agents.find(a => a.id === tab.agentId)
                return (
                  <div className="agent-workspace" key={tab.id}>
                    <AgentDetails
                      agent={agent}
                      results={results}
                      sendCommand={sendCommand}
                      onBack={() => setActiveTabId('dashboard')}
                      onTogglePersistence={togglePersistence}
                      onStartScreen={() => openTab(tab.agentId, 'screen', `${tab.agentId.slice(0,8)} Screen`, '🖥')}
                      onOpenProcesses={() => openTab(tab.agentId, 'processes', `${tab.agentId.slice(0,8)} Processes`, '⚙')}
                      onOpenFiles={() => openTab(tab.agentId, 'files', `${tab.agentId.slice(0,8)} Files`, '📂')}
                      onUninstall={(id) => {
                        uninstallAgent(id)
                        setTabs(prev => prev.filter(t => t.id !== tab.id))
                        setActiveTabId('dashboard')
                      }}
                    />
                    <Terminal agentId={tab.agentId} result={results[tab.agentId]} onCommand={sendCommand} />
                  </div>
                )
              }
              case 'screen':
                return <ScreenView key={tab.id} agentId={tab.agentId} onClose={(e) => closeTab(e || {stopPropagation:()=>{}}, tab.id)} />
              case 'files':
                return <FileManager key={tab.id} agentId={tab.agentId} />
              case 'processes':
                return (
                  <div className="agent-workspace" key={tab.id}>
                    <ProcessManager agentId={tab.agentId} />
                  </div>
                )
              default:
                return null
            }
          })}
        </div>
      </div>
      {showCreateSession && <CreateSession onClose={() => setShowCreateSession(false)} />}
      
      {showConnSettings && (
        <div className="modal-overlay">
          <div className="modal-card">
            <h3>Backend Connection</h3>
            <p>Your dashboard needs to connect to the SCON Backend API.</p>
            <div style={{ margin: '20px 0' }}>
              <label style={{ display: 'block', fontSize: '0.8rem', color: 'var(--text-muted)', marginBottom: '8px' }}>
                Backend URL (e.g. https://your-app.onrender.com)
              </label>
              <input 
                type="text" 
                value={newApiUrl} 
                onChange={(e) => setNewApiUrl(e.target.value)}
                placeholder="https://your-app.onrender.com"
                style={{ width: '100%', padding: '10px', background: 'var(--bg-surface)', border: '1px solid var(--border)', color: 'var(--text-bright)', borderRadius: '4px' }}
              />
            </div>
            <div style={{ display: 'flex', gap: '10px', justifyContent: 'flex-end' }}>
              <button className="secondary-btn" onClick={() => setShowConnSettings(false)}>Cancel</button>
              <button className="primary-btn" onClick={saveApiUrl}>Save & Reload</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

/* ── Root: public routes + protected shell ── */
function App() {
  return (
    <Routes>
      <Route path="/"       element={<Landing />} />
      <Route path="/login"  element={<Login />} />
      <Route path="/signup" element={<Signup />} />
      <Route path="/join"   element={<Join />} />
      <Route path="/join/:code" element={<Join />} />
      <Route
        path="/app/*"
        element={
          <ProtectedRoute>
            <Shell />
          </ProtectedRoute>
        }
      />
    </Routes>
  )
}

export default App
