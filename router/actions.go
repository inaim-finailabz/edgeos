package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"edgeos/internal/mgmtauth"
)

// managementActionsHandler proxies a dashboard action for one node to that
// node's agent. It requires the same shared -management-token as the
// agents do, and forwards it on — operators must configure the token
// identically across the fleet in v0 (see docs/CAPABILITY_SCHEMA.md).
func managementActionsHandler(table *NodeTable, token string) http.HandlerFunc {
	client := &http.Client{Timeout: 90 * time.Second}
	return mgmtauth.RequireBearer(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		action := r.PathValue("action")
		if action != "stop" && action != "reload" {
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}

		node, ok := table.Get(id)
		if !ok {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}

		agentBase := strings.TrimSuffix(node.CapURL, "/v0/capabilities")
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "could not read request body", http.StatusBadRequest)
			return
		}

		outReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, agentBase+"/v0/actions/"+action, strings.NewReader(string(body)))
		if err != nil {
			http.Error(w, "could not build upstream request", http.StatusInternalServerError)
			return
		}
		outReq.Header.Set("Content-Type", "application/json")
		outReq.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(outReq)
		if err != nil {
			writeTypedError(w, http.StatusServiceUnavailable, "agent_unreachable", err.Error())
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	})
}

// evictHandler removes a node from the table immediately, bypassing the
// miss-threshold — this is router-only state, no agent involved.
func evictHandler(table *NodeTable, token string) http.HandlerFunc {
	return mgmtauth.RequireBearer(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !table.Evict(id) {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "evicted"})
	})
}
