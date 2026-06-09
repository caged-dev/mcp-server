package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/caged-dev/mcp-server/internal/mcp"
	"github.com/caged-dev/mcp-server/internal/tools"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	mode := flag.String("mode", envOrDefault("CAGED_MCP_MODE", "stdio"), "transport mode: stdio or ws")
	port := flag.Int("port", envIntOrDefault("CAGED_MCP_PORT", 9090), "WebSocket listen port")
	workspace := flag.String("workspace", envOrDefault("CAGED_MCP_WORKSPACE", "."), "workspace root directory")
	allowedCmds := flag.String("allowed-commands", envOrDefault("CAGED_MCP_ALLOWED_COMMANDS", ""), "comma-separated allowed commands (empty = all)")
	readOnly := flag.Bool("read-only", envBoolOrDefault("CAGED_MCP_READ_ONLY", false), "disable write/exec tools")
	logLevel := flag.String("log-level", envOrDefault("CAGED_MCP_LOG_LEVEL", "info"), "log level")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("caged-mcp %s (%s)\n", version, commit)
		os.Exit(0)
	}

	level := slog.LevelInfo
	switch *logLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	// Resolve workspace to absolute path.
	absWorkspace, err := resolveWorkspace(*workspace)
	if err != nil {
		logger.Error("invalid workspace", "path", *workspace, "error", err)
		os.Exit(1)
	}

	// Build tool set.
	var allowed []string
	if *allowedCmds != "" {
		allowed = strings.Split(*allowedCmds, ",")
	}

	toolSet := tools.NewToolSet(absWorkspace, tools.Options{
		ReadOnly:        *readOnly,
		AllowedCommands: allowed,
	})

	// Create server.
	server := mcp.NewServer(mcp.ServerConfig{
		Name:    "caged-mcp",
		Version: version,
	}, logger, toolSet.All()...)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	switch *mode {
	case "stdio":
		logger.Info("starting MCP server", "mode", "stdio", "workspace", absWorkspace)
		if err := server.ServeStdio(ctx); err != nil && ctx.Err() == nil {
			logger.Error("stdio server error", "error", err)
			os.Exit(1)
		}
	case "ws":
		addr := fmt.Sprintf(":%d", *port)
		logger.Info("starting MCP server", "mode", "ws", "addr", addr, "workspace", absWorkspace)
		if err := server.ServeWebSocket(ctx, addr); err != nil && ctx.Err() == nil {
			logger.Error("websocket server error", "error", err)
			os.Exit(1)
		}
	default:
		logger.Error("unknown mode", "mode", *mode)
		os.Exit(1)
	}
}

func resolveWorkspace(path string) (string, error) {
	if path == "" {
		path = "."
	}
	abs, err := os.Getwd()
	if err != nil {
		return "", err
	}
	if path == "." {
		return abs, nil
	}
	if path[0] == '/' {
		return path, nil
	}
	return abs + "/" + path, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

func envBoolOrDefault(key string, def bool) bool {
	v := os.Getenv(key)
	return v == "true" || v == "1" || (v == "" && def)
}
