package strategy

import (
	"polybalance/backend"
	"time"
)

// Latency picks the backend with the lowest average latecy.
// If a backend has no recorded latency, it is assigned a default high latency value.

type LatencyStrategy struct{}

func NewLatencyStrategy() *LatencyStrategy {
	return &LatencyStrategy{}
}

func (l *LatencyStrategy) NextBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	var selected *backend.Backend // to select the backend with lowest latency
	var bestLatency time.Duration // store the best (lowest) latency found

	for _, b := range backends {
		// b is a pointer to backend.Backend
		if !b.CheckCircuitState() {
			continue // skip unhealthy / circuit-open backends
		}
		latency := b.GetAverageLatency()

		// if no backend selected yet, choose the first healthy one as selected
		if selected == nil {
			selected = b
			bestLatency = latency
			continue
		}

		// if this backend's latency is lower than the best found so far, select it
		if latency < bestLatency {
			selected = b
			bestLatency = latency
		}
	}
	return selected // return the selected backend
}
