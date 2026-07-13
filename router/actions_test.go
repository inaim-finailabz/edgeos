package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newActionsTestMux(table *NodeTable, token string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v0/nodes/{id}/actions/{action}", managementActionsHandler(table, token))
	mux.HandleFunc("POST /v0/nodes/{id}/evict", evictHandler(table, token))
	return mux
}

func TestManagementActionsHandler_RequiresAuth(t *testing.T) {
	mux := newActionsTestMux(NewNodeTable(3), "secret")
	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/n1/actions/stop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestManagementActionsHandler_NodeNotFound(t *testing.T) {
	mux := newActionsTestMux(NewNodeTable(3), "secret")
	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/missing/actions/stop", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestManagementActionsHandler_UnknownAction(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("n1", "http://example.invalid/v0/capabilities")
	mux := newActionsTestMux(table, "secret")
	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/n1/actions/reboot", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestManagementActionsHandler_ProxiesToAgent(t *testing.T) {
	var gotAuth, gotPath, gotBody string
	agent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		gotBody = string(body)
		json.NewEncoder(w).Encode(map[string]string{"status": "reloaded"})
	}))
	defer agent.Close()

	table := NewNodeTable(3)
	table.nodes["n1"] = &NodeState{ID: "n1", CapURL: agent.URL + "/v0/capabilities"}
	mux := newActionsTestMux(table, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/n1/actions/reload", strings.NewReader(`{"model_path":"/m.gguf"}`))
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("agent got Authorization = %q, want Bearer secret", gotAuth)
	}
	if gotPath != "/v0/actions/reload" {
		t.Errorf("agent got path = %q, want /v0/actions/reload", gotPath)
	}
	if gotBody != `{"model_path":"/m.gguf"}` {
		t.Errorf("agent got body = %q", gotBody)
	}
}

func TestManagementActionsHandler_AgentUnreachable(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("n1", "http://127.0.0.1:1/v0/capabilities") // nothing listens here
	mux := newActionsTestMux(table, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/n1/actions/stop", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestEvictHandler(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("n1", "http://example.invalid/v0/capabilities")
	mux := newActionsTestMux(table, "secret")

	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/n1/evict", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, ok := table.Get("n1"); ok {
		t.Error("n1 should be evicted from the table")
	}
}

func TestEvictHandler_NotFound(t *testing.T) {
	mux := newActionsTestMux(NewNodeTable(3), "secret")
	req := httptest.NewRequest(http.MethodPost, "/v0/nodes/missing/evict", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
