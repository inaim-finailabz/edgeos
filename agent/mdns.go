package main

import (
	"fmt"
	"log"

	"github.com/hashicorp/mdns"
)

const mdnsServiceName = "_edgeos._tcp"

// startMDNS announces this agent on _edgeos._tcp.local with the TXT
// records docs/CAPABILITY_SCHEMA.md specifies: v, id, cap.
func startMDNS(nodeID string, capPort int, capPath string) (*mdns.Server, error) {
	svc, err := mdns.NewMDNSService(nodeID, mdnsServiceName, "", "", capPort, nil, []string{
		"v=0",
		"id=" + nodeID,
		"cap=" + capPath,
	})
	if err != nil {
		return nil, fmt.Errorf("new mdns service: %w", err)
	}

	server, err := mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return nil, fmt.Errorf("new mdns server: %w", err)
	}

	log.Printf("edgeos-agent: announcing %s id=%s port=%d", mdnsServiceName, nodeID, capPort)
	return server, nil
}
