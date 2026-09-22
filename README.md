# caged-mcp-server

A standalone **Model Context Protocol (MCP)** server that exposes sandbox tools to AI coding agents. Compatible with Claude Code, Cursor, Aider, and any MCP-compatible client.

## What This Does

This MCP server provides AI agents with tools to interact with an isolated sandbox environment:

| Tool | Description | Structured output |
|------|-------------|-------------------|
| `filesystem_read` | Read file contents | |
| `filesystem_write` | Write/create files | |
| `filesystem_list` | List directory contents | yes |
| `filesystem_delete` | Remove files/directories | |
| `filesystem_search` | Search for files by pattern | |
| `terminal_exec` | Execute shell commands | |
| `git_status` | Get git repository status | yes |
| `git_diff` | Show file changes | |
| `git_commit` | Create commits | |
| `git_log` | View commit history | yes |

Tools marked *structured output* return `structuredContent` against a
declared `outputSchema` as well as the text block, for clients on protocol
revision 2025-06-18 or later.

`--read-only` removes `filesystem_write`, `filesystem_delete`,
`terminal_exec` and `git_commit`.

Setting `--api-key` additionally exposes the Caged platform tools —
`pipeline_*` (list, get, create, delete, run, runs, state) and `a2a_*`
(agents, discover, delegate, tasks). They are absent without it.

*Corrected 2026-09-22: this table used to list `terminal_interactive` and
`network_fetch`. Neither exists in this binary and neither ever has —
`grep` for them returns nothing. They have been removed rather than
implemented, because adding tools is a change to a surface people have
wired into their editors.*

## Installation

The released binary is called **`caged-mcp-server`**.

```bash
# Pre-built binary (releases ship tar.gz archives, one per os/arch)
VERSION=0.1.0
curl -fsSL "https://github.com/caged-dev/mcp-server/releases/download/v${VERSION}/caged-mcp-server_${VERSION}_linux_amd64.tar.gz" \
  | tar xz caged-mcp-server
sudo install -m 0755 caged-mcp-server /usr/local/bin/caged-mcp-server

# Homebrew
brew install caged-dev/tap/caged-mcp-server

# From source (note the /cmd/mcp-server suffix; this installs as `mcp-server`)
go install github.com/caged-dev/mcp-server/cmd/mcp-server@latest

# Docker
docker pull ghcr.io/caged-dev/mcp-server:latest
```

*Corrected 2026-09-22: every line above except the Docker one was wrong, and
each was checked against the published artifacts before being changed.*
`curl .../releases/latest/download/caged-mcp-linux-amd64` returned **404** —
that asset name exists only on an abandoned draft release; the published
release carries `caged-mcp-server_<version>_<os>_<arch>.tar.gz`.
`go install github.com/caged-dev/mcp-server@latest` failed with *"module
found, but does not contain package"*, because the module root holds no
`main` package. And the binary inside the archive, and the one Homebrew
installs, is `caged-mcp-server`, not the `caged-mcp` every configuration
example below used to invoke — so a user who followed both halves of this
README got `command not found`.

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
caged-mcp-server --mode stdio --workspace /path/to/project

# Serve over WebSocket on port 9090
caged-mcp-server --mode ws --port 9090 --workspace /path/to/project
```

### Claude Code Configuration

Add to your `~/.claude/mcp_servers.json`:

```json
{
  "caged": {
    "command": "caged-mcp-server",
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
      "command": "caged-mcp-server",
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
- **Command allowlisting**: Optionally restrict which shell commands can be
  executed. Commands run through `sh -c`, so while an allowlist is
  configured a command containing a shell operator (`;`, `&&`, `||`, `|`,
  `&`, `$(`, backtick, redirection or a newline) is refused outright: only
  the first word is on the allowlist, and an allowlist that can be walked
  past with `;` is not an allowlist. Without an allowlist nothing is
  restricted and shell operators work as before.
- **Read-only mode**: Disable all write operations for safe exploration.

There is no outbound network tool in this binary, with or without an
allowlist. (This section previously claimed `network_fetch` "must be
explicitly enabled"; there is no such tool and no flag that enables one.)

## Building

```bash
go build -o caged-mcp-server ./cmd/mcp-server

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o caged-mcp-server ./cmd/mcp-server
```

## Development

```bash
go test ./...
go test -race ./...
golangci-lint run
```

## Protocol

Implements the [Model Context Protocol](https://modelcontextprotocol.io) as a
**dual-era server**: revision **`2026-07-28`** (current stable, stateless),
and the handshake-based revisions **`2025-11-25`** and **`2024-11-05`**.

*Corrected 2026-09-22: this section used to state `2024-11-05` as fact. That
was true and five revisions out of date. The current specification's
compatibility matrix says a modern client against a legacy server "fails …
the server may reject the request with an implementation-defined error,
**stay silent**, or even process an era-ambiguous method under legacy
semantics" — so the editor configurations below would have broken, silently,
the day their clients went modern-only.*

Which revision you get is decided by how your client opens the connection,
never by configuration:

| Your client sends | You get |
|---|---|
| `_meta.io.modelcontextprotocol/protocolVersion` on a request | that revision, served statelessly — no handshake |
| `initialize` | the legacy handshake, negotiated down to a revision you named or older |
| neither | `2024-11-05`, exactly as before this change |

Implemented from `2026-07-28`:

- per-request `_meta` (`protocolVersion`, `clientInfo`, `clientCapabilities`)
- `server/discover`, which servers MUST implement and which dual-era clients
  use as their era probe
- `resultType: "complete"` on every modern result
- `ttlMs` and `cacheScope` on `tools/list` (and on `server/discover`)
- `UnsupportedProtocolVersionError` (`-32022`) carrying the supported list,
  so a client can retry rather than guess
- `io.modelcontextprotocol/serverInfo` in every modern result's `_meta`
- deterministic tool ordering, for prompt-cache stability

Not implemented, and why:

- **Streamable HTTP transport.** This server offers stdio and WebSocket.
  WebSocket has never been an MCP transport; it is what the Caged platform
  endpoint uses. stdio — the transport every editor configuration below uses
  — is fully conformant.
- **`resources/*`, `prompts/*`, subscriptions, MRTR.** No tool here needs to
  ask the client a question, so no result is ever `input_required`.
- **`ping` is still answered in the modern era**, although `2026-07-28`
  removes it. Clients send it as a keepalive regardless of revision, and
  failing one would drop a working connection for no benefit.
- **A `server/discover` probe carrying no `_meta` at all is answered**
  rather than rejected as malformed, because refusing the era probe would
  hide the very version list the client is asking for. Any other request
  missing a required `_meta` field is rejected with `-32602`, as specified.

`docs/protocol.md` has the era-selection flow, the negotiation table and the
full list of what is and is not implemented.

### Upgrading

Nothing to do. A client that handshakes today keeps getting byte-identical
responses; `structuredContent` and `outputSchema` only appear for clients on
`2025-06-18` or later, the revision that introduced them.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

MIT — see [LICENSE](LICENSE).
