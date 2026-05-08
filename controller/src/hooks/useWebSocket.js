import { useEffect, useState, useCallback, useRef } from 'react'
import { wsService } from '../services/websocket'

// Resolve the backend API base URL the same way as websocket.js
function getApiBase() {
  const stored = localStorage.getItem('scon_api_url')
  if (stored) return stored.replace(/\/$/, '')
  const envUrl = import.meta.env.VITE_API_URL
  if (envUrl) return envUrl.replace(/\/$/, '')
  return 'http://localhost:8080'
}

export function useWebSocket() {
  const [connected, setConnected] = useState(false)
  const [agents, setAgents] = useState([])
  const [messages, setMessages] = useState([])
  const [results, setResults] = useState({})
  const pollRef = useRef(null)

  const [metrics, setMetrics] = useState({
    online_agents: 0,
    total_agents: 0,
    active_controllers: 0,
    websocket_load: 0,
    memory_usage_bytes: 0,
    uptime_seconds: 0
  })

  // Fetch agents from REST API and merge with current state
  const fetchAgentsRest = useCallback(async () => {
    try {
      const res = await fetch(`${getApiBase()}/api/agents`)
      if (!res.ok) return
      const json = await res.json()
      if (Array.isArray(json.agents)) {
        setAgents(prev => {
          const liveIds = new Set(prev.filter(a => a.online).map(a => a.id))
          const merged = [...prev]
          json.agents.forEach(apiAgent => {
            const exists = merged.find(a => a.id === apiAgent.id)
            if (!exists) {
              merged.push({ ...apiAgent })
            } else {
              const idx = merged.indexOf(exists)
              merged[idx] = { ...apiAgent, online: liveIds.has(apiAgent.id) ? true : apiAgent.online }
            }
          })
          return merged
        })
      }
    } catch (_) {}
  }, [])

  // Fetch server metrics
  const fetchMetrics = useCallback(async () => {
    try {
      const res = await fetch(`${getApiBase()}/api/metrics`)
      if (res.ok) {
        const json = await res.json()
        setMetrics(json)
      }
    } catch (_) {}
  }, [])

  useEffect(() => {
    // Connect on mount
    wsService.connect()

    // Fetch once immediately
    fetchAgentsRest()
    fetchMetrics()

    // Poll every 10 seconds
    pollRef.current = setInterval(() => {
      fetchAgentsRest()
      fetchMetrics()
    }, 10000)

    // ... rest of useEffect

    // Setup listeners
    const unsubscribeConnected = wsService.on('connected', () => {
      setConnected(true)
      fetchAgentsRest() // refresh on connect
    })

    const unsubscribeDisconnected = wsService.on('disconnected', () => {
      setConnected(false)
    })

    const unsubscribeAgents = wsService.on('agents', (msg) => {
      if (msg.data) {
        if (Array.isArray(msg.data)) {
          // Full list from server — mark all as online, merge with existing
          setAgents(prev => {
            const liveIds = new Set(msg.data)
            const updated = prev.map(a => ({ ...a, online: liveIds.has(a.id) }))
            msg.data.forEach(id => {
              if (!updated.find(a => a.id === id)) {
                updated.push({ id, online: true })
              }
            })
            return updated
          })
        } else if (typeof msg.data === 'object' && msg.agent_id) {
          // Single agent metadata update
          setAgents(prev => {
            if (prev.find(a => a.id === msg.agent_id)) {
              return prev.map(a => a.id === msg.agent_id ? { ...a, ...msg.data, online: true } : a)
            }
            return [...prev, { id: msg.agent_id, ...msg.data, online: true }]
          })
        }
      }
    })

    const unsubscribeResult = wsService.on('result', (msg) => {
      if (msg.agent_id) {
        setResults(prev => ({
          ...prev,
          [msg.agent_id]: {
            output: msg.output,
            error: msg.error,
            timestamp: Date.now()
          }
        }))
      }
    })

    const unsubscribeError = wsService.on('error', (err) => {
      console.error('WebSocket error:', err)
    })

    // Cleanup
    return () => {
      clearInterval(pollRef.current)
      unsubscribeConnected()
      unsubscribeDisconnected()
      unsubscribeAgents()
      unsubscribeResult()
      unsubscribeError()
    }
  }, [])

  const sendCommand = useCallback((agentId, command) => {
    return wsService.sendCommand(agentId, command)
  }, [])

  const togglePersistence = useCallback((agentId, enabled) => {
    return wsService.togglePersistence(agentId, enabled)
  }, [])

  const refreshAgents = useCallback(() => {
    wsService.send({ type: 'list_agents' })
    fetchAgentsRest() // also hit the REST API
  }, [fetchAgentsRest])

  const uninstallAgent = useCallback((agentId) => {
    return wsService.uninstallAgent(agentId)
  }, [])

  return {
    connected,
    agents,
    metrics,
    results,
    sendCommand,
    togglePersistence,
    refreshAgents,
    uninstallAgent
  }
}
