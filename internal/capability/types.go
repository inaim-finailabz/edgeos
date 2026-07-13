// Package capability defines the wire format for GET /v0/capabilities.
// This is the contract in docs/CAPABILITY_SCHEMA.md — keep the two in sync.
package capability

type Node struct {
	ID         string `json:"id"`
	Hostname   string `json:"hostname"`
	Platform   string `json:"platform"`
	Accel      string `json:"accel"`
	MemTotalMB int    `json:"mem_total_mb"`
	MemFreeMB  int    `json:"mem_free_mb"`
}

type Engine struct {
	Kind     string `json:"kind"`
	Endpoint string `json:"endpoint"`
	Healthy  bool   `json:"healthy"`
}

type Model struct {
	ID        string  `json:"id"`
	State     string  `json:"state"`
	CtxMax    int     `json:"ctx_max"`
	TokPerSec float64 `json:"tok_per_sec"`
}

type Load struct {
	ActiveRequests int `json:"active_requests"`
	QueueDepth     int `json:"queue_depth"`
}

type Response struct {
	Schema string  `json:"schema"`
	Node   Node    `json:"node"`
	Engine Engine  `json:"engine"`
	Models []Model `json:"models"`
	Load   Load    `json:"load"`
}
