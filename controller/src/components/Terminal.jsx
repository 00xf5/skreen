import { useState, useRef, useEffect } from 'react'
import './Terminal.css'

export function Terminal({ agentId, result, onCommand }) {
  const [command, setCommand] = useState('')
  const [history, setHistory] = useState([])
  const outputRef = useRef(null)
  const inputRef = useRef(null)

  // Auto-scroll to bottom when output changes
  useEffect(() => {
    if (outputRef.current) {
      outputRef.current.scrollTop = outputRef.current.scrollHeight
    }
  }, [history, result])

  // Focus input on mount
  useEffect(() => {
    inputRef.current?.focus()
  }, [])

  const handleSubmit = (e) => {
    e.preventDefault()
    if (!command.trim()) return

    // Add to history
    setHistory(prev => [...prev, { type: 'input', text: command }])

    // Send command
    onCommand(agentId, command)

    // Clear input
    setCommand('')
  }

  // Add result to history when it arrives
  useEffect(() => {
    if (result && result.timestamp) {
      const lastHistory = history[history.length - 1]
      if (lastHistory?.type === 'input' && !lastHistory.acknowledged) {
        setHistory(prev => [
          ...prev.slice(0, -1),
          { ...lastHistory, acknowledged: true },
          { 
            type: 'output', 
            text: result.output || result.error || '[No output]',
            isError: !!result.error,
            timestamp: result.timestamp 
          }
        ])
      }
    }
  }, [result])

  return (
    <div className="terminal">
      <div className="terminal-header">
        <span className="terminal-title">Terminal</span>
        <span className="terminal-agent">{agentId}</span>
      </div>

      <div className="terminal-output" ref={outputRef}>
        {history.length === 0 && (
          <div className="terminal-welcome">
            <p>Connected to {agentId}</p>
            <p>Type a command and press Enter</p>
          </div>
        )}

        {history.map((item, idx) => (
          <div key={idx} className={`terminal-line ${item.type}`}>
            {item.type === 'input' ? (
              <>
                <span className="prompt">$</span>
                <span className="command">{item.text}</span>
              </>
            ) : (
              <pre className={item.isError ? 'error' : ''}>
                {item.text}
              </pre>
            )}
          </div>
        ))}
      </div>

      <form className="terminal-input" onSubmit={handleSubmit}>
        <span className="prompt">$</span>
        <input
          ref={inputRef}
          type="text"
          value={command}
          onChange={(e) => setCommand(e.target.value)}
          placeholder="Enter command..."
          autoComplete="off"
          spellCheck={false}
        />
      </form>
    </div>
  )
}
