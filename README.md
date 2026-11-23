# artex-mcp

把 [ARTEX](https://github.com/Autumn-27/artex) 自主渗透平台的 REST API 封装成 **MCP stdio 服务器**，让 Claude Code 等 MCP 客户端用自然语言直接创建、观察和控制 ARTEX 任务。

零第三方依赖（Go 标准库），不需要修改 ARTEX 本体。通过 ARTEX 自身的登录流程（`POST /api/auth/login`）获取 JWT，token 过期自动重新登录。

- **31 个工具**，覆盖任务、漏洞、资产、企业、LLM、对话、活动、流量、同步、Skill、审批拦截、系统设置
- **251 条 API 路由目录**（从 ARTEX 源码生成）+ `artex_api_request` 通用调用，未被封装的端点也能直接访问
- 所有工具带 MCP `annotations`（`readOnlyHint` / `destructiveHint`），便于客户端区分读写

---

## 安装

```bash
git clone https://github.com/<you>/artex-mcp.git
cd artex-mcp

make build          # 产物在 bin/artex-mcp
./bin/artex-mcp -check   # 检查 ARTEX 连通性与登录
```

要求 Go 1.21+。也可以直接：

```bash
go build -trimpath -ldflags "-s -w -X main.version=$(git describe --tags --always)" \
  -o bin/artex-mcp ./cmd/artex-mcp
```

## 配置

默认读取 `~/.config/artex-mcp/config.json`（可用 `-config` 或 `ARTEX_MCP_CONFIG` 指定）：

```json
{
  "base_url": "http://127.0.0.1:8787",
  "username": "ARTEX",
  "password": "你的 ARTEX 管理员密码",
  "token": ""
}
```

```bash
chmod 600 ~/.config/artex-mcp/config.json
```

优先级：命令行 `-base` > 环境变量 > 配置文件 > 默认值。环境变量：`ARTEX_URL`、`ARTEX_USERNAME`、`ARTEX_PASSWORD`、`ARTEX_TOKEN`、`ARTEX_MCP_CONFIG`。

> 不要把密码写进仓库、shell 历史或聊天记录。文件权限保持 0600，生产环境建议使用环境变量或密钥管理。

## 注册到 Claude Code

```bash
claude mcp add artex --scope user -- /绝对路径/bin/artex-mcp
claude mcp list    # 应显示 artex ✔ Connected
```

其他 MCP 客户端使用同样的 stdio 命令即可，例如：

```json
{
  "mcpServers": {
    "artex": {
      "command": "/绝对路径/bin/artex-mcp",
      "args": []
    }
  }
}
```

改完代码后重新 `make build`，并在客户端重连 MCP（Claude Code 中 `/mcp` 重连或重启会话）。

## 工具一览

### 任务

| 工具 | 作用 |
|---|---|
| `artex_list_tasks` | 全部任务及状态、进度、发现数 |
| `artex_get_task` | 单任务详情 |
| `artex_create_task` | 创建任务（`description` + `goal`，可带 LLM 链、企业、合集、超时、种子意图） |
| `artex_control_task` | 批量暂停/恢复 |
| `artex_task_stats` | 运行统计 |
| `artex_task_activity` | 主会话 / 意图会话的活动记录 |
| `artex_send_task_message` | 给任务主 Agent 发指令（可等待回复） |
| `artex_task_admin` | 改名/置顶/删除/设为活跃；范围规则；关联资产；任务 LLM；分类；模板；覆盖度；归档（含批量恢复/删除） |

### 漏洞与资产

| 工具 | 作用 |
|---|---|
| `artex_list_findings` | 全局/按任务列出发现，支持严重度、状态、类型、关键字 |
| `artex_finding_admin` | 详情、统计、分组、资产树、导出、证据链、修改、删除、深挖、复测（发起/查询/进行中）、探索前沿 |
| `artex_list_assets` | 资产查询（类型/任务/企业/DSL） |
| `artex_assets_admin` | 类型计数、批量写入、按 id 删除、资产合集 |
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

### 兜底

| 工具 | 作用 |
|---|---|
| `artex_health` | 服务健康检查（无需登录） |
| `artex_find_routes` | 在 251 条路由目录中按路径/handler 搜索 |
| `artex_api_request` | 直接调用任意 `/api/` 端点（全权入口，标注 destructive） |

## 典型对话

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

## 开发

```bash
make test          # 单元测试（httptest 假 ARTEX，覆盖登录、401 重登、工具分发、协议）
make vet
make fmt
make build
./bin/artex-mcp -list-tools | jq -r '.[].name'
```

目录结构：

```
cmd/artex-mcp/        进程入口（flag、配置、stdio 循环）
internal/config/      配置文件 + 环境变量
internal/artex/       ARTEX HTTP 客户端（登录、401 重试、JSON 解码）
internal/mcp/         MCP JSON-RPC 分发
internal/tools/       工具注册表与各领域工具实现
  routes.json         从 ARTEX 源码生成的 API 路由目录（go:embed）
scripts/gen_routes.py 路由目录生成脚本
```

### 更新路由目录

ARTEX 升级后，同步最新的 API 路由：

```bash
make routes ARTEX_SRC=/path/to/ARTEX-God
# 等价于 python3 scripts/gen_routes.py /path/to/ARTEX-God
```

## 安全说明

- ARTEX 是自主渗透平台，创建/恢复任务会对目标发起**真实测试**。只对已获得明确授权的目标使用。
- `artex_api_request` 可调用任何 API（包括删除类端点），默认应优先使用带领域约束的高层工具。
- 凭据只保存在本地 0600 文件中，不写入日志、不随工具输出返回。
- 桥接层默认连接 `127.0.0.1:8787`；如果 ARTEX 暴露在网络上，请配合防火墙/反向代理和强密码使用。

## License

[MIT](LICENSE)
