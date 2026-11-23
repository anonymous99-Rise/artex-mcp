package tools

import (
	"context"
	"fmt"
	"strconv"

	"artex-mcp/internal/artex"
)

func init() {
	register(Tool{
		Name:  "artex_llm",
		Title: "LLM 配置与可用性",
		Description: "读取/修改 ARTEX 的 LLM 设置与配置档案（profile）。action 取值：\n" +
			"- get：当前运行配置（GET /api/llm）\n" +
			"- set：修改运行配置（provider/model/base_url/api_key/proxy 等，直接传对应字段）\n" +
			"- test：测试一份配置能否连通（字段同 set）\n" +
			"- profiles：列出配置档案\n" +
			"- profile_save：新增/更新档案（必须提供 profile 对象；id=0 为新增）\n" +
			"- profile_delete：删除档案（profile_id）\n" +
			"- profile_activate：把某档案设为全局生效（profile_id）\n" +
			"- pool：LLM 轮询/熔断状态\n" +
			"- pool_reset：重置轮询与熔断\n" +
			"- models：拉取可用模型列表（provider/base_url/api_key 或 profile_id）\n" +
			"- retry_policy / retry_policy_set：读取/写入重试策略",
		InputSchema: obj(map[string]any{
			"action":           actionProp("get", "set", "test", "profiles", "profile_save", "profile_delete", "profile_activate", "pool", "pool_reset", "models", "retry_policy", "retry_policy_set"),
			"provider":         str("anthropic | openai | openai-responses"),
			"model":            str("模型名"),
			"base_url":         str("API 基地址（anthropic 不要带 /v1/messages）"),
			"api_key":          str("API Key"),
			"proxy":            str("请求代理，如 http://127.0.0.1:7890"),
			"rate_per_second":  integer("每秒请求上限"),
			"rate_per_minute":  integer("每分钟请求上限"),
			"context_window_k": integer("上下文窗口（千 token）"),
			"thinking_type":    str("思考模式（如 enabled/disabled）"),
			"reasoning_effort": str("推理强度（low/medium/high）"),
			"profile_id":       integer("档案 id"),
			"profile":          rawObj("profile_save 的完整档案对象（id/name/format/base_url/model/api_key/priority/...）"),
			"policy":           rawObj("retry_policy_set 的重试策略对象"),
		}, "action"),
		Handler: llmTool,
	})
}

func llmTool(ctx context.Context, c *artex.Client, a Args) (any, error) {
	action := a.Str("action")

	settings := func() map[string]any {
		body := map[string]any{}
		for _, k := range []string{"provider", "model", "base_url", "api_key", "proxy", "thinking_type", "reasoning_effort"} {
			if a.Has(k) {
				body[k] = a.Str(k)
			}
		}
		for _, k := range []string{"rate_per_second", "rate_per_minute", "context_window_k"} {
			if a.Has(k) {
				body[k] = a.Int(k, 0)
			}
		}
		return body
	}

	needProfile := func() (string, error) {
		id := a.Int("profile_id", 0)
		if id <= 0 {
			return "", fmt.Errorf("该 action 需要 profile_id")
		}
		return strconv.Itoa(id), nil
	}

	switch action {
	case "get":
		return get(ctx, c, "/api/llm")

	case "set":
		return post(ctx, c, "/api/llm", settings())

	case "test":
		return post(ctx, c, "/api/llm/test", settings())

	case "profiles":
		return get(ctx, c, "/api/llm/profiles")

	case "profile_save":
		p := a.Map("profile")
		if p == nil {
			return nil, fmt.Errorf("profile_save 需要 profile 对象")
		}
		return post(ctx, c, "/api/llm/profiles", p)

	case "profile_delete":
		id, err := needProfile()
		if err != nil {
			return nil, err
		}
		return del(ctx, c, "/api/llm/profiles/"+id, nil)

	case "profile_activate":
		id := a.Int("profile_id", 0)
		if id <= 0 {
			return nil, fmt.Errorf("profile_activate 需要 profile_id")
		}
		return post(ctx, c, "/api/llm/profiles/active", map[string]any{"id": id})

	case "pool":
		return get(ctx, c, "/api/llm/pool")

	case "pool_reset":
		return post(ctx, c, "/api/llm/pool/reset", nil)

	case "models":
		body := map[string]any{}
		for _, k := range []string{"provider", "base_url", "api_key", "proxy"} {
			if v := a.Str(k); v != "" {
				body[k] = v
			}
		}
		if id := a.Int("profile_id", 0); id > 0 {
			body["profile_id"] = id
		}
		return post(ctx, c, "/api/llm/models", body)

	case "retry_policy":
		return get(ctx, c, "/api/llm/retry-policy")

	case "retry_policy_set":
		p := a.Map("policy")
		if p == nil {
			return nil, fmt.Errorf("retry_policy_set 需要 policy 对象")
		}
		return post(ctx, c, "/api/llm/retry-policy", p)
	}
	return nil, fmt.Errorf("未知 action: %s", action)
}
