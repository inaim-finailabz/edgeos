package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBuildTools_ReadOnlyByDefault(t *testing.T) {
	c := newFleetClient("http://example.invalid", "")
	tools := buildTools(c)

	names := map[string]bool{}
	for _, e := range tools {
		names[e.def.Name] = true
	}
	for _, want := range []string{"list_nodes", "fleet_status", "ask_fleet"} {
		if !names[want] {
			t.Errorf("missing read-only tool %q", want)
		}
	}
	for _, unwanted := range []string{"add_node", "remove_node", "stop_node", "reload_node"} {
		if names[unwanted] {
			t.Errorf("write tool %q should not be registered with no management token", unwanted)
		}
	}
}

func TestBuildTools_WriteToolsWhenTokenSet(t *testing.T) {
	c := newFleetClient("http://example.invalid", "secret")
	tools := buildTools(c)

	names := map[string]bool{}
	for _, e := range tools {
		names[e.def.Name] = true
	}
	for _, want := range []string{"add_node", "remove_node", "stop_node", "reload_node"} {
		if !names[want] {
			t.Errorf("missing write tool %q when token is set", want)
		}
	}
}

func TestListNodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/nodes" {
			t.Errorf("path = %s, want /v0/nodes", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{"nodes": []any{}, "summary": map[string]int{"total_nodes": 0}})
	}))
	defer srv.Close()

	c := newFleetClient(srv.URL, "")
	tools := buildTools(c)
	entry := findTool(t, tools, "list_nodes")

	result := entry.handler(context.Background(), nil)
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	if !strings.Contains(result.Content[0].Text, `"total_nodes"`) {
		t.Errorf("text = %s, want it to contain the summary", result.Content[0].Text)
	}
}

func TestFleetStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"nodes":   []any{},
			"summary": map[string]int{"total_nodes": 3, "healthy_nodes": 2},
		})
	}))
	defer srv.Close()

	c := newFleetClient(srv.URL, "")
	entry := findTool(t, buildTools(c), "fleet_status")

	result := entry.handler(context.Background(), nil)
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	var summary map[string]int
	if err := json.Unmarshal([]byte(result.Content[0].Text), &summary); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if summary["total_nodes"] != 3 || summary["healthy_nodes"] != 2 {
		t.Errorf("summary = %+v, want total_nodes=3 healthy_nodes=2", summary)
	}
}

func TestAskFleet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s", r.URL.Path)
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != false {
			t.Error("ask_fleet should request stream:false (non-streaming, single result)")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]string{"content": "42"}}},
		})
	}))
	defer srv.Close()

	c := newFleetClient(srv.URL, "")
	entry := findTool(t, buildTools(c), "ask_fleet")

	args, _ := json.Marshal(map[string]string{"model": "m", "prompt": "what is 6*7?"})
	result := entry.handler(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	if result.Content[0].Text != "42" {
		t.Errorf("text = %q, want 42", result.Content[0].Text)
	}
}

func TestAskFleet_MissingArgs(t *testing.T) {
	c := newFleetClient("http://example.invalid", "")
	entry := findTool(t, buildTools(c), "ask_fleet")

	result := entry.handler(context.Background(), json.RawMessage(`{"model":"m"}`))
	if !result.IsError {
		t.Error("want an error when prompt is missing")
	}
}

func TestStopNode(t *testing.T) {
	var gotAuth, gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
	}))
	defer srv.Close()

	c := newFleetClient(srv.URL, "secret-token")
	entry := findTool(t, buildTools(c), "stop_node")

	args, _ := json.Marshal(map[string]string{"id": "n1"})
	result := entry.handler(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if gotPath != "/v0/nodes/n1/actions/stop" || gotMethod != http.MethodPost {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
}

func TestRemoveNode_UsesDelete(t *testing.T) {
	var gotMethod, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]string{"status": "removed"})
	}))
	defer srv.Close()

	c := newFleetClient(srv.URL, "secret-token")
	entry := findTool(t, buildTools(c), "remove_node")

	args, _ := json.Marshal(map[string]string{"id": "n1"})
	result := entry.handler(context.Background(), args)
	if result.IsError {
		t.Fatalf("unexpected error: %+v", result)
	}
	if gotMethod != http.MethodDelete || gotPath != "/v0/nodes/n1" {
		t.Errorf("got %s %s", gotMethod, gotPath)
	}
}

func TestReloadNode_RequiresModelPath(t *testing.T) {
	c := newFleetClient("http://example.invalid", "secret-token")
	entry := findTool(t, buildTools(c), "reload_node")

	args, _ := json.Marshal(map[string]string{"id": "n1"})
	result := entry.handler(context.Background(), args)
	if !result.IsError {
		t.Error("want an error when model_path is missing")
	}
}

func findTool(t *testing.T, tools []toolEntry, name string) toolEntry {
	t.Helper()
	for _, e := range tools {
		if e.def.Name == name {
			return e
		}
	}
	t.Fatalf("tool %q not found", name)
	return toolEntry{}
}
