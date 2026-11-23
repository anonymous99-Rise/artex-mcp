package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"artex-mcp/internal/artex"
)

// routes.json is generated from the ARTEX source tree by scripts/gen_routes.py.
// It is a discovery aid for the generic artex_api_request escape hatch.
//
//go:embed routes.json
var routesJSON []byte

type routeEntry struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Handler string `json:"handler"`
	Source  string `json:"source"`
}

var routeCatalog []routeEntry

func init() {
	var doc struct {
		Routes map[string]routeEntry `json:"routes"`
	}
	if err := json.Unmarshal(routesJSON, &doc); err == nil {
		for _, r := range doc.Routes {
			routeCatalog = append(routeCatalog, r)
		}
		sort.Slice(routeCatalog, func(i, j int) bool {
			if routeCatalog[i].Path == routeCatalog[j].Path {
				return routeCatalog[i].Method < routeCatalog[j].Method
			}
			return routeCatalog[i].Path < routeCatalog[j].Path
		})
	}

	register(Tool{
		Name:        "artex_find_routes",
		Title:       "查找 ARTEX API 路由",
		Description: "在 ARTEX 全部 API 路由目录中搜索（按路径或 handler 名称，不区分大小写）。当高层工具没有覆盖某个操作时，用本工具找到 method+path，再调用 artex_api_request。",
		InputSchema: obj(map[string]any{
			"query":  str("搜索关键字，如 findings/assets/tasks/{id}/scope"),
			"method": enum("只看某个 HTTP 方法", "GET", "POST", "PATCH", "PUT", "DELETE"),
			"limit":  integer("返回条数，默认 40，最大 200"),
		}),
		ReadOnly: true,
		Handler:  findRoutes,
	})

	register(Tool{
		Name:        "artex_api_request",
		Title:       "直接调用 ARTEX API（兜底）",
		Description: "直接向 ARTEX REST API 发请求，覆盖所有未被高层工具封装的端点。先用 artex_find_routes 找路由。path 必须以 /api/ 开头；query 为查询参数对象；body 为 JSON 请求体。这是全权入口（可写可删），仅在高层工具缺失时使用。",
		InputSchema: obj(map[string]any{
			"method": enum("HTTP 方法", "GET", "POST", "PATCH", "PUT", "DELETE"),
			"path":   str("API 路径，如 /api/tasks/{id}（把 {id} 换成实际值）"),
			"query":  rawObj("查询参数对象，如 {\"task\":\"123\",\"limit\":50}"),
			"body":   map[string]any{"description": "JSON 请求体（任意类型）", "type": []string{"object", "array", "string", "number", "boolean", "null"}},
		}, "method", "path"),
		Destructive: true,
		Handler:     apiRequest,
	})
}

// RouteCount reports how many API routes the embedded catalog holds.
func RouteCount() int { return len(routeCatalog) }

func findRoutes(_ context.Context, _ *artex.Client, a Args) (any, error) {
	query := strings.ToLower(a.Str("query"))
	method := strings.ToUpper(a.Str("method"))
	limit := a.Int("limit", 40)
	if limit <= 0 || limit > 200 {
		limit = 40
	}
	type item struct {
		Method  string `json:"method"`
		Path    string `json:"path"`
		Handler string `json:"handler"`
		Source  string `json:"source"`
	}
	out := make([]item, 0, limit)
	for _, r := range routeCatalog {
		if method != "" && r.Method != method {
			continue
		}
		if query != "" {
			hay := strings.ToLower(r.Path + " " + r.Handler + " " + r.Method)
			if !strings.Contains(hay, query) {
				continue
			}
		}
		out = append(out, item{r.Method, r.Path, r.Handler, r.Source})
		if len(out) >= limit {
			break
		}
	}
	return map[string]any{
		"total_routes": len(routeCatalog),
		"matched":      len(out),
		"routes":       out,
		"hint":         "把 {id} 等占位符替换为实际值后，用 artex_api_request 调用。",
	}, nil
}

func apiRequest(ctx context.Context, c *artex.Client, a Args) (any, error) {
	method := strings.ToUpper(a.Str("method"))
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete:
	default:
		return nil, fmt.Errorf("method 必须是 GET/POST/PATCH/PUT/DELETE")
	}
	path := a.Str("path")
	if !strings.HasPrefix(path, "/api/") {
		return nil, fmt.Errorf("path 必须以 /api/ 开头")
	}
	if strings.Contains(path, "{") || strings.Contains(path, "}") {
		return nil, fmt.Errorf("path 仍含占位符，请替换为实际值")
	}
	if q := a.Map("query"); len(q) > 0 {
		vals := url.Values{}
		for k, v := range q {
			vals.Set(k, toQueryString(v))
		}
		sep := "?"
		if strings.Contains(path, "?") {
			sep = "&"
		}
		path += sep + vals.Encode()
	}

	var out any
	body := a.Any("body")
	if err := c.Do(ctx, method, path, body, &out); err != nil {
		return nil, err
	}
	if out == nil {
		return "OK", nil
	}
	return out, nil
}

func toQueryString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case nil:
		return ""
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprint(t)
		}
		return string(b)
	}
}
