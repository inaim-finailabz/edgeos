package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"edgeos/internal/capability"
)

func TestCapabilitiesHandler_GET(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v0/capabilities", nil)
	rec := httptest.NewRecorder()

	capabilitiesHandler(rec, req)

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
	if resp.Node.ID == "" {
		t.Error("node.id is empty")
	}
	if resp.Engine.Endpoint == "" {
		t.Error("engine.endpoint is empty")
	}
	if len(resp.Models) == 0 {
		t.Error("models is empty")
	}
}

func TestCapabilitiesHandler_MethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v0/capabilities", nil)
	rec := httptest.NewRecorder()

	capabilitiesHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
