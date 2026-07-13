package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
)

// apiError matches the OpenAI error envelope so existing OpenAI clients
// parse it without special-casing EdgeOS.
type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func writeTypedError(w http.ResponseWriter, status int, errType, message string) {
	if status == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "1")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var body apiError
	body.Error.Type = errType
	body.Error.Message = message
	json.NewEncoder(w).Encode(body)
}

// Proxy serves POST /v1/chat/completions: score the fleet, forward the
// request to the winning node's engine (or the cloud fallback), and stream
// the response straight through. It never buffers a full streamed response
// and never silently restarts a stream that's already started — a failure
// after headers are sent just ends the response, per CLAUDE.md's failover
// rule.
type Proxy struct {
	Table         *NodeTable
	CloudEndpoint string // base URL, e.g. https://api.openai.com; empty = no fallback
	CloudAPIKey   string
	Client        *http.Client
	Stats         *RequestStats
}

func NewProxy(table *NodeTable, cloudEndpoint, cloudAPIKey string, stats *RequestStats) *Proxy {
	return &Proxy{
		Table:         table,
		CloudEndpoint: cloudEndpoint,
		CloudAPIKey:   cloudAPIKey,
		Client:        &http.Client{}, // no blanket timeout: streams can run long
		Stats:         stats,
	}
}

func (p *Proxy) ChatCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeTypedError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeTypedError(w, http.StatusBadRequest, "invalid_request", "could not read request body")
		return
	}

	var req chatRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeTypedError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}

	target, usingCloud, ok := p.route(req)
	if !ok {
		p.Stats.Failed.Add(1)
		writeTypedError(w, http.StatusServiceUnavailable, "no_node_available",
			"no local node has the requested model loaded, and no cloud fallback is configured; retry")
		return
	}

	outReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		writeTypedError(w, http.StatusInternalServerError, "internal_error", "could not build upstream request")
		return
	}
	outReq.Header.Set("Content-Type", "application/json")
	if usingCloud && p.CloudAPIKey != "" {
		outReq.Header.Set("Authorization", "Bearer "+p.CloudAPIKey)
	}

	resp, err := p.Client.Do(outReq)
	if err != nil {
		// Nothing has been written to the client yet, so a typed error and
		// client-driven retry is safe — this is the only failover path.
		p.Stats.Failed.Add(1)
		log.Printf("edgeos-router: upstream %s unreachable: %v", target, err)
		writeTypedError(w, http.StatusServiceUnavailable, "upstream_unavailable",
			"the selected node became unreachable; retry")
		return
	}
	defer resp.Body.Close()

	if usingCloud {
		p.Stats.Cloud.Add(1)
	} else {
		p.Stats.Local.Add(1)
	}

	for k, vals := range resp.Header {
		// The router — not the upstream engine — owns CORS policy for its
		// own public-facing API (withCORS already set it); forwarding the
		// engine's own Access-Control-* would duplicate/conflict with it
		// and browsers reject the response outright when that happens.
		if strings.HasPrefix(strings.ToLower(k), "access-control-") {
			continue
		}
		for _, v := range vals {
			w.Header().Add(k, v)
		}
	}
	w.WriteHeader(resp.StatusCode)

	if _, err := io.Copy(flushWriter{w}, resp.Body); err != nil && !errors.Is(err, io.ErrClosedPipe) {
		// The stream already started; per CLAUDE.md we do not silently
		// restart it on another node. Just log and let it end.
		log.Printf("edgeos-router: stream from %s ended early: %v", target, err)
	}
}

// route picks a target URL for the request: a qualifying local node if one
// exists, else the configured cloud fallback, else no route.
func (p *Proxy) route(req chatRequest) (target string, usingCloud, ok bool) {
	nodes := p.Table.Snapshot()
	if node, _, found := selectNode(nodes, req.Model, req.estimatedTokens()); found {
		return strings.TrimSuffix(node.Cap.Engine.Endpoint, "/") + "/v1/chat/completions", false, true
	}
	if p.CloudEndpoint != "" {
		return strings.TrimSuffix(p.CloudEndpoint, "/") + "/v1/chat/completions", true, true
	}
	return "", false, false
}

// flushWriter flushes after every write so SSE chunks reach the client as
// they arrive instead of buffering.
type flushWriter struct {
	w http.ResponseWriter
}

func (f flushWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if flusher, ok := f.w.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
