package artex

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestLoginAndAuthenticatedRequest(t *testing.T) {
	var logins int32
	var gotAuth atomic.Value
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&logins, 1)
		var body struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.Username != "ARTEX" || body.Password != "secret" {
			http.Error(w, `{"error":"bad credentials"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		_ = json.NewEncoder(w).Encode(map[string]any{"tasks": []any{}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "ARTEX", "secret", "")
	var out map[string]any
	if err := c.Do(context.Background(), "GET", "/api/tasks", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if atomic.LoadInt32(&logins) != 1 {
		t.Fatalf("logins = %d, want 1", logins)
	}
	if got := gotAuth.Load(); got != "Bearer tok" {
		t.Fatalf("Authorization = %v, want Bearer tok", got)
	}
}

func TestReloginOnUnauthorized(t *testing.T) {
	var logins int32
	var calls int32
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&logins, 1)
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "tok" + string(rune('0'+n))})
	})
	mux.HandleFunc("/api/protected", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Header.Get("Authorization") != "Bearer tok2" {
			http.Error(w, `{"error":"expired"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := New(srv.URL, "ARTEX", "secret", "")
	var out map[string]any
	if err := c.Do(context.Background(), "GET", "/api/protected", nil, &out); err != nil {
		t.Fatalf("Do: %v", err)
	}
	if logins != 2 {
		t.Fatalf("logins = %d, want 2 (initial + retry)", logins)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if out["ok"] != true {
		t.Fatalf("out = %v", out)
	}
}

func TestAPIErrorParsing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "ARTEX", "", "")
	err := c.DoRaw(context.Background(), "GET", "/api/x", nil, nil)
	var ae *APIError
	if !errors.As(err, &ae) {
		t.Fatalf("err = %v, want *APIError", err)
	}
	if ae.Status != 400 || ae.Msg != "boom" {
		t.Fatalf("APIError = %+v", ae)
	}
	if !strings.Contains(err.Error(), "GET /api/x") {
		t.Fatalf("error text missing method/path: %v", err)
	}
}

func TestDoWithoutPasswordFails(t *testing.T) {
	c := New("http://127.0.0.1:1", "ARTEX", "", "")
	err := c.Do(context.Background(), "GET", "/api/tasks", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "未配置 ARTEX 密码") {
		t.Fatalf("err = %v, want missing-password error", err)
	}
}

func TestDecodesJSONNumbers(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"seq":9007199254740993}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "ARTEX", "", "")
	var out any
	if err := c.DoRaw(context.Background(), "GET", "/api/x", nil, &out); err != nil {
		t.Fatal(err)
	}
	obj := out.(map[string]any)
	if got := obj["seq"].(json.Number).String(); got != "9007199254740993" {
		t.Fatalf("seq = %s, want exact integer", got)
	}
}
