import { useState, useEffect, useCallback } from 'react'
import { wsService } from '../services/websocket'
import './ProcessManager.css'

export function ProcessManager({ agentId }) {
  const [procs, setProcs] = useState([])
  const [loading, setLoading] = useState(false)
  const [filter, setFilter] = useState('')
  const [killing, setKilling] = useState(null)

  const refresh = useCallback(() => {
    setLoading(true)
    wsService.send({ type: 'process_list', agent_id: agentId })
  }, [agentId])

  useEffect(() => {
    const unsub = wsService.on('process_list', (msg) => {
      if (msg.agent_id !== agentId) return
      setLoading(false)
      if (Array.isArray(msg.data)) {
        setProcs(msg.data.sort((a, b) => b.memory_kb - a.memory_kb))
      }
    })
    const unsubStatus = wsService.on('process_kill', (msg) => {
      if (msg.agent_id !== agentId) return
      setKilling(null)
      if (msg.error) alert(msg.error)
      // Refresh after kill
      setTimeout(refresh, 500)
    })
    refresh()
    return () => { unsub(); unsubStatus() }
  }, [agentId, refresh])

  const kill = (pid, name) => {
    if (!window.confirm(`Kill "${name}" (PID: ${pid})?`)) return
    setKilling(pid)
    wsService.send({ type: 'process_kill', agent_id: agentId, pid })
  }

  const filtered = procs.filter(p =>
    !filter || p.name.toLowerCase().includes(filter.toLowerCase()) || String(p.pid).includes(filter)
  )

  const formatMem = (kb) => {
    if (kb >= 1024 * 1024) return `${(kb / 1024 / 1024).toFixed(1)} GB`
    if (kb >= 1024) return `${(kb / 1024).toFixed(0)} MB`
    return `${kb} KB`
  }

  return (
    <div className="proc-manager">
      <div className="proc-toolbar">
        <input
          className="proc-search"
          type="text"
          placeholder="Filter processes..."
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <button className="proc-refresh" onClick={refresh} disabled={loading}>
          {loading ? '⏳' : '🔄'} Refresh
        </button>
        <span className="proc-count">{filtered.length} processes</span>
      </div>

      <div className="proc-table-wrap">
        <table className="proc-table">
          <thead>
            <tr>
              <th>Process</th>
              <th>PID</th>
              <th>Memory</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 && !loading && (
              <tr><td colSpan={4} className="proc-empty">No processes found</td></tr>
            )}
            {filtered.map(p => (
              <tr key={p.pid} className={killing === p.pid ? 'killing' : ''}>
                <td className="proc-name">{p.name}</td>
                <td className="proc-pid">{p.pid}</td>
                <td className="proc-mem">{formatMem(p.memory_kb)}</td>
                <td>
                  <button
                    className="kill-btn"
                    onClick={() => kill(p.pid, p.name)}
                    disabled={killing === p.pid}
                  >
                    {killing === p.pid ? '⏳' : 'Kill'}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
