package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	addr := flag.String("addr", ":8081", "address for the OpenAI-compatible endpoint and /v0/nodes")
	cloudEndpoint := flag.String("cloud-endpoint", "", "base URL of a cloud endpoint to fall back to when no local node qualifies (e.g. https://api.openai.com)")
	cloudAPIKey := flag.String("cloud-api-key", os.Getenv("EDGEOS_CLOUD_API_KEY"), "API key sent to the cloud fallback (default: $EDGEOS_CLOUD_API_KEY)")
	pollInterval := flag.Duration("poll-interval", 2*time.Second, "how often to rediscover and poll agents")
	missThreshold := flag.Int("miss-threshold", 3, "consecutive missed polls before evicting a node")
	managementToken := flag.String("management-token", os.Getenv("EDGEOS_MANAGEMENT_TOKEN"), "bearer token required for node create/delete/actions; must match each agent's own token (default: $EDGEOS_MANAGEMENT_TOKEN)")
	flag.Parse()

	table := NewNodeTable(*missThreshold)
	stats := &RequestStats{}
	proxy := NewProxy(table, *cloudEndpoint, *cloudAPIKey, stats)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go runDiscoveryAndPoll(ctx, table, *pollInterval)

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/chat/completions", proxy.ChatCompletionsHandler)
	mux.HandleFunc("GET /v0/nodes", nodesHandler(table, stats))
	mux.HandleFunc("POST /v0/nodes", createNodeHandler(table, *managementToken))
	mux.HandleFunc("GET /v0/nodes/{id}", getNodeHandler(table))
	mux.HandleFunc("DELETE /v0/nodes/{id}", deleteNodeHandler(table, *managementToken))
	mux.HandleFunc("POST /v0/nodes/{id}/actions/{action}", managementActionsHandler(table, *managementToken))
	httpServer := &http.Server{Addr: *addr, Handler: withCORS(mux)}

	if *managementToken == "" {
		log.Printf("edgeos-router: no -management-token configured; node create/delete/actions disabled")
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	if *cloudEndpoint == "" {
		log.Printf("edgeos-router: no -cloud-endpoint configured; requests with no qualifying local node will fail")
	}
	log.Printf("edgeos-router: serving /v1/chat/completions and /v0/nodes on %s", *addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
