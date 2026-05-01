// WebSocket service for server communication

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080'
const WS_URL = API_URL.replace('http', 'ws') + '/ws/controller'

class WebSocketService {
  constructor() {
    this.ws = null
    this.listeners = new Map()
    this.reconnectAttempts = 0
    this.maxReconnectAttempts = 10
    this.reconnectDelay = 1000
    this.isConnecting = false
    this.messageQueue = []
  }

  connect() {
    if (this.isConnecting || this.ws?.readyState === WebSocket.OPEN) {
      return
    }

    this.isConnecting = true
    console.log('Connecting to', WS_URL)

    try {
      this.ws = new WebSocket(WS_URL)

      this.ws.onopen = () => {
        console.log('WebSocket connected')
        this.isConnecting = false
        this.reconnectAttempts = 0
        this.reconnectDelay = 1000

        // Send any queued messages
        while (this.messageQueue.length > 0) {
          const msg = this.messageQueue.shift()
          this.send(msg)
        }

        // Request agent list
        this.send({ type: 'list_agents' })

        this.emit('connected', null)
      }

      this.ws.onmessage = (event) => {
        try {
          const msg = JSON.parse(event.data)
          this.emit('message', msg)
          this.emit(msg.type, msg)
        } catch (err) {
          console.error('Failed to parse message:', err)
        }
      }

      this.ws.onclose = () => {
        console.log('WebSocket closed')
        this.isConnecting = false
        this.emit('disconnected', null)
        this.attemptReconnect()
      }

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error)
        this.emit('error', error)
      }
    } catch (err) {
      console.error('Failed to create WebSocket:', err)
      this.isConnecting = false
      this.attemptReconnect()
    }
  }

  disconnect() {
    this.reconnectAttempts = this.maxReconnectAttempts // Prevent reconnect
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  send(message) {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) {
      this.messageQueue.push(message)
      return false
    }

    try {
      this.ws.send(JSON.stringify(message))
      return true
    } catch (err) {
      console.error('Failed to send message:', err)
      this.messageQueue.push(message)
      return false
    }
  }

  sendCommand(agentId, command) {
    return this.send({
      type: 'command',
      agent_id: agentId,
      command: command
    })
  }

  togglePersistence(agentId, enabled) {
    return this.send({
      type: 'toggle_persistence',
      agent_id: agentId,
      data: { enabled }
    })
  }

  uninstallAgent(agentId) {
    return this.send({
      type: 'uninstall',
      agent_id: agentId
    })
  }

  attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('Max reconnect attempts reached')
      return
    }

    this.reconnectAttempts++
    const delay = Math.min(this.reconnectDelay * Math.pow(1.5, this.reconnectAttempts - 1), 30000)
    
    console.log(`Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`)
    
    setTimeout(() => {
      this.connect()
    }, delay)
  }

  on(event, callback) {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, [])
    }
    this.listeners.get(event).push(callback)

    // Return unsubscribe function
    return () => {
      const callbacks = this.listeners.get(event)
      const index = callbacks.indexOf(callback)
      if (index > -1) {
        callbacks.splice(index, 1)
      }
    }
  }

  emit(event, data) {
    const callbacks = this.listeners.get(event)
    if (callbacks) {
      callbacks.forEach(cb => {
        try {
          cb(data)
        } catch (err) {
          console.error('Error in event listener:', err)
        }
      })
    }
  }

  isConnected() {
    return this.ws?.readyState === WebSocket.OPEN
  }
}

// Singleton instance
export const wsService = new WebSocketService()
