package strategy

import (
	"polybalance/backend"
)

type LeastConnections struct{}

func NewLeastConnections() *LeastConnections {
	return &LeastConnections{}
}

func (lc *LeastConnections) NextBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}
	var selected *backend.Backend
	var minConnections int64 = -1 // initialize to -1 to indicate no backend selected yet

	for _, b := range backends {
		if !b.CheckCircuitState() {
			continue // skip unhealthy / circuit-open backends
		}

		active := b.GetActiveConnections()

		// first valid backend
		if selected == nil {
			selected = b
			minConnections = active
			continue
		}

		// pick backend with fewer active connections
		if active < minConnections {
			selected = b
			minConnections = active
		}
	}
	return selected
}
