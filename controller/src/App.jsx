import { useState } from 'react'
import { Routes, Route } from 'react-router-dom'
import { useWebSocket } from './hooks/useWebSocket'
import { Dashboard } from './components/Dashboard'
import { AgentDetails } from './components/AgentDetails'
import { ScreenView } from './components/ScreenView'
import { FileTransfer } from './components/FileTransfer'
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
  const { connected, agents, results, sendCommand, togglePersistence, refreshAgents, uninstallAgent } = useWebSocket()
  const [showConnSettings, setShowConnSettings] = useState(false)
  const [newApiUrl, setNewApiUrl] = useState(localStorage.getItem('scon_api_url') || '')

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
    openTab(id, 'details', `${id.slice(0, 8)}`, '⚙')
  }

  const handleLogout = async () => {
    await signOut(auth)
  }

  const onlineCount = agents.filter(a => a.online).length
  const filtered = agents.filter(a =>
    !searchQuery || a.id.toLowerCase().includes(searchQuery.toLowerCase())
  )
  const onlineAgents = filtered.filter(a => a.online)
  const offlineAgents = filtered.filter(a => !a.online)

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

        <button className={`rail-btn ${activeRail === 'access' ? 'active' : ''}`} onClick={() => setActiveRail('access')} title="Access">🖥</button>
        <button className={`rail-btn ${activeRail === 'support' ? 'active' : ''}`} onClick={() => setActiveRail('support')} title="Support">🎧</button>
        <button className={`rail-btn ${activeRail === 'meeting' ? 'active' : ''}`} onClick={() => setActiveRail('meeting')} title="Meeting">👥</button>

        <div className="rail-spacer" />

        <button className="rail-btn" title="Notifications">🔔</button>
        <button className="rail-btn" title="Settings">⚙</button>

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
                <span>⇢</span> Sign out
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
              <span className="agent-dot online" />
              <div className="agent-item-info">
                <div className="agent-item-name">{agent.id.slice(0, 12)}...</div>
                <div className="agent-item-meta">Windows • Online</div>
              </div>
            </div>
          ))}

          {offlineAgents.length > 0 && (
            <>
              <div className="group-label">
                <span>Offline</span>
                <span className="count">{offlineAgents.length}</span>
              </div>
              {offlineAgents.map(agent => (
                <div key={agent.id} className="agent-item" onClick={() => handleSelectAgent(agent.id)}>
                  <span className="agent-dot offline" />
                  <div className="agent-item-info">
                    <div className="agent-item-name">{agent.id.slice(0, 12)}...</div>
                    <div className="agent-item-meta">Offline</div>
                  </div>
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
              if (a?.agentId) openTab(a.agentId, 'screen', `${a.agentId.slice(0,8)} Screen`, '🖥')
            }}>🖥 Join</button>
            <button className="topbar-btn" onClick={refreshAgents}>🔄 Refresh</button>
            <button className="topbar-btn">⋯ More</button>
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
                return <Dashboard key="dash" agents={agents} onSelectAgent={handleSelectAgent} />
              case 'details':
                return (
                  <div className="agent-workspace" key={tab.id}>
                    <AgentDetails
                      agentId={tab.agentId}
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
              case 'screen':
                return <ScreenView key={tab.id} agentId={tab.agentId} onClose={(e) => closeTab(e || {stopPropagation:()=>{}}, tab.id)} />
              case 'files':
                return <FileTransfer key={tab.id} agentId={tab.agentId} />
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
