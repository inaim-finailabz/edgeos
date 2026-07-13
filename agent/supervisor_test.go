package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// fakeEngineServer emulates just enough of llama-server's HTTP surface
// (/health, /completion, /slots) to test the supervisor's polling and
// benchmark logic without needing the real binary — this is what makes
// these tests portable to CI runners that don't have llama-server installed.
func fakeEngineServer(t *testing.T, healthy bool, tokPerSec float64, ctxMax int, processing bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/completion", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"timings": map[string]any{"predicted_per_second": tokPerSec},
		})
	})
	mux.HandleFunc("/slots", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]slot{{NCtx: ctxMax, IsProcessing: processing}})
	})
	return httptest.NewServer(mux)
}

func supervisorFor(t *testing.T, srv *httptest.Server) *Supervisor {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatalf("split addr: %v", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("parse port: %v", err)
	}
	return NewSupervisor(SupervisorConfig{
		ModelPath:  "test-model.gguf",
		EngineHost: host,
		EnginePort: port,
	})
}

func TestSupervisor_WaitForHealthy(t *testing.T) {
	srv := fakeEngineServer(t, true, 0, 0, false)
	defer srv.Close()
	s := supervisorFor(t, srv)

	if err := s.waitForHealthy(context.Background(), time.Second); err != nil {
		t.Fatalf("waitForHealthy: %v", err)
	}
}

func TestSupervisor_WaitForHealthy_TimesOut(t *testing.T) {
	srv := fakeEngineServer(t, false, 0, 0, false)
	defer srv.Close()
	s := supervisorFor(t, srv)

	if err := s.waitForHealthy(context.Background(), 200*time.Millisecond); err == nil {
		t.Fatal("waitForHealthy: want error when engine never reports healthy")
	}
}

func TestSupervisor_RunBenchmark(t *testing.T) {
	srv := fakeEngineServer(t, true, 42.5, 4096, false)
	defer srv.Close()
	s := supervisorFor(t, srv)

	if err := s.runBenchmark(context.Background()); err != nil {
		t.Fatalf("runBenchmark: %v", err)
	}

	snap := s.Snapshot()
	if snap.ModelState != "loaded" {
		t.Errorf("ModelState = %q, want loaded", snap.ModelState)
	}
	if !snap.Healthy {
		t.Error("Healthy = false, want true")
	}
	if snap.TokPerSec != 42.5 {
		t.Errorf("TokPerSec = %v, want 42.5", snap.TokPerSec)
	}
	if snap.CtxMax != 4096 {
		t.Errorf("CtxMax = %d, want 4096", snap.CtxMax)
	}
}

func TestSupervisor_PollOnce_ActiveRequests(t *testing.T) {
	srv := fakeEngineServer(t, true, 0, 0, true)
	defer srv.Close()
	s := supervisorFor(t, srv)

	if err := s.pollOnce(context.Background()); err != nil {
		t.Fatalf("pollOnce: %v", err)
	}

	snap := s.Snapshot()
	if snap.ActiveRequests != 1 {
		t.Errorf("ActiveRequests = %d, want 1", snap.ActiveRequests)
	}
	if snap.QueueDepth != 0 {
		t.Errorf("QueueDepth = %d, want 0 (no native metric in v0)", snap.QueueDepth)
	}
}

func TestNewSupervisor_NoModelIsDisabled(t *testing.T) {
	s := NewSupervisor(SupervisorConfig{})
	snap := s.Snapshot()
	if snap.ModelState != "disabled" {
		t.Errorf("ModelState = %q, want disabled", snap.ModelState)
	}
	if len(snap.modelForCapability()) != 0 {
		t.Error("modelForCapability() should be empty when disabled")
	}
}
