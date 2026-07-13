package main

import (
	"testing"

	"edgeos/internal/capability"
)

func node(id string, healthy bool, activeRequests int, models ...capability.Model) NodeState {
	return NodeState{
		ID: id,
		Cap: capability.Response{
			Node:   capability.Node{ID: id},
			Engine: capability.Engine{Healthy: healthy},
			Models: models,
			Load:   capability.Load{ActiveRequests: activeRequests},
		},
	}
}

func model(id, state string, ctxMax int, tokPerSec float64) capability.Model {
	return capability.Model{ID: id, State: state, CtxMax: ctxMax, TokPerSec: tokPerSec}
}

func TestSelectNode_PicksHighestAdjustedScore(t *testing.T) {
	nodes := []NodeState{
		node("busy", true, 3, model("llama-8b", "loaded", 8192, 100)), // 100/4 = 25
		node("idle", true, 0, model("llama-8b", "loaded", 8192, 50)),  // 50/1 = 50
	}

	got, m, ok := selectNode(nodes, "llama-8b", 100)
	if !ok {
		t.Fatal("selectNode: want a match")
	}
	if got.ID != "idle" {
		t.Errorf("selected node = %q, want idle (higher adjusted score)", got.ID)
	}
	if m.ID != "llama-8b" {
		t.Errorf("selected model = %q, want llama-8b", m.ID)
	}
}

func TestSelectNode_FiltersUnhealthy(t *testing.T) {
	nodes := []NodeState{
		node("unhealthy", false, 0, model("llama-8b", "loaded", 8192, 999)),
		node("healthy", true, 0, model("llama-8b", "loaded", 8192, 10)),
	}

	got, _, ok := selectNode(nodes, "llama-8b", 100)
	if !ok || got.ID != "healthy" {
		t.Errorf("selectNode = (%q, %v), want healthy node", got.ID, ok)
	}
}

func TestSelectNode_FiltersModelNotLoaded(t *testing.T) {
	nodes := []NodeState{
		node("loading", true, 0, model("llama-8b", "loading", 8192, 10)),
	}
	if _, _, ok := selectNode(nodes, "llama-8b", 100); ok {
		t.Error("selectNode: want no match when model isn't loaded")
	}
}

func TestSelectNode_FiltersCtxTooSmall(t *testing.T) {
	nodes := []NodeState{
		node("small-ctx", true, 0, model("llama-8b", "loaded", 512, 10)),
	}
	if _, _, ok := selectNode(nodes, "llama-8b", 4096); ok {
		t.Error("selectNode: want no match when ctx_max < estimated tokens")
	}
}

func TestSelectNode_NoModelMatch(t *testing.T) {
	nodes := []NodeState{
		node("n1", true, 0, model("other-model", "loaded", 8192, 10)),
	}
	if _, _, ok := selectNode(nodes, "llama-8b", 100); ok {
		t.Error("selectNode: want no match for a different model id")
	}
}

func TestChatRequest_EstimatedTokens(t *testing.T) {
	r := chatRequest{MaxTokens: 100}
	r.Messages = append(r.Messages, struct {
		Content string `json:"content"`
	}{Content: "12345678"}) // 8 chars -> 2 tokens

	if got, want := r.estimatedTokens(), 102; got != want {
		t.Errorf("estimatedTokens() = %d, want %d", got, want)
	}
}

func TestChatRequest_EstimatedTokens_DefaultsMaxTokens(t *testing.T) {
	r := chatRequest{}
	if got, want := r.estimatedTokens(), 512; got != want {
		t.Errorf("estimatedTokens() = %d, want %d (default max_tokens)", got, want)
	}
}
