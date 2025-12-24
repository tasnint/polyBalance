package strategy

import (
	"polybalance/backend"
	"sync/atomic"
)

// strategy: Every server gets the same number of requests over time
// cycle through the list of backends in order

type RoundRobin struct {
	counter uint64
}

func NewRoundRobin() *RoundRobin {
	return &RoundRobin{}
}

func (rr *RoundRobin) NextBackend(backends []*backend.Backend) *backend.Backend {
	n := len(backends)
	if n == 0 {
		return nil
	}

	// find next healthy round robin server:
	for attempt := 0; attempt < n; attempt++ {
		idx := atomic.AddUint64(&rr.counter, 1) % uint64(n)
		b := backends[idx]

		if b.CheckCircuitState() {
			return b
		}
	}
	return nil
}
