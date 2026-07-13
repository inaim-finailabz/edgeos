package main

import (
	"encoding/json"
	"net/http"
)

// nodesHandler serves GET /v0/nodes: the router's live view of the fleet,
// for the CLI's `edgeos nodes` and for debugging. See docs/CAPABILITY_SCHEMA.md.
func nodesHandler(table *NodeTable) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"nodes": table.Snapshot()})
	}
}
