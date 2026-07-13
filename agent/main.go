package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"runtime"

	"edgeos/internal/capability"
)

func main() {
	addr := flag.String("addr", ":8090", "address to serve /v0/capabilities on")
	flag.Parse()

	http.HandleFunc("/v0/capabilities", capabilitiesHandler)

	log.Printf("edgeos-agent: serving /v0/capabilities on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

// capabilitiesHandler returns fake-but-schema-shaped data so the router has
// something to talk to before engine supervision and benchmarking exist.
func capabilitiesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	resp := capability.Response{
		Schema: "edgeos/v0",
		Node: capability.Node{
			ID:         "a1b2c3d4",
			Hostname:   hostname,
			Platform:   runtime.GOOS + "/" + runtime.GOARCH,
			Accel:      "cpu",
			MemTotalMB: 7620,
			MemFreeMB:  4100,
		},
		Engine: capability.Engine{
			Kind:     "llama.cpp",
			Endpoint: "http://localhost:8080",
			Healthy:  true,
		},
		Models: []capability.Model{
			{
				ID:        "llama-3.1-8b-instruct-q4_k_m",
				State:     "loaded",
				CtxMax:    8192,
				TokPerSec: 11.4,
			},
		},
		Load: capability.Load{
			ActiveRequests: 0,
			QueueDepth:     0,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("encode capabilities response: %v", err)
	}
}
