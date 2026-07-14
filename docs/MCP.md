# MCP server

`mcp/` exposes the EdgeOS fleet to any MCP client (Claude Desktop, Claude
Code, or anything else speaking the Model Context Protocol) as tools: the
assistant can check what's running on your fleet, ask a node a question,
and — if you configure it — manage nodes directly.

No MCP SDK dependency. The stdio transport's JSON-RPC framing
(newline-delimited JSON, one message per line) is simple enough to
implement correctly against the standard library alone, consistent with
the rest of this project.

## Tools

Always available (no auth — same posture as the router's other read
paths):

- **`list_nodes`** — the full live fleet, same shape as `GET /v0/nodes`.
- **`fleet_status`** — just the KPI summary (node counts, tok/s capacity,
  requests served/failed).
- **`ask_fleet(model, prompt, max_tokens?)`** — sends a prompt to a loaded
  model and returns its response. `model` must match a `list_nodes` id
  exactly — no fuzzy matching, same as everywhere else in EdgeOS.

Only registered if `-management-token` (or `$EDGEOS_MANAGEMENT_TOKEN`) is
set — mirrors the "disabled unless a token is configured" default
everywhere else in EdgeOS, so an MCP client with no token can't discover
these tools at all, let alone call them:

- **`add_node(host, port?)`**
- **`remove_node(id)`**
- **`stop_node(id)`**
- **`reload_node(id, model_path)`**

## Setup

Build it like the other components (`make build` or `make cross`), then
point your MCP client at the binary. For Claude Desktop
(`claude_desktop_config.json`) or Claude Code (`.mcp.json`):

```json
{
  "mcpServers": {
    "edgeos": {
      "command": "/path/to/dist/mcp",
      "args": ["-router", "http://localhost:8081"],
      "env": {
        "EDGEOS_MANAGEMENT_TOKEN": "same token as your agents/router"
      }
    }
  }
}
```

Omit `EDGEOS_MANAGEMENT_TOKEN` (or leave it unset) to run read-only.

## Verifying it works

```sh
echo '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05"}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}' | ./dist/mcp -router http://localhost:8081
```

Should print an `initialize` result, then a `tools/list` result listing
the tools your token configuration allows.
