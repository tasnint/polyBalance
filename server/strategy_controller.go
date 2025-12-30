package server

import (
        "sync"

        "polybalance/strategy"
)

type StrategyController struct {
        mu       sync.RWMutex
        current  strategy.Strategy
        name     string
}

func NewStrategyController(initial strategy.Strategy, name string) *StrategyController {
        return &StrategyController{
                current: initial,
                name:    name,
        }
}

func (sc *StrategyController) Current() strategy.Strategy {
        sc.mu.RLock()
        defer sc.mu.RUnlock()
        return sc.current
}

func (sc *StrategyController) Name() string {
        sc.mu.RLock()
        defer sc.mu.RUnlock()
        return sc.name
}

func (sc *StrategyController) Set(name string) bool {
        var newStrategy strategy.Strategy

        switch name {
        case "round_robin":
                newStrategy = strategy.NewRoundRobin()
        case "least_connections":
                newStrategy = strategy.NewLeastConnections()
        case "latency":
                newStrategy = strategy.NewLatencyStrategy()
        case "consistent_hash":
                newStrategy = strategy.NewConsistentHash(100)
        default:
                return false
        }

        sc.mu.Lock()
        defer sc.mu.Unlock()
        sc.current = newStrategy
        sc.name = name
        return true
}

func (sc *StrategyController) AvailableStrategies() []string {
        return []string{"round_robin", "least_connections", "latency", "consistent_hash"}
}
