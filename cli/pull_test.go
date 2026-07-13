package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveModelURL(t *testing.T) {
	tests := []struct {
		ref     string
		want    string
		wantErr bool
	}{
		{"https://example.com/model.gguf", "https://example.com/model.gguf", false},
		{"http://example.com/model.gguf", "http://example.com/model.gguf", false},
		{"Qwen/Qwen2.5-3B-Instruct-GGUF/qwen2.5-3b-instruct-q4_k_m.gguf",
			"https://huggingface.co/Qwen/Qwen2.5-3B-Instruct-GGUF/resolve/main/qwen2.5-3b-instruct-q4_k_m.gguf", false},
		{"not-enough-parts", "", true},
		{"way/too/many/parts", "", true},
	}
	for _, tt := range tests {
		got, err := resolveModelURL(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("resolveModelURL(%q) error = %v, wantErr %v", tt.ref, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("resolveModelURL(%q) = %q, want %q", tt.ref, got, tt.want)
		}
	}
}

func TestDownloadFile(t *testing.T) {
	const payload = "fake gguf bytes"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(payload))
	}))
	defer srv.Close()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "model.gguf")

	if err := downloadFile(srv.URL, outPath); err != nil {
		t.Fatalf("downloadFile: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read downloaded file: %v", err)
	}
	if string(got) != payload {
		t.Errorf("downloaded content = %q, want %q", got, payload)
	}
	if _, err := os.Stat(outPath + ".part"); err == nil {
		t.Error(".part temp file should be renamed away, not left behind")
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	err := downloadFile(srv.URL, filepath.Join(t.TempDir(), "model.gguf"))
	if err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("downloadFile: err = %v, want an error mentioning 404", err)
	}
}
