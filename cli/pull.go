package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// cmdPull downloads a .gguf model. <ref> is either a direct URL, or a
// "org/repo/file.gguf" shorthand resolved against the Hugging Face Hub.
// This is a plain file download, not a registry — no curated model catalog
// or fleet-wide sync, which CLAUDE.md explicitly keeps out of v0 scope.
func cmdPull(args []string) {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	out := fs.String("out", "", "output path (default: ~/.edgeos/models/<filename>)")
	fs.Parse(args)

	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: edgeos pull [-out path] <url or org/repo/file.gguf>")
		os.Exit(1)
	}

	url, err := resolveModelURL(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "edgeos pull: %v\n", err)
		os.Exit(1)
	}

	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(defaultModelsDir(), filepath.Base(url))
	}

	if err := downloadFile(url, outPath); err != nil {
		fmt.Fprintf(os.Stderr, "edgeos pull: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nsaved to %s\n", outPath)
}

func resolveModelURL(ref string) (string, error) {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return ref, nil
	}
	parts := strings.Split(ref, "/")
	if len(parts) != 3 {
		return "", fmt.Errorf("expected a URL or org/repo/file.gguf shorthand, got %q", ref)
	}
	org, repo, file := parts[0], parts[1], parts[2]
	return fmt.Sprintf("https://huggingface.co/%s/%s/resolve/main/%s", org, repo, file), nil
}

func defaultModelsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "models"
	}
	return filepath.Join(home, ".edgeos", "models")
}

func downloadFile(url, outPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch %s: unexpected status %s", url, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	tmpPath := outPath + ".part"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	pw := &progressWriter{w: f, total: resp.ContentLength, label: filepath.Base(outPath)}
	_, copyErr := io.Copy(pw, resp.Body)
	closeErr := f.Close()
	if copyErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("download %s: %w", url, copyErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return closeErr
	}
	pw.done()
	return os.Rename(tmpPath, outPath)
}

// progressWriter prints throttled download progress to stderr so it
// doesn't interleave with piped stdout.
type progressWriter struct {
	w        io.Writer
	total    int64
	label    string
	written  int64
	lastPrnt time.Time
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)
	if time.Since(p.lastPrnt) > 250*time.Millisecond {
		p.print()
		p.lastPrnt = time.Now()
	}
	return n, err
}

func (p *progressWriter) print() {
	if p.total > 0 {
		fmt.Fprintf(os.Stderr, "\r%s: %.1f%% (%d/%d bytes)", p.label,
			100*float64(p.written)/float64(p.total), p.written, p.total)
		return
	}
	fmt.Fprintf(os.Stderr, "\r%s: %d bytes", p.label, p.written)
}

func (p *progressWriter) done() { p.print() }
