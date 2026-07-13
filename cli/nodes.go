package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"edgeos/internal/capability"
)

type nodeSummary struct {
	ID       string              `json:"id"`
	CapURL   string              `json:"cap_url"`
	Cap      capability.Response `json:"cap"`
	LastSeen time.Time           `json:"last_seen"`
	Misses   int                 `json:"misses"`
}

type nodesResponse struct {
	Nodes []nodeSummary `json:"nodes"`
}

func cmdNodes(args []string) {
	fs := flag.NewFlagSet("nodes", flag.ExitOnError)
	router := fs.String("router", "http://localhost:8081", "router base URL")
	fs.Parse(args)

	resp, err := http.Get(strings.TrimSuffix(*router, "/") + "/v0/nodes")
	if err != nil {
		fmt.Fprintf(os.Stderr, "edgeos nodes: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var nr nodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&nr); err != nil {
		fmt.Fprintf(os.Stderr, "edgeos nodes: decode response: %v\n", err)
		os.Exit(1)
	}

	if len(nr.Nodes) == 0 {
		fmt.Println("No nodes discovered yet.")
		return
	}

	tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tHOSTNAME\tPLATFORM\tACCEL\tMODEL\tSTATE\tTOK/S\tACTIVE\tHEALTHY")
	for _, n := range nr.Nodes {
		model, state, tokPerSec := "-", "-", "-"
		if len(n.Cap.Models) > 0 {
			m := n.Cap.Models[0]
			model, state = m.ID, m.State
			tokPerSec = fmt.Sprintf("%.1f", m.TokPerSec)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%v\n",
			n.ID, n.Cap.Node.Hostname, n.Cap.Node.Platform, n.Cap.Node.Accel,
			model, state, tokPerSec, n.Cap.Load.ActiveRequests, n.Cap.Engine.Healthy)
	}
	tw.Flush()
}
