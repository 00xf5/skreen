import { useState, useEffect, useRef, useCallback } from 'react'
import { wsService } from '../services/websocket'
import './FileTransfer.css'

export function FileTransfer({ agentId }) {
  const [transfers, setTransfers] = useState({})
  const [remotePath, setRemotePath] = useState('')
  const [uploadDir, setUploadDir] = useState('C:\\temp\\')
  const [isDragging, setIsDragging] = useState(false)
  
  const fileInputRef = useRef(null)
  
  const CHUNK_SIZE = 1024 * 1024 // 1MB

  // ── WebSockets ──────────────────────────────────────────
  useEffect(() => {
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
        if (!t || t.action !== 'download' || t.status === 'cancelled') return prev
        
        const newChunks = [...t.chunks, msg.chunk_data]
        const progress = Math.round((newChunks.length / t.chunkCount) * 100)
        
        wsService.send({ type: 'file_ack', agent_id: agentId, transfer_id: msg.transfer_id })

        if (newChunks.length >= t.chunkCount) {
          assembleFile(newChunks, t.name)
          return { ...prev, [msg.transfer_id]: { ...t, chunks: [], progress: 100, status: 'completed' } } // Clear chunks to save RAM
        }

        return { ...prev, [msg.transfer_id]: { ...t, chunks: newChunks, progress } }
      })
    })

    const unsubAck = wsService.on('file_ack', (msg) => {
      if (msg.agent_id !== agentId) return
      
      setTransfers(prev => {
        const t = prev[msg.transfer_id]
        if (!t || t.action !== 'upload' || t.status === 'completed' || t.status === 'cancelled') return prev
        
        if (msg.error) {
          return { ...prev, [msg.transfer_id]: { ...t, status: 'error', error: msg.error } }
        }

        const nextChunkIndex = t.currentChunk + 1
        if (nextChunkIndex >= t.chunkCount) {
          return { ...prev, [msg.transfer_id]: { ...t, progress: 100, status: 'completed' } }
        }

        readAndSendChunk(t.file, msg.transfer_id, nextChunkIndex)
        const progress = Math.round((nextChunkIndex / t.chunkCount) * 100)
        return { ...prev, [msg.transfer_id]: { ...t, currentChunk: nextChunkIndex, progress } }
      })
    })

    return () => { unsubReq(); unsubChunk(); unsubAck() }
  }, [agentId])

  // ── File Assembly ───────────────────────────────────────
  const assembleFile = (base64Chunks, filename) => {
    const byteArrays = base64Chunks.map(b64 => {
      const binaryString = window.atob(b64)
      const bytes = new Uint8Array(binaryString.length)
      for (let i = 0; i < binaryString.length; i++) {
        bytes[i] = binaryString.charCodeAt(i)
      }
      return bytes
    })
    
    const blob = new Blob(byteArrays)
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  // ── Chunk Sending ───────────────────────────────────────
  const readAndSendChunk = (file, transferId, chunkIndex) => {
    const start = chunkIndex * CHUNK_SIZE
    const end = Math.min(start + CHUNK_SIZE, file.size)
    const blob = file.slice(start, end)
    
    const reader = new FileReader()
    reader.onload = (e) => {
      const bytes = new Uint8Array(e.target.result)
      let binary = ''
      for (let i = 0; i < bytes.byteLength; i++) {
        binary += String.fromCharCode(bytes[i])
      }
      wsService.send({
        type: 'file_chunk',
        agent_id: agentId,
        transfer_id: transferId,
        chunk_index: chunkIndex,
        chunk_data: window.btoa(binary)
      })
    }
    reader.readAsArrayBuffer(blob)
  }

  // ── Upload Handlers ─────────────────────────────────────
  const startUpload = (file) => {
    if (!uploadDir) {
      alert("Please specify a remote destination directory first (e.g., C:\\temp\\).")
      return
    }

    const transferId = 'up_' + Date.now().toString()
    const chunkCount = Math.ceil(file.size / CHUNK_SIZE)
    const fullPath = uploadDir.endsWith('\\') || uploadDir.endsWith('/') 
      ? uploadDir + file.name 
      : uploadDir + '\\' + file.name

    setTransfers(prev => ({
      ...prev,
      [transferId]: {
        action: 'upload',
        name: file.name,
        size: file.size,
        file: file,
        chunkCount,
        currentChunk: -1,
        progress: 0,
        status: 'starting'
      }
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

  const handleFileChange = (e) => {
    if (e.target.files[0]) startUpload(e.target.files[0])
    e.target.value = null
  }

  // ── Drag & Drop ─────────────────────────────────────────
  const onDragOver = (e) => { e.preventDefault(); setIsDragging(true) }
  const onDragLeave = () => setIsDragging(false)
  const onDrop = (e) => {
    e.preventDefault(); setIsDragging(false)
    if (e.dataTransfer.files && e.dataTransfer.files[0]) {
      startUpload(e.dataTransfer.files[0])
    }
  }

  // ── Download Handler ────────────────────────────────────
  const handleDownloadClick = () => {
    if (!remotePath) return
    const transferId = 'down_' + Date.now().toString()
    const parts = remotePath.split('\\').pop().split('/')
    const filename = parts[parts.length - 1] || 'downloaded_file'

    setTransfers(prev => ({
      ...prev,
      [transferId]: {
        action: 'download',
        name: filename,
        path: remotePath,
        progress: 0,
        status: 'requesting'
      }
    }))

    wsService.send({
      type: 'file_req',
      agent_id: agentId,
      transfer_id: transferId,
      action: 'download',
      path: remotePath
    })
    
    setRemotePath('') // Clear input after requested
  }

  // ── Cancel/Remove Transfer ──────────────────────────────
  const cancelTransfer = (id) => {
    setTransfers(prev => {
      const t = prev[id]
      if (!t) return prev
      if (t.status === 'completed' || t.status === 'error') {
        // Just remove from UI if done
        const next = { ...prev }
        delete next[id]
        return next
      }
      
      // Cancel active
      wsService.send({ type: 'file_req', agent_id: agentId, transfer_id: id, action: 'cancel' })
      return { ...prev, [id]: { ...t, status: 'cancelled' } }
    })
  }

  const formatSize = (bytes) => {
    if (!bytes) return '0 B'
    if (bytes >= 1024 * 1024 * 1024) return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
    if (bytes >= 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
    if (bytes >= 1024) return (bytes / 1024).toFixed(1) + ' KB'
    return bytes + ' B'
  }

  return (
    <div className="file-transfer">
      
      {/* Upload Zone */}
      <div className="ft-section">
        <h4 className="ft-section-title">Upload to Agent</h4>
        <div className="ft-input-row">
          <span className="ft-label">Destination Directory:</span>
          <input 
            type="text" 
            className="ft-input"
            value={uploadDir}
            onChange={(e) => setUploadDir(e.target.value)}
            placeholder="C:\temp\"
          />
        </div>
        
        <div 
          className={`drop-zone ${isDragging ? 'dragging' : ''}`}
          onDragOver={onDragOver}
          onDragLeave={onDragLeave}
          onDrop={onDrop}
          onClick={() => fileInputRef.current?.click()}
        >
          <div className="drop-icon">📤</div>
          <div className="drop-text">
            <strong>Click to browse</strong> or drag & drop a file here
          </div>
          <div className="drop-subtext">File will be sent to {uploadDir}</div>
          <input type="file" ref={fileInputRef} style={{ display: 'none' }} onChange={handleFileChange} />
        </div>
      </div>

      {/* Download Zone */}
      <div className="ft-section">
        <h4 className="ft-section-title">Download from Agent</h4>
        <div className="ft-input-row download-row">
          <input 
            type="text" 
            className="ft-input"
            value={remotePath}
            onChange={(e) => setRemotePath(e.target.value)}
            placeholder="Absolute path to file (e.g. C:\temp\logs.txt)"
          />
          <button className="primary-btn shrink-btn" onClick={handleDownloadClick} disabled={!remotePath}>
            📥 Download
          </button>
        </div>
      </div>

      {/* Active Transfers */}
      {Object.keys(transfers).length > 0 && (
        <div className="transfers-list">
          {Object.entries(transfers).map(([id, t]) => (
            <div key={id} className={`transfer-item ${t.status}`}>
              <div className="transfer-header">
                <div className="transfer-title">
                  <span className="transfer-icon">{t.action === 'upload' ? '📤' : '📥'}</span>
                  <span className="transfer-name" title={t.name}>{t.name}</span>
                  <span className="transfer-size">{formatSize(t.size)}</span>
                </div>
                <div className="transfer-actions">
                  <button className="cancel-btn" onClick={() => cancelTransfer(id)} title={t.status === 'completed' || t.status === 'error' || t.status === 'cancelled' ? 'Clear' : 'Cancel'}>
                    ✕
                  </button>
                </div>
              </div>

              <div className="transfer-progress">
                <div className="progress-bar-bg">
                  <div className={`progress-bar-fill ${t.status}`} style={{ width: `${t.progress || 0}%` }} />
                </div>
                <div className="progress-status">
                  <span className={`status-text ${t.status}`}>
                    {t.status === 'error' ? `Error: ${t.error}` : 
                     t.status === 'cancelled' ? 'Cancelled' :
                     t.status === 'completed' ? 'Completed' :
                     `${t.status}... ${t.progress}%`}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
