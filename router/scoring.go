package main

import "edgeos/internal/capability"

// chatRequest is the slice of an OpenAI-style chat completion request the
// router needs in order to route it; everything else is forwarded opaquely.
type chatRequest struct {
	Model     string `json:"model"`
	MaxTokens int    `json:"max_tokens"`
	Messages  []struct {
		Content string `json:"content"`
	} `json:"messages"`
}

// estimatedTokens is a v0 heuristic (~4 chars/token), not a real tokenizer —
// enough to reject requests that obviously won't fit ctx_max without
// pulling in a tokenizer dependency.
func (r chatRequest) estimatedTokens() int {
	chars := 0
	for _, m := range r.Messages {
		chars += len(m.Content)
	}
	maxTokens := r.MaxTokens
	if maxTokens == 0 {
		maxTokens = 512
	}
	return chars/4 + maxTokens
}

// selectNode implements the v0 scoring rule from CLAUDE.md: model loaded ->
// ctx fits -> highest tok/s adjusted by active_requests.
func selectNode(nodes []NodeState, model string, estTokens int) (node NodeState, m capability.Model, ok bool) {
	bestScore := -1.0
	for _, n := range nodes {
		if !n.Cap.Engine.Healthy {
			continue
		}
		for _, candidate := range n.Cap.Models {
			if candidate.ID != model || candidate.State != "loaded" || candidate.CtxMax < estTokens {
				continue
			}
			score := candidate.TokPerSec / float64(1+n.Cap.Load.ActiveRequests)
			if !ok || score > bestScore {
				node, m, bestScore, ok = n, candidate, score, true
			}
		}
	}
	return node, m, ok
}
