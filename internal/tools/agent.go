package tools

import (
	"context"
	"fmt"

	"artex-mcp/internal/artex"
)

func init() {
	register(Tool{
		Name:        "artex_goals",
		Title:       "任务目标",
		Description: "管理任务的目标（goal）。action 取值：list / add / edit / delete。add 需要 text（目标文本）与可选 vulnclass；edit/delete 需要 goal_id。新增或修改目标会复活已结束的任务。",
		InputSchema: obj(map[string]any{
			"action":    actionProp("list", "add", "edit", "delete"),
			"task_id":   str("任务 ID（必填）"),
			"goal_id":   integer("目标 id（edit/delete 必填）"),
			"text":      str("目标文本"),
			"vulnclass": str("漏洞类型（可选）"),
		}, "action", "task_id"),
		Handler: goalsTool,
	})

	register(Tool{
		Name:        "artex_constraints",
		Title:       "任务约束",
		Description: "管理任务的操作约束。action 取值：list / add / edit / delete。add 需要 text，kind=allow|deny；edit/delete 需要 constraint_id。",
		InputSchema: obj(map[string]any{
			"action":        actionProp("list", "add", "edit", "delete"),
			"task_id":       str("任务 ID（必填）"),
			"constraint_id": integer("约束 id（edit/delete 必填）"),
			"text":          str("约束文本"),
			"kind":          enum("allow=允许，deny=禁止", "allow", "deny"),
		}, "action", "task_id"),
		Handler: constraintsTool,
	})

	register(Tool{
		Name:  "artex_intents",
		Title: "意图管理",
		Description: "查看与干预任务的执行意图（intent）。action 取值：\n" +
			"- list：列出任务意图（task_id 可选，省略=全部）\n" +
			"- control：暂停/恢复/取消单条意图（task_id/intent_id/control_action= pause|resume|cancel，可带 reason 与 mode=soft|hard）\n" +
			"- message：给某条意图的 worker 发消息（message）\n" +
			"- rerun：重跑单条 blocked/exhausted/stopped 意图\n" +
			"- rerun_blocked：批量重跑本任务全部 blocked 意图",
		InputSchema: obj(map[string]any{
			"action":         actionProp("list", "control", "message", "rerun", "rerun_blocked"),
			"task_id":        str("任务 ID（除 list 外必填）"),
			"intent_id":      str("意图 ID（control/message/rerun 必填）"),
			"control_action": enum("控制动作", "pause", "resume", "cancel"),
			"reason":         str("控制原因"),
			"mode":           enum("soft=保留当前步骤，hard=立即中断", "soft", "hard"),
			"message":        str("发给 worker 的消息"),
			"request_id":     str("幂等请求 id（可选）"),
			"worker":         str("list 时按 worker 过滤"),
			"status":         str("list 时按状态过滤"),
			"limit":          integer("list 数量上限"),
		}, "action"),
		Handler: intentsTool,
	})

	register(Tool{
		Name:        "artex_settings",
		Title:       "全局设置",
		Description: "读取或修改 ARTEX 全局设置。action=get 返回当前设置；action=update 用 settings 对象做部分更新（如 workers、task_concurrency_*、traffic_capture、global_proxy、web_search_*、notify_* 等）。",
		InputSchema: obj(map[string]any{
			"action":   actionProp("get", "update"),
			"settings": rawObj("update 时提交的设置字段对象（只传要改的字段）"),
		}, "action"),
		Handler: settingsTool,
	})
}

func goalsTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	taskID, err := a.NeedStr("task_id")
	if err != nil {
		return nil, err
	}
	base := idPath("/api/tasks", taskID) + "/goals"
	switch a.Str("action") {
	case "list":
		return get(ctx, c, base)
	case "add":
		text, err := a.NeedStr("text")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"text": text}
		if v := a.Str("vulnclass"); v != "" {
			body["vulnclass"] = v
		}
		return post(ctx, c, base, body)
	case "edit":
		id := a.Int("goal_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("edit 需要 goal_id")
		}
		body := map[string]any{}
		if a.Has("text") {
			body["text"] = a.Str("text")
		}
		if a.Has("vulnclass") {
			body["vulnclass"] = a.Str("vulnclass")
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("edit 需要 text 或 vulnclass")
		}
		return patch(ctx, c, fmt.Sprintf("%s/%d", base, id), body)
	case "delete":
		id := a.Int("goal_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("delete 需要 goal_id")
		}
		return del(ctx, c, fmt.Sprintf("%s/%d", base, id), nil)
	}
	return nil, fmt.Errorf("未知 action: %s", a.Str("action"))
}

func constraintsTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	taskID, err := a.NeedStr("task_id")
	if err != nil {
		return nil, err
	}
	base := idPath("/api/tasks", taskID) + "/constraints"
	switch a.Str("action") {
	case "list":
		return get(ctx, c, base)
	case "add":
		text, err := a.NeedStr("text")
		if err != nil {
			return nil, err
		}
		kind := a.Str("kind")
		if kind == "" {
			kind = "deny"
		}
		return post(ctx, c, base, map[string]any{"text": text, "kind": kind})
	case "edit":
		id := a.Int("constraint_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("edit 需要 constraint_id")
		}
		body := map[string]any{}
		if a.Has("text") {
			body["text"] = a.Str("text")
		}
		if v := a.Str("kind"); v != "" {
			body["kind"] = v
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("edit 需要 text 或 kind")
		}
		return patch(ctx, c, fmt.Sprintf("%s/%d", base, id), body)
	case "delete":
		id := a.Int("constraint_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("delete 需要 constraint_id")
		}
		return del(ctx, c, fmt.Sprintf("%s/%d", base, id), nil)
	}
	return nil, fmt.Errorf("未知 action: %s", a.Str("action"))
}

func intentsTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	taskID := a.Str("task_id")

	needTask := func() (string, error) {
		if taskID == "" {
			return "", fmt.Errorf("该 action 需要 task_id")
		}
		return taskID, nil
	}
	needIntent := func() (string, error) {
		v := a.Str("intent_id")
		if v == "" {
			return "", fmt.Errorf("该 action 需要 intent_id")
		}
		return v, nil
	}

	switch action {
	case "list":
		q := map[string]string{}
		if taskID != "" {
			q["task"] = taskID
		}
		if v := a.Str("worker"); v != "" {
			q["worker"] = v
		}
		if v := a.Str("status"); v != "" {
			q["status"] = v
		}
		if a.Int("limit", 0) > 0 {
			q["limit"] = fmt.Sprint(a.Int("limit", 0))
		}
		vals := values(q)
		return getQ(ctx, c, "/api/exploration/intents", vals)

	case "control":
		t, err := needTask()
		if err != nil {
			return nil, err
		}
		iid, err := needIntent()
		if err != nil {
			return nil, err
		}
		ca := a.Str("control_action")
		if ca != "pause" && ca != "resume" && ca != "cancel" {
			return nil, fmt.Errorf("control_action 必须是 pause/resume/cancel")
		}
		body := map[string]any{"action": ca}
		if v := a.Str("reason"); v != "" {
			body["reason"] = v
		}
		if v := a.Str("mode"); v != "" {
			body["mode"] = v
		}
		return post(ctx, c, fmt.Sprintf("/api/tasks/%s/intents/%s/control", esc(t), esc(iid)), body)

	case "message":
		t, err := needTask()
		if err != nil {
			return nil, err
		}
		iid, err := needIntent()
		if err != nil {
			return nil, err
		}
		msg, err := a.NeedStr("message")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"message": msg}
		if v := a.Str("request_id"); v != "" {
			body["request_id"] = v
		}
		return post(ctx, c, fmt.Sprintf("/api/tasks/%s/intents/%s/messages", esc(t), esc(iid)), body)

	case "rerun":
		t, err := needTask()
		if err != nil {
			return nil, err
		}
		iid, err := needIntent()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, fmt.Sprintf("/api/tasks/%s/intents/%s/rerun", esc(t), esc(iid)), nil)

	case "rerun_blocked":
		t, err := needTask()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, fmt.Sprintf("/api/tasks/%s/intents/rerun-blocked", esc(t)), nil)
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}

func settingsTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	switch a.Str("action") {
	case "get":
		return get(ctx, c, "/api/settings")
	case "update":
		body := a.Map("settings")
		if body == nil {
			return nil, fmt.Errorf("update 需要 settings 对象")
		}
		return put(ctx, c, "/api/settings", body)
	}
	return nil, fmt.Errorf("未知 action: %s", a.Str("action"))
}
