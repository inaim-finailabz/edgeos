package capability

import (
	"encoding/json"
	"testing"
)

// TestResponseJSONShape guards the wire format against accidental json-tag
// drift from docs/CAPABILITY_SCHEMA.md — that doc is the contract, not this
// file, so if this test fails, fix the tag, not the doc.
func TestResponseJSONShape(t *testing.T) {
	resp := Response{
		Schema: "edgeos/v0",
		Node: Node{
			ID: "a1b2c3d4", Hostname: "orin-nano", Platform: "linux/arm64",
			Accel: "cuda", MemTotalMB: 7620, MemFreeMB: 4100,
		},
		Engine: Engine{Kind: "llama.cpp", Endpoint: "http://192.168.1.42:8080", Healthy: true},
		Models: []Model{
			{ID: "llama-3.1-8b-instruct-q4_k_m", State: "loaded", CtxMax: 8192, TokPerSec: 11.4},
		},
		Load: Load{ActiveRequests: 0, QueueDepth: 0},
	}

	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	for _, key := range []string{"schema", "node", "engine", "models", "load"} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}

	node := got["node"].(map[string]any)
	for _, key := range []string{"id", "hostname", "platform", "accel", "mem_total_mb", "mem_free_mb"} {
		if _, ok := node[key]; !ok {
			t.Errorf("missing node.%s", key)
		}
	}

	engine := got["engine"].(map[string]any)
	for _, key := range []string{"kind", "endpoint", "healthy"} {
		if _, ok := engine[key]; !ok {
			t.Errorf("missing engine.%s", key)
		}
	}

	models := got["models"].([]any)
	if len(models) != 1 {
		t.Fatalf("models length = %d, want 1", len(models))
	}
	model := models[0].(map[string]any)
	for _, key := range []string{"id", "state", "ctx_max", "tok_per_sec"} {
		if _, ok := model[key]; !ok {
			t.Errorf("missing models[0].%s", key)
		}
	}

	load := got["load"].(map[string]any)
	for _, key := range []string{"active_requests", "queue_depth"} {
		if _, ok := load[key]; !ok {
			t.Errorf("missing load.%s", key)
		}
	}
}
