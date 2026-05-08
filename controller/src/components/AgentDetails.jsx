import { useState, useEffect } from 'react'
import { FileManager } from './FileManager'
import './AgentDetails.css'

export function AgentDetails({ agent, onBack, onTogglePersistence, onStartScreen, onOpenProcesses, onOpenFiles, onUninstall, results, sendCommand }) {
  const [activeSubTab, setActiveSubTab] = useState('summary')
  const [persistence, setPersistence] = useState(agent?.persistent || false)
  const [privilege, setPrivilege] = useState('user') 
  const [isToggling, setIsToggling] = useState(false)
  const [toolResult, setToolResult] = useState(null)
  const [runningTool, setRunningTool] = useState(null)

  const handleTogglePersistence = async () => {
    const newState = !persistence
    setIsToggling(true)
    const success = await onTogglePersistence(agent.id, newState)
    if (success) setPersistence(newState)
    setIsToggling(false)
  }

  const handleElevate = () => {
    alert('To get root/admin access:\n\nWindows: Run agent as Administrator\nLinux: Run with sudo or as root\n\nThe agent executes with the privileges of its process.')
  }

  const tools = [
    { id: 'ipconfig', name: 'IP Config', cmd: 'ipconfig /all', icon: '🌐' },
    { id: 'flushdns', name: 'Flush DNS', cmd: 'ipconfig /flushdns', icon: '🧹' },
    { id: 'sysinfo', name: 'Sys Info', cmd: 'systeminfo', icon: '📋' },
    { id: 'netstat', name: 'Net Connections', cmd: 'netstat -an | findstr LISTENING', icon: '🔌' },
    { id: 'services', name: 'List Services', cmd: 'sc query', icon: '⚙️' },
    { id: 'whoami', name: 'Current User', cmd: 'whoami /all', icon: '👤' },
  ]

  const stats = agent?.stats || {}
  const hostname = agent?.hostname || 'Unknown'
  const os = agent?.os || 'Unknown'
  
  const formatGB = (bytes) => (bytes / (1024 * 1024 * 1024)).toFixed(1) + ' GB'
  const ramTotal = stats.ram_total ? formatGB(stats.ram_total) : 'N/A'
  const ramPercent = stats.ram_total ? Math.round((stats.ram_used / stats.ram_total) * 100) : 0
  const diskTotal = stats.disk_total ? formatGB(stats.disk_total) : 'N/A'
  const diskFree = stats.disk_free ? formatGB(stats.disk_free) : 'N/A'
  const diskPercent = stats.disk_total ? Math.round(((stats.disk_total - stats.disk_free) / stats.disk_total) * 100) : 0

  const formatUptime = (seconds) => {
    if (!seconds) return 'N/A'
    const d = Math.floor(seconds / 86400)
    const h = Math.floor((seconds % 86400) / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    return `${d}d ${h}h ${m}m`
  }

  const runTool = async (tool) => {
    if (!agent?.online) return
    setRunningTool(tool.id)
    setToolResult({ tool: tool.name, output: 'Executing on remote agent...', status: 'loading' })
    sendCommand(agent.id, tool.cmd)
  }

  useEffect(() => {
    if (results && results[agent?.id] && runningTool) {
      const res = results[agent.id]
      setToolResult({
        tool: tools.find(t => t.id === runningTool)?.name || 'Task',
        output: res.output || res.error || 'Command executed with no output.',
        status: res.error ? 'error' : 'success'
      })
      setRunningTool(null)
    }
  }, [results, agent?.id, runningTool])

  const renderContent = () => {
    switch (activeSubTab) {
      case 'summary':
        return (
          <div className="details-grid">
            <div className="detail-card">
              <h4>System Audit</h4>
              <div className="detail-row"><span>Hostname</span><span className="val-bright">{hostname}</span></div>
              <div className="detail-row"><span>OS</span><span className="val-bright">{os}</span></div>
              <div className="detail-row"><span>CPU</span><span className="val-bright" style={{ fontSize: '0.75rem' }}>{stats.cpu || 'Detecting...'}</span></div>
              <div className="detail-row"><span>Uptime</span><span className="val-bright">{formatUptime(stats.uptime)}</span></div>
              
              <div className="telemetry-item">
                <div className="telemetry-label"><span>Memory</span><span>{ramPercent}% of {ramTotal}</span></div>
                <div className="telemetry-bar-wrap">
                  <div className={`telemetry-bar ${ramPercent > 85 ? 'danger' : ramPercent > 60 ? 'warning' : ''}`} style={{ width: `${ramPercent}%` }}></div>
                </div>
              </div>

              <div className="telemetry-item">
                <div className="telemetry-label"><span>Disk (Primary)</span><span>{diskPercent}% ({diskFree} free of {diskTotal})</span></div>
                <div className="telemetry-bar-wrap">
                  <div className={`telemetry-bar ${diskPercent > 90 ? 'danger' : ''}`} style={{ width: `${diskPercent}%` }}></div>
                </div>
              </div>
            </div>

            <div className="detail-card">
              <h4>Networking</h4>
              <div className="detail-row"><span>Local IP</span><span className="val-bright">{stats.local_ip || '127.0.0.1'}</span></div>
              <div className="detail-row"><span>Public IP</span><span className="val-bright">{stats.public_ip || 'Fetching...'}</span></div>
              <div className="detail-row"><span>Username</span><span className="val-bright">{agent?.username || 'unknown'}</span></div>
            </div>

            <div className="detail-card">
              <h4>Privileges</h4>
              <div className="privilege-indicator">
                <span className={`priv-badge ${privilege}`}>
                  {privilege === 'system' ? '🔴 SYSTEM' : privilege === 'admin' ? '🟡 Admin' : '🟢 User'}
                </span>
              </div>
              <p className="detail-note">Agent runs with current user privileges.</p>
              <button className="elevate-btn" onClick={handleElevate}>How to Elevate</button>
            </div>

            <div className="detail-card">
              <h4>Persistence</h4>
              <div className={`toggle-control ${isToggling ? 'toggling' : ''}`}>
                <span>Auto-start on boot</span>
                <label className="switch">
                  <input type="checkbox" checked={persistence} onChange={handleTogglePersistence} disabled={isToggling} />
                  <span className="slider"></span>
                </label>
              </div>
            </div>

            <div className="detail-card danger">
              <h4>Danger Zone</h4>
              <div className="danger-actions">
                <button className="danger-btn" onClick={() => window.confirm('Uninstall?') && onUninstall(agent.id)}>Uninstall Agent</button>
              </div>
            </div>
          </div>
        )
      case 'toolbox':
        return (
          <div className="toolbox-view">
            <div className="toolbox-grid">
              {tools.map(tool => (
                <button key={tool.id} className={`tool-card ${runningTool === tool.id ? 'running' : ''}`} onClick={() => runTool(tool)} disabled={!agent?.online}>
                  <span className="tool-icon">{tool.icon}</span>
                  <span className="tool-name">{tool.name}</span>
                  <span className="tool-cmd">{tool.cmd}</span>
                </button>
              ))}
            </div>
            <div className="custom-tool">
              <h4>Remote Scripting</h4>
              <div className="custom-input-group">
                <input type="text" placeholder="Enter command..." onKeyDown={(e) => e.key === 'Enter' && (runTool({ id: 'custom', name: 'Custom Script', cmd: e.target.value }), e.target.value = '')} />
                <button onClick={(e) => { const i = e.target.previousSibling; runTool({ id: 'custom', name: 'Custom Script', cmd: i.value }); i.value = '' }}>Run</button>
              </div>
            </div>
            {toolResult && (
              <div className="tool-output">
                <div className="output-header"><span>Result for: {toolResult.tool}</span><button onClick={() => setToolResult(null)}>Clear</button></div>
                <pre>{toolResult.output}</pre>
              </div>
            )}
          </div>
        )
      case 'files':
        return (
          <div className="files-view">
            <FileManager agentId={agent?.id} />
          </div>
        )
      default:
        return null
    }
  }

  return (
    <div className="agent-details">
      <div className="details-header">
        <button className="back-btn" onClick={onBack}>← Back</button>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <h3>Agent: {agent?.id?.slice(0, 8)}...</h3>
          <span className={`status-badge ${agent?.online ? 'online' : 'offline'}`}>{agent?.online ? 'Online' : 'Offline'}</span>
        </div>
        <div style={{ marginLeft: 'auto', display: 'flex', gap: '8px' }}>
          <button className="secondary-btn" onClick={onOpenProcesses}>⚙ Processes</button>
          <button className="secondary-btn" onClick={onOpenFiles}>📂 Files</button>
          <button className="primary-btn" onClick={onStartScreen}>🖥 View Screen</button>
        </div>
      </div>

      <div className="subtabs-bar">
        <button className={`subtab-btn ${activeSubTab === 'summary' ? 'active' : ''}`} onClick={() => setActiveSubTab('summary')}>System Audit</button>
        <button className={`subtab-btn ${activeSubTab === 'toolbox' ? 'active' : ''}`} onClick={() => setActiveSubTab('toolbox')}>Tactical Toolbox</button>
        <button className={`subtab-btn ${activeSubTab === 'files' ? 'active' : ''}`} onClick={() => setActiveSubTab('files')}>File Explorer</button>
      </div>

      <div className="agent-details-content">
        {renderContent()}
      </div>
    </div>
  )
}
