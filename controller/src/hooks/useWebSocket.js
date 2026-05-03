import { useEffect, useState, useCallback } from 'react'
import { wsService } from '../services/websocket'

export function useWebSocket() {
  const [connected, setConnected] = useState(false)
  const [agents, setAgents] = useState([])
  const [messages, setMessages] = useState([])
  const [results, setResults] = useState({})

  useEffect(() => {
    // Connect on mount
    wsService.connect()

    // Setup listeners
    const unsubscribeConnected = wsService.on('connected', () => {
      setConnected(true)
    })

    const unsubscribeDisconnected = wsService.on('disconnected', () => {
      setConnected(false)
    })

    const unsubscribeAgents = wsService.on('agents', (msg) => {
      if (msg.data) {
        if (Array.isArray(msg.data)) {
          setAgents(msg.data.map(id => ({ id, online: true })))
        } else if (typeof msg.data === 'object' && msg.agent_id) {
          // Handle single agent metadata update
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
  }, [])

  const uninstallAgent = useCallback((agentId) => {
    return wsService.uninstallAgent(agentId)
  }, [])

  return {
    connected,
    agents,
    results,
    sendCommand,
    togglePersistence,
    refreshAgents,
    uninstallAgent
  }
}
