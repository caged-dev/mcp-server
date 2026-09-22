package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestFileListToolStructuredOutput(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(ws, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}

	tool := &FileListTool{workspace: ws}
	res := tool.Execute(context.Background(), json.RawMessage(`{}`))
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0].Text)
	}

	var out FileListOutput
	if err := json.Unmarshal(res.StructuredContent, &out); err != nil {
		t.Fatalf("decoding structuredContent: %v", err)
	}
	if out.Path != "." {
		t.Errorf("path = %q", out.Path)
	}
	if len(out.Entries) != 2 {
		t.Fatalf("entries = %+v", out.Entries)
	}
	if out.Entries[0].Name != "a.txt" || out.Entries[0].Type != "file" || out.Entries[0].Size != 5 {
		t.Errorf("file entry = %+v", out.Entries[0])
	}
	if out.Entries[1].Name != "sub" || out.Entries[1].Type != "directory" {
		t.Errorf("dir entry = %+v", out.Entries[1])
	}

	// The text block is what existing clients read; it must still be there.
	if !strings.Contains(res.Content[0].Text, "a.txt") {
		t.Errorf("text block = %q", res.Content[0].Text)
	}
}

func TestFileListToolDeclaresOutputSchema(t *testing.T) {
	def := (&FileListTool{workspace: t.TempDir()}).Definition()
	if def.OutputSchema == nil {
		t.Fatal("filesystem_list must declare an outputSchema for its structuredContent")
	}
	raw, err := json.Marshal(def.OutputSchema)
	if err != nil {
		t.Fatalf("marshalling schema: %v", err)
	}
	for _, want := range []string{`"entries"`, `"name"`, `"directory"`} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("outputSchema missing %s: %s", want, raw)
		}
	}
}

func TestFileListToolErrors(t *testing.T) {
	tool := &FileListTool{workspace: t.TempDir()}
	tests := []struct {
		name string
		args string
	}{
		{"missing directory", `{"path":"nope"}`},
		{"path traversal", `{"path":"../../etc"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := tool.Execute(context.Background(), json.RawMessage(tc.args))
			if !res.IsError {
				t.Fatalf("expected an error result, got %+v", res)
			}
			if res.StructuredContent != nil {
				t.Error("error results must not carry structuredContent")
			}
		})
	}
}

func TestSafePath(t *testing.T) {
	ws := t.TempDir()
	tests := []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"empty is the workspace", "", false},
		{"simple relative", "a/b.txt", false},
		{"dot", ".", false},
		{"parent escape", "../secret", true},
		{"deep parent escape", "a/../../secret", true},
		{"absolute outside", "/etc/passwd", true},
		{"absolute inside", filepath.Join(ws, "ok.txt"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := safePath(ws, tc.in)
			if (err != nil) != tc.wantErr {
				t.Errorf("safePath(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			}
		})
	}
}

func TestStructuredResultFallsBackToText(t *testing.T) {
	// A payload that cannot be marshalled must not lose the text block.
	res := structuredResult("plain text", make(chan int))
	if res.StructuredContent != nil {
		t.Error("unmarshallable payload should be dropped")
	}
	if res.Content[0].Text != "plain text" {
		t.Errorf("text block = %q", res.Content[0].Text)
	}
}

func TestFileReadWriteDeleteRoundTrip(t *testing.T) {
	ws := t.TempDir()
	write := &FileWriteTool{workspace: ws}
	read := &FileReadTool{workspace: ws}
	del := &FileDeleteTool{workspace: ws}
	ctx := context.Background()

	if res := write.Execute(ctx, json.RawMessage(`{"path":"nested/dir/file.txt","content":"payload"}`)); res.IsError {
		t.Fatalf("write: %s", res.Content[0].Text)
	}
	res := read.Execute(ctx, json.RawMessage(`{"path":"nested/dir/file.txt"}`))
	if res.IsError || res.Content[0].Text != "payload" {
		t.Fatalf("read = %+v", res)
	}
	if res := del.Execute(ctx, json.RawMessage(`{"path":"nested"}`)); res.IsError {
		t.Fatalf("delete: %s", res.Content[0].Text)
	}
	if res := read.Execute(ctx, json.RawMessage(`{"path":"nested/dir/file.txt"}`)); !res.IsError {
		t.Error("reading a deleted file should be an error")
	}
}

func TestFileToolsRejectTraversalAndBadArguments(t *testing.T) {
	ws := t.TempDir()
	ctx := context.Background()
	tests := []struct {
		name string
		run  func() bool
	}{
		{"read traversal", func() bool {
			return (&FileReadTool{workspace: ws}).Execute(ctx, json.RawMessage(`{"path":"../../etc/passwd"}`)).IsError
		}},
		{"write traversal", func() bool {
			return (&FileWriteTool{workspace: ws}).Execute(ctx, json.RawMessage(`{"path":"../escape.txt","content":"x"}`)).IsError
		}},
		{"delete traversal", func() bool {
			return (&FileDeleteTool{workspace: ws}).Execute(ctx, json.RawMessage(`{"path":"../"}`)).IsError
		}},
		{"read malformed args", func() bool {
			return (&FileReadTool{workspace: ws}).Execute(ctx, json.RawMessage(`nope`)).IsError
		}},
		{"write malformed args", func() bool {
			return (&FileWriteTool{workspace: ws}).Execute(ctx, json.RawMessage(`nope`)).IsError
		}},
		{"delete malformed args", func() bool {
			return (&FileDeleteTool{workspace: ws}).Execute(ctx, json.RawMessage(`nope`)).IsError
		}},
		{"search malformed args", func() bool {
			return (&FileSearchTool{workspace: ws}).Execute(ctx, json.RawMessage(`nope`)).IsError
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.run() {
				t.Error("expected an error result")
			}
		})
	}
}

func TestFileSearchTool(t *testing.T) {
	ws := t.TempDir()
	for _, name := range []string{"one.go", "two.go", "notes.md"} {
		if err := os.WriteFile(filepath.Join(ws, name), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	tool := &FileSearchTool{workspace: ws}

	res := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if res.IsError {
		t.Fatalf("search: %s", res.Content[0].Text)
	}
	for _, want := range []string{"one.go", "two.go"} {
		if !strings.Contains(res.Content[0].Text, want) {
			t.Errorf("matches = %q, want %s", res.Content[0].Text, want)
		}
	}
	if strings.Contains(res.Content[0].Text, "notes.md") {
		t.Errorf("matches = %q, should not include notes.md", res.Content[0].Text)
	}

	if res := tool.Execute(context.Background(), json.RawMessage(`{"pattern":"*.rs"}`)); res.Content[0].Text != "no matches found" {
		t.Errorf("empty search = %q", res.Content[0].Text)
	}
}

func TestFileSearchToolIsBounded(t *testing.T) {
	ws := t.TempDir()
	for i := 0; i < maxSearchMatches+50; i++ {
		if err := os.WriteFile(filepath.Join(ws, "f"+strconv.Itoa(i)+".go"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	res := (&FileSearchTool{workspace: ws}).Execute(context.Background(), json.RawMessage(`{"pattern":"*.go"}`))
	if res.IsError {
		t.Fatalf("search: %s", res.Content[0].Text)
	}
	lines := strings.Split(res.Content[0].Text, "\n")
	if len(lines) != maxSearchMatches+1 {
		t.Fatalf("expected %d matches plus a truncation line, got %d lines", maxSearchMatches, len(lines))
	}
	if !strings.Contains(lines[len(lines)-1], "truncated") {
		t.Errorf("last line = %q, expected a truncation notice", lines[len(lines)-1])
	}
}

func TestFileSearchToolRejectsBadPatterns(t *testing.T) {
	tool := &FileSearchTool{workspace: t.TempDir()}
	for _, tc := range []struct{ name, args string }{
		{"empty pattern", `{"pattern":""}`},
		{"malformed pattern", `{"pattern":"[a-"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if res := tool.Execute(context.Background(), json.RawMessage(tc.args)); !res.IsError {
				t.Errorf("expected an error result, got %q", res.Content[0].Text)
			}
		})
	}
}
