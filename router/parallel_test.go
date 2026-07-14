package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"edgeos/internal/capability"
)

func TestParallelCompletionsHandler_RunsAllConcurrently(t *testing.T) {
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := concurrent.Add(1)
		for {
			cur := maxConcurrent.Load()
			if n <= cur || maxConcurrent.CompareAndSwap(cur, n) {
				break
			}
		}
		defer concurrent.Add(-1)
		// Without this, three fast in-process round trips can complete
		// sequentially fast enough to never actually overlap, making the
		// concurrency assertion below flaky rather than a real check.
		time.Sleep(50 * time.Millisecond)

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["stream"] != false {
			t.Error("sub-request should be forced to stream:false")
		}
		json.NewEncoder(w).Encode(map[string]string{"ok": "local"})
	}))
	defer engine.Close()

	table := tableWithNode("n1", engine.URL, capability.Model{ID: "llama-8b", State: "loaded", CtxMax: 8192, TokPerSec: 50})
	proxy := NewProxy(table, "", "", &RequestStats{})

	body := `{"requests":[
		{"model":"llama-8b","messages":[{"role":"user","content":"one"}],"stream":true},
		{"model":"llama-8b","messages":[{"role":"user","content":"two"}]},
		{"model":"llama-8b","messages":[{"role":"user","content":"three"}]}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/v0/parallel-completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp parallelResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("got %d results, want 3", len(resp.Results))
	}
	for i, r := range resp.Results {
		if r.Index != i || r.Status != "ok" {
			t.Errorf("result[%d] = %+v", i, r)
		}
		if !strings.Contains(string(r.Response), `"local"`) {
			t.Errorf("result[%d].response = %s", i, r.Response)
		}
	}
	if maxConcurrent.Load() < 2 {
		t.Errorf("maxConcurrent = %d, want requests to actually overlap (>=2)", maxConcurrent.Load())
	}
}

func TestParallelCompletionsHandler_PartialFailure(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"ok": "local"})
	}))
	defer engine.Close()

	table := tableWithNode("n1", engine.URL, capability.Model{ID: "llama-8b", State: "loaded", CtxMax: 8192, TokPerSec: 50})
	proxy := NewProxy(table, "", "", &RequestStats{})

	// second request asks for a model nobody has loaded -> no_node_available
	body := `{"requests":[
		{"model":"llama-8b","messages":[{"role":"user","content":"ok"}]},
		{"model":"nonexistent","messages":[{"role":"user","content":"fails"}]}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/v0/parallel-completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	var resp parallelResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Results[0].Status != "ok" {
		t.Errorf("results[0].status = %q, want ok", resp.Results[0].Status)
	}
	if resp.Results[1].Status != "error" || resp.Results[1].Error.Type != "no_node_available" {
		t.Errorf("results[1] = %+v, want a no_node_available error", resp.Results[1])
	}
}

func TestParallelCompletionsHandler_EmptyRequests(t *testing.T) {
	proxy := NewProxy(NewNodeTable(3), "", "", &RequestStats{})
	req := httptest.NewRequest(http.MethodPost, "/v0/parallel-completions", strings.NewReader(`{"requests":[]}`))
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestParallelCompletionsHandler_TooMany(t *testing.T) {
	proxy := NewProxy(NewNodeTable(3), "", "", &RequestStats{})
	reqs := make([]string, maxParallelRequests+1)
	for i := range reqs {
		reqs[i] = `{"model":"m","messages":[]}`
	}
	body := `{"requests":[` + strings.Join(reqs, ",") + `]}`
	req := httptest.NewRequest(http.MethodPost, "/v0/parallel-completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestParallelCompletionsHandler_MethodNotAllowed(t *testing.T) {
	proxy := NewProxy(NewNodeTable(3), "", "", &RequestStats{})
	req := httptest.NewRequest(http.MethodGet, "/v0/parallel-completions", nil)
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestParallelCompletionsHandler_MalformedJSON(t *testing.T) {
	proxy := NewProxy(NewNodeTable(3), "", "", &RequestStats{})
	req := httptest.NewRequest(http.MethodPost, "/v0/parallel-completions", strings.NewReader(`not json`))
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestParallelCompletionsHandler_FallsBackToCloud(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"ok": "cloud"})
	}))
	defer cloud.Close()

	table := NewNodeTable(3) // empty: nothing local qualifies
	proxy := NewProxy(table, cloud.URL, "secret", &RequestStats{})

	body := `{"requests":[{"model":"anything","messages":[]}]}`
	req := httptest.NewRequest(http.MethodPost, "/v0/parallel-completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ParallelCompletionsHandler(rec, req)

	var resp parallelResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Results[0].Status != "ok" || !strings.Contains(string(resp.Results[0].Response), "cloud") {
		t.Errorf("result = %+v, want cloud fallback success", resp.Results[0])
	}
}
