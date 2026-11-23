package tools

import (
	"context"
	"fmt"
	"strconv"

	"artex-mcp/internal/artex"
)

func init() {
	register(Tool{
		Name:  "artex_assets_admin",
		Title: "资产写入与合集",
		Description: "写入/删除资产、查看类型计数、管理 FOFA 资产合集。action 取值：\n" +
			"- counts：各类型资产数量\n" +
			"- insert：批量写入资产（assets 为 ARTEX 资产对象数组，type 决定字段，如 root_domain/ip/service/app）\n" +
			"- delete：按 id 删除资产\n" +
			"- collections：列出资产合集\n" +
			"- collection_delete：删除合集（级联清理成员与该合集的任务引用）",
		InputSchema: obj(map[string]any{
			"action":        actionProp("counts", "insert", "delete", "collections", "collection_delete"),
			"task_id":       str("insert 时归属任务 id（可选）"),
			"assets":        map[string]any{"type": "array", "description": "insert 的资产对象数组", "items": map[string]any{"type": "object", "additionalProperties": true}},
			"ids":           intArr("delete 的资产 id 数组"),
			"collection_id": integer("collection_delete 的合集 id"),
		}, "action"),
		Handler: assetsAdmin,
	})

	register(Tool{
		Name:  "artex_companies",
		Title: "企业管理",
		Description: "管理企业及其资产范围。action 取值：\n" +
			"- list：列出企业\n" +
			"- get：读取企业详情与范围（需要 company_id）\n" +
			"- create：创建企业（name 必填，scope 为 [{kind,value}] 数组）\n" +
			"- scope_add：追加/重置企业范围（scope 数组；reset=true 表示整体替换）\n" +
			"- reattribute：按当前范围规则重新计算资产归属\n" +
			"- delete：删除企业（delete_assets=true 同时删除其资产）",
		InputSchema: obj(map[string]any{
			"action":     actionProp("list", "get", "create", "scope_add", "reattribute", "delete"),
			"company_id": integer("企业 id"),
			"name":       str("create 时的企业名称"),
			"logo":       str("create 时的 logo URL"),
			"scope": map[string]any{
				"type":        "array",
				"description": "范围规则数组，每项 {kind,value}；kind 如 exact_domain/fuzzy_domain/exact_ip/cidr",
				"items":       map[string]any{"type": "object", "properties": map[string]any{"kind": str("范围类型"), "value": str("值")}, "additionalProperties": false},
			},
			"reason":        str("scope_add 的原因说明"),
			"reset":         boolean("scope_add 时是否整体替换现有范围"),
			"delete_assets": boolean("delete 时是否同时删除企业资产"),
		}, "action"),
		Handler: companies,
	})
}

func assetsAdmin(ctx context.Context, c *artex.Client, a Args) (any, error) {
	switch a.Str("action") {
	case "counts":
		return get(ctx, c, "/api/assets/counts")

	case "insert":
		assets, ok := a["assets"].([]any)
		if !ok || len(assets) == 0 {
			return nil, fmt.Errorf("insert 需要非空 assets 数组")
		}
		body := map[string]any{"assets": assets}
		if a.Int("task_id", 0) > 0 {
			body["task_id"] = a.Int("task_id", 0)
		}
		return post(ctx, c, "/api/assets", body)

	case "delete":
		ids, err := a.Int64s("ids")
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, fmt.Errorf("ids 不能为空")
		}
		return del(ctx, c, "/api/assets", map[string]any{"ids": ids})

	case "collections":
		return get(ctx, c, "/api/collections")

	case "collection_delete":
		id := a.Int("collection_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("collection_delete 需要 collection_id")
		}
		return del(ctx, c, "/api/collections/"+strconv.Itoa(id), nil)
	}
	return nil, fmt.Errorf("未知 action: %s", a.Str("action"))
}

func companies(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	needID := func() (string, error) {
		id := a.Int("company_id", 0)
		if id <= 0 {
			return "", fmt.Errorf("该 action 需要 company_id")
		}
		return strconv.Itoa(id), nil
	}
	scope := func() []any {
		if v, ok := a["scope"].([]any); ok {
			return v
		}
		return nil
	}

	switch action {
	case "list":
		return get(ctx, c, "/api/companies")

	case "get":
		id, err := needID()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, idPath("/api/companies", id))

	case "create":
		name, err := a.NeedStr("name")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"name": name}
		if s := a.Str("logo"); s != "" {
			body["logo"] = s
		}
		if sc := scope(); sc != nil {
			body["scope"] = sc
		}
		return post(ctx, c, "/api/companies", body)

	case "scope_add":
		id, err := needID()
		if err != nil {
			return nil, err
		}
		body := map[string]any{"scope": scope(), "reset": a.Bool("reset", false)}
		if v := a.Str("reason"); v != "" {
			body["reason"] = v
		}
		return post(ctx, c, idPath("/api/companies", id)+"/scope", body)

	case "reattribute":
		return post(ctx, c, "/api/companies/reattribute", nil)

	case "delete":
		id, err := needID()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, idPath("/api/companies", id), map[string]any{"delete_assets": a.Bool("delete_assets", false)})
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
