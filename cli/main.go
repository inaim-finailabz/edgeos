package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	args := os.Args[2:]
	switch os.Args[1] {
	case "up":
		cmdUp(args)
	case "nodes":
		cmdNodes(args)
	case "pull":
		cmdPull(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "edgeos: unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `edgeos - control plane for EdgeOS

Usage:
  edgeos up [flags]        run the agent on this device (supervises llama-server)
  edgeos nodes [flags]     list nodes the router currently sees
  edgeos pull <ref> [flags] download a .gguf model

Run "edgeos <command> -h" for flags on a specific command.
`)
}
