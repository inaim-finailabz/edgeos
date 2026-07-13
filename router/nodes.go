package main

import (
	"encoding/json"
	"net/http"
)

// nodesHandler serves GET /v0/nodes: the router's live view of the fleet
// plus a fleet-wide KPI summary, for the CLI's `edgeos nodes`, the
// dashboard, and debugging. See docs/CAPABILITY_SCHEMA.md.
func nodesHandler(table *NodeTable, stats *RequestStats) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nodes := table.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"nodes":   nodes,
			"summary": computeSummary(nodes, stats),
		})
	}
}
