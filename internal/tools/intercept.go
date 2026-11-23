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
		Name:  "artex_intercept",
		Title: "人工审批与拦截规则",
		Description: "管理 ARTEX 的工具调用拦截/审批。action 取值：\n" +
			"- rules_list / rule_create / rule_update / rule_delete / rule_toggle：全局拦截规则\n" +
			"- asset_rules_list / asset_rule_create / asset_rule_update / asset_rule_delete / asset_rule_toggle：全局资产规则\n" +
			"- task_rules / task_rule_create / task_rule_update / task_rule_delete / task_rule_toggle：任务级资产规则（需要 task_id）\n" +
			"- pending / pending_get：待审批调用；decide 需要 pending_id 与 decision=allowed|denied\n" +
			"- history / history_get / history_execution：审批历史\n" +
			"- task_items：某任务的拦截记录\n" +
			"- tool_config / tool_config_set：哪些工具需要人工审批\n" +
			"- judge / judge_set / judge_usage：LLM 兜底审批配置与用量\n" +
			"规则创建/更新用 rule 对象（全局规则字段：name,enabled,priority,match_target,match_type,pattern,action,message,timeout_enabled,timeout_seconds,timeout_action；任务/资产规则字段：action,kind,pattern,note,enabled）。",
		InputSchema: obj(map[string]any{
			"action":        actionProp("rules_list", "rule_create", "rule_update", "rule_delete", "rule_toggle", "asset_rules_list", "asset_rule_create", "asset_rule_update", "asset_rule_delete", "asset_rule_toggle", "task_rules", "task_rule_create", "task_rule_update", "task_rule_delete", "task_rule_toggle", "pending", "pending_get", "decide", "history", "history_get", "history_execution", "task_items", "tool_config", "tool_config_set", "judge", "judge_set", "judge_usage"),
			"rule_id":       integer("规则 id"),
			"pending_id":    integer("待审批 id"),
			"decision":      enum("allowed 或 denied", "allowed", "denied"),
			"task_id":       str("任务 id"),
			"history_id":    integer("审批历史 id"),
			"rule":          rawObj("规则对象"),
			"enabled_tools": strArr("tool_config_set 时启用审批的工具列表"),
			"judge":         rawObj("judge_set 的兜底审批配置对象"),
			"page":          integer("页码"),
			"size":          integer("每页数量"),
			"q":             str("搜索关键字"),
		}, "action"),
		Handler: interceptTool,
	})
}

func interceptTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	ruleID := func() (string, error) {
		id := a.Int("rule_id", 0)
		if id <= 0 {
			return "", fmt.Errorf("该 action 需要 rule_id")
		}
		return strconv.Itoa(id), nil
	}
	paging := func() url.Values {
		q := url.Values{}
		if a.Int("page", 0) > 0 {
			q.Set("page", strconv.Itoa(a.Int("page", 0)))
		}
		if a.Int("size", 0) > 0 {
			q.Set("size", strconv.Itoa(a.Int("size", 0)))
		}
		if v := a.Str("q"); v != "" {
			q.Set("q", v)
		}
		return q
	}
	ruleBody := func() (map[string]any, error) {
		r := a.Map("rule")
		if r == nil {
			return nil, fmt.Errorf("该 action 需要 rule 对象")
		}
		return r, nil
	}
	taskBase := func() (string, error) {
		t := a.Str("task_id")
		if t == "" {
			return "", fmt.Errorf("该 action 需要 task_id")
		}
		return idPath("/api/tasks", t) + "/intercept-rules", nil
	}

	switch action {
	// ---- global tool-call rules ----
	case "rules_list":
		return get(ctx, c, "/api/intercept/rules")
	case "rule_create":
		body, err := ruleBody()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/intercept/rules", body)
	case "rule_update":
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		body, err := ruleBody()
		if err != nil {
			return nil, err
		}
		return put(ctx, c, "/api/intercept/rules/"+id, body)
	case "rule_delete":
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, "/api/intercept/rules/"+id, nil)
	case "rule_toggle":
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/intercept/rules/"+id+"/toggle", nil)

	// ---- global asset rules ----
	case "asset_rules_list":
		return get(ctx, c, "/api/asset-intercept/rules")
	case "asset_rule_create":
		body, err := ruleBody()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/asset-intercept/rules", body)
	case "asset_rule_update":
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		body, err := ruleBody()
		if err != nil {
			return nil, err
		}
		return put(ctx, c, "/api/asset-intercept/rules/"+id, body)
	case "asset_rule_delete":
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, "/api/asset-intercept/rules/"+id, nil)
	case "asset_rule_toggle":
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/asset-intercept/rules/"+id+"/toggle", nil)

	// ---- task-level rules ----
	case "task_rules":
		base, err := taskBase()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, base)
	case "task_rule_create":
		base, err := taskBase()
		if err != nil {
			return nil, err
		}
		body, err := ruleBody()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, base, body)
	case "task_rule_update":
		base, err := taskBase()
		if err != nil {
			return nil, err
		}
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		body, err := ruleBody()
		if err != nil {
			return nil, err
		}
		return put(ctx, c, base+"/"+id, body)
	case "task_rule_delete":
		base, err := taskBase()
		if err != nil {
			return nil, err
		}
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, base+"/"+id, nil)
	case "task_rule_toggle":
		base, err := taskBase()
		if err != nil {
			return nil, err
		}
		id, err := ruleID()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, base+"/"+id+"/toggle", nil)

	// ---- approvals ----
	case "pending":
		return get(ctx, c, "/api/intercept/pending")
	case "pending_get":
		id := a.Int("pending_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("pending_get 需要 pending_id")
		}
		return get(ctx, c, "/api/intercept/pending/"+strconv.Itoa(id))
	case "decide":
		id := a.Int("pending_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("decide 需要 pending_id")
		}
		decision := a.Str("decision")
		if decision != "allowed" && decision != "denied" {
			return nil, fmt.Errorf("decision 必须是 allowed 或 denied")
		}
		return post(ctx, c, fmt.Sprintf("/api/intercept/pending/%d/decide", id), map[string]any{"decision": decision})
	case "history":
		return getQ(ctx, c, "/api/intercept/history", paging())
	case "history_get":
		id := a.Int("history_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("history_get 需要 history_id")
		}
		return get(ctx, c, "/api/intercept/history/"+strconv.Itoa(id))
	case "history_execution":
		id := a.Int("history_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("history_execution 需要 history_id")
		}
		return get(ctx, c, fmt.Sprintf("/api/intercept/history/%d/execution", id))
	case "task_items":
		t := a.Str("task_id")
		if t == "" {
			return nil, fmt.Errorf("task_items 需要 task_id")
		}
		return getQ(ctx, c, "/api/intercept/task/"+esc(t), paging())

	// ---- config ----
	case "tool_config":
		return get(ctx, c, "/api/intercept/tool-config")
	case "tool_config_set":
		tools, err := a.Strings("enabled_tools")
		if err != nil {
			return nil, err
		}
		if tools == nil {
			tools = []string{}
		}
		return put(ctx, c, "/api/intercept/tool-config", map[string]any{"enabled_tools": tools})
	case "judge":
		return get(ctx, c, "/api/intercept/judge")
	case "judge_set":
		body := a.Map("judge")
		if body == nil {
			return nil, fmt.Errorf("judge_set 需要 judge 对象")
		}
		return put(ctx, c, "/api/intercept/judge", body)
	case "judge_usage":
		return get(ctx, c, "/api/intercept/judge/usage")
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
