# 更新日志

本项目遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 与 [语义化版本](https://semver.org/lang/zh-CN/)。

## [1.0.0] - 2026-11-23

首个正式版本。完整的 ARTEX MCP 桥接层，覆盖平台绝大部分管理面。

### 新增

- **MCP stdio 服务器**：JSON-RPC 2.0，支持 `initialize`、`tools/list`、`tools/call`、`ping`，协议版本按客户端回显（缺省 `2025-06-18`），通知类消息不回包。
- **31 个工具**：
  - 任务：`list_tasks` `get_task` `create_task` `control_task` `task_stats` `task_activity` `send_task_message` `task_admin`（30 个 action：生命周期、范围规则、资产关联、任务级 LLM、分类、模板、覆盖度、归档批量操作）
  - 漏洞：`list_findings` `finding_admin`（统计、分组、资产树、导出、证据链、深挖、复测、探索前沿）
  - 资产与企业：`list_assets` `assets_admin` `companies`
  - LLM 与对话：`list_llm_profiles` `llm` `ask` `conversations`
  - 观测：`activity` `traffic`
  - 编排：`goals` `constraints` `intents`
  - 配置：`settings` `mcp_servers` `sync` `workspace` `skills` `intercept`
  - 兜底：`health` `find_routes` `api_request`
- **路由目录**：从 ARTEX 源码扫描生成 251 条 API 路由，`go:embed` 打进二进制；`artex_find_routes` 检索，`artex_api_request` 通用调用（限制 `/api/` 前缀，拒绝未解析占位符）。
- **认证**：复用官方 `POST /api/auth/login`，JWT 过期遇 401/403 自动重登并重放请求。
- **工具注解**：所有工具带 `readOnlyHint`，非只读工具带 `destructiveHint`。
- **配置**：flag / 环境变量 / JSON 文件三级覆盖，凭据文件权限建议 0600。
- **CLI**：`-check`（连通性与登录自检）、`-list-tools`、`-version`。
- **工程化**：`cmd/` + `internal/{config,artex,mcp,tools}` 分层，零第三方依赖；httptest 假 ARTEX 覆盖登录、401 重登、工具分发、协议边界；GitHub Actions 执行 gofmt/vet/`go test -race`/构建。
- **发布**：`make release` 交叉编译 linux/darwin × amd64/arm64，生成 `checksums.txt`；推送 `main` 后由 GitHub Actions 自动做语义化版本自增、打 tag 并发布 Release。

### 安全

- 仓库不含任何凭据；密码仅存于本地 0600 配置或环境变量。
- `artex_api_request` 标注为 destructive，优先使用高层领域工具。

[1.0.0]: https://github.com/anonymous99-Rise/artex-mcp/releases/tag/v1.0.0
