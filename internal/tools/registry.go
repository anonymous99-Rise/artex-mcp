// Package tools defines the MCP tool surface exposed over ARTEX.
//
// Every tool is a thin, typed wrapper over one or more ARTEX REST endpoints.
// Read-only tools are annotated so MCP clients can auto-approve them; tools that
// mutate or delete data are marked destructive.
package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"artex-mcp/internal/artex"
)

// ErrUnknownTool is returned by Dispatch for an unregistered name.
var ErrUnknownTool = errors.New("unknown tool")

// Args is the decoded JSON object passed to a tool.
type Args map[string]any

// Tool is one MCP tool.
type Tool struct {
	Name        string
	Title       string
	Description string
	InputSchema map[string]any
	ReadOnly    bool
	Destructive bool
	Handler     func(ctx context.Context, c *artex.Client, a Args) (any, error)
}

var registry []Tool

// register appends a tool; called from init() in the tools_*.go files.
func register(t Tool) { registry = append(registry, t) }

// All returns the registered tools in definition order (alphabetical by file).
func All() []Tool { return registry }

// Find looks a tool up by name.
func Find(name string) (Tool, bool) {
	for _, t := range registry {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}

// Defs renders tools/list entries.
func Defs() []map[string]any {
	out := make([]map[string]any, 0, len(registry))
	for _, t := range registry {
		m := map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.InputSchema,
		}
		if t.Title != "" {
			m["title"] = t.Title
		}
		ann := map[string]any{"readOnlyHint": t.ReadOnly}
		if !t.ReadOnly {
			ann["destructiveHint"] = t.Destructive
		}
		m["annotations"] = ann
		out = append(out, m)
	}
	return out
}

// Dispatch executes a tool call and renders its result as text.
func Dispatch(ctx context.Context, c *artex.Client, name string, a map[string]any) (string, error) {
	t, ok := Find(name)
	if !ok {
		return "", ErrUnknownTool
	}
	if a == nil {
		a = map[string]any{}
	}
	v, err := t.Handler(ctx, c, Args(a))
	if err != nil {
		return "", err
	}
	if s, ok := v.(string); ok {
		return truncate(s, maxToolText), nil
	}
	return jsonText(v), nil
}

// ---------- schema helpers ----------

func obj(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	s := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
func enum(desc string, vals ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": vals}
}
func integer(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}
func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func strArr(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "string"}}
}
func intArr(desc string) map[string]any {
	return map[string]any{"type": "array", "description": desc, "items": map[string]any{"type": "integer"}}
}
func rawObj(desc string) map[string]any {
	return map[string]any{"type": "object", "description": desc, "additionalProperties": true}
}

// actionProp builds the shared "action" discriminator property.
func actionProp(vals ...string) map[string]any {
	return enum("操作类型（必填）: "+strings.Join(vals, " | "), vals...)
}

// ---------- argument accessors ----------

func (a Args) Has(k string) bool {
	v, ok := a[k]
	return ok && v != nil
}

func (a Args) Str(k string) string {
	switch v := a[k].(type) {
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

func (a Args) NeedStr(k string) (string, error) {
	v := a.Str(k)
	if v == "" {
		return "", fmt.Errorf("%s 不能为空", k)
	}
	return v, nil
}

func (a Args) Int(k string, def int) int {
	switch v := a[k].(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
	case float64:
		return int(v)
	case int:
		return v
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func (a Args) Bool(k string, def bool) bool {
	switch v := a[k].(type) {
	case bool:
		return v
	case string:
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return def
}

func (a Args) Int64s(k string) ([]int64, error) {
	raw, ok := a[k]
	if !ok || raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s 必须是整数数组", k)
	}
	out := make([]int64, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case json.Number:
			n, err := v.Int64()
			if err != nil {
				return nil, fmt.Errorf("%s 含无效整数: %v", k, item)
			}
			out = append(out, n)
		case float64:
			out = append(out, int64(v))
		case string:
			n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s 含无效整数: %v", k, item)
			}
			out = append(out, n)
		default:
			return nil, fmt.Errorf("%s 含无效整数: %v", k, item)
		}
	}
	return out, nil
}

func (a Args) Strings(k string) ([]string, error) {
	raw, ok := a[k]
	if !ok || raw == nil {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s 必须是字符串数组", k)
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case string:
			if s := strings.TrimSpace(v); s != "" {
				out = append(out, s)
			}
		case json.Number:
			out = append(out, v.String())
		case float64:
			out = append(out, strconv.FormatFloat(v, 'f', -1, 64))
		default:
			return nil, fmt.Errorf("%s 含无效字符串: %v", k, item)
		}
	}
	return out, nil
}

// Map returns a nested JSON object.
func (a Args) Map(k string) map[string]any {
	if m, ok := a[k].(map[string]any); ok {
		return m
	}
	return nil
}

// Any returns the raw decoded value.
func (a Args) Any(k string) any { return a[k] }

// ---------- output helpers ----------

const maxToolText = 80000

func jsonText(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return truncate(string(b), maxToolText)
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "\n…(输出已截断)"
}

func firstLine(s string, n int) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// ---------- HTTP helpers used by handlers ----------

func get(ctx context.Context, c *artex.Client, path string) (any, error) {
	return c.GetJSON(ctx, path)
}

func getQ(ctx context.Context, c *artex.Client, path string, q url.Values) (any, error) {
	if s := q.Encode(); s != "" {
		path += "?" + s
	}
	return c.GetJSON(ctx, path)
}

func send(ctx context.Context, c *artex.Client, method, path string, body any) (any, error) {
	var out any
	if err := c.Do(ctx, method, path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func post(ctx context.Context, c *artex.Client, path string, body any) (any, error) {
	return send(ctx, c, http.MethodPost, path, body)
}

func put(ctx context.Context, c *artex.Client, path string, body any) (any, error) {
	return send(ctx, c, http.MethodPut, path, body)
}

func patch(ctx context.Context, c *artex.Client, path string, body any) (any, error) {
	return send(ctx, c, http.MethodPatch, path, body)
}

func del(ctx context.Context, c *artex.Client, path string, body any) (any, error) {
	return send(ctx, c, http.MethodDelete, path, body)
}

func esc(s string) string { return url.PathEscape(s) }

func values(m map[string]string) url.Values {
	v := url.Values{}
	for k, s := range m {
		if s != "" {
			v.Set(k, s)
		}
	}
	return v
}

func idPath(base, id string) string { return base + "/" + esc(id) }
