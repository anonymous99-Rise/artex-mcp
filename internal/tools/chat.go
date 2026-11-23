package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"artex-mcp/internal/artex"
)

func init() {
	register(Tool{
		Name:        "artex_conversations",
		Title:       "会话管理",
		Description: "管理 ARTEX 对话（平台助手/渗透 Agent 的会话）：list 列表、messages 读取消息、send 发送消息（不等待）、stop 中止当前生成、rename 改名/置顶、set_profile 指定 LLM、delete 删除、delete_batch 批量删除、agents 列出可用 agent。",
		InputSchema: obj(map[string]any{
			"action":           actionProp("list", "messages", "send", "stop", "rename", "set_profile", "delete", "delete_batch", "agents"),
			"conversation_id":  integer("会话 id（除 list/agents/delete_batch 外必填）"),
			"agent_key":        str("创建/筛选用的 agent key（list 时可按 agent 过滤）"),
			"message":          str("send 时发送的文本"),
			"title":            str("rename 时的新标题"),
			"pinned":           boolean("rename 时是否置顶"),
			"llm_profile_id":   integer("set_profile 时的 LLM 配置 id；0 表示清除覆盖"),
			"conversation_ids": intArr("delete_batch 时的会话 id 数组（最多 100）"),
			"since":            integer("messages 时只返回 seq > since 的消息"),
			"before":           integer("messages 时向前翻页游标"),
			"limit":            integer("messages 条数，默认 50"),
		}, "action"),
		Handler: conversations,
	})
}

func conversations(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	switch action {
	case "list":
		return get(ctx, c, "/api/conversations")

	case "agents":
		return get(ctx, c, "/api/agents")

	case "messages":
		id, err := a.NeedStr("conversation_id")
		if err != nil {
			return nil, err
		}
		q := url.Values{}
		if a.Has("since") {
			q.Set("since", strconv.Itoa(a.Int("since", 0)))
		}
		if a.Has("before") {
			q.Set("before", strconv.Itoa(a.Int("before", 0)))
		}
		if a.Has("limit") {
			q.Set("limit", strconv.Itoa(a.Int("limit", 50)))
		}
		return getQ(ctx, c, idPath("/api/conversations", id)+"/messages", q)

	case "send":
		id, err := a.NeedStr("conversation_id")
		if err != nil {
			return nil, err
		}
		msg, err := a.NeedStr("message")
		if err != nil {
			return nil, err
		}
		if _, err := post(ctx, c, idPath("/api/conversations", id)+"/messages", map[string]any{"message": msg}); err != nil {
			return nil, err
		}
		return fmt.Sprintf("消息已发送到会话 #%s。用 artex_conversations(action=messages) 读取进展，或 artex_ask(conversation_id=%s) 等待回复。", id, id), nil

	case "stop":
		id, err := a.NeedStr("conversation_id")
		if err != nil {
			return nil, err
		}
		return post(ctx, c, idPath("/api/conversations", id)+"/stop", nil)

	case "rename":
		id, err := a.NeedStr("conversation_id")
		if err != nil {
			return nil, err
		}
		body := map[string]any{}
		if a.Has("title") {
			body["title"] = a.Str("title")
		}
		if a.Has("pinned") {
			body["pinned"] = a.Bool("pinned", false)
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("rename 需要 title 或 pinned")
		}
		return patch(ctx, c, idPath("/api/conversations", id), body)

	case "set_profile":
		id, err := a.NeedStr("conversation_id")
		if err != nil {
			return nil, err
		}
		// null clears the per-conversation override; 0 in tool args means clear.
		var profile any
		if v := a.Int("llm_profile_id", 0); v > 0 {
			profile = v
		}
		return patch(ctx, c, idPath("/api/conversations", id)+"/profile", map[string]any{"llm_profile_id": profile})

	case "delete":
		id, err := a.NeedStr("conversation_id")
		if err != nil {
			return nil, err
		}
		return del(ctx, c, idPath("/api/conversations", id), nil)

	case "delete_batch":
		ids, err := a.Int64s("conversation_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("conversation_ids 不能为空")
		}
		if len(ids) > 100 {
			return nil, fmt.Errorf("单次最多删除 100 个会话")
		}
		return post(ctx, c, "/api/conversations/delete/batch", map[string]any{"ids": ids})
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}

// ---------- async conversation polling ----------

type convItem struct {
	Seq     int64  `json:"seq"`
	Worker  string `json:"worker"`
	Kind    string `json:"kind"`
	IsError bool   `json:"is_error"`
	Summary string `json:"summary"`
	TS      string `json:"ts"`
}

// lastAssistantIndex prefers the newest assistant text block; the terminal
// 'result' record is only a fallback because its summary is a status note, not
// the answer.
func lastAssistantIndex(items []convItem) int {
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if it.Kind == "text" && !it.IsError && strings.TrimSpace(it.Summary) != "" {
			return i
		}
	}
	for i := len(items) - 1; i >= 0; i-- {
		it := items[i]
		if it.Kind == "result" && !it.IsError && strings.TrimSpace(it.Summary) != "" {
			return i
		}
	}
	return -1
}

func lastAssistantText(items []convItem) string {
	if i := lastAssistantIndex(items); i >= 0 {
		return items[i].Summary
	}
	return ""
}

func ask(ctx context.Context, c *artex.Client, a Args) (any, error) {
	msg := a.Str("message")
	convID := int64(a.Int("conversation_id", 0))
	wait := clampWait(a.Int("wait_seconds", 45), 300)

	if msg == "" {
		if convID <= 0 {
			return nil, fmt.Errorf("message 不能为空（或提供 conversation_id 读取最新回复）")
		}
		var page struct {
			Items []convItem `json:"items"`
		}
		p := fmt.Sprintf("/api/conversations/%d/messages?limit=50", convID)
		if err := c.Do(ctx, http.MethodGet, p, nil, &page); err != nil {
			return nil, err
		}
		text := lastAssistantText(page.Items)
		if text == "" {
			return "该会话暂无助手文本回复。", nil
		}
		return text, nil
	}

	var since int64
	if convID <= 0 {
		agentKey := a.Str("agent_key")
		if agentKey == "" {
			agentKey = "auto"
		}
		var conv struct {
			ID int64 `json:"id"`
		}
		body := map[string]any{"agent_key": agentKey, "title": firstLine(msg, 40)}
		if err := c.Do(ctx, http.MethodPost, "/api/conversations", body, &conv); err != nil {
			return nil, err
		}
		convID = conv.ID
		if convID == 0 {
			return nil, fmt.Errorf("创建 ARTEX 会话失败：响应缺少 id")
		}
	} else {
		var page struct {
			Cursor int64 `json:"cursor"`
		}
		p := fmt.Sprintf("/api/conversations/%d/messages?limit=1", convID)
		if err := c.Do(ctx, http.MethodGet, p, nil, &page); err != nil {
			return nil, err
		}
		since = page.Cursor
	}

	if err := c.Do(ctx, http.MethodPost, fmt.Sprintf("/api/conversations/%d/messages", convID), map[string]any{"message": msg}, nil); err != nil {
		return nil, err
	}
	if wait == 0 {
		return fmt.Sprintf("消息已发送到 ARTEX 会话 #%d，助手正在后台处理。可用 artex_ask(conversation_id=%d) 读取最新回复。", convID, convID), nil
	}
	items, running, err := pollConversation(ctx, c, convID, since, time.Duration(wait)*time.Second)
	if err != nil {
		return nil, err
	}
	final := lastAssistantText(items)
	if final == "" && !running {
		return fmt.Sprintf("ARTEX 会话 #%d 本轮已结束，但没有文本回复。", convID), nil
	}
	if final == "" {
		return fmt.Sprintf("ARTEX 助手仍在处理（会话 #%d）。稍后用 artex_ask(conversation_id=%d) 读取最新回复。", convID, convID), nil
	}
	if detail := convDetail(ctx, c, convID, items); detail != "" {
		final = detail
	}
	if running {
		final += "\n\n（仍在生成中，以上为目前最后一段文本；稍后可继续读取。）"
	}
	return final, nil
}

func pollConversation(ctx context.Context, c *artex.Client, id, since int64, wait time.Duration) ([]convItem, bool, error) {
	deadline := time.Now().Add(wait)
	var items []convItem
	cursor := since
	for {
		var page struct {
			Items   []convItem `json:"items"`
			Cursor  int64      `json:"cursor"`
			Running bool       `json:"running"`
		}
		p := fmt.Sprintf("/api/conversations/%d/messages?since=%d", id, cursor)
		if err := c.Do(ctx, http.MethodGet, p, nil, &page); err != nil {
			return items, false, err
		}
		items = append(items, page.Items...)
		cursor = page.Cursor
		if !page.Running {
			return items, false, nil
		}
		if time.Now().After(deadline) {
			return items, true, nil
		}
		select {
		case <-ctx.Done():
			return items, true, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

// convDetail fetches the full text of the last assistant item, since list
// summaries are truncated to one line.
func convDetail(ctx context.Context, c *artex.Client, convID int64, items []convItem) string {
	i := lastAssistantIndex(items)
	if i < 0 || items[i].Seq == 0 {
		return ""
	}
	var out struct {
		Detail string `json:"detail"`
	}
	p := fmt.Sprintf("/api/conversations/%d/messages/%d", convID, items[i].Seq)
	if err := c.Do(ctx, http.MethodGet, p, nil, &out); err != nil {
		return ""
	}
	return strings.TrimSpace(out.Detail)
}
