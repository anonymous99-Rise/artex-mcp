package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"artex-mcp/internal/artex"
	"artex-mcp/internal/tools"
)

func run(t *testing.T, s *Server, lines ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")+"\n"), &out); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("Serve: %v", err)
	}
	var resp []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad response line %q: %v", line, err)
		}
		resp = append(resp, m)
	}
	return resp
}

func TestHandshakeAndNotification(t *testing.T) {
	s := &Server{Version: "test"}
	resp := run(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","clientInfo":{"name":"x","version":"1"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"ping"}`,
	)
	if len(resp) != 3 {
		t.Fatalf("got %d responses, want 3 (notification must not be answered)", len(resp))
	}
	init := resp[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-06-18" {
		t.Fatalf("protocol = %v", init["protocolVersion"])
	}
	info := init["serverInfo"].(map[string]any)
	if info["name"] != "artex-mcp" || info["version"] != "test" {
		t.Fatalf("serverInfo = %v", info)
	}
	list := resp[1]["result"].(map[string]any)
	defs := list["tools"].([]any)
	if len(defs) != len(tools.All()) {
		t.Fatalf("tools/list = %d, registry = %d", len(defs), len(tools.All()))
	}
}

func TestProtocolFallback(t *testing.T) {
	s := &Server{Version: "test"}
	resp := run(t, s, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"bogus"}}`)
	init := resp[0]["result"].(map[string]any)
	if init["protocolVersion"] != "2025-06-18" {
		t.Fatalf("fallback protocol = %v", init["protocolVersion"])
	}
}

func TestUnknownMethodAndTool(t *testing.T) {
	s := &Server{Version: "test"}
	resp := run(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"does/not/exist"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nope","arguments":{}}}`,
	)
	if resp[0]["error"] == nil {
		t.Fatal("unknown method should error")
	}
	errObj := resp[1]["error"].(map[string]any)
	if errObj["code"].(float64) != -32602 {
		t.Fatalf("code = %v", errObj["code"])
	}
}

func TestToolErrorIsReportedAsContent(t *testing.T) {
	// Port 1 is closed: the tool fails at the transport layer.
	s := &Server{API: artex.New("http://127.0.0.1:1", "ARTEX", "", "tok"), Version: "test"}
	resp := run(t, s, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"artex_list_tasks","arguments":{}}}`)
	if resp[0]["error"] != nil {
		t.Fatalf("transport failures should be tool content, got RPC error: %v", resp[0]["error"])
	}
	result := resp[0]["result"].(map[string]any)
	if result["isError"] != true {
		t.Fatalf("isError = %v", result["isError"])
	}
}
