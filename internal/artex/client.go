// Package artex is a small typed-ish HTTP client for the ARTEX REST API.
//
// It owns authentication (JWT obtained from /api/auth/login) and transparently
// re-logs-in once when a token expires. Callers only deal with method, path and
// JSON payloads.
package artex

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// APIError is returned for any non-2xx response.
type APIError struct {
	Status int
	Method string
	Path   string
	Msg    string
}

func (e *APIError) Error() string {
	if e.Msg == "" {
		return fmt.Sprintf("%s %s: HTTP %d", e.Method, e.Path, e.Status)
	}
	return fmt.Sprintf("%s %s: HTTP %d: %s", e.Method, e.Path, e.Status, e.Msg)
}

// Client talks to one ARTEX instance.
type Client struct {
	base string
	user string
	pass string

	mu    sync.Mutex
	token string
	hc    *http.Client
}

// New builds a client. token may be pre-set (e.g. from config) to skip login.
func New(baseURL, username, password, token string) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		user:  username,
		pass:  password,
		token: token,
		hc:    &http.Client{Timeout: 60 * time.Second},
	}
}

// BaseURL reports the configured endpoint, for diagnostics.
func (c *Client) BaseURL() string { return c.base }

// Login exchanges username/password for a JWT.
func (c *Client) Login(ctx context.Context) error {
	if c.pass == "" {
		return errors.New("未配置 ARTEX 密码（配置文件 password 或环境变量 ARTEX_PASSWORD）")
	}
	var out struct {
		Token string `json:"token"`
	}
	body := map[string]string{"username": c.user, "password": c.pass}
	if err := c.raw(ctx, http.MethodPost, "/api/auth/login", body, &out, ""); err != nil {
		return fmt.Errorf("登录 ARTEX 失败: %w", err)
	}
	if out.Token == "" {
		return errors.New("登录 ARTEX 失败: 响应中没有 token")
	}
	c.mu.Lock()
	c.token = out.Token
	c.mu.Unlock()
	return nil
}

func (c *Client) currentToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

// Do performs an authenticated request, logging in first when no token is held
// and retrying once after a 401/403.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	tok := c.currentToken()
	if tok == "" {
		if err := c.Login(ctx); err != nil {
			return err
		}
		tok = c.currentToken()
	}
	err := c.raw(ctx, method, path, body, out, tok)
	var ae *APIError
	if errors.As(err, &ae) && (ae.Status == http.StatusUnauthorized || ae.Status == http.StatusForbidden) && c.pass != "" {
		if lerr := c.Login(ctx); lerr != nil {
			return err
		}
		return c.raw(ctx, method, path, body, out, c.currentToken())
	}
	return err
}

// DoRaw performs an unauthenticated request (health, login).
func (c *Client) DoRaw(ctx context.Context, method, path string, body, out any) error {
	return c.raw(ctx, method, path, body, out, "")
}

// GetJSON is a convenience wrapper returning a decoded generic value.
func (c *Client) GetJSON(ctx context.Context, path string) (any, error) {
	var out any
	if err := c.Do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) raw(ctx context.Context, method, path string, body, out any, token string) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("请求 ARTEX 失败: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return fmt.Errorf("读取 ARTEX 响应失败: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		msg := strings.TrimSpace(string(data))
		var e struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(data, &e) == nil && e.Error != "" {
			msg = e.Error
		}
		msg = truncateRunes(msg, 2000)
		return &APIError{Status: resp.StatusCode, Method: method, Path: path, Msg: msg}
	}
	if out != nil && len(data) > 0 {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		if err := dec.Decode(out); err != nil {
			return fmt.Errorf("解析 ARTEX 响应失败: %w", err)
		}
	}
	return nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
