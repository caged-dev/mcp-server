package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
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
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"clean": map[string]string{"type": "boolean", "description": "True when the working tree has no changes"},
				"files": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"path":        map[string]string{"type": "string"},
							"index":       map[string]string{"type": "string", "description": "Porcelain v1 index status character (space when unmodified)"},
							"workTree":    map[string]string{"type": "string", "description": "Porcelain v1 work tree status character (space when unmodified)"},
							"renamedFrom": map[string]string{"type": "string", "description": "Previous path, for renames and copies"},
						},
						"required": []string{"path", "index", "workTree"},
					},
				},
			},
			"required": []string{"clean", "files"},
		},
	}
}

// GitStatusEntry is one changed path in a git_status result.
type GitStatusEntry struct {
	Path        string `json:"path"`
	Index       string `json:"index"`
	WorkTree    string `json:"workTree"`
	RenamedFrom string `json:"renamedFrom,omitempty"`
}

// GitStatusOutput is the structured result of git_status.
type GitStatusOutput struct {
	Clean bool             `json:"clean"`
	Files []GitStatusEntry `json:"files"`
}

func (t *GitStatusTool) Execute(ctx context.Context, _ json.RawMessage) mcp.ToolCallResult {
	res, stdout := runGitRaw(ctx, t.workspace, "status", "--porcelain=v1")
	if res.IsError {
		return res
	}
	return withStructured(res, parseGitStatus(stdout))
}

// parseGitStatus reads `git status --porcelain=v1`. The format is stable and
// documented: two status characters, a space, then the path, with renames
// and copies written as "ORIG -> PATH". Paths containing unusual bytes are
// C-quoted, which strconv.Unquote reverses.
func parseGitStatus(stdout string) GitStatusOutput {
	out := GitStatusOutput{Files: []GitStatusEntry{}}
	for _, line := range strings.Split(stdout, "\n") {
		if len(line) < 4 {
			continue
		}
		entry := GitStatusEntry{
			Index:    string(line[0]),
			WorkTree: string(line[1]),
		}
		rest := line[3:]
		if orig, path, found := strings.Cut(rest, " -> "); found {
			entry.RenamedFrom = unquoteGitPath(orig)
			entry.Path = unquoteGitPath(path)
		} else {
			entry.Path = unquoteGitPath(rest)
		}
		out.Files = append(out.Files, entry)
	}
	out.Clean = len(out.Files) == 0
	return out
}

func unquoteGitPath(p string) string {
	if !strings.HasPrefix(p, "\"") {
		return p
	}
	if unquoted, err := strconv.Unquote(p); err == nil {
		return unquoted
	}
	return p
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
	_ = json.Unmarshal(args, &params)

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
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"commits": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"sha":     map[string]string{"type": "string", "description": "Abbreviated commit hash"},
							"subject": map[string]string{"type": "string", "description": "First line of the commit message"},
						},
						"required": []string{"sha", "subject"},
					},
				},
			},
			"required": []string{"commits"},
		},
	}
}

// GitCommit is one entry of a git_log result.
type GitCommit struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
}

// GitLogOutput is the structured result of git_log.
type GitLogOutput struct {
	Commits []GitCommit `json:"commits"`
}

// parseGitLog reads the `git log --oneline` output this tool already
// produced, so the text block is unchanged from previous releases.
func parseGitLog(stdout string) GitLogOutput {
	out := GitLogOutput{Commits: []GitCommit{}}
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		sha, subject, _ := strings.Cut(line, " ")
		out.Commits = append(out.Commits, GitCommit{SHA: sha, Subject: subject})
	}
	return out
}

func (t *GitLogTool) Execute(ctx context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(args, &params)
	if params.Count <= 0 {
		params.Count = 10
	}
	res, stdout := runGitRaw(ctx, t.workspace, "log", "--oneline", fmt.Sprintf("-n%d", params.Count))
	if res.IsError {
		return res
	}
	return withStructured(res, parseGitLog(stdout))
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
	res, _ := runGitRaw(ctx, dir, args...)
	return res
}

// runGitRaw runs git and returns both the tool result and git's raw stdout,
// so callers that also produce structured output can parse it without
// running the command twice.
func runGitRaw(ctx context.Context, dir string, args ...string) (mcp.ToolCallResult, string) {
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
		}, stdout.String()
	}

	text := output.String()
	if text == "" {
		text = "(no output)"
	}
	return textResult(text), stdout.String()
}
