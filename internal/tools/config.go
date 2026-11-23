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
		Name:        "artex_mcp_servers",
		Title:       "ARTEX 内的 MCP 服务器",
		Description: "管理 ARTEX 自身挂载的 MCP 服务器（供 ARTEX 的 Agent 调用）。action 取值：list / save / delete / refresh（重新发现工具）/ tools（查看已缓存工具）。save 需要 server 对象：{id,name,transport(command|http),command,args,env,url,enabled,insecure}；id=0 为新增。",
		InputSchema: obj(map[string]any{
			"action":    actionProp("list", "save", "delete", "refresh", "tools"),
			"server_id": integer("MCP 服务器 id（delete/refresh/tools 必填）"),
			"server":    rawObj("save 的服务器对象"),
		}, "action"),
		Handler: mcpServersTool,
	})

	register(Tool{
		Name:  "artex_sync",
		Title: "外部数据源同步（ScopeSentry / FOFA）",
		Description: "配置并从外部资产平台同步资产。action 取值：\n" +
			"- scopesentry_status：连接与配置状态\n" +
			"- scopesentry_datasource：保存 ScopeSentry 地址与 API Key（url/api_key）\n" +
			"- scopesentry_projects：列出可同步项目（page/size/search）\n" +
			"- scopesentry_tasks：列出可同步任务（page/size/search）\n" +
			"- scopesentry_run：执行同步（dimension=project|task, targets 数组, asset_types, create_company, page_size）\n" +
			"- fofa_status / fofa_datasource / fofa_query / fofa_run：FOFA 状态、配置、预览、收集入合集",
		InputSchema: obj(map[string]any{
			"action":          actionProp("scopesentry_status", "scopesentry_datasource", "scopesentry_projects", "scopesentry_tasks", "scopesentry_run", "fofa_status", "fofa_datasource", "fofa_query", "fofa_run"),
			"url":             str("ScopeSentry 地址"),
			"api_key":         str("ScopeSentry API Key"),
			"email":           str("FOFA 邮箱"),
			"key":             str("FOFA API Key"),
			"query":           str("FOFA 查询语句"),
			"size":            integer("FOFA 结果数量"),
			"collection_name": str("FOFA 同步写入的合集名"),
			"dimension":       enum("project 或 task", "project", "task"),
			"targets":         strArr("ScopeSentry 项目 ObjectID 或任务名数组"),
			"asset_types":     strArr("同步的资产类型（默认 subdomain/service/app）"),
			"create_company":  boolean("是否顺手创建企业"),
			"page_size":       integer("每页数量"),
			"page":            integer("页码"),
			"search":          str("搜索关键字"),
		}, "action"),
		Handler: syncTool,
	})

	register(Tool{
		Name:        "artex_workspace",
		Title:       "工作区文件",
		Description: "读写 ARTEX 工作区（data 目录，Agent 共享文件区）。action 取值：list / read / write / mkdir / delete。path 为相对工作区根的路径。",
		InputSchema: obj(map[string]any{
			"action":  actionProp("list", "read", "write", "mkdir", "delete"),
			"path":    str("相对路径（list/read/delete 必填；write/mkdir 必填）"),
			"content": str("write 的文件内容"),
		}, "action"),
		Handler: workspaceTool,
	})

	register(Tool{
		Name:        "artex_skills",
		Title:       "Skill 管理",
		Description: "管理 ARTEX 的 skill（agentskills.io 结构）。action 取值：list / create / delete / files / read_file / write_file / delete_file / mkdir / meta / usage / missing。读写文件时 skill 为 skill 名，file 为 skill 内相对路径。",
		InputSchema: obj(map[string]any{
			"action":        actionProp("list", "create", "delete", "files", "read_file", "write_file", "delete_file", "mkdir", "meta", "usage", "missing"),
			"skill":         str("skill 名"),
			"file":          str("skill 内相对文件/目录路径"),
			"content":       str("write_file 的文件内容"),
			"description":   str("create/meta 的描述"),
			"license":       str("create/meta 的 license"),
			"compatibility": str("create/meta 的兼容性说明"),
			"instructions":  str("create 时的 SKILL.md 指令内容（可空，自动脚手架）"),
			"mcps":          strArr("该 skill 解锁的 MCP 服务器列表"),
		}, "action"),
		Handler: skillsTool,
	})
}

func mcpServersTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	needID := func() (string, error) {
		id := a.Int("server_id", 0)
		if id <= 0 {
			return "", fmt.Errorf("该 action 需要 server_id")
		}
		return strconv.Itoa(id), nil
	}
	switch action {
	case "list":
		return get(ctx, c, "/api/mcp")
	case "save":
		srv := a.Map("server")
		if srv == nil {
			return nil, fmt.Errorf("save 需要 server 对象")
		}
		return post(ctx, c, "/api/mcp", srv)
	case "delete":
		id, err := needID()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, "/api/mcp/"+id, nil)
	case "refresh":
		id, err := needID()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, "/api/mcp/"+id+"/refresh", nil)
	case "tools":
		id, err := needID()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, "/api/mcp/"+id+"/tools")
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}

func syncTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	page := func() url.Values {
		q := url.Values{}
		q.Set("page", strconv.Itoa(a.Int("page", 1)))
		q.Set("size", strconv.Itoa(a.Int("size", 50)))
		if v := a.Str("search"); v != "" {
			q.Set("search", v)
		}
		return q
	}
	switch action {
	case "scopesentry_status":
		return get(ctx, c, "/api/sync/scopesentry/status")
	case "scopesentry_datasource":
		return post(ctx, c, "/api/sync/scopesentry/datasource", map[string]any{
			"url": a.Str("url"), "api_key": a.Str("api_key"),
		})
	case "scopesentry_projects":
		return getQ(ctx, c, "/api/sync/scopesentry/projects", page())
	case "scopesentry_tasks":
		return getQ(ctx, c, "/api/sync/scopesentry/tasks", page())
	case "scopesentry_run":
		dim := a.Str("dimension")
		if dim != "project" && dim != "task" {
			return nil, fmt.Errorf("dimension 必须是 project 或 task")
		}
		targets, err := a.Strings("targets")
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return nil, fmt.Errorf("targets 不能为空")
		}
		types, err := a.Strings("asset_types")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"dimension": dim, "targets": targets, "create_company": a.Bool("create_company", false)}
		if len(types) > 0 {
			body["asset_types"] = types
		}
		if a.Int("page_size", 0) > 0 {
			body["page_size"] = a.Int("page_size", 0)
		}
		return post(ctx, c, "/api/sync/scopesentry/sync", body)

	case "fofa_status":
		return get(ctx, c, "/api/sync/fofa/status")
	case "fofa_datasource":
		return post(ctx, c, "/api/sync/fofa/datasource", map[string]any{
			"email": a.Str("email"), "key": a.Str("key"),
		})
	case "fofa_query":
		q, err := a.NeedStr("query")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"query": q}
		if a.Int("size", 0) > 0 {
			body["size"] = a.Int("size", 0)
		}
		return post(ctx, c, "/api/sync/fofa/query", body)
	case "fofa_run":
		q, err := a.NeedStr("query")
		if err != nil {
			return nil, err
		}
		body := map[string]any{"query": q}
		if a.Int("size", 0) > 0 {
			body["size"] = a.Int("size", 0)
		}
		if v := a.Str("collection_name"); v != "" {
			body["collection_name"] = v
		}
		if a.Int("page_size", 0) > 0 {
			body["page_size"] = a.Int("page_size", 0)
		}
		return post(ctx, c, "/api/sync/fofa/sync", body)
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}

func workspaceTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	path := a.Str("path")
	if path == "" && action != "list" {
		return nil, fmt.Errorf("%s 需要 path", action)
	}
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	switch action {
	case "list":
		return getQ(ctx, c, "/api/workspace/list", q)
	case "read":
		return getQ(ctx, c, "/api/workspace/read", q)
	case "write":
		return post(ctx, c, "/api/workspace/write", map[string]any{"path": path, "content": a.Str("content")})
	case "mkdir":
		return post(ctx, c, "/api/workspace/mkdir", map[string]any{"path": path})
	case "delete":
		return del(ctx, c, "/api/workspace/delete?"+q.Encode(), nil)
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}

func skillsTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	name := a.Str("skill")
	needName := func() (string, error) {
		if name == "" {
			return "", fmt.Errorf("该 action 需要 skill")
		}
		return name, nil
	}
	fileURL := func() string {
		return idPath("/api/skills", name) + "/files/" + a.Str("file")
	}
	switch action {
	case "list":
		return get(ctx, c, "/api/skills")
	case "missing":
		return get(ctx, c, "/api/skills/missing")
	case "create":
		n, err := needName()
		if err != nil {
			return nil, err
		}
		body := map[string]any{"name": n, "description": a.Str("description"), "instructions": a.Str("instructions")}
		if v := a.Str("license"); v != "" {
			body["license"] = v
		}
		if v := a.Str("compatibility"); v != "" {
			body["compatibility"] = v
		}
		if mcps, err := a.Strings("mcps"); err != nil {
			return nil, err
		} else if len(mcps) > 0 {
			body["mcps"] = mcps
		}
		return post(ctx, c, "/api/skills", body)
	case "delete":
		n, err := needName()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, idPath("/api/skills", n), nil)
	case "files":
		n, err := needName()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, idPath("/api/skills", n)+"/files")
	case "read_file":
		if _, err := needName(); err != nil {
			return nil, err
		}
		if a.Str("file") == "" {
			return nil, fmt.Errorf("read_file 需要 file")
		}
		return get(ctx, c, fileURL())
	case "write_file":
		if _, err := needName(); err != nil {
			return nil, err
		}
		if a.Str("file") == "" {
			return nil, fmt.Errorf("write_file 需要 file")
		}
		return put(ctx, c, fileURL(), map[string]any{"content": a.Str("content")})
	case "delete_file":
		if _, err := needName(); err != nil {
			return nil, err
		}
		if a.Str("file") == "" {
			return nil, fmt.Errorf("delete_file 需要 file")
		}
		return del(ctx, c, fileURL(), nil)
	case "mkdir":
		n, err := needName()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, idPath("/api/skills", n)+"/dirs", map[string]any{"path": a.Str("file")})
	case "meta":
		n, err := needName()
		if err != nil {
			return nil, err
		}
		body := map[string]any{}
		if a.Has("description") {
			body["description"] = a.Str("description")
		}
		if a.Has("license") {
			body["license"] = a.Str("license")
		}
		if a.Has("compatibility") {
			body["compatibility"] = a.Str("compatibility")
		}
		if mcps, err := a.Strings("mcps"); err != nil {
			return nil, err
		} else if mcps != nil {
			body["mcps"] = mcps
		}
		return put(ctx, c, idPath("/api/skills", n)+"/meta", body)
	case "usage":
		n, err := needName()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, idPath("/api/skills", n)+"/usage")
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
