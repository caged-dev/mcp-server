# caged-mcp

A standalone **Model Context Protocol (MCP)** server that exposes sandbox tools to AI coding agents. Compatible with Claude Code, Cursor, Aider, and any MCP-compatible client.

## What This Does

This MCP server provides AI agents with tools to interact with an isolated sandbox environment:

| Tool | Description |
|------|-------------|
| `filesystem_read` | Read file contents |
| `filesystem_write` | Write/create files |
| `filesystem_list` | List directory contents |
| `filesystem_delete` | Remove files/directories |
| `filesystem_search` | Search for files by pattern |
| `terminal_exec` | Execute shell commands |
| `terminal_interactive` | Start interactive terminal sessions |
| `git_status` | Get git repository status |
| `git_diff` | Show file changes |
| `git_commit` | Create commits |
| `git_log` | View commit history |
| `network_fetch` | Make HTTP requests |

## Installation

```bash
# Pre-built binary
curl -fsSL https://github.com/caged-dev/mcp-server/releases/latest/download/caged-mcp-linux-amd64 -o /usr/local/bin/caged-mcp
chmod +x /usr/local/bin/caged-mcp

# From source
go install github.com/caged-dev/mcp-server@latest

# Docker
docker pull ghcr.io/caged-dev/mcp-server:latest
```

## Usage

### With Caged Platform (Recommended)

The MCP server is built into the Caged platform. Connect via WebSocket:

```
wss://api.caged.dev/v1/sandboxes/{id}/mcp?token={session_token}
```

### Standalone Mode

Run the MCP server locally, pointing at any directory:

```bash
# Serve current directory over stdio (for direct MCP client connection)
caged-mcp --mode stdio --workspace /path/to/project

# Serve over WebSocket on port 9090
caged-mcp --mode ws --port 9090 --workspace /path/to/project
```

### Claude Code Configuration

Add to your `~/.claude/mcp_servers.json`:

```json
{
  "caged": {
    "command": "caged-mcp",
    "args": ["--mode", "stdio", "--workspace", "/path/to/project"]
  }
}
```

### Cursor Configuration

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "caged": {
      "command": "caged-mcp",
      "args": ["--mode", "stdio", "--workspace", "."]
    }
  }
}
```

## Configuration

| Flag | Env | Default | Description |
|------|-----|---------|-------------|
| `--mode` | `CAGED_MCP_MODE` | `stdio` | Transport: `stdio` or `ws` |
| `--port` | `CAGED_MCP_PORT` | `9090` | WebSocket listen port |
| `--workspace` | `CAGED_MCP_WORKSPACE` | `.` | Root workspace directory |
| `--allowed-commands` | `CAGED_MCP_ALLOWED_COMMANDS` | (all) | Comma-separated allowed shell commands |
| `--read-only` | `CAGED_MCP_READ_ONLY` | `false` | Disable write/exec tools |
| `--log-level` | `CAGED_MCP_LOG_LEVEL` | `info` | Log level |

## Security

The MCP server enforces:

- **Path sandboxing**: All file operations are confined to the workspace directory. Path traversal (`../`) is blocked.
- **Command allowlisting**: Optionally restrict which shell commands can be executed.
- **Read-only mode**: Disable all write operations for safe exploration.
- **No network by default**: The `network_fetch` tool must be explicitly enabled.

## Building

```bash
go build -o caged-mcp ./cmd/mcp-server

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o caged-mcp ./cmd/mcp-server
```

## Development

```bash
go test ./...
go test -race ./...
golangci-lint run
```

## Protocol

Implements the [Model Context Protocol](https://modelcontextprotocol.io) specification (version `2024-11-05`).

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE).
