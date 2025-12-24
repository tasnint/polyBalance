package strategy

import "polybalance/backend"

// strategy defines the interface that all load balancing strategies must implement

type Strategy interface {
	NextBackend(backends []*backend.Backend) *backend.Backend
}
