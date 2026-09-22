package mcp

import (
	"encoding/json"
	"testing"
)

func TestSupportsStructuredOutput(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{VersionModern, true},
		{VersionLegacy20251125, true},
		{"2025-06-18", true},
		{"2025-03-26", false},
		{VersionLegacy20241105, false},
		{"", false},
	}
	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			if got := supportsStructuredOutput(tc.version); got != tc.want {
				t.Errorf("supportsStructuredOutput(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestSupportedVersionsIsACopy(t *testing.T) {
	got := SupportedVersions()
	if len(got) != 3 || got[0] != VersionModern {
		t.Fatalf("SupportedVersions() = %v", got)
	}
	got[0] = "tampered"
	if SupportedVersions()[0] != VersionModern {
		t.Fatal("SupportedVersions() handed out the package's own slice")
	}
}

func TestIsNotification(t *testing.T) {
	tests := []struct {
		name string
		id   string
		want bool
	}{
		{"absent", "", true},
		{"null", "null", true},
		{"number", "1", false},
		{"string", `"abc"`, false},
		{"zero", "0", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := JSONRPCRequest{Method: "x"}
			if tc.id != "" {
				req.ID = json.RawMessage(tc.id)
			}
			if got := req.IsNotification(); got != tc.want {
				t.Errorf("IsNotification() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestErrorResponseDataShape(t *testing.T) {
	resp := errorResponseData(json.RawMessage("1"), CodeUnsupportedProtocolVersion, "Unsupported protocol version",
		UnsupportedVersionData{Supported: SupportedVersions(), Requested: "1900-01-01"})
	raw, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	want := `{"jsonrpc":"2.0","id":1,"error":{"code":-32022,"message":"Unsupported protocol version","data":{"supported":["2026-07-28","2025-11-25","2024-11-05"],"requested":"1900-01-01"}}}`
	if string(raw) != want {
		t.Errorf("wire form:\n got %s\nwant %s", raw, want)
	}
}
