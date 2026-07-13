package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"edgeos/internal/capability"
)

type fakeEngine struct {
	snap     EngineSnapshot
	endpoint string
}

func (f fakeEngine) Snapshot() EngineSnapshot { return f.snap }
func (f fakeEngine) Endpoint() string         { return f.endpoint }

func TestCapabilitiesHandler_GET_EngineLoaded(t *testing.T) {
	s := &server{
		nodeID: "a1b2c3d4",
		accel:  "metal",
		engine: fakeEngine{
			endpoint: "http://127.0.0.1:8080",
			snap: EngineSnapshot{
				Healthy:        true,
				ModelState:     "loaded",
				ModelID:        "qwen3-1.7b-q4_k_m.gguf",
				CtxMax:         4096,
				TokPerSec:      42.5,
				ActiveRequests: 1,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/capabilities", nil)
	rec := httptest.NewRecorder()
	s.capabilitiesHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var resp capability.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Schema != "edgeos/v0" {
		t.Errorf("schema = %q, want edgeos/v0", resp.Schema)
	}
	if resp.Node.ID != "a1b2c3d4" {
		t.Errorf("node.id = %q, want a1b2c3d4", resp.Node.ID)
	}
	if !resp.Engine.Healthy {
		t.Error("engine.healthy = false, want true")
	}
	if len(resp.Models) != 1 || resp.Models[0].TokPerSec != 42.5 {
		t.Errorf("models = %+v, want one model with tok_per_sec 42.5", resp.Models)
	}
	if resp.Load.ActiveRequests != 1 {
		t.Errorf("load.active_requests = %d, want 1", resp.Load.ActiveRequests)
	}
}

func TestCapabilitiesHandler_GET_NoEngine(t *testing.T) {
	s := &server{
		nodeID: "a1b2c3d4",
		accel:  "cpu",
		engine: fakeEngine{
			endpoint: "http://127.0.0.1:8080",
			snap:     EngineSnapshot{ModelState: "disabled"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v0/capabilities", nil)
	rec := httptest.NewRecorder()
	s.capabilitiesHandler(rec, req)

	var resp capability.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Engine.Healthy {
		t.Error("engine.healthy = true, want false with no engine configured")
	}
	if len(resp.Models) != 0 {
		t.Errorf("models = %+v, want empty", resp.Models)
	}
}

func TestCapabilitiesHandler_MethodNotAllowed(t *testing.T) {
	s := &server{engine: fakeEngine{}}
	req := httptest.NewRequest(http.MethodPost, "/v0/capabilities", nil)
	rec := httptest.NewRecorder()

	s.capabilitiesHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
