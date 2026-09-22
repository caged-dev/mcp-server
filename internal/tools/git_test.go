package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGitStatus(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   GitStatusOutput
	}{
		{
			name:   "clean tree",
			stdout: "",
			want:   GitStatusOutput{Clean: true, Files: []GitStatusEntry{}},
		},
		{
			name:   "modified and untracked",
			stdout: " M internal/mcp/server.go\n?? notes.txt\n",
			want: GitStatusOutput{Clean: false, Files: []GitStatusEntry{
				{Path: "internal/mcp/server.go", Index: " ", WorkTree: "M"},
				{Path: "notes.txt", Index: "?", WorkTree: "?"},
			}},
		},
		{
			name:   "staged add",
			stdout: "A  cmd/new.go\n",
			want: GitStatusOutput{Clean: false, Files: []GitStatusEntry{
				{Path: "cmd/new.go", Index: "A", WorkTree: " "},
			}},
		},
		{
			name:   "rename keeps the original path",
			stdout: "R  old/name.go -> new/name.go\n",
			want: GitStatusOutput{Clean: false, Files: []GitStatusEntry{
				{Path: "new/name.go", Index: "R", WorkTree: " ", RenamedFrom: "old/name.go"},
			}},
		},
		{
			name:   "quoted path is unquoted",
			stdout: "?? \"odd\\tname.txt\"\n",
			want: GitStatusOutput{Clean: false, Files: []GitStatusEntry{
				{Path: "odd\tname.txt", Index: "?", WorkTree: "?"},
			}},
		},
		{
			name:   "short lines are ignored",
			stdout: "\nx\n",
			want:   GitStatusOutput{Clean: true, Files: []GitStatusEntry{}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseGitStatus(tc.stdout)
			gotJSON, _ := json.Marshal(got)
			wantJSON, _ := json.Marshal(tc.want)
			if string(gotJSON) != string(wantJSON) {
				t.Errorf("\n got %s\nwant %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestParseGitLog(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   GitLogOutput
	}{
		{"empty", "", GitLogOutput{Commits: []GitCommit{}}},
		{
			name:   "two commits",
			stdout: "151b8ad feat: add A2A MCP tools\nabc1234 fix: something else\n",
			want: GitLogOutput{Commits: []GitCommit{
				{SHA: "151b8ad", Subject: "feat: add A2A MCP tools"},
				{SHA: "abc1234", Subject: "fix: something else"},
			}},
		},
		{
			name:   "subject-less commit",
			stdout: "151b8ad\n",
			want:   GitLogOutput{Commits: []GitCommit{{SHA: "151b8ad"}}},
		},
		{
			name:   "carriage returns trimmed",
			stdout: "151b8ad subject\r\n",
			want:   GitLogOutput{Commits: []GitCommit{{SHA: "151b8ad", Subject: "subject"}}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, _ := json.Marshal(parseGitLog(tc.stdout))
			want, _ := json.Marshal(tc.want)
			if string(got) != string(want) {
				t.Errorf("\n got %s\nwant %s", got, want)
			}
		})
	}
}

// newGitRepo builds a throwaway repository so the git tools run against a
// real git, not a stub.
func newGitRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ws := t.TempDir()
	env := append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = ws
		cmd.Env = env
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(ws, "first.txt"), []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "feat: the first commit")
	return ws
}

func TestGitStatusToolAgainstRealRepo(t *testing.T) {
	ws := newGitRepo(t)
	tool := &GitStatusTool{workspace: ws}

	res := tool.Execute(context.Background(), nil)
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0].Text)
	}
	var clean GitStatusOutput
	if err := json.Unmarshal(res.StructuredContent, &clean); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if !clean.Clean || len(clean.Files) != 0 {
		t.Errorf("fresh repo should be clean: %+v", clean)
	}

	if err := os.WriteFile(filepath.Join(ws, "second.txt"), []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	res = tool.Execute(context.Background(), nil)
	var dirty GitStatusOutput
	if err := json.Unmarshal(res.StructuredContent, &dirty); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if dirty.Clean || len(dirty.Files) != 1 || dirty.Files[0].Path != "second.txt" {
		t.Errorf("expected one untracked file, got %+v", dirty)
	}
	if !strings.Contains(res.Content[0].Text, "second.txt") {
		t.Errorf("text block = %q", res.Content[0].Text)
	}
}

func TestGitLogToolAgainstRealRepo(t *testing.T) {
	ws := newGitRepo(t)
	res := (&GitLogTool{workspace: ws}).Execute(context.Background(), json.RawMessage(`{"count":5}`))
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0].Text)
	}
	var out GitLogOutput
	if err := json.Unmarshal(res.StructuredContent, &out); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(out.Commits) != 1 {
		t.Fatalf("commits = %+v", out.Commits)
	}
	if out.Commits[0].Subject != "feat: the first commit" {
		t.Errorf("subject = %q", out.Commits[0].Subject)
	}
	if out.Commits[0].SHA == "" {
		t.Error("sha is empty")
	}
}

func TestGitToolsOutsideARepoReportErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	ws := t.TempDir()

	status := (&GitStatusTool{workspace: ws}).Execute(context.Background(), nil)
	if !status.IsError {
		t.Error("git_status outside a repository should be an error result")
	}
	if status.StructuredContent != nil {
		t.Error("error results must not carry structuredContent")
	}

	logRes := (&GitLogTool{workspace: ws}).Execute(context.Background(), nil)
	if !logRes.IsError {
		t.Error("git_log outside a repository should be an error result")
	}
}

func TestGitToolsDeclareOutputSchemas(t *testing.T) {
	ws := t.TempDir()
	for _, def := range []struct {
		name string
		got  interface{}
	}{
		{"git_status", (&GitStatusTool{workspace: ws}).Definition().OutputSchema},
		{"git_log", (&GitLogTool{workspace: ws}).Definition().OutputSchema},
	} {
		if def.got == nil {
			t.Errorf("%s must declare an outputSchema", def.name)
		}
	}
	if (&GitDiffTool{workspace: ws}).Definition().OutputSchema != nil {
		t.Error("git_diff has no natural structured shape and should not claim one")
	}
}
