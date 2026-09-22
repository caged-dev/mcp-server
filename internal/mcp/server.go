// Package mcp implements the Model Context Protocol server.
//
// The server is dual-era: it serves the modern, stateless revision
// (2026-07-28) to clients that put their protocol version in a per-request
// `_meta` object, and the handshake-based revisions (2025-11-25, 2024-11-05)
// to clients that open with `initialize`. Which one a client gets is decided
// by how it opens, per the 2026-07-28 compatibility matrix, and never by
// configuration.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Tool is the interface that all MCP tools must implement.
type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) ToolCallResult
}

// ServerConfig holds server metadata.
type ServerConfig struct {
	Name    string
	Version string
	// Instructions is optional natural-language guidance returned by
	// `server/discover`.
	Instructions string
}

// Server handles MCP protocol communication.
type Server struct {
	config ServerConfig
	tools  map[string]Tool
	// order preserves tool registration order. The specification asks
	// servers to return tools deterministically so clients can cache the
	// list and so prompt caches stay stable; ranging over the map did not.
	order  []string
	logger *slog.Logger
	mu     sync.RWMutex
}

// ConnState holds the only state a connection can carry: the protocol
// revision a legacy client negotiated with `initialize`. Modern requests
// never read it — they are self-describing.
type ConnState struct {
	mu            sync.Mutex
	legacyVersion string
}

func (c *ConnState) version() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.legacyVersion == "" {
		return VersionLegacy20241105
	}
	return c.legacyVersion
}

func (c *ConnState) setVersion(v string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.legacyVersion = v
}

// NewServer creates a new MCP server.
func NewServer(config ServerConfig, logger *slog.Logger, tools ...Tool) *Server {
	s := &Server{
		config: config,
		tools:  make(map[string]Tool),
		logger: logger,
	}
	for _, t := range tools {
		name := t.Definition().Name
		if _, dup := s.tools[name]; dup {
			continue
		}
		s.tools[name] = t
		s.order = append(s.order, name)
	}
	return s
}

// ServeStdio runs the MCP server over stdin/stdout (JSON-RPC over stdio).
func (s *Server) ServeStdio(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout
	state := &ConnState{}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("reading stdin: %w", err)
		}

		response := s.HandleMessage(ctx, state, line)
		if response != nil {
			data, err := json.Marshal(response)
			if err != nil {
				return fmt.Errorf("encoding response: %w", err)
			}
			data = append(data, '\n')
			if _, err := writer.Write(data); err != nil {
				return fmt.Errorf("writing stdout: %w", err)
			}
		}
	}
}

// ServeWebSocket runs the MCP server on a WebSocket endpoint.
func (s *Server) ServeWebSocket(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"},
		})
		if err != nil {
			s.logger.Error("websocket accept error", "error", err)
			return
		}
		defer conn.CloseNow() //nolint:errcheck

		s.handleWebSocket(ctx, conn)
	})

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		if err := server.Close(); err != nil {
			s.logger.Error("closing websocket server", "error", err)
		}
	}()

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serving websocket: %w", err)
	}
	return nil
}

func (s *Server) handleWebSocket(ctx context.Context, conn *websocket.Conn) {
	// Each connection negotiates its own era.
	state := &ConnState{}

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		response := s.HandleMessage(ctx, state, data)
		if response != nil {
			respData, err := json.Marshal(response)
			if err != nil {
				s.logger.Error("encoding response", "error", err)
				continue
			}
			if err := conn.Write(ctx, websocket.MessageText, respData); err != nil {
				return
			}
		}
	}
}

// NewConnState returns the per-connection state HandleMessage needs. Every
// independent client connection must have its own.
func NewConnState() *ConnState {
	return &ConnState{}
}

// HandleMessage dispatches one JSON-RPC message and returns the response, or
// nil when the message is a notification and must not be answered.
func (s *Server) HandleMessage(ctx context.Context, state *ConnState, data []byte) *JSONRPCResponse {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return errorResponse(nil, CodeParseError, "Parse error")
	}

	// A notification carries no ID and MUST NOT be answered — not even with
	// an error. Answering `notifications/initialized` with "method not
	// found", as this server used to, is a protocol violation and confuses
	// clients that correlate on IDs.
	if req.IsNotification() {
		s.logger.Debug("notification received", "method", req.Method)
		return nil
	}

	// `initialize` selects legacy semantics whatever else the message
	// carries: it is the legacy era's opening move and does not exist in the
	// modern revision.
	if req.Method == "initialize" {
		return s.handleInitialize(state, &req)
	}

	version, errResp := s.resolveVersion(state, &req)
	if errResp != nil {
		return errResp
	}
	modern := version == VersionModern

	switch req.Method {
	case "server/discover":
		return s.handleDiscover(&req)
	case "tools/list":
		return s.handleToolsList(&req, version, modern)
	case "tools/call":
		return s.handleToolsCall(ctx, &req, version, modern)
	case "ping":
		// `ping` was removed in 2026-07-28. It is still answered in both
		// eras: clients use it as a keepalive regardless of revision, and
		// failing one would drop a working connection to no benefit.
		return successResponse(req.ID, &EmptyResult{
			ResultType: s.resultType(modern),
			Meta:       s.resultMeta(modern),
		})
	default:
		return errorResponse(req.ID, CodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

// resolveVersion determines which protocol revision a request is speaking.
//
// A request whose `_meta` names a protocol version is self-describing and is
// served statelessly at that revision. A request without one belongs to the
// legacy era and is served at the revision `initialize` negotiated for this
// connection (2024-11-05 if it never did).
func (s *Server) resolveVersion(state *ConnState, req *JSONRPCRequest) (string, *JSONRPCResponse) {
	meta, err := parseRequestMeta(req.Params)
	if err != nil {
		return "", errorResponse(req.ID, CodeInvalidParams, "Invalid params: malformed _meta")
	}

	if meta == nil || meta.ProtocolVersion == "" {
		return state.version(), nil
	}

	if !isSupportedVersion(meta.ProtocolVersion) {
		return "", errorResponseData(req.ID, CodeUnsupportedProtocolVersion,
			"Unsupported protocol version",
			UnsupportedVersionData{Supported: SupportedVersions(), Requested: meta.ProtocolVersion})
	}

	// `io.modelcontextprotocol/clientCapabilities` is required on every
	// modern request; a request missing it is malformed. `server/discover`
	// is exempt only when it carries no `_meta` at all, because that shape
	// is an era probe and answering it is more useful than rejecting it.
	if len(meta.ClientCapabilities) == 0 {
		return "", errorResponse(req.ID, CodeInvalidParams,
			"Invalid params: _meta."+metaClientCapabilities+" is required")
	}

	return meta.ProtocolVersion, nil
}

func parseRequestMeta(params json.RawMessage) (*requestMeta, error) {
	if len(params) == 0 || string(params) == "null" {
		return nil, nil
	}
	var p requestParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parsing request params: %w", err)
	}
	return p.Meta, nil
}

func isSupportedVersion(v string) bool {
	for _, s := range supportedVersions {
		if s == v {
			return true
		}
	}
	return false
}

func (s *Server) resultType(modern bool) string {
	if !modern {
		return ""
	}
	return ResultTypeComplete
}

func (s *Server) resultMeta(modern bool) *ResultMeta {
	if !modern {
		return nil
	}
	return &ResultMeta{ServerInfo: &Implementation{Name: s.config.Name, Version: s.config.Version}}
}

func (s *Server) capabilities() ServerCapabilities {
	return ServerCapabilities{Tools: &ToolsCapability{ListChanged: false}}
}

// handleInitialize serves the legacy handshake and records the revision the
// rest of this connection will be served at.
func (s *Server) handleInitialize(state *ConnState, req *JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if len(req.Params) > 0 {
		// A malformed `initialize` params object is not fatal: the version
		// is the only field this server reads and it has a safe default.
		_ = json.Unmarshal(req.Params, &params)
	}

	version := negotiateLegacyVersion(params.ProtocolVersion)
	state.setVersion(version)
	s.logger.Debug("legacy initialize", "requested", params.ProtocolVersion, "negotiated", version)

	return successResponse(req.ID, &InitializeResult{
		ProtocolVersion: version,
		ServerInfo:      Implementation{Name: s.config.Name, Version: s.config.Version},
		Capabilities:    s.capabilities(),
	})
}

// handleDiscover answers `server/discover`, which servers MUST implement as
// of 2026-07-28 and which dual-era clients use as their era probe.
func (s *Server) handleDiscover(req *JSONRPCRequest) *JSONRPCResponse {
	return successResponse(req.ID, &DiscoverResult{
		ResultType:        ResultTypeComplete,
		SupportedVersions: SupportedVersions(),
		Capabilities:      s.capabilities(),
		Instructions:      s.config.Instructions,
		TTLMs:             listTTLMs,
		CacheScope:        CacheScopePublic,
		Meta:              s.resultMeta(true),
	})
}

func (s *Server) handleToolsList(req *JSONRPCRequest, version string, modern bool) *JSONRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	structured := supportsStructuredOutput(version)
	toolDefs := make([]ToolDefinition, 0, len(s.order))
	for _, name := range s.order {
		def := s.tools[name].Definition()
		if !structured {
			// `outputSchema` did not exist before 2025-06-18; do not send a
			// field the negotiated revision cannot describe.
			def.OutputSchema = nil
		}
		toolDefs = append(toolDefs, def)
	}

	result := &ToolsListResult{
		Tools:      toolDefs,
		ResultType: s.resultType(modern),
		Meta:       s.resultMeta(modern),
	}
	if modern {
		// `tools/list` results are cacheable as of 2026-07-28, and the hints
		// are required on them. The set is fixed for the process lifetime
		// and identical for every caller, so: an hour, public.
		ttl := int64(listTTLMs)
		result.TTLMs = &ttl
		result.CacheScope = CacheScopePublic
	}
	return successResponse(req.ID, result)
}

func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest, version string, modern bool) *JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, CodeInvalidParams, "Invalid params")
	}
	if params.Name == "" {
		return errorResponse(req.ID, CodeInvalidParams, "Invalid params: name is required")
	}

	s.mu.RLock()
	tool, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return errorResponse(req.ID, CodeInvalidParams, fmt.Sprintf("Unknown tool: %s", params.Name))
	}

	result := tool.Execute(ctx, params.Arguments)
	if !supportsStructuredOutput(version) {
		result.StructuredContent = nil
	}
	result.ResultType = s.resultType(modern)
	result.Meta = s.resultMeta(modern)
	return successResponse(req.ID, result)
}
