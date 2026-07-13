package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/hashicorp/mdns"
)

const mdnsServiceName = "_edgeos._tcp"

// discardLogger silences the mdns library's own logging (e.g. its harmless
// "failed to bind udp6" noise when only IPv4 multicast is available).
var discardLogger = log.New(io.Discard, "", 0)

// discoverOnce runs a bounded mDNS lookup and registers any agents found
// into the table. Failures to resolve an individual entry are skipped,
// not fatal to the whole lookup.
func discoverOnce(table *NodeTable, timeout time.Duration) {
	entries := make(chan *mdns.ServiceEntry, 16)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for e := range entries {
			id, capPath, ok := parseTXT(e.InfoFields)
			if !ok {
				continue
			}
			addr := entryAddr(e)
			if addr == "" {
				continue
			}
			table.Discovered(id, fmt.Sprintf("http://%s:%d%s", addr, e.Port, capPath))
		}
	}()

	params := mdns.DefaultParams(mdnsServiceName)
	params.Entries = entries
	params.Timeout = timeout
	params.Logger = discardLogger
	if err := mdns.Query(params); err != nil {
		log.Printf("edgeos-router: mDNS query: %v", err)
	}
	close(entries)
	<-done
}

// parseTXT extracts id and cap from TXT fields like "v=0", "id=...",
// "cap=/v0/capabilities" per docs/CAPABILITY_SCHEMA.md.
func parseTXT(fields []string) (id, capPath string, ok bool) {
	for _, f := range fields {
		k, v, found := strings.Cut(f, "=")
		if !found {
			continue
		}
		switch k {
		case "id":
			id = v
		case "cap":
			capPath = v
		}
	}
	return id, capPath, id != "" && capPath != ""
}

func entryAddr(e *mdns.ServiceEntry) string {
	if e.AddrV4 != nil {
		return e.AddrV4.String()
	}
	if e.AddrV6IPAddr != nil {
		return e.AddrV6IPAddr.String()
	}
	return ""
}

// runDiscoveryAndPoll ticks every interval: rediscover new agents via mDNS,
// then poll every known agent's capabilities. Blocks until ctx is done.
func runDiscoveryAndPoll(ctx context.Context, table *NodeTable, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			discoverOnce(table, interval/2)
			table.PollAll(ctx)
		}
	}
}
