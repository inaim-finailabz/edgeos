package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"edgeos/internal/capability"
)

// NodeState is what the router knows about one agent: where to poll it,
// its last-fetched capabilities, and how many consecutive polls it's missed.
type NodeState struct {
	ID       string              `json:"id"`
	CapURL   string              `json:"cap_url"`
	Cap      capability.Response `json:"cap"`
	LastSeen time.Time           `json:"last_seen"`
	Misses   int                 `json:"misses"`
}

// NodeTable is the router's live view of the fleet: discovered via mDNS or
// added manually by address, kept fresh by polling GET <cap-url> every
// tick, evicted after MissThreshold consecutive failures — per
// docs/CAPABILITY_SCHEMA.md.
type NodeTable struct {
	MissThreshold int

	client  *http.Client
	mu      sync.RWMutex
	nodes   map[string]*NodeState
	removed map[string]bool // explicitly deleted; mDNS won't re-add until restored
}

func NewNodeTable(missThreshold int) *NodeTable {
	return &NodeTable{
		MissThreshold: missThreshold,
		client:        &http.Client{Timeout: 3 * time.Second},
		nodes:         make(map[string]*NodeState),
		removed:       make(map[string]bool),
	}
}

// Discovered registers a node found via mDNS. It's a no-op if the node is
// already known (so rediscovery doesn't reset an in-progress miss count)
// or was explicitly removed via Remove.
func (t *NodeTable) Discovered(id, capURL string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.removed[id] {
		return
	}
	if _, ok := t.nodes[id]; ok {
		return
	}
	t.nodes[id] = &NodeState{ID: id, CapURL: capURL}
}

// AddByURL registers a node by address rather than waiting for mDNS —
// e.g. one on a different subnet where multicast doesn't reach. It fetches
// capabilities once synchronously to validate reachability and learn the
// node's real id, and clears any prior Remove for that id.
func (t *NodeTable) AddByURL(ctx context.Context, capURL string) (NodeState, error) {
	cap, err := t.fetchCapabilities(ctx, capURL)
	if err != nil {
		return NodeState{}, err
	}
	state := NodeState{ID: cap.Node.ID, CapURL: capURL, Cap: *cap, LastSeen: time.Now()}

	t.mu.Lock()
	delete(t.removed, state.ID)
	t.nodes[state.ID] = &state
	t.mu.Unlock()

	return state, nil
}

// Snapshot returns a point-in-time copy of the table for scoring/listing.
func (t *NodeTable) Snapshot() []NodeState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]NodeState, 0, len(t.nodes))
	for _, n := range t.nodes {
		out = append(out, *n)
	}
	return out
}

// Get returns one node's state by id.
func (t *NodeTable) Get(id string) (NodeState, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n, ok := t.nodes[id]
	if !ok {
		return NodeState{}, false
	}
	return *n, true
}

// Remove deletes a node immediately (bypassing the miss-threshold) and
// blacklists its id so mDNS rediscovery won't silently bring it back —
// the dashboard's Delete action. Reports whether the node was present.
func (t *NodeTable) Remove(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.nodes[id]
	delete(t.nodes, id)
	t.removed[id] = true
	return ok
}

// PollAll fetches capabilities for every known node concurrently, updating
// each on success and evicting any that hit MissThreshold consecutive misses.
func (t *NodeTable) PollAll(ctx context.Context) {
	t.mu.RLock()
	ids := make([]string, 0, len(t.nodes))
	urls := make(map[string]string, len(t.nodes))
	for id, n := range t.nodes {
		ids = append(ids, id)
		urls[id] = n.CapURL
	}
	t.mu.RUnlock()

	var wg sync.WaitGroup
	results := make(chan struct {
		id  string
		cap *capability.Response
	}, len(ids))

	for _, id := range ids {
		wg.Add(1)
		go func(id, url string) {
			defer wg.Done()
			cap, err := t.fetchCapabilities(ctx, url)
			if err != nil {
				cap = nil
			}
			results <- struct {
				id  string
				cap *capability.Response
			}{id, cap}
		}(id, urls[id])
	}
	go func() { wg.Wait(); close(results) }()

	for r := range results {
		t.recordPoll(r.id, r.cap)
	}
}

func (t *NodeTable) fetchCapabilities(ctx context.Context, url string) (*capability.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("capabilities poll: status %d", resp.StatusCode)
	}
	var c capability.Response
	if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (t *NodeTable) recordPoll(id string, cap *capability.Response) {
	t.mu.Lock()
	defer t.mu.Unlock()
	n, ok := t.nodes[id]
	if !ok {
		return
	}
	if cap == nil {
		n.Misses++
		if n.Misses >= t.MissThreshold {
			delete(t.nodes, id)
		}
		return
	}
	n.Cap = *cap
	n.LastSeen = time.Now()
	n.Misses = 0
}
