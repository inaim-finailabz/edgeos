package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"

	"edgeos/internal/capability"
)

type server struct {
	nodeID string
	accel  string
	engine EngineSupervisor
}

func (s *server) capabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	totalMB, freeMB := memStatsMB()
	snap := s.engine.Snapshot()

	resp := capability.Response{
		Schema: "edgeos/v0",
		Node: capability.Node{
			ID:         s.nodeID,
			Hostname:   hostname,
			Platform:   runtime.GOOS + "/" + runtime.GOARCH,
			Accel:      s.accel,
			MemTotalMB: totalMB,
			MemFreeMB:  freeMB,
		},
		Engine: capability.Engine{
			Kind:     "llama.cpp",
			Endpoint: s.engine.Endpoint(),
			Healthy:  snap.Healthy,
		},
		Models: snap.modelForCapability(),
		Load: capability.Load{
			ActiveRequests: snap.ActiveRequests,
			QueueDepth:     snap.QueueDepth,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode capabilities response: %v", err)
	}
}
