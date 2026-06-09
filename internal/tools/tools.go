// Package tools implements MCP tool definitions for sandbox interaction.
package tools

import (
	"github.com/caged-dev/mcp-server/internal/mcp"
)

// Options configures the tool set.
type Options struct {
	ReadOnly        bool
	AllowedCommands []string
}

// ToolSet holds all available tools for a workspace.
type ToolSet struct {
	workspace string
	options   Options
	tools     []mcp.Tool
}

// NewToolSet creates a tool set for the given workspace.
func NewToolSet(workspace string, opts Options) *ToolSet {
	ts := &ToolSet{
		workspace: workspace,
		options:   opts,
	}

	// Always include read tools.
	ts.tools = append(ts.tools,
		&FileReadTool{workspace: workspace},
		&FileListTool{workspace: workspace},
		&FileSearchTool{workspace: workspace},
		&GitStatusTool{workspace: workspace},
		&GitDiffTool{workspace: workspace},
		&GitLogTool{workspace: workspace},
	)

	// Write/exec tools only if not read-only.
	if !opts.ReadOnly {
		ts.tools = append(ts.tools,
			&FileWriteTool{workspace: workspace},
			&FileDeleteTool{workspace: workspace},
			&TerminalExecTool{workspace: workspace, allowed: opts.AllowedCommands},
			&GitCommitTool{workspace: workspace},
		)
	}

	return ts
}

// All returns all configured tools.
func (ts *ToolSet) All() []mcp.Tool {
	return ts.tools
}
