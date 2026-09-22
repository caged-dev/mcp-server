package mcp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// fakeTool is a minimal Tool with an output schema, so the structured-output
// gating can be exercised without touching the filesystem.
type fakeTool struct {
	name       string
	structured bool
}

func (f *fakeTool) Definition() ToolDefinition {
	def := ToolDefinition{
		Name:        f.name,
		Description: "fake tool",
		InputSchema: map[string]interface{}{"type": "object"},
	}
	if f.structured {
		def.OutputSchema = map[string]interface{}{"type": "object"}
	}
	return def
}

func (f *fakeTool) Execute(_ context.Context, args json.RawMessage) ToolCallResult {
	res := ToolCallResult{Content: []ContentBlock{{Type: "text", Text: "ran " + f.name + " with " + string(args)}}}
	if f.structured {
		res.StructuredContent = json.RawMessage(`{"ok":true}`)
	}
	return res
}

func newTestServer(tools ...Tool) *Server {
	if len(tools) == 0 {
		tools = []Tool{&fakeTool{name: "alpha", structured: true}, &fakeTool{name: "beta"}}
	}
	return NewServer(ServerConfig{Name: "caged-mcp", Version: "test", Instructions: "test instructions"}, testLogger(), tools...)
}

// handle is a helper that runs one message through a fresh connection.
func handle(t *testing.T, s *Server, state *ConnState, msg string) *JSONRPCResponse {
	t.Helper()
	if state == nil {
		state = NewConnState()
	}
	return s.HandleMessage(context.Background(), state, []byte(msg))
}

// resultFields re-marshals a result so tests assert on the wire bytes rather
// than on Go structs: the wire is what clients see.
func resultFields(t *testing.T, resp *JSONRPCResponse) map[string]json.RawMessage {
	t.Helper()
	if resp == nil {
		t.Fatal("expected a response, got nil")
	}
	if resp.Error != nil {
		t.Fatalf("expected a result, got error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	raw, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshalling result: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatalf("unmarshalling result: %v", err)
	}
	return fields
}

const modernMeta = `"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientInfo":{"name":"test","version":"1"},"io.modelcontextprotocol/clientCapabilities":{}}`

func TestHandleMessageParseError(t *testing.T) {
	s := newTestServer()
	resp := handle(t, s, nil, "{not json")
	if resp == nil || resp.Error == nil || resp.Error.Code != CodeParseError {
		t.Fatalf("expected parse error, got %+v", resp)
	}
}

func TestNotificationsAreNeverAnswered(t *testing.T) {
	s := newTestServer()
	tests := []struct {
		name string
		msg  string
	}{
		{"initialized", `{"jsonrpc":"2.0","method":"notifications/initialized"}`},
		{"cancelled", `{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":1}}`},
		{"unknown", `{"jsonrpc":"2.0","method":"notifications/nonsense"}`},
		{"null id", `{"jsonrpc":"2.0","id":null,"method":"notifications/initialized"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if resp := handle(t, s, nil, tc.msg); resp != nil {
				t.Fatalf("notification was answered: %+v", resp)
			}
		})
	}
}

func TestLegacyInitializeNegotiation(t *testing.T) {
	s := newTestServer()
	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{"no version keeps the historical answer", "", VersionLegacy20241105},
		{"exact 2024-11-05", VersionLegacy20241105, VersionLegacy20241105},
		{"exact 2025-11-25", VersionLegacy20251125, VersionLegacy20251125},
		{"2025-06-18 rounds down, never up", "2025-06-18", VersionLegacy20241105},
		{"2025-03-26 rounds down", "2025-03-26", VersionLegacy20241105},
		{"modern via initialize gets newest legacy", VersionModern, VersionLegacy20251125},
		{"unknown ancient version", "1900-01-01", VersionLegacy20241105},
		{"unparseable version", "banana", VersionLegacy20251125},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
			if tc.requested != "" {
				msg = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"` + tc.requested + `"}}`
			}
			fields := resultFields(t, handle(t, s, nil, msg))
			var got string
			if err := json.Unmarshal(fields["protocolVersion"], &got); err != nil {
				t.Fatalf("decoding protocolVersion: %v", err)
			}
			if got != tc.want {
				t.Errorf("negotiated %q, want %q", got, tc.want)
			}
			if _, ok := fields["resultType"]; ok {
				t.Error("legacy initialize result must not carry resultType")
			}
		})
	}
}

// TestLegacyResponsesAreUnchanged pins the exact bytes a 2024-11-05 client
// sees. Those clients are wired into editors today; this is the regression
// test for "do not break an existing configuration".
func TestLegacyResponsesAreUnchanged(t *testing.T) {
	s := newTestServer(&fakeTool{name: "alpha", structured: true})
	state := NewConnState()

	init := resultFields(t, handle(t, s, state, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`))
	if string(init["capabilities"]) != `{"tools":{"listChanged":false}}` {
		t.Errorf("capabilities = %s", init["capabilities"])
	}
	if string(init["serverInfo"]) != `{"name":"caged-mcp","version":"test"}` {
		t.Errorf("serverInfo = %s", init["serverInfo"])
	}

	list := resultFields(t, handle(t, s, state, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	for _, forbidden := range []string{"resultType", "ttlMs", "cacheScope", "_meta"} {
		if _, ok := list[forbidden]; ok {
			t.Errorf("legacy tools/list leaked %q", forbidden)
		}
	}
	if strings.Contains(string(list["tools"]), "outputSchema") {
		t.Error("legacy tools/list leaked outputSchema, which 2024-11-05 does not define")
	}

	call := resultFields(t, handle(t, s, state, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"alpha","arguments":{}}}`))
	if _, ok := call["structuredContent"]; ok {
		t.Error("legacy tools/call leaked structuredContent")
	}
	if _, ok := call["resultType"]; ok {
		t.Error("legacy tools/call leaked resultType")
	}
}

// TestLegacy20251125GetsStructuredOutput checks the middle era: a client
// that negotiated 2025-11-25 does get structured output, because that
// revision defines it.
func TestLegacy20251125GetsStructuredOutput(t *testing.T) {
	s := newTestServer(&fakeTool{name: "alpha", structured: true})
	state := NewConnState()
	handle(t, s, state, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)

	list := resultFields(t, handle(t, s, state, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	if !strings.Contains(string(list["tools"]), "outputSchema") {
		t.Error("2025-11-25 session should see outputSchema")
	}
	call := resultFields(t, handle(t, s, state, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"alpha","arguments":{}}}`))
	if string(call["structuredContent"]) != `{"ok":true}` {
		t.Errorf("structuredContent = %s", call["structuredContent"])
	}
	if _, ok := call["resultType"]; ok {
		t.Error("a legacy revision must not carry resultType")
	}
}

func TestModernToolsList(t *testing.T) {
	s := newTestServer()
	fields := resultFields(t, handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{`+modernMeta+`}}`))

	if string(fields["resultType"]) != `"complete"` {
		t.Errorf("resultType = %s", fields["resultType"])
	}
	if string(fields["ttlMs"]) != "3600000" {
		t.Errorf("ttlMs = %s", fields["ttlMs"])
	}
	if string(fields["cacheScope"]) != `"public"` {
		t.Errorf("cacheScope = %s", fields["cacheScope"])
	}
	if !strings.Contains(string(fields["_meta"]), `"io.modelcontextprotocol/serverInfo"`) {
		t.Errorf("_meta = %s", fields["_meta"])
	}
	if !strings.Contains(string(fields["tools"]), "outputSchema") {
		t.Error("modern tools/list should carry outputSchema")
	}
}

// TestToolsListIsDeterministic guards the prompt-cache stability the
// specification asks for. Ranging over a map did not give it.
func TestToolsListIsDeterministic(t *testing.T) {
	tools := make([]Tool, 0, 12)
	for _, n := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		tools = append(tools, &fakeTool{name: n})
	}
	s := newTestServer(tools...)

	var first string
	for i := 0; i < 20; i++ {
		fields := resultFields(t, handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		if i == 0 {
			first = string(fields["tools"])
			continue
		}
		if string(fields["tools"]) != first {
			t.Fatal("tools/list order changed between calls")
		}
	}
	if !strings.HasPrefix(first, `[{"name":"a"`) {
		t.Errorf("registration order not preserved: %s", first[:40])
	}
}

func TestModernToolsCall(t *testing.T) {
	s := newTestServer()
	fields := resultFields(t, handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"alpha","arguments":{"x":1},`+modernMeta+`}}`))

	if string(fields["resultType"]) != `"complete"` {
		t.Errorf("resultType = %s", fields["resultType"])
	}
	if string(fields["structuredContent"]) != `{"ok":true}` {
		t.Errorf("structuredContent = %s", fields["structuredContent"])
	}
	if !strings.Contains(string(fields["content"]), `ran alpha with {\"x\":1}`) {
		t.Errorf("content = %s", fields["content"])
	}
}

func TestToolsCallErrors(t *testing.T) {
	s := newTestServer()
	tests := []struct {
		name string
		msg  string
		code int
	}{
		{"unknown tool", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope"}}`, CodeInvalidParams},
		{"missing name", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"arguments":{}}}`, CodeInvalidParams},
		{"malformed params", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":"nope"}`, CodeInvalidParams},
		{"unknown method", `{"jsonrpc":"2.0","id":1,"method":"resources/list"}`, CodeMethodNotFound},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := handle(t, s, nil, tc.msg)
			if resp == nil || resp.Error == nil {
				t.Fatalf("expected an error, got %+v", resp)
			}
			if resp.Error.Code != tc.code {
				t.Errorf("code = %d, want %d", resp.Error.Code, tc.code)
			}
		})
	}
}

func TestUnsupportedProtocolVersion(t *testing.T) {
	s := newTestServer()
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"1900-01-01","io.modelcontextprotocol/clientCapabilities":{}}}}`
	resp := handle(t, s, nil, msg)
	if resp == nil || resp.Error == nil {
		t.Fatalf("expected an error, got %+v", resp)
	}
	if resp.Error.Code != CodeUnsupportedProtocolVersion {
		t.Fatalf("code = %d, want %d", resp.Error.Code, CodeUnsupportedProtocolVersion)
	}
	data, ok := resp.Error.Data.(UnsupportedVersionData)
	if !ok {
		t.Fatalf("data = %T, want UnsupportedVersionData", resp.Error.Data)
	}
	if data.Requested != "1900-01-01" {
		t.Errorf("requested = %q", data.Requested)
	}
	if len(data.Supported) != 3 || data.Supported[0] != VersionModern {
		t.Errorf("supported = %v", data.Supported)
	}
}

func TestModernRequestMissingClientCapabilities(t *testing.T) {
	s := newTestServer()
	msg := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28"}}}`
	resp := handle(t, s, nil, msg)
	if resp == nil || resp.Error == nil || resp.Error.Code != CodeInvalidParams {
		t.Fatalf("expected invalid params, got %+v", resp)
	}
	if !strings.Contains(resp.Error.Message, "clientCapabilities") {
		t.Errorf("error message should name the missing field: %q", resp.Error.Message)
	}
}

func TestMalformedMeta(t *testing.T) {
	s := newTestServer()
	resp := handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{"_meta":"not an object"}}`)
	if resp == nil || resp.Error == nil || resp.Error.Code != CodeInvalidParams {
		t.Fatalf("expected invalid params, got %+v", resp)
	}
}

func TestServerDiscover(t *testing.T) {
	s := newTestServer()
	tests := []struct {
		name string
		msg  string
	}{
		{"with modern meta", `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{` + modernMeta + `}}`},
		{"bare probe with no _meta", `{"jsonrpc":"2.0","id":1,"method":"server/discover"}`},
		{"empty params", `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fields := resultFields(t, handle(t, s, nil, tc.msg))
			if string(fields["resultType"]) != `"complete"` {
				t.Errorf("resultType = %s", fields["resultType"])
			}
			if string(fields["supportedVersions"]) != `["2026-07-28","2025-11-25","2024-11-05"]` {
				t.Errorf("supportedVersions = %s", fields["supportedVersions"])
			}
			if string(fields["cacheScope"]) != `"public"` || string(fields["ttlMs"]) != "3600000" {
				t.Errorf("cache hints = %s / %s", fields["ttlMs"], fields["cacheScope"])
			}
			if string(fields["instructions"]) != `"test instructions"` {
				t.Errorf("instructions = %s", fields["instructions"])
			}
		})
	}
}

// TestDiscoverWithUnsupportedVersionIsAModernError is the deterministic
// failure the stdio probe relies on: a modern error identifies a modern
// server, and the client must retry rather than fall back to initialize.
func TestDiscoverWithUnsupportedVersionIsAModernError(t *testing.T) {
	s := newTestServer()
	msg := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2027-01-01","io.modelcontextprotocol/clientCapabilities":{}}}}`
	resp := handle(t, s, nil, msg)
	if resp == nil || resp.Error == nil || resp.Error.Code != CodeUnsupportedProtocolVersion {
		t.Fatalf("expected -32022, got %+v", resp)
	}
}

func TestPingBothEras(t *testing.T) {
	s := newTestServer()

	legacy := resultFields(t, handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"ping"}`))
	if len(legacy) != 0 {
		t.Errorf("legacy ping result should be empty, got %v", legacy)
	}

	modern := resultFields(t, handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"ping","params":{`+modernMeta+`}}`))
	if string(modern["resultType"]) != `"complete"` {
		t.Errorf("modern ping resultType = %s", modern["resultType"])
	}
}

// TestEraIsPerConnection: a legacy handshake on one connection must not
// change what another connection is served. The WebSocket mode multiplexes
// connections over one Server.
func TestEraIsPerConnection(t *testing.T) {
	s := newTestServer(&fakeTool{name: "alpha", structured: true})

	a := NewConnState()
	b := NewConnState()
	handle(t, s, a, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`)

	aList := resultFields(t, handle(t, s, a, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))
	bList := resultFields(t, handle(t, s, b, `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`))

	if !strings.Contains(string(aList["tools"]), "outputSchema") {
		t.Error("connection A negotiated 2025-11-25 and should see outputSchema")
	}
	if strings.Contains(string(bList["tools"]), "outputSchema") {
		t.Error("connection B never handshook and must still be served 2024-11-05")
	}
}

func TestConcurrentRequests(t *testing.T) {
	s := newTestServer()
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			state := NewConnState()
			msgs := []string{
				`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25"}}`,
				`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
				`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"alpha","arguments":{}}}`,
				`{"jsonrpc":"2.0","id":4,"method":"server/discover"}`,
				`{"jsonrpc":"2.0","id":5,"method":"tools/list","params":{` + modernMeta + `}}`,
			}
			for _, m := range msgs {
				if resp := s.HandleMessage(context.Background(), state, []byte(m)); resp == nil {
					t.Errorf("nil response for %s", m)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestNewServerIgnoresDuplicateToolNames(t *testing.T) {
	s := newTestServer(&fakeTool{name: "alpha"}, &fakeTool{name: "alpha"})
	fields := resultFields(t, handle(t, s, nil, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	var tools []ToolDefinition
	if err := json.Unmarshal(fields["tools"], &tools); err != nil {
		t.Fatalf("decoding tools: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
}
