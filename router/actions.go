package main

import (
	"context"
	"encoding/json"
	"fmt"
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

// deleteNodeHandler is the CRUD Delete: removes a node immediately and
// blacklists its id so mDNS won't silently bring it back. Router-only
// state, no agent involved.
func deleteNodeHandler(table *NodeTable, token string) http.HandlerFunc {
	return mgmtauth.RequireBearer(token, func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !table.Remove(id) {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
	})
}

// createNodeRequest lets an operator register a node by address instead of
// waiting for mDNS — e.g. one on a different subnet where multicast
// doesn't reach. Either cap_url directly, or host[:port] (port defaults to
// the agent's own default, 8090).
type createNodeRequest struct {
	CapURL string `json:"cap_url"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
}

// createNodeHandler is the CRUD Create.
func createNodeHandler(table *NodeTable, token string) http.HandlerFunc {
	return mgmtauth.RequireBearer(token, func(w http.ResponseWriter, r *http.Request) {
		var req createNodeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "malformed JSON body", http.StatusBadRequest)
			return
		}

		capURL := req.CapURL
		if capURL == "" {
			if req.Host == "" {
				http.Error(w, `one of "cap_url" or "host" is required`, http.StatusBadRequest)
				return
			}
			port := req.Port
			if port == 0 {
				port = 8090
			}
			capURL = fmt.Sprintf("http://%s:%d/v0/capabilities", req.Host, port)
		}

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		node, err := table.AddByURL(ctx, capURL)
		if err != nil {
			writeTypedError(w, http.StatusServiceUnavailable, "node_unreachable", err.Error())
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(node)
	})
}

// getNodeHandler is the CRUD single-item Read.
func getNodeHandler(table *NodeTable) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		node, ok := table.Get(r.PathValue("id"))
		if !ok {
			http.Error(w, "node not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(node)
	}
}
