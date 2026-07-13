package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"edgeos/internal/capability"
)

func fakeCapServer(t *testing.T, resp capability.Response) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestNodeTable_PollAll_Success(t *testing.T) {
	srv := fakeCapServer(t, capability.Response{Schema: "edgeos/v0", Node: capability.Node{ID: "n1"}})
	defer srv.Close()

	table := NewNodeTable(3)
	table.Discovered("n1", srv.URL+"/v0/capabilities")
	table.PollAll(context.Background())

	nodes := table.Snapshot()
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1", len(nodes))
	}
	if nodes[0].Misses != 0 {
		t.Errorf("Misses = %d, want 0", nodes[0].Misses)
	}
	if nodes[0].LastSeen.IsZero() {
		t.Error("LastSeen not set after successful poll")
	}
}

func TestNodeTable_EvictsAfterMissThreshold(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("dead", "http://127.0.0.1:1/v0/capabilities") // nothing listens here

	for i := 0; i < 3; i++ {
		table.PollAll(context.Background())
	}

	if nodes := table.Snapshot(); len(nodes) != 0 {
		t.Fatalf("expected eviction after 3 misses, got %d nodes", len(nodes))
	}
}

func TestNodeTable_SurvivesUnderMissThreshold(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("flaky", "http://127.0.0.1:1/v0/capabilities")

	table.PollAll(context.Background())
	table.PollAll(context.Background())

	nodes := table.Snapshot()
	if len(nodes) != 1 {
		t.Fatalf("expected node to survive 2 misses, got %d nodes", len(nodes))
	}
	if nodes[0].Misses != 2 {
		t.Errorf("Misses = %d, want 2", nodes[0].Misses)
	}
}

func TestNodeTable_Discovered_NoOpIfKnown(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("n1", "http://example.invalid/v0/capabilities")
	table.nodes["n1"].Misses = 2 // simulate an in-progress miss streak

	table.Discovered("n1", "http://example.invalid/v0/capabilities")

	if table.nodes["n1"].Misses != 2 {
		t.Error("rediscovery should not reset an existing node's miss count")
	}
}

func TestNodeTable_Get(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("n1", "http://example.invalid/v0/capabilities")

	if _, ok := table.Get("missing"); ok {
		t.Error("Get(missing) should report not found")
	}
	n, ok := table.Get("n1")
	if !ok || n.ID != "n1" {
		t.Errorf("Get(n1) = (%+v, %v), want a node with ID n1", n, ok)
	}
}

func TestNodeTable_Evict(t *testing.T) {
	table := NewNodeTable(3)
	table.Discovered("n1", "http://example.invalid/v0/capabilities")

	if !table.Evict("n1") {
		t.Error("Evict(n1) should report true when the node was present")
	}
	if _, ok := table.Get("n1"); ok {
		t.Error("n1 should be gone after Evict")
	}
	if table.Evict("n1") {
		t.Error("Evict(n1) again should report false")
	}
}
