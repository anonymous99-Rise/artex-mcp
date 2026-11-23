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
		Name:  "artex_finding_admin",
		Title: "漏洞发现管理",
		Description: "对单条漏洞发现做读取、修改、深挖与复测。action 取值：\n" +
			"- get：读取详情\n" +
			"- stats / groups / asset_tree / export：按筛选条件统计、分组、资产树、导出\n" +
			"- lineage：发现的证据链\n" +
			"- patch：修改 status/severity/name/vulnclass\n" +
			"- delete：删除发现\n" +
			"- deepen：让 retester Agent 按 description 深挖\n" +
			"- retests：列出该发现的复测记录\n" +
			"- retest_start：发起复测（notes 可选）\n" +
			"- active_retests：列出所有进行中的复测\n" +
			"- frontier：查看探索前沿（按任务）",
		InputSchema: obj(map[string]any{
			"action":      actionProp("get", "stats", "groups", "asset_tree", "export", "lineage", "patch", "delete", "deepen", "retests", "retest_start", "active_retests", "frontier"),
			"finding_id":  integer("发现 id（多数 action 必填）"),
			"task_id":     str("任务 id（stats/frontier 过滤）"),
			"severity":    str("筛选/修改后的严重度"),
			"status":      str("筛选/修改后的状态"),
			"vulnclass":   str("筛选/修改后的漏洞类型"),
			"name":        str("修改后的名称"),
			"q":           str("关键字搜索"),
			"description": str("deepen 时给复测 Agent 的深挖说明"),
			"notes":       str("retest_start 的补充说明（最多 4000 字）"),
			"asset_scope": str("stats/groups 的资产范围过滤"),
			"limit":       integer("数量上限"),
		}, "action"),
		Handler: findingAdmin,
	})
}

func findingAdmin(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")
	fid := ""
	if a.Int("finding_id", 0) > 0 {
		fid = strconv.Itoa(a.Int("finding_id", 0))
	} else {
		fid = a.Str("finding_id")
	}
	needF := func() (string, error) {
		if fid == "" {
			return "", fmt.Errorf("该 action 需要 finding_id")
		}
		return idPath("/api/exploration/findings", fid), nil
	}

	filter := func() url.Values {
		q := url.Values{}
		for _, k := range []string{"task_id", "severity", "status", "vulnclass", "q", "asset_scope"} {
			if v := a.Str(k); v != "" {
				q.Set(k, v)
			}
		}
		if a.Int("limit", 0) > 0 {
			q.Set("limit", strconv.Itoa(a.Int("limit", 0)))
		}
		return q
	}

	switch action {
	case "get":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p)

	case "stats":
		return getQ(ctx, c, "/api/exploration/findings/stats", filter())

	case "groups":
		return getQ(ctx, c, "/api/exploration/findings/groups", filter())

	case "asset_tree":
		return getQ(ctx, c, "/api/exploration/findings/asset-tree", filter())

	case "export":
		return getQ(ctx, c, "/api/exploration/findings/export", filter())

	case "frontier":
		q := url.Values{}
		if v := a.Str("task_id"); v != "" {
			q.Set("task", v)
		}
		return getQ(ctx, c, "/api/exploration/frontier", q)

	case "lineage":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p+"/lineage")

	case "patch":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		body := map[string]any{}
		for _, k := range []string{"status", "severity", "name", "vulnclass"} {
			if a.Has(k) {
				body[k] = a.Str(k)
			}
		}
		if len(body) == 0 {
			return nil, fmt.Errorf("patch 需要 status/severity/name/vulnclass 中至少一项")
		}
		return patch(ctx, c, p, body)

	case "delete":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, p, nil)

	case "deepen":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		desc, err := a.NeedStr("description")
		if err != nil {
			return nil, err
		}
		return post(ctx, c, p+"/deepen", map[string]any{"description": desc})

	case "retests":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		return get(ctx, c, p+"/retests")

	case "retest_start":
		p, err := needF()
		if err != nil {
			return nil, err
		}
		return post(ctx, c, p+"/retests", map[string]any{"notes": a.Str("notes")})

	case "active_retests":
		return get(ctx, c, "/api/exploration/findings/retests/active")
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
