// strategy: ensures that the same client/request key always maps to the same backend server — unless that backend goes away
// useful for sticky sessions, caching, etc.

package strategy

import (
	"crypto/sha256"
	"encoding/binary"
	"polybalance/backend"
	"sort"
	"strconv"
)

// ringEntry represents one virtual node on the hash ring
type ringEntry struct {
	Hash uint64
	Idx  int // index into the original backend slice
}

type ConsistentHash struct {
	ring         []ringEntry
	virtualNodes int
}

func NewConsistentHash(virtualNodes int) *ConsistentHash {
	if virtualNodes <= 0 {
		virtualNodes = 50
	}
	return &ConsistentHash{
		virtualNodes: virtualNodes,
	}
}

// --- hashing helper ---
func hashKey(s string) uint64 {
	h := sha256.Sum256([]byte(s))
	return binary.BigEndian.Uint64(h[:8])
}

// --- build the hash ring ---
func (c *ConsistentHash) buildRing(backends []*backend.Backend) {
	var ring []ringEntry

	for i, b := range backends {
		if !b.CheckCircuitState() {
			continue // skip unhealthy / circuit-open backends
		}

		for v := 0; v < c.virtualNodes; v++ {
			key := b.URL.String() + "#" + strconv.Itoa(v) // FIXED: string concat
			hash := hashKey(key)
			ring = append(ring, ringEntry{Hash: hash, Idx: i})
		}
	}

	// Must sort ring for binary search
	sort.Slice(ring, func(i, j int) bool {
		return ring[i].Hash < ring[j].Hash
	})

	c.ring = ring
}

// --- Strategy Interface Implementation ---
func (c *ConsistentHash) NextBackend(backends []*backend.Backend) *backend.Backend {
	if len(backends) == 0 {
		return nil
	}

	// Rebuild ring if empty or backend count changed
	if len(c.ring) == 0 || !c.validateRing(backends) {
		c.buildRing(backends)
	}

	if len(c.ring) == 0 {
		return nil // no healthy backend
	}

	// TODO: Replace with real request key (e.g., client IP, session cookie, JWT, user ID)
	key := "default"
	h := hashKey(key)

	// Binary search on ring
	idx := sort.Search(len(c.ring), func(i int) bool {
		return c.ring[i].Hash >= h
	})

	if idx == len(c.ring) {
		idx = 0 // wrap around
	}

	beIdx := c.ring[idx].Idx
	if beIdx < 0 || beIdx >= len(backends) {
		return nil // safety guard
	}

	return backends[beIdx]
}

// Ensure ring corresponds to current backend set
func (c *ConsistentHash) validateRing(backends []*backend.Backend) bool {
	// Simple heuristic: if ring too small, rebuild
	expected := 0
	for _, b := range backends {
		if b.CheckCircuitState() {
			expected += c.virtualNodes
		}
	}
	return len(c.ring) == expected
}
