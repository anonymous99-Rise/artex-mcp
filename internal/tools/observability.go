package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"artex-mcp/internal/artex"
)

func init() {
	register(Tool{
		Name:  "artex_activity",
		Title: "活动、日志与用量",
		Description: "读取 ARTEX 的运行可观测数据。action 取值：\n" +
			"- history：会话活动历史（task/session/limit/before）\n" +
			"- detail：单条活动详情（seq）\n" +
			"- activity：实时活动流快照（task）\n" +
			"- main_sessions / new_main_session：主会话列表 / 新建主会话\n" +
			"- tokens / daily_tokens / conversation_tokens / usage：token 用量\n" +
			"- logs / logs_history：服务端日志\n" +
			"- audit：审计记录\n" +
			"- commands / commands_stats：工具调用记录与聚合\n" +
			"- llm_records / llm_record_get / llm_tasks / llm_by_model：LLM 请求记录\n" +
			"- llm_records_delete：删除 LLM 请求记录",
		InputSchema: obj(map[string]any{
			"action":    actionProp("history", "detail", "activity", "main_sessions", "new_main_session", "tokens", "daily_tokens", "conversation_tokens", "usage", "logs", "logs_history", "audit", "commands", "commands_stats", "llm_records", "llm_record_get", "llm_tasks", "llm_by_model", "llm_records_delete"),
			"task_id":   str("任务 id"),
			"session":   str("会话：main | plan | intent:<id>"),
			"seq":       integer("活动 seq（detail 必填）"),
			"record_id": integer("LLM 记录 id（llm_record_get 必填）"),
			"worker":    str("worker 过滤"),
			"since":     integer("只返回该序号之后的活动"),
			"before":    integer("向前翻页游标"),
			"limit":     integer("数量上限"),
			"page":      integer("页码"),
			"size":      integer("每页数量"),
			"model":     str("模型过滤"),
		}, "action"),
		Handler: activityTool,
	})

	register(Tool{
		Name:        "artex_traffic",
		Title:       "MITM 流量",
		Description: "查看/清理 ARTEX MITM 代理记录的 HTTP 流量。action 取值：list（按 host/method/q/path/status 过滤分页）、hosts（主机列表）、exchange（单条完整请求响应，需要 exchange_id）、delete（按 host 子串删除）、delete_hosts（按 hosts 数组删除）、clear（清空全部）。",
		InputSchema: obj(map[string]any{
			"action":      actionProp("list", "hosts", "exchange", "delete", "delete_hosts", "clear"),
			"host":        str("主机过滤"),
			"method":      str("HTTP 方法"),
			"q":           str("自由文本"),
			"path":        str("路径过滤"),
			"body":        str("正文过滤"),
			"status":      str("状态码过滤"),
			"exchange_id": str("单条流量的 id"),
			"hosts":       strArr("delete_hosts 的主机数组"),
			"page":        integer("页码，默认 0"),
			"size":        integer("每页数量，默认 100，最大 500"),
		}, "action"),
		Handler: trafficTool,
	})
}

func activityTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	taskID := a.Str("task_id")
	paging := func() url.Values {
		q := url.Values{}
		if taskID != "" {
			q.Set("task", taskID)
		}
		if v := a.Str("session"); v != "" {
			q.Set("session", v)
		}
		if v := a.Str("worker"); v != "" {
			q.Set("worker", v)
		}
		if a.Int("limit", 0) > 0 {
			q.Set("limit", strconv.Itoa(a.Int("limit", 0)))
		}
		if a.Int("page", 0) > 0 {
			q.Set("page", strconv.Itoa(a.Int("page", 0)))
		}
		if a.Int("size", 0) > 0 {
			q.Set("size", strconv.Itoa(a.Int("size", 0)))
		}
		if a.Has("before") {
			q.Set("before", strconv.Itoa(a.Int("before", 0)))
		}
		if a.Has("since") {
			q.Set("since", strconv.Itoa(a.Int("since", 0)))
		}
		if v := a.Str("model"); v != "" {
			q.Set("model", v)
		}
		return q
	}

	switch action {
	case "history":
		return getQ(ctx, c, "/api/exploration/activity/history", paging())

	case "detail":
		seq := a.Int("seq", 0)
		if seq <= 0 {
			return nil, fmt.Errorf("detail 需要 seq")
		}
		return get(ctx, c, "/api/exploration/activity/"+strconv.Itoa(seq))

	case "activity":
		return getQ(ctx, c, "/api/exploration/activity", paging())

	case "main_sessions":
		q := url.Values{}
		if taskID != "" {
			q.Set("task", taskID)
		}
		return getQ(ctx, c, "/api/exploration/main-sessions", q)

	case "new_main_session":
		return post(ctx, c, "/api/exploration/main-session/new", map[string]any{"task": taskID})

	case "tokens":
		return getQ(ctx, c, "/api/exploration/tokens", paging())

	case "daily_tokens":
		return getQ(ctx, c, "/api/tokens/daily", paging())

	case "conversation_tokens":
		return getQ(ctx, c, "/api/tokens/conversations", paging())

	case "usage":
		return getQ(ctx, c, "/api/tokens/usage", paging())

	case "logs":
		return getQ(ctx, c, "/api/logs", paging())

	case "logs_history":
		return getQ(ctx, c, "/api/logs/history", paging())

	case "audit":
		return getQ(ctx, c, "/api/audit", paging())

	case "commands":
		return getQ(ctx, c, "/api/commands", paging())

	case "commands_stats":
		return get(ctx, c, "/api/commands/stats")

	case "llm_records":
		return getQ(ctx, c, "/api/llm/records", paging())

	case "llm_record_get":
		id := a.Int("record_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("llm_record_get 需要 record_id")
		}
		return get(ctx, c, "/api/llm/records/"+strconv.Itoa(id))

	case "llm_records_delete":
		return del(ctx, c, "/api/llm/records", paging())

	case "llm_tasks":
		return get(ctx, c, "/api/llm/records/tasks")

	case "llm_by_model":
		return getQ(ctx, c, "/api/llm/records/by-model", paging())
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}

func trafficTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	q := url.Values{}
	for _, k := range []string{"host", "method", "q", "path", "body", "status"} {
		if v := a.Str(k); v != "" {
			q.Set(k, v)
		}
	}

	switch action {
	case "list":
		q.Set("page", strconv.Itoa(a.Int("page", 0)))
		size := a.Int("size", 100)
		if size <= 0 || size > 500 {
			size = 100
		}
		q.Set("size", strconv.Itoa(size))
		return getQ(ctx, c, "/api/traffic", q)

	case "hosts":
		return get(ctx, c, "/api/traffic/hosts")

	case "exchange":
		id := a.Str("exchange_id")
		if id == "" {
			return nil, fmt.Errorf("exchange 需要 exchange_id")
		}
		return getQ(ctx, c, "/api/traffic/exchange", url.Values{"id": {id}})

	case "delete":
		if q.Get("host") == "" {
			return nil, fmt.Errorf("delete 需要 host")
		}
		return del(ctx, c, "/api/traffic?"+q.Encode(), nil)

	case "delete_hosts":
		hosts, err := a.Strings("hosts")
		if err != nil {
			return nil, err
		}
		if len(hosts) == 0 {
			return nil, fmt.Errorf("hosts 不能为空")
		}
		return del(ctx, c, "/api/traffic/hosts", map[string]any{"hosts": hosts})

	case "clear":
		return del(ctx, c, "/api/traffic/all", nil)
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
