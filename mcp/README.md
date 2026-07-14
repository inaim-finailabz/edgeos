# edgeos-mcp (Go)
MCP (Model Context Protocol) server over stdio, exposing the fleet to Claude Desktop/Claude Code/other MCP clients as tools: `list_nodes`, `fleet_status`, `ask_fleet`, `ask_fleet_parallel`, plus `add_node`/`remove_node`/`stop_node`/`reload_node` when `-management-token` is set. No SDK dependency — see docs/MCP.md.
