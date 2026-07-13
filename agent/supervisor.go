package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"edgeos/internal/capability"
)

// SupervisorConfig configures the llama-server process a Supervisor manages.
// ModelPath == "" means "no engine": the supervisor reports a disabled
// engine so the agent can still serve /v0/capabilities on its own.
type SupervisorConfig struct {
	LlamaServerBin string
	ModelPath      string
	EngineHost     string
	EnginePort     int
	CtxSize        int
}

func (c SupervisorConfig) endpoint() string {
	return fmt.Sprintf("http://%s:%d", c.EngineHost, c.EnginePort)
}

// EngineSnapshot is the engine-derived slice of a capabilities response.
type EngineSnapshot struct {
	Healthy        bool
	ModelState     string // "disabled" | "loading" | "loaded" | "error"
	ModelID        string
	CtxMax         int
	TokPerSec      float64
	ActiveRequests int
	QueueDepth     int
}

// EngineSupervisor is what the capabilities handler needs from an engine
// supervisor; a fake implementation backs handler tests.
type EngineSupervisor interface {
	Snapshot() EngineSnapshot
	Endpoint() string
}

// Supervisor spawns and monitors a llama-server process, benchmarks it at
// load time, and polls it for load stats. It never sits in the token path —
// the router talks to the engine endpoint directly.
type Supervisor struct {
	cfg    SupervisorConfig
	client *http.Client

	mu    sync.RWMutex
	state EngineSnapshot
}

func NewSupervisor(cfg SupervisorConfig) *Supervisor {
	s := &Supervisor{
		cfg:    cfg,
		client: &http.Client{Timeout: 5 * time.Second},
	}
	s.state = EngineSnapshot{ModelState: "disabled"}
	if cfg.ModelPath != "" {
		s.state.ModelState = "loading"
		s.state.ModelID = filepath.Base(cfg.ModelPath)
	}
	return s
}

func (s *Supervisor) Endpoint() string { return s.cfg.endpoint() }

func (s *Supervisor) Snapshot() EngineSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

func (s *Supervisor) setState(fn func(*EngineSnapshot)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.state)
}

// Run spawns llama-server (if configured), waits for it to become healthy,
// benchmarks it, then polls load stats until ctx is cancelled. It blocks.
func (s *Supervisor) Run(ctx context.Context) error {
	if s.cfg.ModelPath == "" {
		log.Printf("edgeos-agent: no -model configured, engine disabled")
		<-ctx.Done()
		return nil
	}

	cmd := exec.CommandContext(ctx, s.cfg.LlamaServerBin,
		"-m", s.cfg.ModelPath,
		"--host", s.cfg.EngineHost,
		"--port", strconv.Itoa(s.cfg.EnginePort),
		"-c", strconv.Itoa(s.cfg.CtxSize),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		s.setState(func(st *EngineSnapshot) { st.ModelState = "error" })
		return fmt.Errorf("start %s: %w", s.cfg.LlamaServerBin, err)
	}
	log.Printf("edgeos-agent: spawned %s (pid %d) for %s", s.cfg.LlamaServerBin, cmd.Process.Pid, s.state.ModelID)

	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	if err := s.waitForHealthy(ctx, 60*time.Second); err != nil {
		s.setState(func(st *EngineSnapshot) { st.ModelState = "error"; st.Healthy = false })
		return fmt.Errorf("engine did not become healthy: %w", err)
	}

	if err := s.runBenchmark(ctx); err != nil {
		s.setState(func(st *EngineSnapshot) { st.ModelState = "error"; st.Healthy = false })
		return fmt.Errorf("benchmark: %w", err)
	}
	log.Printf("edgeos-agent: %s loaded, measured %.1f tok/s", s.state.ModelID, s.Snapshot().TokPerSec)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-exited:
			if ctx.Err() != nil {
				return nil
			}
			s.setState(func(st *EngineSnapshot) { st.ModelState = "error"; st.Healthy = false })
			return fmt.Errorf("%s exited: %w", s.cfg.LlamaServerBin, err)
		case <-ticker.C:
			if err := s.pollOnce(ctx); err != nil {
				log.Printf("edgeos-agent: poll engine: %v", err)
				s.setState(func(st *EngineSnapshot) { st.Healthy = false })
			}
		}
	}
}

func (s *Supervisor) waitForHealthy(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.endpoint()+"/health", nil)
		if resp, err := s.client.Do(req); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	return fmt.Errorf("timed out after %s waiting for %s/health", timeout, s.cfg.endpoint())
}

type completionTimings struct {
	PredictedPerSecond float64 `json:"predicted_per_second"`
}

type completionResponse struct {
	Timings completionTimings `json:"timings"`
}

type slot struct {
	NCtx         int  `json:"n_ctx"`
	IsProcessing bool `json:"is_processing"`
}

// runBenchmark measures real tok/s with a short generation against the
// live engine, and reads ctx_max off its slots — never estimated, per
// docs/CAPABILITY_SCHEMA.md.
func (s *Supervisor) runBenchmark(ctx context.Context) error {
	slots, err := s.fetchSlots(ctx)
	if err != nil {
		return err
	}
	ctxMax := 0
	if len(slots) > 0 {
		ctxMax = slots[0].NCtx
	}

	body, _ := json.Marshal(map[string]any{
		"prompt":    "The quick brown fox jumps over the lazy dog. Tell me a short story about",
		"n_predict": 50,
		"stream":    false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.endpoint()+"/completion", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("benchmark request: %w", err)
	}
	defer resp.Body.Close()

	var cr completionResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return fmt.Errorf("decode benchmark response: %w", err)
	}

	s.setState(func(st *EngineSnapshot) {
		st.ModelState = "loaded"
		st.Healthy = true
		st.CtxMax = ctxMax
		st.TokPerSec = cr.Timings.PredictedPerSecond
	})
	return nil
}

func (s *Supervisor) fetchSlots(ctx context.Context) ([]slot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.cfg.endpoint()+"/slots", nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch slots: %w", err)
	}
	defer resp.Body.Close()

	var slots []slot
	if err := json.NewDecoder(resp.Body).Decode(&slots); err != nil {
		return nil, fmt.Errorf("decode slots: %w", err)
	}
	return slots, nil
}

// pollOnce refreshes active_requests from /slots. queue_depth stays 0:
// llama-server has no native queue-depth metric to report in v0.
func (s *Supervisor) pollOnce(ctx context.Context) error {
	slots, err := s.fetchSlots(ctx)
	if err != nil {
		return err
	}
	active := 0
	for _, sl := range slots {
		if sl.IsProcessing {
			active++
		}
	}
	s.setState(func(st *EngineSnapshot) {
		st.Healthy = true
		st.ActiveRequests = active
	})
	return nil
}

// modelForCapability converts the engine snapshot into the schema's model
// list: empty when the engine isn't loaded, one entry otherwise.
func (snap EngineSnapshot) modelForCapability() []capability.Model {
	if snap.ModelState != "loaded" {
		return nil
	}
	return []capability.Model{{
		ID:        snap.ModelID,
		State:     snap.ModelState,
		CtxMax:    snap.CtxMax,
		TokPerSec: snap.TokPerSec,
	}}
}
