package tools

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"artex-mcp/internal/artex"
)

func init() {
	register(Tool{
		Name:        "artex_health",
		Title:       "ARTEX 健康检查",
		Description: "检查 ARTEX 服务是否在线（无需登录）。返回版本与服务状态，并附带桥接层使用的 base_url。",
		InputSchema: obj(nil),
		ReadOnly:    true,
		Handler: func(ctx context.Context, c *artex.Client, _ Args) (any, error) {
			var out map[string]any
			if err := c.DoRaw(ctx, http.MethodGet, "/api/health", nil, &out); err != nil {
				return nil, err
			}
			out["base_url"] = c.BaseURL()
			return out, nil
		},
	})

	register(Tool{
		Name:        "artex_list_llm_profiles",
		Title:       "列出 LLM 配置",
		Description: "列出 ARTEX 已保存的 LLM 配置（id、名称、模型、是否生效、优先级）。创建任务需要 llm_profile_ids 时先看这里。",
		InputSchema: obj(nil),
		ReadOnly:    true,
		Handler: func(ctx context.Context, c *artex.Client, _ Args) (any, error) {
			return get(ctx, c, "/api/llm/profiles")
		},
	})

	register(Tool{
		Name:        "artex_list_tasks",
		Title:       "列出任务",
		Description: "列出 ARTEX 全部任务：状态（created/running/paused/done/failed）、token 消耗、目标进度、运行中 worker 数、各严重度发现数。",
		InputSchema: obj(nil),
		ReadOnly:    true,
		Handler: func(ctx context.Context, c *artex.Client, _ Args) (any, error) {
			return get(ctx, c, "/api/tasks")
		},
	})

	register(Tool{
		Name:        "artex_get_task",
		Title:       "读取任务",
		Description: "读取单个 ARTEX 任务的详情（描述、目标、状态、进度、LLM 配置、关联资产/企业等）。",
		InputSchema: obj(map[string]any{
			"task_id": str("任务 ID"),
		}, "task_id"),
		ReadOnly: true,
		Handler: func(ctx context.Context, c *artex.Client, a Args) (any, error) {
			id, err := a.NeedStr("task_id")
			if err != nil {
				return nil, err
			}
			return get(ctx, c, idPath("/api/tasks", id))
		},
	})

	register(Tool{
		Name:        "artex_create_task",
		Title:       "创建任务",
		Description: "在 ARTEX 创建一个渗透/探索任务。任务创建后由 ARTEX 引擎异步规划并执行（planner/worker），本调用立即返回。description 写清目标范围与背景，goal 写清要达成的最终结果。",
		InputSchema: obj(map[string]any{
			"description":            str("任务描述：目标范围、背景、约束（必填）"),
			"goal":                   str("最终目标：要拿到什么结果（必填）"),
			"name":                   str("任务名称（可选）"),
			"llm_profile_ids":        intArr("任务使用的 LLM 配置 id 列表（有序，第一个初始生效；省略=全局激活配置）"),
			"collection_ids":         intArr("关联的资产合集 id（FOFA 收集的显式资产集合）"),
			"company_ids":            intArr("关联的企业 id（快照关联企业资产）"),
			"timeout_seconds":        integer("任务超时秒数；0=不限时"),
			"plan_heartbeat_seconds": integer("planner 心跳间隔秒数；默认 600"),
			"seed_first_intent":      boolean("true=创建时立即下发种子意图（描述+目标）直接开跑，跳过首轮 planner；默认 false"),
			"coverage_enabled":       boolean("资产覆盖度功能；默认 true"),
		}, "description", "goal"),
		Handler: createTask,
	})

	register(Tool{
		Name:        "artex_control_task",
		Title:       "暂停/恢复任务",
		Description: "批量暂停或恢复 ARTEX 任务（单次最多 100 个）。",
		InputSchema: obj(map[string]any{
			"task_ids": strArr("任务 ID 数组"),
			"action":   enum("pause=暂停，resume=恢复", "pause", "resume"),
		}, "task_ids", "action"),
		Handler: func(ctx context.Context, c *artex.Client, a Args) (any, error) {
			ids, err := a.Strings("task_ids")
			if err != nil {
				return nil, err
			}
			if len(ids) == 0 {
				return nil, fmt.Errorf("task_ids 不能为空")
			}
			if len(ids) > 100 {
				return nil, fmt.Errorf("单次最多控制 100 个任务")
			}
			action := a.Str("action")
			if action != "pause" && action != "resume" {
				return nil, fmt.Errorf("action 必须是 pause 或 resume")
			}
			return post(ctx, c, "/api/tasks/control/batch", map[string]any{"task_ids": ids, "action": action})
		},
	})

	register(Tool{
		Name:        "artex_task_stats",
		Title:       "任务统计",
		Description: "查看运行统计：引擎模式、LLM 是否就绪、流量/资产计数、探索统计、运行中意图数、目标达成数、最后活动时间。",
		InputSchema: obj(map[string]any{
			"task_id": str("任务 ID；省略=当前激活任务"),
		}),
		ReadOnly: true,
		Handler: func(ctx context.Context, c *artex.Client, a Args) (any, error) {
			q := url.Values{}
			if id := a.Str("task_id"); id != "" {
				q.Set("task", id)
			}
			return getQ(ctx, c, "/api/stats", q)
		},
	})

	register(Tool{
		Name:        "artex_list_findings",
		Title:       "列出漏洞发现",
		Description: "列出 ARTEX 已登记的漏洞发现。task_id 存在时返回该任务的全部发现；否则返回全局分页发现，可按严重度/状态/类型/关键字筛选。",
		InputSchema: obj(map[string]any{
			"task_id":   str("任务 ID（可选）"),
			"page":      integer("页码，默认 1"),
			"limit":     integer("每页数量，默认 20，最大 200"),
			"severity":  str("严重度筛选（critical/high/medium/low/info）"),
			"status":    str("状态筛选"),
			"vulnclass": str("漏洞类型筛选"),
			"q":         str("关键字搜索"),
			"sort":      str("排序字段"),
		}),
		ReadOnly: true,
		Handler: func(ctx context.Context, c *artex.Client, a Args) (any, error) {
			q := url.Values{}
			if taskID := a.Str("task_id"); taskID != "" {
				q.Set("task", taskID)
				return getQ(ctx, c, "/api/exploration/findings", q)
			}
			q.Set("page", strconv.Itoa(a.Int("page", 1)))
			limit := a.Int("limit", 20)
			if limit <= 0 || limit > 200 {
				limit = 20
			}
			q.Set("limit", strconv.Itoa(limit))
			for _, k := range []string{"severity", "status", "vulnclass", "q", "sort"} {
				if v := a.Str(k); v != "" {
					q.Set(k, v)
				}
			}
			return getQ(ctx, c, "/api/exploration/findings", q)
		},
	})

	register(Tool{
		Name:        "artex_list_assets",
		Title:       "查询资产",
		Description: "查询 ARTEX 资产。可按任务、企业或资产类型过滤，也支持 ARTEX 搜索 DSL。",
		InputSchema: obj(map[string]any{
			"type":       str("资产类型（root_domain/subdomain/ip/port/site/endpoint 等）"),
			"task_id":    str("只查该任务的资产"),
			"company_id": str("只查该企业的资产"),
			"dsl":        str("ARTEX 资产搜索 DSL"),
			"limit":      integer("返回数量，默认 50，最大 500"),
			"offset":     integer("偏移量，默认 0"),
		}),
		ReadOnly: true,
		Handler: func(ctx context.Context, c *artex.Client, a Args) (any, error) {
			q := url.Values{}
			for _, k := range []string{"type", "task_id", "company_id", "dsl"} {
				if v := a.Str(k); v != "" {
					q.Set(k, v)
				}
			}
			limit := a.Int("limit", 50)
			if limit <= 0 || limit > 500 {
				limit = 50
			}
			q.Set("limit", strconv.Itoa(limit))
			q.Set("offset", strconv.Itoa(a.Int("offset", 0)))
			return getQ(ctx, c, "/api/assets", q)
		},
	})

	register(Tool{
		Name:        "artex_task_activity",
		Title:       "任务活动记录",
		Description: "读取任务某个会话的最近活动记录。session=main 是主 Agent 编排会话；limit 默认 50。",
		InputSchema: obj(map[string]any{
			"task_id": str("任务 ID"),
			"session": str("会话：main（默认）或 intent:<意图ID>"),
			"limit":   integer("条数，默认 50，最大 500"),
		}, "task_id"),
		ReadOnly: true,
		Handler: func(ctx context.Context, c *artex.Client, a Args) (any, error) {
			taskID, err := a.NeedStr("task_id")
			if err != nil {
				return nil, err
			}
			q := url.Values{}
			q.Set("task", taskID)
			sess := a.Str("session")
			if sess == "" {
				sess = "main"
			}
			q.Set("session", sess)
			limit := a.Int("limit", 50)
			if limit <= 0 || limit > 500 {
				limit = 50
			}
			q.Set("limit", strconv.Itoa(limit))
			return getQ(ctx, c, "/api/exploration/activity/history", q)
		},
	})

	register(Tool{
		Name:        "artex_send_task_message",
		Title:       "给任务主 Agent 发消息",
		Description: "向任务的主 Agent（人机接口）发送一条指令或问题，等同于在 ARTEX Web 的任务对话里发消息。wait_seconds>0 时等待其处理完并返回回复文本。",
		InputSchema: obj(map[string]any{
			"task_id":      str("任务 ID"),
			"message":      str("发给主 Agent 的消息"),
			"wait_seconds": integer("等待回复的秒数；0=只发送立即返回（默认 0），最大 300"),
		}, "task_id", "message"),
		Handler: sendTaskMessage,
	})

	register(Tool{
		Name:        "artex_ask",
		Title:       "与 ARTEX 助手对话",
		Description: "与 ARTEX 平台操作助手（Auto agent）对话：可用自然语言让它建任务、看进展、给提示、管理资产/skill/MCP。也可传 conversation_id 继续已有会话；message 留空则读取该会话最新回复。",
		InputSchema: obj(map[string]any{
			"message":         str("发给 ARTEX 助手的话；继续会话且只想读回复时可留空"),
			"conversation_id": integer("已有会话 id（可选）"),
			"agent_key":       str("agent：auto（平台操作，默认）或 pentest（独立渗透）"),
			"wait_seconds":    integer("等待回复秒数，默认 45，最大 300；0=只发送"),
		}),
		Handler: ask,
	})
}

func createTask(ctx context.Context, c *artex.Client, a Args) (any, error) {
	desc, err := a.NeedStr("description")
	if err != nil {
		return nil, err
	}
	goal, err := a.NeedStr("goal")
	if err != nil {
		return nil, err
	}
	body := map[string]any{"description": desc, "goal": goal}
	if v := a.Str("name"); v != "" {
		body["name"] = v
	}
	for _, k := range []string{"llm_profile_ids", "collection_ids", "company_ids"} {
		ids, err := a.Int64s(k)
		if err != nil {
			return nil, err
		}
		if len(ids) > 0 {
			body[k] = ids
		}
	}
	if a.Has("timeout_seconds") {
		body["timeout_seconds"] = a.Int("timeout_seconds", 0)
	}
	if a.Has("plan_heartbeat_seconds") {
		body["plan_heartbeat_seconds"] = a.Int("plan_heartbeat_seconds", 0)
	}
	if a.Has("seed_first_intent") {
		body["seed_first_intent"] = a.Bool("seed_first_intent", false)
	}
	if a.Has("coverage_enabled") {
		body["coverage_enabled"] = a.Bool("coverage_enabled", true)
	}
	out, err := post(ctx, c, "/api/tasks", body)
	if err != nil {
		return nil, err
	}
	return "任务已创建，ARTEX 引擎已在后台开始规划执行。\n" + jsonText(out), nil
}

func sendTaskMessage(ctx context.Context, c *artex.Client, a Args) (any, error) {
	taskID, err := a.NeedStr("task_id")
	if err != nil {
		return nil, err
	}
	msg, err := a.NeedStr("message")
	if err != nil {
		return nil, err
	}
	wait := clampWait(a.Int("wait_seconds", 0), 300)
	var out map[string]any
	path := "/api/chat?task=" + url.QueryEscape(taskID)
	if err := c.Do(ctx, http.MethodPost, path, map[string]any{"message": msg}, &out); err != nil {
		return nil, err
	}
	if reply, ok := out["reply"].(string); ok && reply != "" {
		return reply, nil
	}
	if wait == 0 {
		return "消息已发送给任务主 Agent，它会在后台处理。可稍后用 artex_task_activity(session=main) 或 artex_send_task_message(wait_seconds>0) 查看回复。", nil
	}
	if err := waitTaskChat(ctx, c, taskID, time.Duration(wait)*time.Second); err != nil {
		return nil, err
	}
	var hist struct {
		Items []convItem `json:"items"`
	}
	hp := "/api/exploration/activity/history?task=" + url.QueryEscape(taskID) + "&session=main&limit=30"
	if err := c.Do(ctx, http.MethodGet, hp, nil, &hist); err != nil {
		return nil, err
	}
	text := lastAssistantText(hist.Items)
	if text == "" {
		return "主 Agent 本轮已结束，但没有产生文本回复。可用 artex_task_activity 查看完整活动。", nil
	}
	return text, nil
}

func waitTaskChat(ctx context.Context, c *artex.Client, taskID string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		var st struct {
			Running bool `json:"running"`
		}
		p := idPath("/api/tasks", taskID) + "/chat/status"
		if err := c.Do(ctx, http.MethodGet, p, nil, &st); err != nil {
			return err
		}
		if !st.Running {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("等待超时（%s），主 Agent 仍在处理；稍后再查看", wait)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func clampWait(v, max int) int {
	if v < 0 {
		return 0
	}
	if v > max {
		return max
	}
	return v
}
