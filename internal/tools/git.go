package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/caged-dev/mcp-server/internal/mcp"
)

// --- GitStatusTool ---

type GitStatusTool struct{ workspace string }

func (t *GitStatusTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "git_status",
		Description: "Show the working tree status (git status).",
		InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}},
	}
}

func (t *GitStatusTool) Execute(ctx context.Context, _ json.RawMessage) mcp.ToolCallResult {
	return runGit(ctx, t.workspace, "status", "--porcelain=v1")
}

// --- GitDiffTool ---

type GitDiffTool struct{ workspace string }

func (t *GitDiffTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "git_diff",
		Description: "Show changes in the working directory (git diff).",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"staged": map[string]interface{}{"type": "boolean", "description": "Show staged changes", "default": false},
			},
		},
	}
}

func (t *GitDiffTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Staged bool `json:"staged"`
	}
	json.Unmarshal(args, &params)

	if params.Staged {
		return runGit(ctx, t.workspace, "diff", "--cached")
	}
	return runGit(ctx, t.workspace, "diff")
}

// --- GitLogTool ---

type GitLogTool struct{ workspace string }

func (t *GitLogTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "git_log",
		Description: "Show recent commit history.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"count": map[string]interface{}{"type": "integer", "description": "Number of commits (default: 10)", "default": 10},
			},
		},
	}
}

func (t *GitLogTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Count int `json:"count"`
	}
	json.Unmarshal(args, &params)
	if params.Count <= 0 {
		params.Count = 10
	}
	return runGit(ctx, t.workspace, "log", "--oneline", fmt.Sprintf("-n%d", params.Count))
}

// --- GitCommitTool ---

type GitCommitTool struct{ workspace string }

func (t *GitCommitTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "git_commit",
		Description: "Stage all changes and create a commit.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]string{"type": "string", "description": "Commit message"},
			},
			"required": []string{"message"},
		},
	}
}

func (t *GitCommitTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	if params.Message == "" {
		return errorResult("message is required")
	}

	// Stage all.
	result := runGit(ctx, t.workspace, "add", "-A")
	if result.IsError {
		return result
	}

	// Commit.
	return runGit(ctx, t.workspace, "commit", "-m", params.Message)
}

// --- helper ---

func runGit(ctx context.Context, dir string, args ...string) mcp.ToolCallResult {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	var output strings.Builder
	if stdout.Len() > 0 {
		output.WriteString(stdout.String())
	}
	if stderr.Len() > 0 {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString(stderr.String())
	}

	if err != nil {
		return mcp.ToolCallResult{
			Content: []mcp.ContentBlock{{Type: "text", Text: output.String()}},
			IsError: true,
		}
	}

	text := output.String()
	if text == "" {
		text = "(no output)"
	}
	return textResult(text)
}
