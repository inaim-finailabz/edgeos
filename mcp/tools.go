package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// fleetClient is a thin HTTP client for the router's API — the same one
// the CLI and dashboard talk to. Write tools are only registered when
// ManagementToken is set, mirroring how the dashboard hides controls it
// can't use and the router/agent 404 write routes with no token configured.
type fleetClient struct {
	RouterURL       string
	ManagementToken string
	HTTP            *http.Client
}

func newFleetClient(routerURL, token string) *fleetClient {
	return &fleetClient{
		RouterURL:       strings.TrimSuffix(routerURL, "/"),
		ManagementToken: token,
		HTTP:            &http.Client{Timeout: 90 * time.Second},
	}
}

func (c *fleetClient) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.RouterURL+path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *fleetClient) write(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.RouterURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.ManagementToken)
	return c.do(req)
}

func (c *fleetClient) do(req *http.Request) ([]byte, error) {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to router: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("router returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// toolHandler executes a tool call given its raw JSON arguments.
type toolHandler func(ctx context.Context, args json.RawMessage) toolsCallResult

// toolEntry pairs a tool's advertised definition with its handler.
type toolEntry struct {
	def     tool
	handler toolHandler
}

// buildTools returns the read-only tools plus, if a management token is
// configured, the write tools -- mirroring the "disabled unless a token is
// set" default everywhere else in EdgeOS.
func buildTools(c *fleetClient) []toolEntry {
	tools := []toolEntry{
		{
			def: tool{
				Name:        "list_nodes",
				Description: "List every node in the EdgeOS fleet with its live capabilities: model, state, measured tok/s, health, active requests.",
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
			},
			handler: func(ctx context.Context, _ json.RawMessage) toolsCallResult {
				body, err := c.get(ctx, "/v0/nodes")
				if err != nil {
					return errorResult(err.Error())
				}
				return textResult(string(body))
			},
		},
		{
			def: tool{
				Name:        "fleet_status",
				Description: "Fleet-wide KPI summary: total/healthy node counts, active requests, total tok/s capacity, distinct models loaded, requests served (local/cloud) and failed.",
				InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
			},
			handler: func(ctx context.Context, _ json.RawMessage) toolsCallResult {
				body, err := c.get(ctx, "/v0/nodes")
				if err != nil {
					return errorResult(err.Error())
				}
				var parsed struct {
					Summary json.RawMessage `json:"summary"`
				}
				if err := json.Unmarshal(body, &parsed); err != nil {
					return errorResult("decode fleet status: " + err.Error())
				}
				return textResult(string(parsed.Summary))
			},
		},
		{
			def: tool{
				Name:        "ask_fleet",
				Description: "Send a prompt to a loaded model on the fleet and get back its response. model must match a model id from list_nodes exactly.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"model":      map[string]any{"type": "string", "description": "model id, e.g. from list_nodes"},
						"prompt":     map[string]any{"type": "string"},
						"max_tokens": map[string]any{"type": "integer", "description": "default 512"},
					},
					"required": []string{"model", "prompt"},
				},
			},
			handler: handleAskFleet(c),
		},
		{
			def: tool{
				Name: "ask_fleet_parallel",
				Description: "Send several independent prompts to the fleet concurrently instead of one at a time -- a fan-out primitive. " +
					"Each entry is scored and routed like a separate ask_fleet call would be; deciding how to split a task into these " +
					"prompts and how to combine the results is the caller's job. Max 32 prompts per call.",
				InputSchema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"prompts": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"model":      map[string]any{"type": "string"},
									"prompt":     map[string]any{"type": "string"},
									"max_tokens": map[string]any{"type": "integer", "description": "default 512"},
								},
								"required": []string{"model", "prompt"},
							},
						},
					},
					"required": []string{"prompts"},
				},
			},
			handler: handleAskFleetParallel(c),
		},
	}

	if c.ManagementToken != "" {
		tools = append(tools,
			toolEntry{
				def: tool{
					Name:        "add_node",
					Description: "Register a node by address for the router to poll -- for nodes mDNS can't reach (different subnet, etc). Requires a management token to be configured on this MCP server.",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"host": map[string]any{"type": "string"},
							"port": map[string]any{"type": "integer", "description": "defaults to 8090"},
						},
						"required": []string{"host"},
					},
				},
				handler: handleAddNode(c),
			},
			toolEntry{
				def: tool{
					Name:        "remove_node",
					Description: "Permanently remove a node from the fleet (blacklists it against mDNS rediscovery until re-added).",
					InputSchema: map[string]any{
						"type":       "object",
						"properties": map[string]any{"id": map[string]any{"type": "string"}},
						"required":   []string{"id"},
					},
				},
				handler: handleNodeAction(c, "remove"),
			},
			toolEntry{
				def: tool{
					Name:        "stop_node",
					Description: "Stop the running engine on a node.",
					InputSchema: map[string]any{
						"type":       "object",
						"properties": map[string]any{"id": map[string]any{"type": "string"}},
						"required":   []string{"id"},
					},
				},
				handler: handleNodeAction(c, "stop"),
			},
			toolEntry{
				def: tool{
					Name:        "reload_node",
					Description: "Reload a node's engine against a different model path.",
					InputSchema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"id":         map[string]any{"type": "string"},
							"model_path": map[string]any{"type": "string"},
						},
						"required": []string{"id", "model_path"},
					},
				},
				handler: handleNodeAction(c, "reload"),
			},
		)
	}

	return tools
}

func handleAskFleet(c *fleetClient) toolHandler {
	return func(ctx context.Context, args json.RawMessage) toolsCallResult {
		var params struct {
			Model     string `json:"model"`
			Prompt    string `json:"prompt"`
			MaxTokens int    `json:"max_tokens"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("invalid arguments: " + err.Error())
		}
		if params.Model == "" || params.Prompt == "" {
			return errorResult(`"model" and "prompt" are required`)
		}
		if params.MaxTokens == 0 {
			params.MaxTokens = 512
		}

		body, _ := json.Marshal(map[string]any{
			"model":      params.Model,
			"messages":   []map[string]string{{"role": "user", "content": params.Prompt}},
			"stream":     false,
			"max_tokens": params.MaxTokens,
		})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RouterURL+"/v1/chat/completions", bytes.NewReader(body))
		if err != nil {
			return errorResult(err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.do(req)
		if err != nil {
			return errorResult(err.Error())
		}

		var parsed struct {
			Choices []struct {
				Message struct {
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(resp, &parsed); err != nil {
			return errorResult("decode response: " + err.Error())
		}
		if len(parsed.Choices) == 0 {
			return errorResult("no choices in response")
		}
		msg := parsed.Choices[0].Message
		if msg.Content != "" {
			return textResult(msg.Content)
		}
		return textResult(msg.ReasoningContent)
	}
}

func handleAskFleetParallel(c *fleetClient) toolHandler {
	return func(ctx context.Context, args json.RawMessage) toolsCallResult {
		var params struct {
			Prompts []struct {
				Model     string `json:"model"`
				Prompt    string `json:"prompt"`
				MaxTokens int    `json:"max_tokens"`
			} `json:"prompts"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("invalid arguments: " + err.Error())
		}
		if len(params.Prompts) == 0 {
			return errorResult(`"prompts" must be a non-empty array`)
		}

		requests := make([]map[string]any, len(params.Prompts))
		for i, p := range params.Prompts {
			if p.Model == "" || p.Prompt == "" {
				return errorResult(fmt.Sprintf("prompts[%d]: \"model\" and \"prompt\" are required", i))
			}
			maxTokens := p.MaxTokens
			if maxTokens == 0 {
				maxTokens = 512
			}
			requests[i] = map[string]any{
				"model":      p.Model,
				"messages":   []map[string]string{{"role": "user", "content": p.Prompt}},
				"stream":     false,
				"max_tokens": maxTokens,
			}
		}

		body, _ := json.Marshal(map[string]any{"requests": requests})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.RouterURL+"/v0/parallel-completions", bytes.NewReader(body))
		if err != nil {
			return errorResult(err.Error())
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := c.do(req)
		if err != nil {
			return errorResult(err.Error())
		}

		var parsed struct {
			Results []struct {
				Index    int    `json:"index"`
				Status   string `json:"status"`
				Response struct {
					Choices []struct {
						Message struct {
							Content          string `json:"content"`
							ReasoningContent string `json:"reasoning_content"`
						} `json:"message"`
					} `json:"choices"`
				} `json:"response"`
				Error *struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			} `json:"results"`
		}
		if err := json.Unmarshal(resp, &parsed); err != nil {
			return errorResult("decode response: " + err.Error())
		}

		var out strings.Builder
		anyError := false
		for _, r := range parsed.Results {
			fmt.Fprintf(&out, "[%d] ", r.Index)
			if r.Status != "ok" {
				anyError = true
				fmt.Fprintf(&out, "error (%s): %s\n", r.Error.Type, r.Error.Message)
				continue
			}
			if len(r.Response.Choices) == 0 {
				fmt.Fprintln(&out, "error: no choices in response")
				anyError = true
				continue
			}
			msg := r.Response.Choices[0].Message
			text := msg.Content
			if text == "" {
				text = msg.ReasoningContent
			}
			fmt.Fprintf(&out, "%s\n", text)
		}

		result := textResult(out.String())
		result.IsError = anyError && len(parsed.Results) == 1 // only surface as a tool error if the sole result failed
		return result
	}
}

func handleAddNode(c *fleetClient) toolHandler {
	return func(ctx context.Context, args json.RawMessage) toolsCallResult {
		var params struct {
			Host string `json:"host"`
			Port int    `json:"port"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("invalid arguments: " + err.Error())
		}
		if params.Host == "" {
			return errorResult(`"host" is required`)
		}
		body, _ := json.Marshal(params)
		resp, err := c.write(ctx, http.MethodPost, "/v0/nodes", body)
		if err != nil {
			return errorResult(err.Error())
		}
		return textResult(string(resp))
	}
}

func handleNodeAction(c *fleetClient, action string) toolHandler {
	return func(ctx context.Context, args json.RawMessage) toolsCallResult {
		var params struct {
			ID        string `json:"id"`
			ModelPath string `json:"model_path"`
		}
		if err := json.Unmarshal(args, &params); err != nil {
			return errorResult("invalid arguments: " + err.Error())
		}
		if params.ID == "" {
			return errorResult(`"id" is required`)
		}

		var (
			resp []byte
			err  error
		)
		switch action {
		case "remove":
			resp, err = c.write(ctx, http.MethodDelete, "/v0/nodes/"+params.ID, nil)
		case "stop":
			resp, err = c.write(ctx, http.MethodPost, "/v0/nodes/"+params.ID+"/actions/stop", nil)
		case "reload":
			if params.ModelPath == "" {
				return errorResult(`"model_path" is required`)
			}
			body, _ := json.Marshal(map[string]string{"model_path": params.ModelPath})
			resp, err = c.write(ctx, http.MethodPost, "/v0/nodes/"+params.ID+"/actions/reload", body)
		}
		if err != nil {
			return errorResult(err.Error())
		}
		return textResult(string(resp))
	}
}
