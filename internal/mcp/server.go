// Package mcp implements the stdio JSON-RPC 2.0 transport and the MCP methods
// artex-mcp serves: initialize, tools/list, tools/call, ping and the empty
// resource/prompt lists.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"

	"artex-mcp/internal/artex"
	"artex-mcp/internal/tools"
)

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// Server serves one MCP connection over a byte stream.
type Server struct {
	API     *artex.Client
	Version string
}

// Serve loops over newline-delimited JSON until EOF.
func (s *Server) Serve(r io.Reader, w io.Writer) error {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	enc := json.NewEncoder(w)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			return err
		}
		if resp := s.handle(&req); resp != nil {
			if err := enc.Encode(resp); err != nil {
				return err
			}
		}
	}
}

func (s *Server) handle(req *request) *response {
	result, rerr := s.dispatch(req.Method, req.Params)
	if len(req.ID) == 0 {
		return nil // notification
	}
	if rerr != nil {
		return &response{JSONRPC: "2.0", ID: req.ID, Error: rerr}
	}
	return &response{JSONRPC: "2.0", ID: req.ID, Result: result}
}

func (s *Server) dispatch(method string, params json.RawMessage) (any, *rpcError) {
	switch method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
			ClientInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"clientInfo"`
		}
		_ = json.Unmarshal(params, &p)
		proto := p.ProtocolVersion
		if !strings.HasPrefix(proto, "2024-") && !strings.HasPrefix(proto, "2025-") {
			proto = "2025-06-18"
		}
		log.Printf("initialize from %s %s (protocol %s)", p.ClientInfo.Name, p.ClientInfo.Version, proto)
		return map[string]any{
			"protocolVersion": proto,
			"capabilities": map[string]any{
				"tools": map[string]any{"listChanged": false},
			},
			"serverInfo":   map[string]any{"name": "artex-mcp", "version": s.Version},
			"instructions": instructions,
		}, nil

	case "ping":
		return map[string]any{}, nil

	case "tools/list":
		return map[string]any{"tools": tools.Defs()}, nil

	case "tools/call":
		var p struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "invalid tools/call params: " + err.Error()}
		}
		text, err := tools.Dispatch(context.Background(), s.API, p.Name, p.Arguments)
		if errors.Is(err, tools.ErrUnknownTool) {
			return nil, &rpcError{Code: -32602, Message: "unknown tool: " + p.Name}
		}
		if err != nil {
			return toolContent("错误: "+err.Error(), true), nil
		}
		return toolContent(text, false), nil

	case "resources/list":
		return map[string]any{"resources": []any{}}, nil
	case "resources/templates/list":
		return map[string]any{"resourceTemplates": []any{}}, nil
	case "prompts/list":
		return map[string]any{"prompts": []any{}}, nil
	case "logging/setLevel":
		return map[string]any{}, nil

	case "notifications/initialized", "notifications/cancelled", "notifications/progress":
		return nil, nil
	}
	return nil, &rpcError{Code: -32601, Message: "method not found: " + method}
}

func toolContent(text string, isErr bool) map[string]any {
	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": text}},
		"isError": isErr,
	}
}

// Check verifies connectivity and authentication, for `artex-mcp -check`.
func Check(ctx context.Context, c *artex.Client) error {
	var health map[string]any
	if err := c.DoRaw(ctx, "GET", "/api/health", nil, &health); err != nil {
		return err
	}
	fmt.Printf("health: %v\n", health)
	var tasks struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := c.Do(ctx, "GET", "/api/tasks", nil, &tasks); err != nil {
		return err
	}
	fmt.Printf("login: ok (%d tasks, %d tools, %d routes)\n", len(tasks.Tasks), len(tools.All()), tools.RouteCount())
	return nil
}
