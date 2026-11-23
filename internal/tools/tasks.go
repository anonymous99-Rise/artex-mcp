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
		Name:  "artex_task_admin",
		Title: "任务管理（生命周期/范围/资产/归档）",
		Description: "对任务做除创建/暂停外的管理操作。action 取值：\n" +
			"- update：改名称/置顶（PATCH /api/tasks/{id}）\n" +
			"- delete：删除任务\n" +
			"- set_active：设为当前活跃任务并启动引擎\n" +
			"- archives：列出任务归档（支持 q/state 过滤）\n" +
			"- archive / archive_batch：把任务归档\n" +
			"- archive_get / restore / archive_delete：按归档 id 查询、恢复、删除归档\n" +
			"- archive_restore_batch / archive_delete_batch：批量恢复/删除归档\n" +
			"- scope_list / scope_add / scope_delete：任务资产范围规则（kind 如 exact_domain/exact_ip/cidr/fuzzy_domain，value 为具体值）\n" +
			"- asset_refs / attach_assets / detach_assets：任务关联资产（attach 需要 asset_ids）\n" +
			"- set_llm：更新任务级 LLM 配置链\n" +
			"- set_category / categories / category_create / category_rename / category_delete / category_batch：任务分类\n" +
			"- templates / template_create / template_update / template_delete：任务模板\n" +
			"- coverage / coverage_graph：资产覆盖度",
		InputSchema: obj(map[string]any{
			"action":                actionProp("update", "delete", "set_active", "archives", "archive", "archive_batch", "archive_get", "restore", "archive_delete", "archive_restore_batch", "archive_delete_batch", "scope_list", "scope_add", "scope_delete", "asset_refs", "attach_assets", "detach_assets", "set_llm", "set_category", "categories", "category_create", "category_rename", "category_delete", "category_batch", "templates", "template_create", "template_update", "template_delete", "coverage", "coverage_graph"),
			"task_id":               str("任务 ID（多数 action 必填）"),
			"task_ids":              strArr("批量 action（archive_batch/category_batch）的任务 ID"),
			"name":                  str("update 时的新名称"),
			"pinned":                boolean("update 时是否置顶"),
			"q":                     str("archives 搜索关键字"),
			"state":                 str("archives 状态过滤"),
			"page":                  integer("archives 页码，默认 1"),
			"size":                  integer("archives 每页数量，默认 20"),
			"archive_id":            integer("归档 id"),
			"archive_ids":           intArr("批量归档 id（最多 100）"),
			"scope_id":              integer("scope_delete 时的范围规则 id"),
			"kind":                  str("scope_add 的范围类型（exact_domain/exact_ip/exact_url/fuzzy_domain/fuzzy_ip/fuzzy_url/cidr 等）"),
			"value":                 str("scope_add 的范围值"),
			"reason":                str("scope_add 的原因说明"),
			"asset_ids":             intArr("attach_assets/detach_assets 的资产 id"),
			"source_summary":        str("attach_assets 的来源说明"),
			"scope":                 str("attach_assets 的关联范围（如 snapshot）"),
			"llm_profile_ids":       intArr("set_llm 的 LLM 配置 id 有序链"),
			"active_llm_profile_id": integer("set_llm 当前生效的 LLM 配置 id"),
			"category_id":           integer("set_category/category_batch 的分类 id；0 表示清除"),
			"category_name":         str("category_create/category_rename 的名称"),
			"template_id":           integer("template_update/template_delete 的模板 id"),
			"template_body":         rawObj("template_create/template_update 的完整请求体"),
		}, "action"),
		Handler: taskAdmin,
	})
}

func taskAdmin(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	taskID := a.Str("task_id")

	needTask := func() (string, error) {
		if taskID == "" {
			return "", fmt.Errorf("该 action 需要 task_id")
		}
		return idPath("/api/tasks", taskID), nil
	}
	needID := func(k string) (string, error) {
		v := a.Str(k)
		if v == "" && a.Int(k, 0) > 0 {
			v = strconv.Itoa(a.Int(k, 0))
		}
		if v == "" {
			return "", fmt.Errorf("该 action 需要 %s", k)
		}
		return v, nil
	}

	switch action {
	case "update":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		body := map[string]any{}
		if a.Has("name") {
			body["name"] = a.Str("name")
		}
		if a.Has("pinned") {
			body["pinned"] = a.Bool("pinned", false)
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("update 需要 name 或 pinned")
		}
		return patch(ctx, c, p, body)

	case "delete":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, p, nil)

	case "set_active":
		id, err := needID("task_id")
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/active", map[string]any{"id": id})

	case "archives":
		q := url.Values{}
		if v := a.Str("q"); v != "" {
			q.Set("q", v)
		}
		if v := a.Str("state"); v != "" {
			q.Set("state", v)
		}
		q.Set("page", strconv.Itoa(a.Int("page", 1)))
		q.Set("size", strconv.Itoa(a.Int("size", 20)))
		return getQ(ctx, c, "/api/task-archives", q)

	case "archive":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, p+"/archive", nil)

	case "archive_batch":
		ids, err := a.Strings("task_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("task_ids 不能为空")
		}
		return post(ctx, c, "/api/tasks/archive/batch", map[string]any{"task_ids": ids})

	case "archive_get":
		id, err := needID("archive_id")
		if err != nil {
			return nil, err
		}
		return get(ctx, c, idPath("/api/task-archives", id))

	case "restore":
		id, err := needID("archive_id")
		if err != nil {
			return nil, err
		}
		return post(ctx, c, idPath("/api/task-archives", id)+"/restore", nil)

	case "archive_delete":
		id, err := needID("archive_id")
		if err != nil {
			return nil, err
		}
		return del(ctx, c, idPath("/api/task-archives", id), nil)

	case "archive_restore_batch", "archive_delete_batch":
		ids, err := a.Int64s("archive_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("archive_ids 不能为空")
		}
		verb := "restore"
		if action == "archive_delete_batch" {
			verb = "delete"
		}
		return post(ctx, c, "/api/task-archives/"+verb+"/batch", map[string]any{"archive_ids": ids})

	case "scope_list":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p+"/scope")

	case "scope_add":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		kind, err := a.NeedStr("kind")
		if err != nil {
			return nil, err
		}
		value, err := a.NeedStr("value")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"kind": kind, "value": value}
		if v := a.Str("reason"); v != "" {
			body["reason"] = v
		}
		return post(ctx, c, p+"/scope", body)

	case "scope_delete":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		sid, err := needID("scope_id")
		if err != nil {
			return nil, err
		}
		return del(ctx, c, p+"/scope/"+esc(sid), nil)

	case "asset_refs":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p+"/asset-refs")

	case "attach_assets":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		ids, err := a.Int64s("asset_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("asset_ids 不能为空")
		}
		body := map[string]any{"asset_ids": ids}
		if v := a.Str("source_summary"); v != "" {
			body["source_summary"] = v
		}
		if v := a.Str("scope"); v != "" {
			body["scope"] = v
		}
		return post(ctx, c, p+"/assets", body)

	case "detach_assets":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		ids, err := a.Int64s("asset_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("asset_ids 不能为空")
		}
		results := make([]any, 0, len(ids))
		for _, id := range ids {
			out, err := del(ctx, c, fmt.Sprintf("%s/assets/%d", p, id), nil)
			item := map[string]any{"asset_id": id}
			if err != nil {
				item["error"] = err.Error()
			} else {
				item["result"] = out
			}
			results = append(results, item)
		}
		return results, nil

	case "set_llm":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		body := map[string]any{}
		if ids, err := a.Int64s("llm_profile_ids"); err != nil {
			return nil, err
		} else if len(ids) > 0 {
			body["llm_profile_ids"] = ids
		}
		if a.Has("active_llm_profile_id") {
			body["active_llm_profile_id"] = a.Int("active_llm_profile_id", 0)
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("set_llm 需要 llm_profile_ids 或 active_llm_profile_id")
		}
		return put(ctx, c, p+"/llm", body)

	case "set_category":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		var v any
		if id := a.Int("category_id", 0); id > 0 {
			v = id
		}
		return patch(ctx, c, p+"/category", map[string]any{"category_id": v})

	case "categories":
		return get(ctx, c, "/api/task-categories")

	case "category_create":
		name, err := a.NeedStr("category_name")
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/task-categories", map[string]any{"name": name})

	case "category_rename":
		id, err := needID("category_id")
		if err != nil {
			return nil, err
		}
		name, err := a.NeedStr("category_name")
		if err != nil {
			return nil, err
		}
		return patch(ctx, c, idPath("/api/task-categories", id), map[string]any{"name": name})

	case "category_delete":
		id, err := needID("category_id")
		if err != nil {
			return nil, err
		}
		return del(ctx, c, idPath("/api/task-categories", id), nil)

	case "category_batch":
		ids, err := a.Strings("task_ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("task_ids 不能为空")
		}
		var category any
		if id := a.Int("category_id", 0); id > 0 {
			category = id
		}
		return post(ctx, c, "/api/tasks/category/batch", map[string]any{"task_ids": ids, "category_id": category})

	case "templates":
		return get(ctx, c, "/api/task-templates")

	case "template_create":
		body := a.Map("template_body")
		if body == nil {
			return nil, fmt.Errorf("template_create 需要 template_body")
		}
		return post(ctx, c, "/api/task-templates", body)

	case "template_update":
		id, err := needID("template_id")
		if err != nil {
			return nil, err
		}
		body := a.Map("template_body")
		if body == nil {
			return nil, fmt.Errorf("template_update 需要 template_body")
		}
		return patch(ctx, c, idPath("/api/task-templates", id), body)

	case "template_delete":
		id, err := needID("template_id")
		if err != nil {
			return nil, err
		}
		return del(ctx, c, idPath("/api/task-templates", id), nil)

	case "coverage":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p+"/coverage")

	case "coverage_graph":
		p, err := needTask()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p+"/coverage-graph")
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
