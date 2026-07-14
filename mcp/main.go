package main

import (
	"context"
	"flag"
	"log"
	"os"
)

func main() {
	defaultRouter := os.Getenv("EDGEOS_ROUTER_ADDR")
	if defaultRouter == "" {
		defaultRouter = "http://localhost:8081"
	}
	router := flag.String("router", defaultRouter, "EdgeOS router base URL (default: $EDGEOS_ROUTER_ADDR)")
	token := flag.String("management-token", os.Getenv("EDGEOS_MANAGEMENT_TOKEN"), "bearer token for add/remove/stop/reload tools; omit to expose read-only tools only (default: $EDGEOS_MANAGEMENT_TOKEN)")
	flag.Parse()

	// Every log line must go to stderr: stdout is the JSON-RPC wire on
	// stdio transport, and a stray line there corrupts it for the client.
	errLog := log.New(os.Stderr, "edgeos-mcp: ", log.LstdFlags)

	client := newFleetClient(*router, *token)
	srv := NewServer(buildTools(client))

	errLog.Printf("serving MCP over stdio, router=%s, write tools=%v", *router, *token != "")
	if err := srv.Run(context.Background(), os.Stdin, os.Stdout, errLog); err != nil {
		errLog.Fatalf("server: %v", err)
	}
}
