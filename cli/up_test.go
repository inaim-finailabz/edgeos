package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindSiblingBinary_PrefersExecutableDir(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}

	siblingPath := filepath.Join(filepath.Dir(exe), "edgeos-test-sibling")
	if err := os.WriteFile(siblingPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Skipf("cannot write into executable's directory: %v", err)
	}
	defer os.Remove(siblingPath)

	got, err := findSiblingBinary("edgeos-test-sibling")
	if err != nil {
		t.Fatalf("findSiblingBinary: %v", err)
	}
	if got != siblingPath {
		t.Errorf("findSiblingBinary() = %q, want %q", got, siblingPath)
	}
}

func TestFindSiblingBinary_NotFound(t *testing.T) {
	if _, err := findSiblingBinary("edgeos-definitely-does-not-exist"); err == nil {
		t.Error("findSiblingBinary: want error for a binary that exists nowhere")
	}
}
