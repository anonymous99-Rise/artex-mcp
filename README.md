# artex-mcp

> 用自然语言控制 [ARTEX](https://github.com/Autumn-27/artex) 自主渗透平台。

[![Release](https://img.shields.io/github/v/release/anonymous99-Rise/artex-mcp?color=blue)](https://github.com/anonymous99-Rise/artex-mcp/releases)
[![CI](https://github.com/anonymous99-Rise/artex-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/anonymous99-Rise/artex-mcp/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.21%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Tools](https://img.shields.io/badge/MCP%20tools-31-green)](#工具一览)
[![Routes](https://img.shields.io/badge/API%20routes-251-orange)](#兜底入口)
[![License](https://img.shields.io/github/license/anonymous99-Rise/artex-mcp)](LICENSE)

`artex-mcp` 把 ARTEX 的 REST API 封装成一个 **MCP stdio 服务器**，让 Claude Code、Codex、Cursor 等支持 MCP 的客户端直接创建任务、观察进度、读取漏洞证据、干预 Agent 行为——不用打开 Web UI。

- **零第三方依赖**：纯 Go 标准库，单文件二进制，不修改 ARTEX 本体
- **31 个工具 + 251 条路由目录**：领域工具做语义化封装，`artex_api_request` 兜底任意端点
- **面向生产**：自动登录与 401 重登、结构化错误、输出截断、读写注解（`readOnlyHint` / `destructiveHint`）
- **工程化**：分层包结构、httptest 单元测试、`-race`、gofmt/vet CI、可复现构建

---

## 目录

- [快速开始](#快速开始)
- [它是怎么工作的](#它是怎么工作的)
- [配置](#配置)
- [注册到 MCP 客户端](#注册到-mcp-客户端)
- [工具一览](#工具一览)
- [使用示例](#使用示例)
- [设计原则](#设计原则)
- [开发与发布](#开发与发布)
- [安全说明](#安全说明)
- [更新日志](#更新日志)

## 快速开始

前置条件：ARTEX 已在本机运行（默认 `127.0.0.1:8787`），Go 1.21+。

```bash
git clone https://github.com/anonymous99-Rise/artex-mcp.git
cd artex-mcp
make build
./bin/artex-mcp -check        # 检查 ARTEX 连通性并完成一次登录
```

`-check` 输出示例：

```
ARTEX: http://127.0.0.1:8787
health: ok
login: ok (ARTEX)
tools: 31, routes: 251
```

写入凭据（**不要提交到仓库**）：

```bash
mkdir -p ~/.config/artex-mcp
cat > ~/.config/artex-mcp/config.json <<'JSON'
{
  "base_url": "http://127.0.0.1:8787",
  "username": "ARTEX",
  "password": "你的 ARTEX 管理员密码"
}
JSON
chmod 600 ~/.config/artex-mcp/config.json
```

注册并验证：

```bash
claude mcp add artex --scope user -- "$(pwd)/bin/artex-mcp"
claude mcp list        # 应显示 artex ✔ Connected
```

之后在 Claude Code 里直接说「列出 ARTEX 任务」「给我看高危漏洞的证据」即可。也可以从 [Releases](https://github.com/anonymous99-Rise/artex-mcp/releases) 下载对应平台的预编译二进制。

## 它是怎么工作的

```text
┌──────────────┐   stdio JSON-RPC 2.0   ┌──────────────┐   HTTP + JWT   ┌───────────────┐
│ MCP client   │ ◄────────────────────► │  artex-mcp   │ ◄────────────► │ ARTEX :8787   │
│ (Claude Code)│   tools/list, call     │              │  /api/...      │ Go + Next.js  │
└──────────────┘                        └──────────────┘                └───────────────┘
                                              │
                                              ├─ internal/config   配置/环境变量
                                              ├─ internal/artex    HTTP 客户端、自动登录、401 重登
                                              ├─ internal/mcp      JSON-RPC 分发、能力协商
                                              └─ internal/tools    31 个工具 + 内嵌路由目录
```

- 首次调用业务工具前自动 `POST /api/auth/login` 获取 JWT（默认 7 天）
- token 失效返回 401/403 时，用配置中的密码**重登一次并重放请求**
- 通知（`notifications/*`）不回包；协议版本按客户端请求回显，缺省 `2025-06-18`
- 工具结果统一为 `content: [{type: "text"}]`，超过 80 KB 安全截断（按 rune 边界）

## 配置

优先级：**命令行 flag > 环境变量 > 配置文件 > 默认值**。

| 方式 | 键 |
|---|---|
| flag | `-config` `-base` |
| 环境变量 | `ARTEX_URL` `ARTEX_USERNAME` `ARTEX_PASSWORD` `ARTEX_TOKEN` `ARTEX_MCP_CONFIG` |
| 配置文件 | 默认 `~/.config/artex-mcp/config.json` |

配置文件字段：`base_url`、`username`（默认 `ARTEX`）、`password`、`token`（可选，直接复用已有 JWT）。文件不存在时使用默认值，不会报错。

> 密码只保存在本地 0600 文件中；桥接层不会把凭据写入日志或工具输出，生产环境建议改用环境变量或密钥管理。

## 注册到 MCP 客户端

**Claude Code**

```bash
claude mcp add artex --scope user -- /绝对路径/bin/artex-mcp
```

**通用 JSON 配置**（Claude Desktop、Cursor、Codex 等）

```json
{
  "mcpServers": {
    "artex": {
      "command": "/绝对路径/bin/artex-mcp",
      "args": [],
      "env": {
        "ARTEX_URL": "http://127.0.0.1:8787"
      }
    }
  }
}
```

改完代码后重新 `make build`，并在客户端重连 MCP（Claude Code 用 `/mcp` 重连或重启会话）。

## 工具一览

### 概览

| 领域 | 工具 | 典型用途 |
|---|---|---|
| 任务 | `artex_list_tasks` `artex_get_task` `artex_create_task` `artex_control_task` `artex_task_stats` `artex_task_activity` `artex_send_task_message` `artex_task_admin` | 创建/暂停/恢复任务、改范围、发指令、归档 |
| 漏洞 | `artex_list_findings` `artex_finding_admin` | 查询、证据链、导出、深挖、复测 |
| 资产 | `artex_list_assets` `artex_assets_admin` `artex_companies` | 资产查询/写入、企业范围管理 |
| LLM | `artex_list_llm_profiles` `artex_llm` | 模型配置、连通性测试、档案切换、重试策略 |
| 对话 | `artex_ask` `artex_conversations` | 操作助手对话、会话管理 |
| 观测 | `artex_activity` `artex_traffic` | 日志、审计、token 用量、MITM 流量 |
| 编排 | `artex_goals` `artex_constraints` `artex_intents` | 目标/约束/意图的控制与干预 |
| 配置 | `artex_settings` `artex_mcp_servers` `artex_sync` `artex_workspace` `artex_skills` `artex_intercept` | 全局设置、ScopeSentry/FOFA、工作区、Skill、审批拦截 |
| 兜底 | `artex_health` `artex_find_routes` `artex_api_request` | 健康检查、路由检索、任意 API 调用 |

### 任务

| 工具 | 作用 |
|---|---|
| `artex_list_tasks` | 全部任务及状态、进度、发现数 |
| `artex_get_task` | 单任务详情 |
| `artex_create_task` | 创建任务（`description` + `goal`，可带 LLM 链、企业、资产合集、超时、种子意图） |
| `artex_control_task` | 批量暂停/恢复 |
| `artex_task_stats` | 运行统计 |
| `artex_task_activity` | 主会话 / 意图会话的活动记录 |
| `artex_send_task_message` | 给任务主 Agent 发指令（可选等待回复） |
| `artex_task_admin` | 改名/置顶/删除/设为活跃；范围规则；关联资产；任务级 LLM；分类；模板；覆盖度；归档（含批量恢复/删除） |

### 漏洞与资产

| 工具 | 作用 |
|---|---|
| `artex_list_findings` | 全局/按任务列出发现，支持严重度、状态、类型、关键字 |
| `artex_finding_admin` | 详情、统计、分组、资产树、导出、证据链、修改、删除、深挖、复测（发起/查询/进行中）、探索前沿 |
| `artex_list_assets` | 资产查询（类型/任务/企业/DSL） |
| `artex_assets_admin` | 类型计数、批量写入、按 id 删除、资产合集管理 |
| `artex_companies` | 企业列表/详情/创建/范围维护/重新归属/删除 |

### LLM 与对话

| 工具 | 作用 |
|---|---|
| `artex_list_llm_profiles` | 列出 LLM 配置档案 |
| `artex_llm` | 当前配置、修改、连通性测试、档案增删与激活、轮询状态、模型列表、重试策略 |
| `artex_ask` | 与平台操作助手对话（新建或继续会话，可等待回复） |
| `artex_conversations` | 会话列表/消息/发送/停止/改名/指定 LLM/删除/Agent 列表 |

### 可观测与配置

| 工具 | 作用 |
|---|---|
| `artex_activity` | 活动历史与详情、主会话、token 用量、服务日志、审计、工具调用、LLM 请求记录 |
| `artex_traffic` | MITM 流量列表/主机/单条请求响应/清理 |
| `artex_goals` / `artex_constraints` | 任务目标 / 约束的增删改查 |
| `artex_intents` | 意图列表、暂停/恢复/取消、给 worker 发消息、重跑 |
| `artex_settings` | 全局设置读取/部分更新 |
| `artex_mcp_servers` | ARTEX 内部挂载的 MCP 服务器管理 |
| `artex_sync` | ScopeSentry / FOFA 配置、预览、同步 |
| `artex_workspace` | 工作区文件读写 |
| `artex_skills` | Skill 列表/创建/文件读写/元数据/使用记录 |
| `artex_intercept` | 工具拦截规则、资产规则、任务级规则、待审批、审批历史、审批工具配置、LLM 兜底审批 |

### 兜底入口

| 工具 | 作用 |
|---|---|
| `artex_health` | 服务健康检查（无需登录） |
| `artex_find_routes` | 在 251 条路由目录中按路径/handler 搜索 |
| `artex_api_request` | 直接调用任意 `/api/` 端点（仅允许 `/api/` 前缀、拒绝未解析的 `{占位符}`，标注 destructive） |

## 使用示例

```text
你：帮我对 example.com 做一次授权渗透，重点是未授权访问和弱口令，
    先用我已有的企业资产，找到高危漏洞就停下来汇报。

Claude（自动执行）：
  1. artex_list_llm_profiles      确认模型已配置
  2. artex_companies(list)        找到目标企业 id
  3. artex_create_task            description=范围+约束, goal=要找的高危问题
  4. 轮询 artex_task_stats / artex_list_findings
  5. artex_finding_admin(get)     读取高危漏洞证据
```

```text
你：把目标范围收窄到 10.0.0.0/24，暂停其他任务，然后继续。

Claude：
  artex_task_admin(scope_add, kind="cidr", value="10.0.0.0/24")
  artex_control_task(action="pause", ...)
```

## 设计原则

1. **高层工具表达意图，兜底工具表达能力**
   常用操作有语义化工具和参数校验；冷门端点通过 `artex_find_routes` 检索后交给 `artex_api_request`，不为了覆盖率牺牲可读性。
2. **API 形状来自 ARTEX 源码，而不是猜测**
   `scripts/gen_routes.py` 扫描 `HandleFunc("METHOD /api/...")` 生成路由目录；每个领域工具的请求体都对照 ARTEX handler 的 struct 实现。
3. **只做桥接，不改 ARTEX**
   认证复用官方登录流程，升级 ARTEX 后只需 `make routes` 刷新目录。
4. **安全默认值**
   只读工具显式标注 `readOnlyHint`，破坏性操作标注 `destructiveHint`；凭据不落盘到仓库、不进日志。

## 开发与发布

```bash
make test          # httptest 假 ARTEX：登录、401 重登、分发、协议、路由
make vet
make fmt
make build
./bin/artex-mcp -list-tools | jq -r '.[].name'
make release       # 交叉编译 linux/darwin × amd64/arm64 并生成 checksums.txt
```

目录结构：

```text
cmd/artex-mcp/        进程入口（flag、配置、stdio 循环）
internal/config/      配置文件 + 环境变量
internal/artex/       ARTEX HTTP 客户端（登录、401 重试、JSON 解码）
internal/mcp/         MCP JSON-RPC 分发与能力协商
internal/tools/       工具注册表与各领域实现
  routes.json         从 ARTEX 源码生成的 API 路由目录（go:embed）
scripts/gen_routes.py 路由目录生成脚本
```

### 更新路由目录

ARTEX 升级后同步最新路由：

```bash
make routes ARTEX_SRC=/path/to/ARTEX-God
# 等价于 python3 -I scripts/gen_routes.py /path/to/ARTEX-God
```

## 安全说明

- ARTEX 是自主渗透平台，创建或恢复任务会对目标发起**真实测试**。只对已获得明确授权的目标使用。
- `artex_api_request` 可调用任何 API（包括删除类端点），应优先使用带领域约束的高层工具。
- 凭据只保存在本地 0600 文件中，不随工具输出返回；仓库内不含任何凭据。
- 默认连接 `127.0.0.1:8787`；若 ARTEX 暴露在网络中，请配合防火墙/反向代理与强密码。

## 更新日志

见 [CHANGELOG.md](CHANGELOG.md)。最新版本：[v1.0.0](https://github.com/anonymous99-Rise/artex-mcp/releases/tag/v1.0.0)。

## License

[MIT](LICENSE)
