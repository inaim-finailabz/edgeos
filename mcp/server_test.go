package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"strings"
	"testing"
)

func testTools() []toolEntry {
	return []toolEntry{
		{
			def: tool{Name: "echo", Description: "echoes its input", InputSchema: map[string]any{"type": "object"}},
			handler: func(ctx context.Context, args json.RawMessage) toolsCallResult {
				return textResult(string(args))
			},
		},
		{
			def: tool{Name: "boom", Description: "always errors"},
			handler: func(ctx context.Context, args json.RawMessage) toolsCallResult {
				return errorResult("boom failed")
			},
		},
	}
}

func runLine(t *testing.T, srv *Server, line string) rpcResponse {
	t.Helper()
	var out bytes.Buffer
	errLog := log.New(io.Discard, "", 0)
	if err := srv.Run(context.Background(), strings.NewReader(line+"\n"), &out, errLog); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Len() == 0 {
		return rpcResponse{} // notification: no response written
	}
	var resp rpcResponse
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %s: %v", out.String(), err)
	}
	return resp
}

func TestServer_Initialize(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}`)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("result type = %T", resp.Result)
	}
	if result["protocolVersion"] != mcpProtocolVersion {
		t.Errorf("protocolVersion = %v, want %s", result["protocolVersion"], mcpProtocolVersion)
	}
}

func TestServer_NotificationGetsNoResponse(t *testing.T) {
	srv := NewServer(testTools())
	var out bytes.Buffer
	errLog := log.New(io.Discard, "", 0)
	err := srv.Run(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"), &out, errLog)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("notification should produce no output, got %q", out.String())
	}
}

func TestServer_ToolsList(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	tools := result["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("got %d tools, want 2", len(tools))
	}
	first := tools[0].(map[string]any)
	if first["name"] != "echo" {
		t.Errorf("first tool = %v, want echo", first["name"])
	}
}

func TestServer_ToolsCall_Success(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"echo","arguments":{"x":1}}}`)

	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}
	result := resp.Result.(map[string]any)
	if result["isError"] == true {
		t.Error("isError should be unset/false for a successful call")
	}
	content := result["content"].([]any)[0].(map[string]any)
	if content["text"] != `{"x":1}` {
		t.Errorf("text = %v, want the echoed arguments", content["text"])
	}
}

func TestServer_ToolsCall_HandlerError(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"boom","arguments":{}}}`)

	result := resp.Result.(map[string]any)
	if result["isError"] != true {
		t.Error("isError should be true when the tool handler fails")
	}
}

func TestServer_ToolsCall_UnknownTool(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"nope","arguments":{}}}`)

	if resp.Error == nil || resp.Error.Code != errInvalidParams {
		t.Fatalf("error = %+v, want errInvalidParams", resp.Error)
	}
}

func TestServer_UnknownMethod(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{"jsonrpc":"2.0","id":6,"method":"nonexistent"}`)

	if resp.Error == nil || resp.Error.Code != errMethodNotFound {
		t.Fatalf("error = %+v, want errMethodNotFound", resp.Error)
	}
}

func TestServer_MalformedJSON(t *testing.T) {
	srv := NewServer(testTools())
	resp := runLine(t, srv, `{not json`)

	if resp.Error == nil || resp.Error.Code != errParseError {
		t.Fatalf("error = %+v, want errParseError", resp.Error)
	}
}
