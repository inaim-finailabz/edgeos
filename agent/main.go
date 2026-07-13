package main

import (
	"context"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":8090", "address to serve /v0/capabilities on")
	modelPath := flag.String("model", "", "path to a .gguf model file; omit to run with no engine")
	llamaServerBin := flag.String("llama-server-bin", "llama-server", "path to the llama-server binary")
	engineHost := flag.String("engine-host", "127.0.0.1", "host llama-server listens on")
	enginePort := flag.Int("engine-port", 8080, "port llama-server listens on")
	ctxSize := flag.Int("ctx-size", 4096, "context size passed to llama-server (-c)")
	accel := flag.String("accel", "", "override accelerator reported in capabilities (default: best-effort detection)")
	stateDir := flag.String("state-dir", defaultStateDir(), "directory for persisted agent state (node id)")
	enableMDNS := flag.Bool("mdns", true, "announce this agent via mDNS")
	managementToken := flag.String("management-token", os.Getenv("EDGEOS_MANAGEMENT_TOKEN"), "bearer token required for /v0/actions/*; empty disables them (default: $EDGEOS_MANAGEMENT_TOKEN)")
	flag.Parse()

	nodeID, err := loadOrCreateNodeID(*stateDir)
	if err != nil {
		log.Fatalf("edgeos-agent: node id: %v", err)
	}

	resolvedAccel := *accel
	if resolvedAccel == "" {
		resolvedAccel = detectAccel()
	}

	sup := NewSupervisor(SupervisorConfig{
		LlamaServerBin: *llamaServerBin,
		ModelPath:      *modelPath,
		EngineHost:     *engineHost,
		EnginePort:     *enginePort,
		CtxSize:        *ctxSize,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go func() {
		if err := sup.Run(ctx); err != nil && ctx.Err() == nil {
			log.Printf("edgeos-agent: supervisor: %v", err)
		}
	}()

	if *enableMDNS {
		mdnsServer, err := startMDNS(nodeID, capabilitiesPort(*addr, *enginePort), "/v0/capabilities")
		if err != nil {
			log.Printf("edgeos-agent: mDNS disabled: %v", err)
		} else {
			defer mdnsServer.Shutdown()
		}
	}

	srv := &server{nodeID: nodeID, accel: resolvedAccel, engine: sup}
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/capabilities", srv.capabilitiesHandler)
	mux.Handle("/v0/actions/", actionsMux(sup, *managementToken))
	httpServer := &http.Server{Addr: *addr, Handler: mux}

	if *managementToken == "" {
		log.Printf("edgeos-agent: no -management-token configured; /v0/actions/* disabled")
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("edgeos-agent: id=%s serving /v0/capabilities on %s", nodeID, *addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func defaultStateDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".edgeos"
	}
	return filepath.Join(home, ".edgeos")
}

// capabilitiesPort extracts the port number mDNS should advertise from an
// "-addr" flag like ":8090" or "0.0.0.0:8090".
func capabilitiesPort(addr string, fallback int) int {
	_, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return fallback
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return fallback
	}
	return p
}
