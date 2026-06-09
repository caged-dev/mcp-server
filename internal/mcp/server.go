package mcp
// Package mcp implements the Model Context Protocol server.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"

	"github.com/coder/websocket"
)

const protocolVersion = "2024-11-05"

// Tool is the interface that all MCP tools must implement.
type Tool interface {
	Definition() ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) ToolCallResult
}

// ServerConfig holds server metadata.
type ServerConfig struct {
	Name    string
	Version string
}

// Server handles MCP protocol communication.
type Server struct {
	config ServerConfig
	tools  map[string]Tool
	logger *slog.Logger
	mu     sync.RWMutex
}

// NewServer creates a new MCP server.
func NewServer(config ServerConfig, logger *slog.Logger, tools ...Tool) *Server {
	s := &Server{
		config: config,
		tools:  make(map[string]Tool),
		logger: logger,
	}
	for _, t := range tools {
		s.tools[t.Definition().Name] = t
	}
	return s
}

// ServeStdio runs the MCP server over stdin/stdout (JSON-RPC over stdio).
func (s *Server) ServeStdio(ctx context.Context) error {
	reader := bufio.NewReader(os.Stdin)
	writer := os.Stdout

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("reading stdin: %w", err)
		}

		response := s.handleMessage(ctx, line)
		if response != nil {
			data, _ := json.Marshal(response)
			data = append(data, '\n')
			writer.Write(data)
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
		defer conn.CloseNow()

		s.handleWebSocket(ctx, conn)
	})

	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		<-ctx.Done()
		server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) handleWebSocket(ctx context.Context, conn *websocket.Conn) {
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		response := s.handleMessage(ctx, data)
		if response != nil {
			respData, _ := json.Marshal(response)
			conn.Write(ctx, websocket.MessageText, respData)
		}
	}
}

func (s *Server) handleMessage(ctx context.Context, data []byte) *JSONRPCResponse {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return errorResponse(nil, -32700, "Parse error")
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(&req)
	case "tools/list":
		return s.handleToolsList(&req)
	case "tools/call":
		return s.handleToolsCall(ctx, &req)
	case "ping":
		return successResponse(req.ID, map[string]string{})
	default:
		return errorResponse(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *Server) handleInitialize(req *JSONRPCRequest) *JSONRPCResponse {
	result := map[string]interface{}{
		"protocolVersion": protocolVersion,
		"serverInfo": map[string]string{
			"name":    s.config.Name,
			"version": s.config.Version,
		},
		"capabilities": map[string]interface{}{
			"tools": map[string]bool{"listChanged": false},
		},
	}
	return successResponse(req.ID, result)
}

func (s *Server) handleToolsList(req *JSONRPCRequest) *JSONRPCResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var toolDefs []ToolDefinition
	for _, t := range s.tools {
		toolDefs = append(toolDefs, t.Definition())
	}

	result := map[string]interface{}{
		"tools": toolDefs,
	}
	return successResponse(req.ID, result)
}

func (s *Server) handleToolsCall(ctx context.Context, req *JSONRPCRequest) *JSONRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return errorResponse(req.ID, -32602, "Invalid params")
	}

	s.mu.RLock()
	tool, ok := s.tools[params.Name]
	s.mu.RUnlock()

	if !ok {
		return errorResponse(req.ID, -32602, fmt.Sprintf("Unknown tool: %s", params.Name))
	}

	result := tool.Execute(ctx, params.Arguments)
	return successResponse(req.ID, result)
}
