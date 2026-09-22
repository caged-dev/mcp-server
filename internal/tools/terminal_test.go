package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestTerminalExecAllowlist(t *testing.T) {
	tool := &TerminalExecTool{workspace: t.TempDir(), allowed: []string{"echo", "true"}}
	tests := []struct {
		name    string
		command string
		wantErr string
	}{
		{"allowed command runs", "echo hi", ""},
		{"disallowed command refused", "cat /etc/passwd", "not in allowlist"},
		{"chained with semicolon refused", "echo hi; cat /etc/passwd", "shell operator"},
		{"chained with && refused", "echo hi && cat /etc/passwd", "shell operator"},
		{"piped refused", "echo hi | cat", "shell operator"},
		{"substitution refused", "echo $(cat /etc/passwd)", "shell operator"},
		{"backtick refused", "echo `cat /etc/passwd`", "shell operator"},
		{"redirect refused", "echo hi > /tmp/x", "shell operator"},
		{"whitespace-only command does not panic", "   ", "command is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args, err := json.Marshal(map[string]string{"command": tc.command})
			if err != nil {
				t.Fatal(err)
			}
			res := tool.Execute(context.Background(), args)
			if tc.wantErr == "" {
				if res.IsError {
					t.Fatalf("expected success, got %q", res.Content[0].Text)
				}
				return
			}
			if !res.IsError {
				t.Fatalf("expected refusal, got %q", res.Content[0].Text)
			}
			if !strings.Contains(res.Content[0].Text, tc.wantErr) {
				t.Errorf("error = %q, want it to mention %q", res.Content[0].Text, tc.wantErr)
			}
		})
	}
}

// Without an allowlist the tool is unrestricted by design (the README says
// so), and shell operators keep working.
func TestTerminalExecWithoutAllowlist(t *testing.T) {
	tool := &TerminalExecTool{workspace: t.TempDir()}
	res := tool.Execute(context.Background(), json.RawMessage(`{"command":"echo one | tr a-z A-Z"}`))
	if res.IsError {
		t.Fatalf("unexpected error: %s", res.Content[0].Text)
	}
	if !strings.Contains(res.Content[0].Text, "ONE") {
		t.Errorf("output = %q", res.Content[0].Text)
	}
}

func TestTerminalExecErrors(t *testing.T) {
	tool := &TerminalExecTool{workspace: t.TempDir()}
	tests := []struct {
		name string
		args string
	}{
		{"malformed arguments", `not json`},
		{"empty command", `{"command":""}`},
		{"non-zero exit", `{"command":"exit 3"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := tool.Execute(context.Background(), json.RawMessage(tc.args))
			if !res.IsError {
				t.Fatalf("expected an error result, got %+v", res)
			}
		})
	}
}

func TestTerminalExecTimeout(t *testing.T) {
	tool := &TerminalExecTool{workspace: t.TempDir()}
	res := tool.Execute(context.Background(), json.RawMessage(`{"command":"sleep 5","timeout":1}`))
	if !res.IsError {
		t.Fatal("expected the timeout to end the command with an error result")
	}
}
