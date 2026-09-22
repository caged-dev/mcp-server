package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/caged-dev/mcp-server/internal/mcp"
)

// --- FileReadTool ---

type FileReadTool struct{ workspace string }

func (t *FileReadTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "filesystem_read",
		Description: "Read the contents of a file.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]string{"type": "string", "description": "File path relative to workspace"},
			},
			"required": []string{"path"},
		},
	}
}

func (t *FileReadTool) Execute(_ context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	fullPath, err := safePath(t.workspace, params.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return errorResult("read error: " + err.Error())
	}

	return textResult(string(data))
}

// --- FileWriteTool ---

type FileWriteTool struct{ workspace string }

func (t *FileWriteTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "filesystem_write",
		Description: "Write content to a file. Creates parent directories if needed.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":    map[string]string{"type": "string", "description": "File path relative to workspace"},
				"content": map[string]string{"type": "string", "description": "File content to write"},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (t *FileWriteTool) Execute(_ context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	fullPath, err := safePath(t.workspace, params.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return errorResult("mkdir error: " + err.Error())
	}

	if err := os.WriteFile(fullPath, []byte(params.Content), 0644); err != nil {
		return errorResult("write error: " + err.Error())
	}

	return textResult(fmt.Sprintf("wrote %d bytes to %s", len(params.Content), params.Path))
}

// --- FileListTool ---

type FileListTool struct{ workspace string }

func (t *FileListTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "filesystem_list",
		Description: "List files and directories in a path.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]string{"type": "string", "description": "Directory path relative to workspace"},
			},
		},
		OutputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]string{"type": "string", "description": "Directory listed, relative to the workspace"},
				"entries": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"name": map[string]string{"type": "string"},
							"type": map[string]interface{}{"type": "string", "enum": []string{"file", "directory"}},
							"size": map[string]string{"type": "integer", "description": "Size in bytes as reported by the filesystem"},
						},
						"required": []string{"name", "type", "size"},
					},
				},
			},
			"required": []string{"path", "entries"},
		},
	}
}

// FileEntry is one row of a filesystem_list result.
type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

// FileListOutput is the structured result of filesystem_list.
type FileListOutput struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

func (t *FileListTool) Execute(_ context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(args, &params)
	if params.Path == "" {
		params.Path = "."
	}

	fullPath, err := safePath(t.workspace, params.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return errorResult("list error: " + err.Error())
	}

	var lines []string
	out := FileListOutput{Path: params.Path, Entries: make([]FileEntry, 0, len(entries))}
	for _, e := range entries {
		info, _ := e.Info()
		prefix := "  "
		kind := "file"
		if e.IsDir() {
			prefix = "d "
			kind = "directory"
		}
		size := int64(0)
		if info != nil {
			size = info.Size()
		}
		lines = append(lines, fmt.Sprintf("%s%8d  %s", prefix, size, e.Name()))
		out.Entries = append(out.Entries, FileEntry{Name: e.Name(), Type: kind, Size: size})
	}

	return structuredResult(strings.Join(lines, "\n"), out)
}

// --- FileDeleteTool ---

type FileDeleteTool struct{ workspace string }

func (t *FileDeleteTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "filesystem_delete",
		Description: "Delete a file or directory.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]string{"type": "string", "description": "Path relative to workspace"},
			},
			"required": []string{"path"},
		},
	}
}

func (t *FileDeleteTool) Execute(_ context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	fullPath, err := safePath(t.workspace, params.Path)
	if err != nil {
		return errorResult(err.Error())
	}

	if err := os.RemoveAll(fullPath); err != nil {
		return errorResult("delete error: " + err.Error())
	}

	return textResult("deleted: " + params.Path)
}

// --- FileSearchTool ---

type FileSearchTool struct{ workspace string }

func (t *FileSearchTool) Definition() mcp.ToolDefinition {
	return mcp.ToolDefinition{
		Name:        "filesystem_search",
		Description: "Search for files matching a glob pattern.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"pattern": map[string]string{"type": "string", "description": "Glob pattern (e.g. **/*.go)"},
			},
			"required": []string{"pattern"},
		},
	}
}

func (t *FileSearchTool) Execute(_ context.Context, args json.RawMessage) mcp.ToolCallResult {
	var params struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	var matches []string
	_ = filepath.Walk(t.workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(t.workspace, path)
		matched, _ := filepath.Match(params.Pattern, filepath.Base(path))
		if matched {
			matches = append(matches, rel)
		}
		return nil
	})

	if len(matches) == 0 {
		return textResult("no matches found")
	}
	return textResult(strings.Join(matches, "\n"))
}

// --- helpers ---

func safePath(workspace, relPath string) (string, error) {
	if relPath == "" {
		return workspace, nil
	}
	// Clean and resolve.
	cleaned := filepath.Clean(relPath)
	if filepath.IsAbs(cleaned) {
		// Allow absolute paths within workspace.
		if !strings.HasPrefix(cleaned, workspace) {
			return "", fmt.Errorf("path traversal blocked: %s", relPath)
		}
		return cleaned, nil
	}
	full := filepath.Join(workspace, cleaned)
	// Verify still within workspace after resolution.
	resolved, err := filepath.Abs(full)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}
	absWorkspace, _ := filepath.Abs(workspace)
	if !strings.HasPrefix(resolved, absWorkspace) {
		return "", fmt.Errorf("path traversal blocked: %s", relPath)
	}
	return resolved, nil
}

// structuredResult returns a tool result carrying both the human-readable
// text and its machine-readable form. `structuredContent` was introduced in
// protocol revision 2025-06-18; the server drops it for sessions negotiated
// below that. The text block is always present, which is what the
// specification asks of a tool that returns structured content.
func structuredResult(text string, payload interface{}) mcp.ToolCallResult {
	res := textResult(text)
	raw, err := json.Marshal(payload)
	if err != nil {
		// A result the caller can still read beats an error: the text block
		// carries the same information.
		return res
	}
	res.StructuredContent = raw
	return res
}

// withStructured attaches a machine-readable payload to an existing result.
func withStructured(res mcp.ToolCallResult, payload interface{}) mcp.ToolCallResult {
	raw, err := json.Marshal(payload)
	if err != nil {
		return res
	}
	res.StructuredContent = raw
	return res
}

func textResult(text string) mcp.ToolCallResult {
	return mcp.ToolCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: text}},
	}
}

func errorResult(msg string) mcp.ToolCallResult {
	return mcp.ToolCallResult{
		Content: []mcp.ContentBlock{{Type: "text", Text: msg}},
		IsError: true,
	}
}
