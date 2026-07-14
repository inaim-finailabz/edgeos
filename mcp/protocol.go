// Package main implements a minimal MCP (Model Context Protocol) server
// over stdio, exposing the EdgeOS fleet as tools an MCP client (Claude
// Desktop, Claude Code, etc.) can call: list nodes, check fleet status,
// send a prompt to the fleet, and — if a management token is configured —
// add/remove nodes and stop/reload an engine.
//
// No MCP SDK dependency: the stdio JSON-RPC framing (newline-delimited
// JSON, one message per line) is simple enough to implement correctly
// against stdlib alone, consistent with the rest of this project.
package main

import "encoding/json"

const jsonRPCVersion = "2.0"

// rpcRequest covers both requests (id present) and notifications (id
// omitted) coming from the client.
type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// JSON-RPC 2.0 standard error codes.
const (
	errParseError     = -32700
	errInvalidRequest = -32600
	errMethodNotFound = -32601
	errInvalidParams  = -32602
	errInternal       = -32603
)

func newResult(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: jsonRPCVersion, ID: id, Result: result}
}

func newError(id json.RawMessage, code int, message string) rpcResponse {
	return rpcResponse{JSONRPC: jsonRPCVersion, ID: id, Error: &rpcError{Code: code, Message: message}}
}

// --- MCP-specific shapes (the subset of the spec this server needs) ---

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	ClientInfo      map[string]any `json:"clientInfo"`
}

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      serverInfo     `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// mcpProtocolVersion is pinned to a known-good spec revision rather than
// echoing whatever the client sends, so behavior doesn't shift under us if
// a client claims a newer version this server hasn't been checked against.
const mcpProtocolVersion = "2024-11-05"

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type toolsCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolsCallResult struct {
	Content []content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

func textResult(s string) toolsCallResult {
	return toolsCallResult{Content: []content{{Type: "text", Text: s}}}
}

func errorResult(s string) toolsCallResult {
	return toolsCallResult{Content: []content{{Type: "text", Text: s}}, IsError: true}
}
