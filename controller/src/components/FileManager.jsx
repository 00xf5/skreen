import { useState, useEffect, useRef, useCallback } from 'react'
import { wsService } from '../services/websocket'
import './FileManager.css'

export function FileManager({ agentId }) {
  const [currentPath, setCurrentPath] = useState('C:\\')
  const [files, setFiles] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [transfers, setTransfers] = useState({})
  const [isDragging, setIsDragging] = useState(false)
  
  const fileInputRef = useRef(null)
  const CHUNK_SIZE = 1024 * 1024 // 1MB

  // ── WebSocket Listeners ──────────────────────────────────
  useEffect(() => {
    const unsubList = wsService.on('file_list', (msg) => {
      if (msg.agent_id !== agentId) return
      setLoading(false)
      if (msg.error) {
        setError(msg.error)
      } else {
        setFiles(msg.data || [])
        setError(null)
      }
    })

    const unsubReq = wsService.on('file_req', (msg) => {
      if (msg.agent_id !== agentId) return
      if (msg.action === 'download') {
        setTransfers(prev => ({
          ...prev,
          [msg.transfer_id]: {
            ...prev[msg.transfer_id],
            size: msg.file_size,
            chunkCount: msg.chunk_count,
            chunks: [],
            status: 'downloading',
            progress: 0
          }
        }))
        wsService.send({ type: 'file_ack', agent_id: agentId, transfer_id: msg.transfer_id })
      }
    })

    const unsubChunk = wsService.on('file_chunk', (msg) => {
      if (msg.agent_id !== agentId) return
      setTransfers(prev => {
        const t = prev[msg.transfer_id]
        if (!t || t.status === 'cancelled') return prev
        
        const newChunks = [...t.chunks, msg.chunk_data]
        const progress = Math.round((newChunks.length / t.chunkCount) * 100)
        
        wsService.send({ type: 'file_ack', agent_id: agentId, transfer_id: msg.transfer_id })

        if (newChunks.length >= t.chunkCount) {
          assembleFile(newChunks, t.name)
          return { ...prev, [msg.transfer_id]: { ...t, chunks: [], progress: 100, status: 'completed' } }
        }

        return { ...prev, [msg.transfer_id]: { ...t, chunks: newChunks, progress } }
      })
    })

    const unsubAck = wsService.on('file_ack', (msg) => {
      if (msg.agent_id !== agentId) return
      setTransfers(prev => {
        const t = prev[msg.transfer_id]
        if (!t || t.action !== 'upload' || t.status === 'completed' || t.status === 'cancelled') return prev
        if (msg.error) return { ...prev, [msg.transfer_id]: { ...t, status: 'error', error: msg.error } }

        const nextChunkIndex = (msg.chunk_index !== undefined) ? msg.chunk_index + 1 : 0
        if (nextChunkIndex >= t.chunkCount) {
          return { ...prev, [msg.transfer_id]: { ...t, progress: 100, status: 'completed' } }
        }

        readAndSendChunk(t.file, msg.transfer_id, nextChunkIndex)
        const progress = Math.round((nextChunkIndex / t.chunkCount) * 100)
        return { ...prev, [msg.transfer_id]: { ...t, currentChunk: nextChunkIndex, progress } }
      })
    })

    const unsubOp = wsService.on('file_op', (msg) => {
      if (msg.agent_id !== agentId) return
      if (msg.error) {
        alert(`File operation failed: ${msg.error}`)
      }
    })

    return () => { unsubList(); unsubReq(); unsubChunk(); unsubAck(); unsubOp() }
  }, [agentId])

  // Initial load
  useEffect(() => {
    browse(currentPath)
  }, [agentId])

  const browse = (path) => {
    setLoading(true)
    setCurrentPath(path)
    wsService.send({ type: 'file_list', agent_id: agentId, path })
  }

  const navigateUp = () => {
    const parts = currentPath.split(/[\\/]/).filter(Boolean)
    if (parts.length === 0) return // Root
    if (parts.length === 1 && currentPath.includes(':')) {
        // We are at a drive root like C:\
        browse('')
        return
    }
    parts.pop()
    const newPath = parts.join('\\') + (parts.length > 0 ? '\\' : '')
    browse(newPath || 'C:\\')
  }

  // ── Actions ─────────────────────────────────────────────
  const downloadFile = (file) => {
    const fullPath = currentPath + (currentPath.endsWith('\\') ? '' : '\\') + file.name
    const transferId = 'down_' + Date.now().toString()
    setTransfers(prev => ({
      ...prev,
      [transferId]: { action: 'download', name: file.name, progress: 0, status: 'requesting' }
    }))
    wsService.send({ type: 'file_req', agent_id: agentId, transfer_id: transferId, action: 'download', path: fullPath })
  }

  const startUpload = (file) => {
    const transferId = 'up_' + Date.now().toString()
    const chunkCount = Math.ceil(file.size / CHUNK_SIZE)
    const fullPath = currentPath + (currentPath.endsWith('\\') ? '' : '\\') + file.name

    setTransfers(prev => ({
      ...prev,
      [transferId]: { action: 'upload', name: file.name, size: file.size, file, chunkCount, progress: 0, status: 'starting' }
    }))

    wsService.send({
      type: 'file_req',
      agent_id: agentId,
      transfer_id: transferId,
      action: 'upload',
      path: fullPath,
      file_size: file.size,
      chunk_count: chunkCount
    })
  }

  const assembleFile = (chunks, name) => {
    const byteArrays = chunks.map(b64 => {
      const binary = window.atob(b64)
      const bytes = new Uint8Array(binary.length)
      for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i)
      return bytes
    })
    const blob = new Blob(byteArrays)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url; a.download = name; a.click()
    URL.revokeObjectURL(url)
  }

  const readAndSendChunk = (file, tid, idx) => {
    const start = idx * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const reader = new FileReader()
    reader.onload = (e) => {
      const bytes = new Uint8Array(e.target.result)
      let bin = ''
      for (let i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i])
      wsService.send({ type: 'file_chunk', agent_id: agentId, transfer_id: tid, chunk_index: idx, chunk_data: window.btoa(bin) })
    }
    reader.readAsArrayBuffer(file.slice(start, end))
  }

  const formatSize = (b) => {
    if (!b) return '0 B'
    const i = Math.floor(Math.log(b) / Math.log(1024))
    return (b / Math.pow(1024, i)).toFixed(1) + ' ' + ['B','KB','MB','GB','TB'][i]
  }

  const deleteFile = (file) => {
    if (!window.confirm(`Are you sure you want to delete ${file.name}?`)) return
    const fullPath = currentPath + (currentPath.endsWith('\\') ? '' : '\\') + file.name
    wsService.send({ type: 'file_op', agent_id: agentId, action: 'delete', path: fullPath })
  }

  const renameFile = (file) => {
    const newName = window.prompt(`Rename ${file.name} to:`, file.name)
    if (!newName || newName === file.name) return
    const oldPath = currentPath + (currentPath.endsWith('\\') ? '' : '\\') + file.name
    const newPath = currentPath + (currentPath.endsWith('\\') ? '' : '\\') + newName
    wsService.send({ type: 'file_op', agent_id: agentId, action: 'rename', path: oldPath, new_path: newPath })
  }

  return (
    <div className="file-manager">
      <div className="fm-header">
        <div className="fm-toolbar">
          <button className="fm-btn" onClick={navigateUp} title="Back">⬅</button>
          <div className="fm-path">
            <input 
              value={currentPath} 
              onChange={e => setCurrentPath(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && browse(currentPath)}
            />
          </div>
          <button className="fm-btn primary" onClick={() => fileInputRef.current.click()}>📤 Upload</button>
          <input type="file" ref={fileInputRef} hidden onChange={e => e.target.files[0] && startUpload(e.target.files[0])} />
        </div>
      </div>

      <div className="fm-content">
        <table className="fm-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Size</th>
              <th>Modified</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {loading && <tr><td colSpan="4" className="fm-loading">Loading...</td></tr>}
            {!loading && files.length === 0 && <tr><td colSpan="4" className="fm-empty">Empty Directory</td></tr>}
            {files.map(f => (
              <tr key={f.name} onDoubleClick={() => f.is_dir ? browse(currentPath + (currentPath.endsWith('\\') ? '' : '\\') + f.name) : downloadFile(f)}>
                <td className="fm-name-cell">
                  <span className="fm-icon">{f.is_dir ? '📁' : '📄'}</span>
                  <span>{f.name}</span>
                </td>
                <td>{f.is_dir ? '--' : formatSize(f.size)}</td>
                <td>{f.mod_time ? new Date(f.mod_time * 1000).toLocaleString() : '--'}</td>
                <td className="fm-actions-cell">
                  <button className="fm-action-btn" title="Rename" onClick={() => renameFile(f)}>✏️</button>
                  {!f.is_dir && <button className="fm-action-btn" title="Download" onClick={() => downloadFile(f)}>📥</button>}
                  <button className="fm-action-btn danger" title="Delete" onClick={() => deleteFile(f)}>🗑</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {Object.keys(transfers).length > 0 && (
        <div className="fm-footer">
          {Object.entries(transfers).map(([id, t]) => (
            <div key={id} className="fm-transfer">
              <span className="fmt-icon">{t.action === 'upload' ? '📤' : '📥'}</span>
              <span className="fmt-name">{t.name}</span>
              <div className="fmt-progress">
                <div className="fmt-bar" style={{ width: `${t.progress}%` }}></div>
              </div>
              <span className="fmt-percent">{t.progress}%</span>
              <button className="fmt-cancel" onClick={() => setTransfers(prev => { const n = {...prev}; delete n[id]; return n })}>✕</button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
