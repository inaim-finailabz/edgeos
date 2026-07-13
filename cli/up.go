package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
)

// cmdUp is a thin wrapper around the agent binary: it's the friendly
// "start this device" entry point, but the agent process does the real
// work (mDNS announce, engine supervision), consistent with agent/router/
// cli being separate binaries per CLAUDE.md.
func cmdUp(args []string) {
	fs := flag.NewFlagSet("up", flag.ExitOnError)
	addr := fs.String("addr", ":8090", "address to serve /v0/capabilities on")
	model := fs.String("model", "", "path to a .gguf model file; omit to run with no engine")
	enginePort := fs.Int("engine-port", 8080, "port llama-server listens on")
	ctxSize := fs.Int("ctx-size", 4096, "context size passed to llama-server")
	agentBin := fs.String("agent-bin", "", "path to the agent binary (default: alongside this executable, then $PATH)")
	fs.Parse(args)

	bin := *agentBin
	if bin == "" {
		var err error
		bin, err = findSiblingBinary("agent")
		if err != nil {
			fmt.Fprintf(os.Stderr, "edgeos up: %v\n", err)
			os.Exit(1)
		}
	}

	cmdArgs := []string{
		"-addr", *addr,
		"-engine-port", strconv.Itoa(*enginePort),
		"-ctx-size", strconv.Itoa(*ctxSize),
	}
	if *model != "" {
		cmdArgs = append(cmdArgs, "-model", *model)
	}

	cmd := exec.Command(bin, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "edgeos up: start %s: %v\n", bin, err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-ctx.Done()
		cmd.Process.Signal(syscall.SIGTERM)
	}()

	if err := cmd.Wait(); err != nil {
		os.Exit(1)
	}
}

// findSiblingBinary looks for name next to the currently running
// executable first (the common case: agent/router/cli built together into
// the same dist/ directory), then falls back to $PATH.
func findSiblingBinary(name string) (string, error) {
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), name)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath(name); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("could not find %q binary alongside this executable or on $PATH; pass -agent-bin", name)
}
