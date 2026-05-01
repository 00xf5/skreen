import { useEffect, useRef, useState, useCallback } from 'react'
import { wsService } from '../services/websocket'
import './ScreenView.css'

export function ScreenView({ agentId, onClose }) {
  const [status, setStatus] = useState('Connecting...')
  const [error, setError] = useState(null)
  const [hasControl, setHasControl] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [toolbarVisible, setToolbarVisible] = useState(true)
  const [quality, setQuality] = useState('balanced') // 'low' | 'balanced' | 'high'
  const [clipboardStatus, setClipboardStatus] = useState(null) // 'syncing' | 'synced' | 'error'

  const pcRef = useRef(null)
  const imgRef = useRef(null)
  const containerRef = useRef(null)
  const lastMoveRef = useRef(0)
  const toolbarTimerRef = useRef(null)

  // ── WebRTC Setup ──────────────────────────────────────────
  useEffect(() => {
    wsService.send({ type: 'start_stream', agent_id: agentId })

    const pc = new RTCPeerConnection({
      iceServers: [
        { urls: 'stun:stun.l.google.com:19302' },
        { 
          urls: 'turn:127.0.0.1:3478',
          username: 'scon',
          credential: 'scon_super_secret_webrtc_key'
        }
      ]
    })
    pcRef.current = pc

    pc.onicecandidate = (event) => {
      if (event.candidate) {
        wsService.send({ type: 'ice_candidate', agent_id: agentId, candidate: JSON.stringify(event.candidate) })
      }
    }

    pc.oniceconnectionstatechange = () => {
      const state = pc.iceConnectionState
      if (state === 'failed' || state === 'disconnected') {
        setError('Connection lost — attempting recovery...')
        setStatus('disconnected')
        // Auto-recover: restart ICE
        pc.restartIce()
      } else if (state === 'connected') {
        setStatus('Connected')
        setError(null)
      } else if (state === 'checking') {
        setStatus('Connecting...')
      }
    }

    pc.ondatachannel = (event) => {
      const channel = event.channel
      if (channel.label === 'screen') {
        channel.onmessage = (e) => {
          const blob = new Blob([e.data], { type: 'image/jpeg' })
          const url = URL.createObjectURL(blob)
          if (imgRef.current) {
            if (imgRef.current.src?.startsWith('blob:')) URL.revokeObjectURL(imgRef.current.src)
            imgRef.current.src = url
          }
        }
      }
    }

    const unsubOffer = wsService.on('webrtc_offer', async (msg) => {
      if (msg.agent_id !== agentId) return
      try {
        await pc.setRemoteDescription({ type: 'offer', sdp: msg.sdp })
        const answer = await pc.createAnswer()
        await pc.setLocalDescription(answer)
        wsService.send({ type: 'webrtc_answer', agent_id: agentId, sdp: answer.sdp })
      } catch (err) {
        console.error('Offer handling failed:', err)
        setError('Failed to negotiate connection')
      }
    })

    const unsubIce = wsService.on('ice_candidate', async (msg) => {
      if (msg.agent_id !== agentId) return
      try { await pc.addIceCandidate(JSON.parse(msg.candidate)) } catch {}
    })

    const unsubStopped = wsService.on('stream_stopped', (msg) => {
      if (msg.agent_id === agentId) setError('Stream ended by agent')
    })

    const unsubControl = wsService.on('control_request', (msg) => {
      if (msg.agent_id !== agentId) return
      if (msg.output === 'approved') setHasControl(true)
      else { setHasControl(false) }
    })

    const unsubClipboard = wsService.on('clipboard_data', (msg) => {
      if (msg.agent_id !== agentId) return
      if (msg.data) {
        navigator.clipboard.writeText(msg.data).then(() => {
          setClipboardStatus('synced')
          setTimeout(() => setClipboardStatus(null), 2000)
        }).catch(() => setClipboardStatus('error'))
      }
    })

    return () => {
      unsubOffer(); unsubIce(); unsubStopped(); unsubControl(); unsubClipboard()
      wsService.send({ type: 'stop_stream', agent_id: agentId })
      wsService.send({ type: 'control_stop', agent_id: agentId })
      if (pcRef.current) { pcRef.current.close(); pcRef.current = null }
      if (imgRef.current?.src?.startsWith('blob:')) URL.revokeObjectURL(imgRef.current.src)
      if (document.fullscreenElement) document.exitFullscreen().catch(() => {})
    }
  }, [agentId])

  // ── Fullscreen ────────────────────────────────────────────
  const toggleFullscreen = useCallback(async () => {
    if (!document.fullscreenElement) {
      await containerRef.current?.requestFullscreen()
      setIsFullscreen(true)
    } else {
      await document.exitFullscreen()
      setIsFullscreen(false)
    }
  }, [])

  useEffect(() => {
    const handler = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', handler)
    return () => document.removeEventListener('fullscreenchange', handler)
  }, [])

  // ── Floating toolbar auto-hide ────────────────────────────
  const resetToolbarTimer = useCallback(() => {
    if (!isFullscreen) return
    setToolbarVisible(true)
    clearTimeout(toolbarTimerRef.current)
    toolbarTimerRef.current = setTimeout(() => setToolbarVisible(false), 2500)
  }, [isFullscreen])

  useEffect(() => {
    if (!isFullscreen) {
      setToolbarVisible(true)
      clearTimeout(toolbarTimerRef.current)
    } else {
      resetToolbarTimer()
    }
    return () => clearTimeout(toolbarTimerRef.current)
  }, [isFullscreen, resetToolbarTimer])

  // ── ESC exits fullscreen (keyboard hook) ──────────────────
  useEffect(() => {
    const handleKey = (e) => {
      if (hasControl) {
        // In control mode: intercept all keys
        if (e.key === 'Escape' && isFullscreen) {
          // ESC exits control first, then fullscreen on second press
          wsService.send({ type: 'control_stop', agent_id: agentId })
          setHasControl(false)
          return
        }
        e.preventDefault()
        wsService.send({ type: 'input_keyboard', agent_id: agentId, key: e.key, state: 'down' })
      }
    }
    const handleKeyUp = (e) => {
      if (hasControl && e.key !== 'Escape') {
        e.preventDefault()
        wsService.send({ type: 'input_keyboard', agent_id: agentId, key: e.key, state: 'up' })
      }
    }
    window.addEventListener('keydown', handleKey, { passive: false })
    window.addEventListener('keyup', handleKeyUp, { passive: false })
    return () => {
      window.removeEventListener('keydown', handleKey)
      window.removeEventListener('keyup', handleKeyUp)
    }
  }, [hasControl, agentId, isFullscreen])

  // ── Quality — tell agent to adjust ───────────────────────
  const setQualityMode = (q) => {
    setQuality(q)
    wsService.send({ type: 'stream_quality', agent_id: agentId, quality: q })
  }

  // ── Clipboard sync ────────────────────────────────────────
  const syncClipboard = async () => {
    setClipboardStatus('syncing')
    try {
      const text = await navigator.clipboard.readText()
      wsService.send({ type: 'clipboard_set', agent_id: agentId, data: text })
      setClipboardStatus('synced')
    } catch {
      // Fallback: request from agent
      wsService.send({ type: 'clipboard_get', agent_id: agentId })
    }
    setTimeout(() => setClipboardStatus(null), 2000)
  }

  // ── Normalized coords (letterbox-safe) ────────────────────
  const getNormalizedCoords = (e, el) => {
    const rect = el.getBoundingClientRect()
    if (!el.naturalWidth || !el.naturalHeight) {
      return { x: (e.clientX - rect.left) / rect.width, y: (e.clientY - rect.top) / rect.height }
    }
    const imgAspect = el.naturalWidth / el.naturalHeight
    const elemAspect = rect.width / rect.height
    let rw, rh, ox, oy
    if (elemAspect > imgAspect) {
      rh = rect.height; rw = rh * imgAspect; ox = (rect.width - rw) / 2; oy = 0
    } else {
      rw = rect.width; rh = rw / imgAspect; ox = 0; oy = (rect.height - rh) / 2
    }
    return {
      x: Math.min(Math.max((e.clientX - rect.left - ox) / rw, 0), 1),
      y: Math.min(Math.max((e.clientY - rect.top - oy) / rh, 0), 1)
    }
  }

  const handleMouseMove = (e) => {
    if (isFullscreen) resetToolbarTimer()
    if (!hasControl || !imgRef.current) return
    const now = Date.now()
    if (now - lastMoveRef.current < 30) return
    lastMoveRef.current = now
    const { x, y } = getNormalizedCoords(e, imgRef.current)
    wsService.send({ type: 'input_mouse', agent_id: agentId, event: 'move', x, y })
  }

  const handleMouseAction = (e, state) => {
    if (!hasControl) return
    e.preventDefault()
    const button = e.button === 2 ? 'right' : e.button === 1 ? 'center' : 'left'
    wsService.send({ type: 'input_mouse', agent_id: agentId, event: 'click', button, state })
  }

  const toggleControl = () => {
    if (hasControl) {
      wsService.send({ type: 'control_stop', agent_id: agentId })
      setHasControl(false)
    } else {
      wsService.send({ type: 'control_request', agent_id: agentId })
    }
  }

  // ── Status label ──────────────────────────────────────────
  const statusLabel = error || status
  const statusClass = status === 'Connected' ? 'connected' : error ? 'failed' : ''

  const clipIcon = clipboardStatus === 'syncing' ? '⏳' : clipboardStatus === 'synced' ? '✅' : '📋'

  return (
    <div
      ref={containerRef}
      className={`screen-view ${isFullscreen ? 'fullscreen-mode' : ''}`}
      onMouseMove={handleMouseMove}
    >
      {/* ── Header toolbar (always shown when not fullscreen) ── */}
      {!isFullscreen && (
        <div className="screen-header">
          <div className="screen-title">
            <span className={`stream-dot ${status === 'Connected' ? 'live' : 'dead'}`} />
            <span>Screen View</span>
            <span className={`status-indicator ${statusClass}`}>{statusLabel}</span>
          </div>
          <div className="header-controls">
            <div className="quality-selector">
              {['low', 'balanced', 'high'].map(q => (
                <button key={q} className={`q-btn ${quality === q ? 'active' : ''}`} onClick={() => setQualityMode(q)}>
                  {q.charAt(0).toUpperCase() + q.slice(1)}
                </button>
              ))}
            </div>
            <button className="hdr-btn" onClick={syncClipboard} title="Sync Clipboard">{clipIcon}</button>
            <button className={`hdr-btn control-btn ${hasControl ? 'active' : ''}`} onClick={toggleControl} disabled={status !== 'Connected'}>
              {hasControl ? '🛑 Stop' : '🖱 Control'}
            </button>
            <button className="hdr-btn" onClick={toggleFullscreen} title="Fullscreen">⛶</button>
            <button className="close-btn" onClick={onClose} title="Close">✕</button>
          </div>
        </div>
      )}

      {/* ── Fullscreen floating toolbar (auto-hides) ── */}
      {isFullscreen && (
        <div className={`floating-toolbar ${toolbarVisible ? 'visible' : 'hidden'}`} onMouseEnter={resetToolbarTimer}>
          <span className={`stream-dot ${status === 'Connected' ? 'live' : 'dead'}`} />
          <span className="ft-status">{statusLabel}</span>
          <div className="ft-sep" />
          <div className="quality-selector">
            {['low', 'balanced', 'high'].map(q => (
              <button key={q} className={`q-btn ${quality === q ? 'active' : ''}`} onClick={() => setQualityMode(q)}>
                {q.charAt(0).toUpperCase() + q.slice(1)}
              </button>
            ))}
          </div>
          <button className="hdr-btn" onClick={syncClipboard}>{clipIcon} Clipboard</button>
          <button className={`hdr-btn control-btn ${hasControl ? 'active' : ''}`} onClick={toggleControl}>
            {hasControl ? '🛑 Stop Control' : '🖱 Control'}
          </button>
          <div className="ft-sep" />
          <button className="hdr-btn" onClick={toggleFullscreen} title="Exit Fullscreen">⊠ Exit</button>
        </div>
      )}

      {/* ── Control active badge ── */}
      {hasControl && (
        <div className="control-badge">🔴 Remote Control Active — ESC to exit</div>
      )}

      {/* ── Stream canvas ── */}
      <div className="screen-content">
        {status !== 'Connected' && !error && (
          <div className="loading-overlay">
            <div className="loader-ring" />
            <span>Negotiating P2P connection...</span>
          </div>
        )}
        {error && (
          <div className="loading-overlay error-overlay">
            <span>⚠️ {error}</span>
          </div>
        )}
        <img
          ref={imgRef}
          className={`screen-image ${hasControl ? 'interactive' : ''}`}
          alt="Agent Screen"
          onMouseDown={(e) => handleMouseAction(e, 'down')}
          onMouseUp={(e) => handleMouseAction(e, 'up')}
          onContextMenu={(e) => e.preventDefault()}
          draggable={false}
        />
      </div>
    </div>
  )
}
