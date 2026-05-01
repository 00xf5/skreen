package metrics

import (
	"sync"
	"time"

	"scon/server/internal/domain"
)

// Collector gathers and aggregates system metrics
type Collector struct {
	mu              sync.RWMutex
	commandStarts   map[string]time.Time // commandID -> start time
	agentLatencies  map[string][]time.Duration
	agentFailures   map[string]int64
	agentSuccesses  map[string]int64
	totalCommands   int64
	totalFailures   int64
	snapshotHistory []domain.MetricsSnapshot
	maxHistory      int
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		commandStarts:   make(map[string]time.Time),
		agentLatencies:  make(map[string][]time.Duration),
		agentFailures:   make(map[string]int64),
		agentSuccesses:  make(map[string]int64),
		snapshotHistory: make([]domain.MetricsSnapshot, 0, 100),
		maxHistory:      100,
	}
}

// RecordCommandStart records the start of a command
func (c *Collector) RecordCommandStart(commandID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commandStarts[commandID] = time.Now()
}

// RecordCommandComplete records command completion with latency
func (c *Collector) RecordCommandComplete(agentID, commandID string, success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Calculate latency
	start, ok := c.commandStarts[commandID]
	if !ok {
		start = time.Now().Add(-time.Second) // Fallback
	}
	latency := time.Since(start)
	delete(c.commandStarts, commandID)

	// Update per-agent metrics
	c.agentLatencies[agentID] = append(c.agentLatencies[agentID], latency)
	if len(c.agentLatencies[agentID]) > 100 {
		// Keep only last 100 measurements
		c.agentLatencies[agentID] = c.agentLatencies[agentID][len(c.agentLatencies[agentID])-100:]
	}

	c.totalCommands++
	if success {
		c.agentSuccesses[agentID]++
	} else {
		c.agentFailures[agentID]++
		c.totalFailures++
	}
}

// GetAgentMetrics returns metrics for a specific agent
func (c *Collector) GetAgentMetrics(agentID string) (total int64, failures int64, avgLatency time.Duration) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total = c.agentSuccesses[agentID] + c.agentFailures[agentID]
	failures = c.agentFailures[agentID]

	// Calculate average latency
	latencies := c.agentLatencies[agentID]
	if len(latencies) > 0 {
		var sum time.Duration
		for _, l := range latencies {
			sum += l
		}
		avgLatency = sum / time.Duration(len(latencies))
	}

	return
}

// GetFailureRate returns the failure rate for an agent (0.0 - 1.0)
func (c *Collector) GetFailureRate(agentID string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	successes := c.agentSuccesses[agentID]
	failures := c.agentFailures[agentID]
	total := successes + failures

	if total == 0 {
		return 0.0
	}
	return float64(failures) / float64(total)
}

// GetSnapshot creates a metrics snapshot
func (c *Collector) GetSnapshot(registry domain.AgentRegistry) domain.MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	snapshot := domain.MetricsSnapshot{
		Timestamp:      time.Now(),
		CommandsTotal:  c.totalCommands,
		CommandsFailed: c.totalFailures,
		AgentLatencies: make(map[string]time.Duration),
		CommandRates:   make(map[string]float64),
	}

	// Get agent counts from registry
	agents, _ := registry.List()
	snapshot.TotalAgents = len(agents)
	for _, agent := range agents {
		if agent.IsOnline {
			snapshot.OnlineAgents++
		} else {
			snapshot.OfflineAgents++
		}
	}

	// Calculate per-agent metrics
	var totalLatency time.Duration
	latencyCount := 0

	for agentID, latencies := range c.agentLatencies {
		if len(latencies) > 0 {
			var sum time.Duration
			for _, l := range latencies {
				sum += l
			}
			avg := sum / time.Duration(len(latencies))
			snapshot.AgentLatencies[agentID] = avg
			totalLatency += avg
			latencyCount++
		}

		snapshot.CommandRates[agentID] = c.GetFailureRate(agentID)
	}

	// Calculate overall average latency
	if latencyCount > 0 {
		snapshot.AvgLatency = totalLatency / time.Duration(latencyCount)
	}

	return snapshot
}

// StoreSnapshot saves a snapshot to history
func (c *Collector) StoreSnapshot(snapshot domain.MetricsSnapshot) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.snapshotHistory = append(c.snapshotHistory, snapshot)
	if len(c.snapshotHistory) > c.maxHistory {
		c.snapshotHistory = c.snapshotHistory[1:]
	}
}

// GetHistory returns stored snapshots
func (c *Collector) GetHistory() []domain.MetricsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]domain.MetricsSnapshot, len(c.snapshotHistory))
	copy(result, c.snapshotHistory)
	return result
}

// Reset clears all metrics (use with caution)
func (c *Collector) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.commandStarts = make(map[string]time.Time)
	c.agentLatencies = make(map[string][]time.Duration)
	c.agentFailures = make(map[string]int64)
	c.agentSuccesses = make(map[string]int64)
	c.totalCommands = 0
	c.totalFailures = 0
}
