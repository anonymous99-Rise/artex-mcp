package mcp

const instructions = `这些工具用于操控本机已部署的 ARTEX 自动化渗透平台（自主规划、执行、留证的 Agent 系统）。

典型工作流：
1. artex_health 确认服务在线；artex_llm profiles 确认 LLM 已配置（若无配置，用 artex_llm 的 set/profile_save 写入）。
2. 用户用自然语言描述渗透需求时，先整理成 description（授权范围、目标背景、约束）和 goal（最终要拿到什么），再调用 artex_create_task。可先用 artex_list_assets / artex_companies / artex_assets_admin(collections) 确认已有资产或企业，用 company_ids / collection_ids 关联。
3. 任务由服务端异步执行。用 artex_list_tasks / artex_get_task / artex_task_stats / artex_list_findings 跟踪；artex_task_activity 看 Agent 实际动作；artex_traffic 看 MITM 抓到的请求。
4. 需要干预时：artex_control_task 暂停/恢复，artex_send_task_message 给主 Agent 下指令，artex_intents 控制单条意图，artex_goals/artex_constraints 调整目标与约束。
5. 漏洞发现用 artex_list_findings 过滤，artex_finding_admin 读取详情、修改状态、深挖（deepen）或发起复测（retest_start）。

工具分组：任务（create/list/get/control/admin）、漏洞（list/finding_admin）、资产与企业（list/assets_admin/companies）、LLM（list_llm_profiles/llm）、对话（ask/conversations/send_task_message）、可观测（task_stats/task_activity/activity/traffic）、配置（settings/mcp_servers/sync/skills/workspace/intercept）。

找不到对应高层工具时：artex_find_routes 搜索路由目录，再用 artex_api_request 直接调用任意 ARTEX API（这是全权兜底入口）。

注意：创建/运行任务即对目标发起真实测试，只对用户明确授权的目标执行。`
