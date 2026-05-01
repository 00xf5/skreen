import './Agents.css'

export function Agents({ agents, selected, onSelect }) {
  const agentList = agents || []

  return (
    <div className="agents">
      <div className="agents-header">
        <h2>Agents</h2>
        <span className="count">{agentList.length}</span>
      </div>

      {agentList.length === 0 ? (
        <div className="agents-empty">
          <p>No agents connected</p>
          <span>Waiting for connections...</span>
        </div>
      ) : (
        <ul className="agents-list">
          {agentList.map(agent => (
            <li
              key={agent.id}
              className={`agent-item ${selected === agent.id ? 'selected' : ''}`}
              onClick={() => onSelect(agent.id)}
            >
              <div className="agent-status">
                <span className={`status-dot ${agent.online ? 'online' : 'offline'}`} />
              </div>
              <div className="agent-info">
                <span className="agent-id">{agent.id}</span>
                <span className="agent-meta">
                  {agent.online ? 'Online' : 'Offline'}
                </span>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
