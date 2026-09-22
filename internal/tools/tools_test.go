package tools

import (
	"testing"

	"github.com/caged-dev/mcp-server/internal/mcp"
)

func toolNames(tools []mcp.Tool) map[string]bool {
	names := make(map[string]bool, len(tools))
	for _, t := range tools {
		names[t.Definition().Name] = true
	}
	return names
}

func TestToolSetReadOnly(t *testing.T) {
	ws := t.TempDir()
	tests := []struct {
		name     string
		opts     Options
		present  []string
		excluded []string
	}{
		{
			name:     "default exposes the full local surface",
			opts:     Options{},
			present:  []string{"filesystem_read", "filesystem_list", "filesystem_search", "filesystem_write", "filesystem_delete", "terminal_exec", "git_status", "git_diff", "git_log", "git_commit"},
			excluded: nil,
		},
		{
			name:     "read-only drops every mutating tool",
			opts:     Options{ReadOnly: true},
			present:  []string{"filesystem_read", "filesystem_list", "filesystem_search", "git_status", "git_diff", "git_log"},
			excluded: []string{"filesystem_write", "filesystem_delete", "terminal_exec", "git_commit"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			names := toolNames(NewToolSet(ws, tc.opts).All())
			for _, n := range tc.present {
				if !names[n] {
					t.Errorf("%s missing", n)
				}
			}
			for _, n := range tc.excluded {
				if names[n] {
					t.Errorf("%s should not be exposed in read-only mode", n)
				}
			}
		})
	}
}

// The tool surface is what users have wired into their editors; a rename or
// a removal is a breaking change, so it is pinned here.
func TestToolSurfaceIsStable(t *testing.T) {
	want := []string{
		"filesystem_read", "filesystem_list", "filesystem_search",
		"filesystem_write", "filesystem_delete",
		"terminal_exec",
		"git_status", "git_diff", "git_log", "git_commit",
	}
	names := toolNames(NewToolSet(t.TempDir(), Options{}).All())
	if len(names) != len(want) {
		t.Errorf("tool count = %d, want %d: %v", len(names), len(want), names)
	}
	for _, n := range want {
		if !names[n] {
			t.Errorf("tool %s disappeared from the surface", n)
		}
	}
}
