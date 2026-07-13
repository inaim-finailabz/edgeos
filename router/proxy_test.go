package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"edgeos/internal/capability"
)

func tableWithNode(id, engineEndpoint string, models ...capability.Model) *NodeTable {
	table := NewNodeTable(3)
	table.nodes[id] = &NodeState{
		ID: id,
		Cap: capability.Response{
			Node:   capability.Node{ID: id},
			Engine: capability.Engine{Endpoint: engineEndpoint, Healthy: true},
			Models: models,
		},
	}
	return table
}

func TestChatCompletionsHandler_RoutesToLocalNode(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("engine got path %q", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]string{"ok": "local"})
	}))
	defer engine.Close()

	table := tableWithNode("n1", engine.URL, capability.Model{ID: "llama-8b", State: "loaded", CtxMax: 8192, TokPerSec: 50})
	proxy := NewProxy(table, "", "", &RequestStats{})

	body := `{"model":"llama-8b","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ChatCompletionsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"local"`) {
		t.Errorf("body = %s, want response forwarded from local engine", rec.Body.String())
	}
}

func TestChatCompletionsHandler_FallsBackToCloud(t *testing.T) {
	cloud := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("Authorization = %q, want Bearer secret", got)
		}
		json.NewEncoder(w).Encode(map[string]string{"ok": "cloud"})
	}))
	defer cloud.Close()

	table := NewNodeTable(3) // empty: nothing local qualifies
	proxy := NewProxy(table, cloud.URL, "secret", &RequestStats{})

	body := `{"model":"anything","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ChatCompletionsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cloud"`) {
		t.Errorf("body = %s, want response forwarded from cloud", rec.Body.String())
	}
}

func TestChatCompletionsHandler_TypedErrorWhenNoRoute(t *testing.T) {
	table := NewNodeTable(3)
	proxy := NewProxy(table, "", "", &RequestStats{}) // no cloud fallback configured either

	body := `{"model":"anything","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ChatCompletionsHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header not set")
	}
	var e apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if e.Error.Type != "no_node_available" {
		t.Errorf("error.type = %q, want no_node_available", e.Error.Type)
	}
}

func TestChatCompletionsHandler_TypedErrorWhenUpstreamUnreachable(t *testing.T) {
	table := tableWithNode("n1", "http://127.0.0.1:1", // nothing listens here
		capability.Model{ID: "llama-8b", State: "loaded", CtxMax: 8192, TokPerSec: 50})
	proxy := NewProxy(table, "", "", &RequestStats{})

	body := `{"model":"llama-8b","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ChatCompletionsHandler(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header not set")
	}
}

func TestChatCompletionsHandler_StreamsSSEIncrementally(t *testing.T) {
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher := w.(http.Flusher)
		for _, chunk := range []string{"data: one\n\n", "data: two\n\n", "data: [DONE]\n\n"} {
			w.Write([]byte(chunk))
			flusher.Flush()
		}
	}))
	defer engine.Close()

	table := tableWithNode("n1", engine.URL, capability.Model{ID: "llama-8b", State: "loaded", CtxMax: 8192, TokPerSec: 50})
	proxy := NewProxy(table, "", "", &RequestStats{})

	body := `{"model":"llama-8b","messages":[],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ChatCompletionsHandler(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want text/event-stream", ct)
	}

	scanner := bufio.NewScanner(bytes.NewReader(rec.Body.Bytes()))
	var lines []string
	for scanner.Scan() {
		if l := scanner.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	want := []string{"data: one", "data: two", "data: [DONE]"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines %v, want %v", len(lines), lines, want)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, lines[i], want[i])
		}
	}
}

func TestChatCompletionsHandler_MethodNotAllowed(t *testing.T) {
	proxy := NewProxy(NewNodeTable(3), "", "", &RequestStats{})
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	rec := httptest.NewRecorder()
	proxy.ChatCompletionsHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
