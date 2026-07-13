package main

import "testing"

func TestComputeSummary(t *testing.T) {
	nodes := []NodeState{
		node("healthy1", true, 2, model("llama-8b", "loaded", 8192, 50)),
		node("healthy2", true, 0, model("llama-8b", "loaded", 8192, 30)),
		node("unhealthy", false, 0, model("qwen-3b", "loaded", 4096, 20)),
		node("loading", true, 0, model("qwen-3b", "loading", 4096, 0)),
	}

	stats := &RequestStats{}
	stats.Local.Add(5)
	stats.Cloud.Add(2)
	stats.Failed.Add(1)

	s := computeSummary(nodes, stats)

	if s.TotalNodes != 4 {
		t.Errorf("TotalNodes = %d, want 4", s.TotalNodes)
	}
	if s.HealthyNodes != 3 {
		t.Errorf("HealthyNodes = %d, want 3", s.HealthyNodes)
	}
	if s.TotalActiveRequests != 2 {
		t.Errorf("TotalActiveRequests = %d, want 2", s.TotalActiveRequests)
	}
	if s.TotalTokPerSec != 100 { // 50 + 30 + 20 ("loading" excluded)
		t.Errorf("TotalTokPerSec = %v, want 100", s.TotalTokPerSec)
	}
	if s.DistinctModels != 2 { // llama-8b, qwen-3b (loaded only)
		t.Errorf("DistinctModels = %d, want 2", s.DistinctModels)
	}
	if s.RequestsServedLocal != 5 || s.RequestsServedCloud != 2 || s.RequestsFailed != 1 {
		t.Errorf("request counters = (%d,%d,%d), want (5,2,1)", s.RequestsServedLocal, s.RequestsServedCloud, s.RequestsFailed)
	}
}

func TestComputeSummary_Empty(t *testing.T) {
	s := computeSummary(nil, &RequestStats{})
	if s.TotalNodes != 0 || s.HealthyNodes != 0 || s.DistinctModels != 0 {
		t.Errorf("summary of empty fleet should be all zero, got %+v", s)
	}
}
