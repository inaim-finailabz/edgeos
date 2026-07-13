package main

import "sync/atomic"

// RequestStats are fleet-wide KPI counters, reset on restart (not
// persisted — good enough for the dashboard's "since this router started"
// numbers in v0).
type RequestStats struct {
	Local  atomic.Int64
	Cloud  atomic.Int64
	Failed atomic.Int64
}

// Summary is the aggregate view the dashboard's KPI strip renders,
// embedded alongside the node list in GET /v0/nodes.
type Summary struct {
	TotalNodes          int     `json:"total_nodes"`
	HealthyNodes        int     `json:"healthy_nodes"`
	TotalActiveRequests int     `json:"total_active_requests"`
	TotalTokPerSec      float64 `json:"total_tok_per_sec"`
	DistinctModels      int     `json:"distinct_models"`
	RequestsServedLocal int64   `json:"requests_served_local"`
	RequestsServedCloud int64   `json:"requests_served_cloud"`
	RequestsFailed      int64   `json:"requests_failed"`
}

func computeSummary(nodes []NodeState, stats *RequestStats) Summary {
	var s Summary
	models := make(map[string]bool)

	s.TotalNodes = len(nodes)
	for _, n := range nodes {
		if n.Cap.Engine.Healthy {
			s.HealthyNodes++
		}
		s.TotalActiveRequests += n.Cap.Load.ActiveRequests
		for _, m := range n.Cap.Models {
			if m.State == "loaded" {
				s.TotalTokPerSec += m.TokPerSec
				models[m.ID] = true
			}
		}
	}
	s.DistinctModels = len(models)

	if stats != nil {
		s.RequestsServedLocal = stats.Local.Load()
		s.RequestsServedCloud = stats.Cloud.Load()
		s.RequestsFailed = stats.Failed.Load()
	}
	return s
}
