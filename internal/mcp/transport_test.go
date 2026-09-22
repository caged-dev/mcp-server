package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestServeStreamFraming drives the stdio wire format end to end: several
// messages in, newline-delimited responses out, and no line at all for the
// notification in the middle.
func TestServeStreamFraming(t *testing.T) {
	s := newTestServer()
	in := strings.NewReader(strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"server/discover"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{` + modernMeta + `}}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	}, "\n") + "\n")

	var out strings.Builder
	if err := s.ServeStream(context.Background(), in, &out); err != nil {
		t.Fatalf("ServeStream: %v", err)
	}

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 response lines (the notification must not be answered), got %d:\n%s", len(lines), out.String())
	}
	for i, line := range lines {
		var resp JSONRPCResponse
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			t.Fatalf("line %d is not valid JSON: %v", i, err)
		}
		if resp.Error != nil {
			t.Errorf("line %d carried an error: %+v", i, resp.Error)
		}
	}
}

func TestServeStreamStopsOnContextCancel(t *testing.T) {
	s := newTestServer()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() {
		// A reader that never returns would block forever if cancellation
		// were not checked before the read.
		done <- s.ServeStream(ctx, strings.NewReader(""), io.Discard)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("ServeStream: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeStream did not return after context cancellation")
	}
}

// TestWebSocketBothEras drives a real WebSocket connection, which is the
// transport the Caged platform endpoint and the --mode ws flag use.
func TestWebSocketBothEras(t *testing.T) {
	s := newTestServer(&fakeTool{name: "alpha", structured: true})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	srv := httptest.NewServer(s.WebSocketHandler(ctx))
	defer srv.Close()

	call := func(t *testing.T, conn *websocket.Conn, msg string) map[string]json.RawMessage {
		t.Helper()
		if err := conn.Write(ctx, websocket.MessageText, []byte(msg)); err != nil {
			t.Fatalf("write: %v", err)
		}
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		var resp JSONRPCResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			t.Fatalf("decoding response: %v", err)
		}
		return resultFields(t, &resp)
	}

	dial := func(t *testing.T) *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http"), nil)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		return conn
	}

	t.Run("modern client never handshakes", func(t *testing.T) {
		conn := dial(t)
		defer conn.CloseNow() //nolint:errcheck

		discover := call(t, conn, `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{`+modernMeta+`}}`)
		if string(discover["supportedVersions"]) != `["2026-07-28","2025-11-25","2024-11-05"]` {
			t.Errorf("supportedVersions = %s", discover["supportedVersions"])
		}
		list := call(t, conn, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{`+modernMeta+`}}`)
		if string(list["resultType"]) != `"complete"` || string(list["cacheScope"]) != `"public"` {
			t.Errorf("modern tools/list envelope = %v", list)
		}
	})

	t.Run("legacy client handshakes", func(t *testing.T) {
		conn := dial(t)
		defer conn.CloseNow() //nolint:errcheck

		init := call(t, conn, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`)
		if string(init["protocolVersion"]) != `"2024-11-05"` {
			t.Errorf("protocolVersion = %s", init["protocolVersion"])
		}
		res := call(t, conn, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"alpha","arguments":{}}}`)
		if _, ok := res["structuredContent"]; ok {
			t.Error("a 2024-11-05 session must not receive structuredContent")
		}
	})
}
