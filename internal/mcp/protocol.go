package mcp

import "encoding/json"

// Protocol revisions this server speaks.
//
// The Model Context Protocol splits into two eras (see the 2026-07-28
// specification, basic/versioning):
//
//   - "modern" revisions carry the protocol version, client identity and
//     client capabilities in a per-request `_meta` object and have no
//     handshake at all;
//   - "legacy" revisions (2025-11-25 and earlier) open with an `initialize`
//     handshake that negotiates a single version for the connection.
//
// This server is dual-era: it serves both, selected by how the client opens.
const (
	// VersionModern is the current stable revision (modern, stateless).
	VersionModern = "2026-07-28"
	// VersionLegacy20251125 is the newest handshake-based revision.
	VersionLegacy20251125 = "2025-11-25"
	// VersionLegacy20241105 is the original revision, and the version this
	// server pinned before it became dual-era. It remains the default for a
	// legacy client that sends no version at all.
	VersionLegacy20241105 = "2024-11-05"

	// versionStructuredOutput is the first revision that defined
	// `outputSchema` and `structuredContent`. Sessions negotiated below this
	// revision never see either field.
	versionStructuredOutput = "2025-06-18"
)

// supportedVersions lists every revision this server can serve, newest first.
// It is the `supported` list of an UnsupportedProtocolVersionError and the
// `supportedVersions` of a DiscoverResult.
var supportedVersions = []string{VersionModern, VersionLegacy20251125, VersionLegacy20241105}

// legacyVersions lists the handshake-based revisions this server can
// negotiate through `initialize`, newest first.
var legacyVersions = []string{VersionLegacy20251125, VersionLegacy20241105}

// SupportedVersions returns the protocol revisions this server serves.
func SupportedVersions() []string {
	out := make([]string, len(supportedVersions))
	copy(out, supportedVersions)
	return out
}

// Reserved `_meta` keys from the 2026-07-28 specification.
const (
	metaProtocolVersion    = "io.modelcontextprotocol/protocolVersion"
	metaClientCapabilities = "io.modelcontextprotocol/clientCapabilities"
)

// JSON-RPC and MCP error codes. -32020..-32099 is reserved for the
// specification; -32022 is the only code in that range this server emits.
const (
	CodeParseError                 = -32700
	CodeInvalidRequest             = -32600
	CodeMethodNotFound             = -32601
	CodeInvalidParams              = -32602
	CodeUnsupportedProtocolVersion = -32022
)

// ResultTypeComplete marks a result as final. This server never returns
// `input_required`: no tool here asks the client a question.
const ResultTypeComplete = "complete"

// Cache scopes for cacheable results.
const (
	CacheScopePublic  = "public"
	CacheScopePrivate = "private"
)

// listTTLMs is the freshness hint on `tools/list` and `server/discover`.
// The tool set is fixed when the process starts, so an hour is honest.
const listTTLMs = 3600000

// JSONRPCRequest represents a JSON-RPC 2.0 request or notification. A
// notification is a request with no ID and MUST NOT be answered.
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// IsNotification reports whether the message is a JSON-RPC notification.
func (r *JSONRPCRequest) IsNotification() bool {
	return len(r.ID) == 0 || string(r.ID) == "null"
}

// JSONRPCResponse represents a JSON-RPC 2.0 response.
type JSONRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC 2.0 error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// UnsupportedVersionData is the `data` member of an
// UnsupportedProtocolVersionError (-32022).
type UnsupportedVersionData struct {
	Supported []string `json:"supported"`
	Requested string   `json:"requested"`
}

// Implementation identifies a client or server by name and version.
type Implementation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ResultMeta carries the per-response protocol fields a modern server
// SHOULD include in every result.
type ResultMeta struct {
	ServerInfo *Implementation `json:"io.modelcontextprotocol/serverInfo,omitempty"`
}

// ToolsCapability describes the server's tools capability.
type ToolsCapability struct {
	ListChanged bool `json:"listChanged"`
}

// ServerCapabilities describes what the server supports.
type ServerCapabilities struct {
	Tools *ToolsCapability `json:"tools,omitempty"`
}

// ToolDefinition describes an MCP tool.
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
	// OutputSchema describes the shape of StructuredContent, when the tool
	// produces any. It is omitted for sessions negotiated below 2025-06-18,
	// the revision that introduced it.
	OutputSchema interface{} `json:"outputSchema,omitempty"`
}

// ToolCallResult is the result of executing a tool.
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	// StructuredContent is the machine-readable form of the same result. It
	// is omitted for sessions negotiated below 2025-06-18.
	StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
	IsError           bool            `json:"isError,omitempty"`

	ResultType string      `json:"resultType,omitempty"`
	Meta       *ResultMeta `json:"_meta,omitempty"`
}

// ContentBlock represents a content block in tool results.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// DiscoverResult answers `server/discover`. Servers MUST implement that
// method as of 2026-07-28; it is also the era probe a dual-era client sends
// before anything else on stdio.
type DiscoverResult struct {
	ResultType        string             `json:"resultType"`
	SupportedVersions []string           `json:"supportedVersions"`
	Capabilities      ServerCapabilities `json:"capabilities"`
	Instructions      string             `json:"instructions,omitempty"`
	TTLMs             int64              `json:"ttlMs"`
	CacheScope        string             `json:"cacheScope"`
	Meta              *ResultMeta        `json:"_meta,omitempty"`
}

// InitializeResult answers the legacy `initialize` handshake. Its shape is
// fixed by the negotiated legacy revision and carries none of the modern
// fields.
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      Implementation     `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

// ToolsListResult answers `tools/list`. The caching hints and `resultType`
// are only populated for modern requests.
type ToolsListResult struct {
	Tools      []ToolDefinition `json:"tools"`
	ResultType string           `json:"resultType,omitempty"`
	TTLMs      *int64           `json:"ttlMs,omitempty"`
	CacheScope string           `json:"cacheScope,omitempty"`
	Meta       *ResultMeta      `json:"_meta,omitempty"`
}

// EmptyResult answers methods with no payload (`ping`).
type EmptyResult struct {
	ResultType string      `json:"resultType,omitempty"`
	Meta       *ResultMeta `json:"_meta,omitempty"`
}

// requestParams is the envelope every request shares: the `_meta` object
// that carries the per-request protocol fields in the modern era.
type requestParams struct {
	Meta *requestMeta `json:"_meta"`
}

// requestMeta holds the reserved `io.modelcontextprotocol/*` fields.
// ClientCapabilities is kept raw because only its presence is required —
// this server relies on no client capability and so never returns
// MissingRequiredClientCapabilityError.
type requestMeta struct {
	ProtocolVersion    string          `json:"io.modelcontextprotocol/protocolVersion"`
	ClientInfo         *Implementation `json:"io.modelcontextprotocol/clientInfo"`
	ClientCapabilities json.RawMessage `json:"io.modelcontextprotocol/clientCapabilities"`
}

func successResponse(id json.RawMessage, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

func errorResponse(id json.RawMessage, code int, message string) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message},
	}
}

func errorResponseData(id json.RawMessage, code int, message string, data interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &JSONRPCError{Code: code, Message: message, Data: data},
	}
}

// supportsStructuredOutput reports whether the negotiated revision defines
// `outputSchema` and `structuredContent`. Revisions are ISO dates, so a
// lexical comparison is a chronological one.
func supportsStructuredOutput(version string) bool {
	return version >= versionStructuredOutput
}

// negotiateLegacyVersion picks the revision to serve a legacy `initialize`.
// The client gets its own requested version when this server supports it,
// and otherwise the newest supported revision no newer than the request —
// never something newer than the client asked for, which it may not
// understand. A client that names nothing gets 2024-11-05, the version this
// server answered before it became dual-era.
func negotiateLegacyVersion(requested string) string {
	if requested == "" {
		return VersionLegacy20241105
	}
	for _, v := range legacyVersions {
		if v <= requested {
			return v
		}
	}
	return VersionLegacy20241105
}
