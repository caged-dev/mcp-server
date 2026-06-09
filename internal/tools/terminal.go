package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/caged-dev/mcp-server/internal/mcp"
)

// TerminalExecTool executes shell commands.
type TerminalExecTool struct {
	workspace string
	allowed   []string // Empty = allow all.
}

func (t *TerminalExecTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "terminal_exec",
		Description: "Execute a shell command in the workspace.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]string{"type": "string", "description": "Shell command to execute"},
				"timeout": map[string]interface{}{"type": "integer", "description": "Timeout in seconds (default: 30)", "default": 30},
			},
			"required": []string{"command"},
		},
	}
}

func (t *TerminalExecTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	if params.Command == "" {
		return errorResult("command is required")
	}

	// Check allowlist.
	if len(t.allowed) > 0 {
		cmdName := strings.Fields(params.Command)[0]
		allowed := false
		for _, a := range t.allowed {
			if a == cmdName {
				allowed = true
				break
			}
		}
		if !allowed {
			return errorResult(fmt.Sprintf("command %q not in allowlist", cmdName))
		}
	}

	timeout := 30 * time.Second
	if params.Timeout > 0 {
		timeout = time.Duration(params.Timeout) * time.Second
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "sh", "-c", params.Command)
	cmd.Dir = t.workspace

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var result strings.Builder
	if stdout.Len() > 0 {
		result.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderr.String())
	}
	if err != nil {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString(fmt.Sprintf("exit: %v", err))
	}

	if err != nil {
		return mcp.ToolCallResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: result.String()}},
			IsError: true,
		}
	}
	return textResult(result.String())
}
