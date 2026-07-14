package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

// Server dispatches JSON-RPC requests read from an input stream to MCP
// method handlers, writing one JSON response per line to an output
// stream. Never write anything but protocol messages to the output
// stream: on stdio transport, stdout is the wire, and any stray log line
// there corrupts the JSON-RPC stream for the client.
type Server struct {
	tools map[string]toolEntry
	order []string // preserves registration order for tools/list
}

func NewServer(entries []toolEntry) *Server {
	s := &Server{tools: make(map[string]toolEntry, len(entries))}
	for _, e := range entries {
		s.tools[e.def.Name] = e
		s.order = append(s.order, e.def.Name)
	}
	return s
}

// Run reads newline-delimited JSON-RPC messages from r and writes
// responses to w until r is exhausted (EOF, e.g. the client closed
// stdin). errLog receives anything that isn't a protocol message.
func (s *Server) Run(ctx context.Context, r io.Reader, w io.Writer, errLog *log.Logger) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeMsg(w, newError(nil, errParseError, "parse error: "+err.Error()))
			continue
		}

		resp, isNotification := s.handle(ctx, req)
		if isNotification {
			continue // notifications (no id) get no response, per JSON-RPC
		}
		if err := writeMsg(w, resp); err != nil {
			errLog.Printf("write response: %v", err)
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, req rpcRequest) (rpcResponse, bool) {
	isNotification := len(req.ID) == 0

	switch req.Method {
	case "initialize":
		return newResult(req.ID, initializeResult{
			ProtocolVersion: mcpProtocolVersion,
			Capabilities:    map[string]any{"tools": map[string]any{}},
			ServerInfo:      serverInfo{Name: "edgeos-mcp", Version: "0.1.0"},
		}), isNotification

	case "notifications/initialized", "notifications/cancelled":
		return rpcResponse{}, true // no response expected for notifications

	case "ping":
		return newResult(req.ID, map[string]any{}), isNotification

	case "tools/list":
		list := make([]tool, 0, len(s.order))
		for _, name := range s.order {
			list = append(list, s.tools[name].def)
		}
		return newResult(req.ID, toolsListResult{Tools: list}), isNotification

	case "tools/call":
		var params toolsCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return newError(req.ID, errInvalidParams, "invalid params: "+err.Error()), isNotification
		}
		entry, ok := s.tools[params.Name]
		if !ok {
			return newError(req.ID, errInvalidParams, fmt.Sprintf("unknown tool %q", params.Name)), isNotification
		}
		return newResult(req.ID, entry.handler(ctx, params.Arguments)), isNotification

	default:
		if isNotification {
			return rpcResponse{}, true // unknown notifications are silently ignored
		}
		return newError(req.ID, errMethodNotFound, "method not found: "+req.Method), isNotification
	}
}

func writeMsg(w io.Writer, msg rpcResponse) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
