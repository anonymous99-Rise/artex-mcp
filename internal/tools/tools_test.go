package tools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"artex-mcp/internal/artex"
)

type fakeARTEX struct {
	mu      sync.Mutex
	lastIDs map[string]any
	lastURL string
	srv     *httptest.Server
}

func newFakeARTEX(t *testing.T) *fakeARTEX {
	t.Helper()
	f := &fakeARTEX{lastIDs: map[string]any{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"token":"tok"}`))
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.lastIDs = body
			f.mu.Unlock()
			_, _ = w.Write([]byte(`{"id":"task-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"tasks":[]}`))
	})
	mux.HandleFunc("/api/tasks/task-1/scope", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.lastIDs = body
			f.mu.Unlock()
			_, _ = w.Write([]byte(`{"id":3,"kind":"cidr","value":"10.0.0.0/8"}`))
			return
		}
		_, _ = w.Write([]byte(`{"scope":[]}`))
	})
	mux.HandleFunc("/api/anything", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.lastURL = r.URL.RequestURI()
		f.mu.Unlock()
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeARTEX) client() *artex.Client { return artex.New(f.srv.URL, "ARTEX", "", "tok") }

func TestCreateTaskBuildsBody(t *testing.T) {
	f := newFakeARTEX(t)
	out, err := Dispatch(context.Background(), f.client(), "artex_create_task", map[string]any{
		"description":       "example.com 授权测试",
		"goal":              "找出可利用的高危漏洞",
		"name":              "demo",
		"llm_profile_ids":   []any{json.Number("1"), float64(2)},
		"seed_first_intent": true,
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !strings.Contains(out, "task-1") {
		t.Fatalf("output missing id: %s", out)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastIDs["description"] != "example.com 授权测试" || f.lastIDs["goal"] != "找出可利用的高危漏洞" {
		t.Fatalf("body = %v", f.lastIDs)
	}
	ids, _ := f.lastIDs["llm_profile_ids"].([]any)
	if len(ids) != 2 {
		t.Fatalf("llm_profile_ids = %v", f.lastIDs["llm_profile_ids"])
	}
	if f.lastIDs["seed_first_intent"] != true {
		t.Fatalf("seed_first_intent = %v", f.lastIDs["seed_first_intent"])
	}
}

func TestTaskAdminScopeAdd(t *testing.T) {
	f := newFakeARTEX(t)
	_, err := Dispatch(context.Background(), f.client(), "artex_task_admin", map[string]any{
		"action":  "scope_add",
		"task_id": "task-1",
		"kind":    "cidr",
		"value":   "10.0.0.0/8",
		"reason":  "unit test",
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastIDs["kind"] != "cidr" || f.lastIDs["value"] != "10.0.0.0/8" {
		t.Fatalf("body = %v", f.lastIDs)
	}
}

func TestAPIRequestQueryAndSafety(t *testing.T) {
	f := newFakeARTEX(t)
	_, err := Dispatch(context.Background(), f.client(), "artex_api_request", map[string]any{
		"method": "GET",
		"path":   "/api/anything",
		"query":  map[string]any{"task": "42", "limit": json.Number("5")},
	})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	f.mu.Lock()
	got := f.lastURL
	f.mu.Unlock()
	if !strings.Contains(got, "task=42") || !strings.Contains(got, "limit=5") {
		t.Fatalf("request URI = %s", got)
	}

	if _, err := Dispatch(context.Background(), f.client(), "artex_api_request", map[string]any{"method": "GET", "path": "/etc/passwd"}); err == nil {
		t.Fatal("expected rejection of non-/api/ path")
	}
	if _, err := Dispatch(context.Background(), f.client(), "artex_api_request", map[string]any{"method": "GET", "path": "/api/tasks/{id}"}); err == nil {
		t.Fatal("expected rejection of unresolved placeholder")
	}
}

func TestFindRoutes(t *testing.T) {
	out, err := Dispatch(context.Background(), nil, "artex_find_routes", map[string]any{"query": "findings", "limit": 5})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !strings.Contains(out, "/api/exploration/findings") {
		t.Fatalf("routes output did not match: %s", out)
	}
}

func TestUnknownActionIsAnError(t *testing.T) {
	f := newFakeARTEX(t)
	_, err := Dispatch(context.Background(), f.client(), "artex_task_admin", map[string]any{"action": "bogus"})
	if err == nil || !strings.Contains(err.Error(), "未知 action") {
		t.Fatalf("err = %v", err)
	}
}
