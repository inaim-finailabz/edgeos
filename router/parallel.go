package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
)

// maxParallelRequests bounds fan-out size -- a safety limit, not a
// performance tune: without it a single call could dispatch an unbounded
// number of concurrent requests across the fleet.
const maxParallelRequests = 32

type parallelRequest struct {
	Requests []json.RawMessage `json:"requests"`
}

type parallelError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type parallelResult struct {
	Index    int             `json:"index"`
	Status   string          `json:"status"` // "ok" | "error"
	Response json.RawMessage `json:"response,omitempty"`
	Error    *parallelError  `json:"error,omitempty"`
}

type parallelResponse struct {
	Results []parallelResult `json:"results"`
}

// ParallelCompletionsHandler serves POST /v0/parallel-completions: a fan-out
// primitive, not a prompt-engineering framework. Each entry in "requests" is
// an independent chat completion request (its own model/messages/etc, same
// shape as /v1/chat/completions) scored and routed exactly like a single
// request would be, dispatched concurrently. Splitting a task into
// sub-prompts and combining the results back into one answer is the
// caller's job -- EdgeOS orchestrates where each request runs, not what the
// prompts mean or how their outputs relate. Non-streaming only: aggregating
// N concurrent SSE streams into one response isn't supported in v0.
func (p *Proxy) ParallelCompletionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeTypedError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use POST")
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeTypedError(w, http.StatusBadRequest, "invalid_request", "could not read request body")
		return
	}

	var req parallelRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeTypedError(w, http.StatusBadRequest, "invalid_request", "malformed JSON body")
		return
	}
	if len(req.Requests) == 0 {
		writeTypedError(w, http.StatusBadRequest, "invalid_request", `"requests" must be a non-empty array`)
		return
	}
	if len(req.Requests) > maxParallelRequests {
		writeTypedError(w, http.StatusBadRequest, "invalid_request", "too many requests in one call (max 32)")
		return
	}

	results := make([]parallelResult, len(req.Requests))
	var wg sync.WaitGroup
	for i, raw := range req.Requests {
		wg.Add(1)
		go func(i int, raw json.RawMessage) {
			defer wg.Done()
			results[i] = p.runOne(r, i, raw)
		}(i, raw)
	}
	wg.Wait()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parallelResponse{Results: results})
}

// runOne scores, routes, and executes a single sub-request exactly like
// ChatCompletionsHandler does, but non-streaming: it reads the full
// response and returns it as one JSON value instead of proxying an SSE
// stream to the client.
func (p *Proxy) runOne(r *http.Request, index int, raw json.RawMessage) parallelResult {
	// Force non-streaming regardless of what the caller passed: this
	// handler collects each sub-request's full response as one JSON
	// value, not a stream to relay.
	body, req, err := forceNonStreaming(raw)
	if err != nil {
		return errResult(index, "invalid_request", err.Error())
	}

	target, usingCloud, ok := p.route(req)
	if !ok {
		p.Stats.Failed.Add(1)
		return errResult(index, "no_node_available",
			"no local node has the requested model loaded, and no cloud fallback is configured; retry")
	}

	outReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return errResult(index, "internal_error", "could not build upstream request")
	}
	outReq.Header.Set("Content-Type", "application/json")
	if usingCloud && p.CloudAPIKey != "" {
		outReq.Header.Set("Authorization", "Bearer "+p.CloudAPIKey)
	}

	resp, err := p.Client.Do(outReq)
	if err != nil {
		p.Stats.Failed.Add(1)
		return errResult(index, "upstream_unavailable", "the selected node became unreachable; retry")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		p.Stats.Failed.Add(1)
		return errResult(index, "upstream_unavailable", "reading the upstream response failed")
	}
	if resp.StatusCode >= 300 {
		p.Stats.Failed.Add(1)
		return errResult(index, "upstream_error", strings.TrimSpace(string(respBody)))
	}

	if usingCloud {
		p.Stats.Cloud.Add(1)
	} else {
		p.Stats.Local.Add(1)
	}
	return parallelResult{Index: index, Status: "ok", Response: json.RawMessage(respBody)}
}

func errResult(index int, errType, message string) parallelResult {
	return parallelResult{Index: index, Status: "error", Error: &parallelError{Type: errType, Message: message}}
}

// forceNonStreaming decodes a sub-request for routing while rewriting its
// "stream" field to false in the bytes actually forwarded upstream,
// preserving every other field (temperature, top_p, etc.) the caller sent.
func forceNonStreaming(raw json.RawMessage) (body []byte, req chatRequest, err error) {
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, chatRequest{}, err
	}

	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, chatRequest{}, err
	}
	fields["stream"] = false

	body, err = json.Marshal(fields)
	return body, req, err
}
